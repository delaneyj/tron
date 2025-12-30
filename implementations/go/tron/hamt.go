package tron

import (
	"bytes"
	"fmt"
	"sort"
)

const (
	hamtSlots  = 16
	hamtMask   = 0xF
	maxDepth32 = 7
)

// MapBuilder builds a HAMT map tree from key/value pairs.
type MapBuilder struct {
	entries   []MapLeafEntry
	workspace *encodeWorkspace
}

// NewMapBuilder creates an empty MapBuilder.
func NewMapBuilder() *MapBuilder {
	return &MapBuilder{entries: make([]MapLeafEntry, 0)}
}

func newMapBuilderWithWorkspace(workspace *encodeWorkspace) *MapBuilder {
	return &MapBuilder{
		entries:   make([]MapLeafEntry, 0),
		workspace: workspace,
	}
}

// Set stores a key/value pair without copying the key bytes.
func (b *MapBuilder) Set(key []byte, v Value) {
	for i := range b.entries {
		if bytes.Equal(b.entries[i].Key, key) {
			b.entries[i].Value = v
			return
		}
	}
	b.entries = append(b.entries, MapLeafEntry{Key: key, Value: v})
}

// SetString stores a key/value pair from a string key.
func (b *MapBuilder) SetString(key string, v Value) {
	b.Set([]byte(key), v)
}

// Build encodes the map into nodes and returns the root offset.
func (b *MapBuilder) Build(builder *Builder) (uint32, error) {
	if builder == nil {
		return 0, fmt.Errorf("nil builder")
	}
	root := buildMapNodeFromEntries(b.entries, 0, b.workspace)
	return encodeMapNode(builder, root, b.workspace)
}

type mapEntry struct {
	Key   []byte
	Value Value
	Hash  uint32
}

type mapNode struct {
	kind         NodeKind
	bitmap       uint16
	children     []*mapNode
	entries      []mapEntry
	bodyLen      int
	ownsChildren bool
	ownsEntries  bool
}

func buildMapNodeFromEntries(entries []MapLeafEntry, depth int, workspace *encodeWorkspace) *mapNode {
	if len(entries) == 0 {
		node := getMapNodeWithWorkspace(workspace)
		node.kind = NodeLeaf
		node.entries = nil
		node.ownsEntries = false
		return node
	}
	mapEntries := getMapEntrySliceWithWorkspace(len(entries), workspace)
	for i, entry := range entries {
		mapEntries[i] = mapEntry{
			Key:   entry.Key,
			Value: entry.Value,
			Hash:  XXH32(entry.Key, 0),
		}
	}
	return buildMapNode(mapEntries, depth, true, workspace)
}

func buildMapNode(entries []mapEntry, depth int, owned bool, workspace *encodeWorkspace) *mapNode {
	if len(entries) == 0 {
		node := getMapNodeWithWorkspace(workspace)
		node.kind = NodeLeaf
		node.entries = nil
		node.ownsEntries = false
		node.bodyLen = 0
		return node
	}
	if len(entries) == 1 {
		bodyLen := encodedBytesLenNoErr(len(entries[0].Key)) + encodedValueLenNoErr(entries[0].Value)
		node := getMapNodeWithWorkspace(workspace)
		node.kind = NodeLeaf
		node.entries = entries
		node.ownsEntries = owned
		node.bodyLen = bodyLen
		return node
	}
	if depth >= maxDepth32 {
		bodyLen := 0
		for _, entry := range entries {
			bodyLen += encodedBytesLenNoErr(len(entry.Key)) + encodedValueLenNoErr(entry.Value)
		}
		sort.Slice(entries, func(i, j int) bool {
			return bytes.Compare(entries[i].Key, entries[j].Key) < 0
		})
		node := getMapNodeWithWorkspace(workspace)
		node.kind = NodeLeaf
		node.entries = entries
		node.ownsEntries = owned
		node.bodyLen = bodyLen
		return node
	}

	var counts [hamtSlots]int
	groupCount := 0
	for _, entry := range entries {
		slot := int((entry.Hash >> (depth * 4)) & hamtMask)
		if counts[slot] == 0 {
			groupCount++
		}
		counts[slot]++
	}
	if groupCount == 1 {
		slot := int((entries[0].Hash >> (depth * 4)) & hamtMask)
		child := buildMapNode(entries, depth+1, owned, workspace)
		children := getMapNodeSliceWithWorkspace(1, workspace)
		children[0] = child
		node := getMapNodeWithWorkspace(workspace)
		node.kind = NodeBranch
		node.bitmap = 1 << uint16(slot)
		node.children = children
		node.ownsChildren = true
		return node
	}

	var starts [hamtSlots]int
	sum := 0
	for slot := 0; slot < hamtSlots; slot++ {
		if counts[slot] == 0 {
			continue
		}
		starts[slot] = sum
		sum += counts[slot]
	}
	offsets := starts
	for i := 0; i < len(entries); {
		slot := int((entries[i].Hash >> (depth * 4)) & hamtMask)
		start := starts[slot]
		end := start + counts[slot]
		if i >= start && i < end {
			i++
			continue
		}
		target := offsets[slot]
		entries[i], entries[target] = entries[target], entries[i]
		offsets[slot]++
	}

	children := getMapNodeSliceWithWorkspace(groupCount, workspace)
	var bitmap uint16
	childIdx := 0
	for slot := 0; slot < hamtSlots; slot++ {
		if counts[slot] == 0 {
			continue
		}
		bitmap |= 1 << uint16(slot)
		start := starts[slot]
		end := start + counts[slot]
		children[childIdx] = buildMapNode(entries[start:end], depth+1, false, workspace)
		childIdx++
	}
	node := getMapNodeWithWorkspace(workspace)
	node.kind = NodeBranch
	node.bitmap = bitmap
	node.children = children
	node.ownsChildren = true
	if owned {
		node.entries = entries
		node.ownsEntries = true
	} else {
		node.entries = nil
		node.ownsEntries = false
	}
	return node
}

func encodeMapNode(builder *Builder, node *mapNode, workspace *encodeWorkspace) (uint32, error) {
	if node.kind == NodeLeaf {
		off, err := appendMapLeafNodeSortedEntriesWithLen(builder, node.entries, node.bodyLen)
		releaseMapNode(node, workspace)
		return off, err
	}
	childrenOffsets := getUint32SliceWithWorkspace(len(node.children), workspace)
	for i, child := range node.children {
		off, err := encodeMapNode(builder, child, workspace)
		if err != nil {
			putUint32SliceWithWorkspace(childrenOffsets, workspace)
			releaseMapNode(node, workspace)
			return 0, err
		}
		childrenOffsets[i] = off
	}
	branch := MapBranchNode{
		Header:   NodeHeader{Kind: NodeBranch, KeyType: KeyMap},
		Bitmap:   node.bitmap,
		Children: childrenOffsets,
	}
	off, err := appendMapBranchNode(builder, branch)
	putUint32SliceWithWorkspace(childrenOffsets, workspace)
	releaseMapNode(node, workspace)
	return off, err
}

func releaseMapNode(node *mapNode, workspace *encodeWorkspace) {
	if node == nil {
		return
	}
	if node.ownsEntries {
		putMapEntrySliceWithWorkspace(node.entries, workspace)
	}
	if node.ownsChildren {
		putMapNodeSliceWithWorkspace(node.children, workspace)
	}
	putMapNodeWithWorkspace(node, workspace)
}
