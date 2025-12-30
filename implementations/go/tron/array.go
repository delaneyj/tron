package tron

import (
	"encoding/binary"
	"fmt"
)

// ArrayBuilder builds a vector trie array from contiguous values.
type ArrayBuilder struct {
	values    []Value
	workspace *encodeWorkspace
}

// NewArrayBuilder creates an empty ArrayBuilder.
func NewArrayBuilder() *ArrayBuilder {
	return &ArrayBuilder{values: make([]Value, 0)}
}

func newArrayBuilderWithWorkspace(workspace *encodeWorkspace) *ArrayBuilder {
	return &ArrayBuilder{
		values:    make([]Value, 0),
		workspace: workspace,
	}
}

// Append adds a value to the end of the array.
func (b *ArrayBuilder) Append(v Value) {
	b.values = append(b.values, v)
}

// Set replaces the value at an existing index.
func (b *ArrayBuilder) Set(index int, v Value) error {
	if index < 0 || index >= len(b.values) {
		return fmt.Errorf("array index out of range: %d", index)
	}
	b.values[index] = v
	return nil
}

// Build encodes the array into nodes and returns the root offset.
func (b *ArrayBuilder) Build(builder *Builder) (uint32, error) {
	if builder == nil {
		return 0, fmt.Errorf("nil builder")
	}
	if len(b.values) > int(^uint32(0)) {
		return 0, fmt.Errorf("array length exceeds u32")
	}
	length := uint32(len(b.values))
	entries := getArrayEntrySliceWithWorkspace(len(b.values), b.workspace)
	for i, v := range b.values {
		entries[i] = arrayEntry{index: uint32(i), value: v}
	}
	shift := arrayRootShift(length)
	root, err := buildArrayNode(entries, shift, length, b.workspace)
	putArrayEntrySliceWithWorkspace(entries, b.workspace)
	if err != nil {
		return 0, err
	}
	return encodeArrayNode(builder, root, b.workspace)
}

type arrayEntry struct {
	index uint32
	value Value
}

type arrayNode struct {
	kind         NodeKind
	shift        uint8
	bitmap       uint16
	length       uint32
	children     []*arrayNode
	values       []Value
	bodyLen      int
	ownsChildren bool
	ownsValues   bool
}

func arrayRootShift(length uint32) uint8 {
	if length == 0 {
		return 0
	}
	maxIndex := length - 1
	var shift uint8
	for (maxIndex >> shift) > 0xF {
		shift += 4
	}
	return shift
}

func buildArrayNode(entries []arrayEntry, shift uint8, length uint32, workspace *encodeWorkspace) (*arrayNode, error) {
	if shift%4 != 0 {
		return nil, fmt.Errorf("array node shift must be multiple of 4")
	}
	if len(entries) == 0 && shift == 0 {
		node := getArrayNodeWithWorkspace(workspace)
		node.kind = NodeLeaf
		node.shift = 0
		node.bitmap = 0
		node.length = length
		node.values = nil
		node.bodyLen = 8
		node.ownsValues = false
		return node, nil
	}

	var bitmap uint16
	if shift == 0 {
		var slotValues [16]Value
		for _, entry := range entries {
			slot := uint8(entry.index & 0xF)
			if ((bitmap >> slot) & 1) == 1 {
				return nil, fmt.Errorf("duplicate index in slot %d", slot)
			}
			bitmap |= 1 << uint16(slot)
			slotValues[slot] = entry.value
		}
		count := popcount16(bitmap)
		values := getValueSliceWithWorkspace(count, workspace)
		idx := 0
		bodyLen := 8
		for slot := 0; slot < 16; slot++ {
			if ((bitmap >> uint16(slot)) & 1) == 0 {
				continue
			}
			values[idx] = slotValues[slot]
			bodyLen += encodedValueLenNoErr(values[idx])
			idx++
		}
		node := getArrayNodeWithWorkspace(workspace)
		node.kind = NodeLeaf
		node.shift = 0
		node.bitmap = bitmap
		node.length = length
		node.values = values
		node.bodyLen = bodyLen
		node.ownsValues = true
		return node, nil
	}

	prevSlot := uint8((entries[0].index >> shift) & 0xF)
	groupCount := 1
	for i := 1; i < len(entries); i++ {
		slot := uint8((entries[i].index >> shift) & 0xF)
		if slot < prevSlot {
			return nil, fmt.Errorf("array entries not sorted for shift %d", shift)
		}
		if slot != prevSlot {
			groupCount++
			prevSlot = slot
		}
	}

	children := getArrayNodeSliceWithWorkspace(groupCount, workspace)
	prevSlot = uint8((entries[0].index >> shift) & 0xF)
	start := 0
	childIdx := 0
	for i := 1; i <= len(entries); i++ {
		var slot uint8
		if i < len(entries) {
			slot = uint8((entries[i].index >> shift) & 0xF)
			if slot < prevSlot {
				return nil, fmt.Errorf("array entries not sorted for shift %d", shift)
			}
		}
		if i == len(entries) || slot != prevSlot {
			group := entries[start:i]
			child, err := buildArrayNode(group, shift-4, 0, workspace)
			if err != nil {
				putArrayNodeSliceWithWorkspace(children, workspace)
				return nil, err
			}
			bitmap |= 1 << uint16(prevSlot)
			children[childIdx] = child
			childIdx++
			start = i
			prevSlot = slot
		}
	}
	node := getArrayNodeWithWorkspace(workspace)
	node.kind = NodeBranch
	node.shift = shift
	node.bitmap = bitmap
	node.length = length
	node.children = children
	node.ownsChildren = true
	return node, nil
}

func encodeArrayNode(builder *Builder, node *arrayNode, workspace *encodeWorkspace) (uint32, error) {
	if node.kind == NodeLeaf {
		off, err := appendArrayLeafNodeWithLen(builder, node)
		releaseArrayNode(node, workspace)
		return off, err
	}
	childrenOffsets := getUint32SliceWithWorkspace(len(node.children), workspace)
	for i, child := range node.children {
		off, err := encodeArrayNode(builder, child, workspace)
		if err != nil {
			putUint32SliceWithWorkspace(childrenOffsets, workspace)
			releaseArrayNode(node, workspace)
			return 0, err
		}
		childrenOffsets[i] = off
	}
	branch := ArrayBranchNode{
		Header:   NodeHeader{Kind: NodeBranch, KeyType: KeyArr},
		Shift:    node.shift,
		Bitmap:   node.bitmap,
		Length:   node.length,
		Children: childrenOffsets,
	}
	off, err := appendArrayBranchNode(builder, branch)
	putUint32SliceWithWorkspace(childrenOffsets, workspace)
	releaseArrayNode(node, workspace)
	return off, err
}

func appendArrayLeafNodeWithLen(builder *Builder, node *arrayNode) (uint32, error) {
	bodyLen := node.bodyLen
	if bodyLen < 8 {
		bodyLen = 8
	}
	body, off := appendNodeWithBodyLen(builder, NodeLeaf, KeyArr, uint32(len(node.values)), bodyLen)
	body[0] = node.shift
	body[1] = 0
	binary.LittleEndian.PutUint16(body[2:4], node.bitmap)
	binary.LittleEndian.PutUint32(body[4:8], node.length)
	p := 8
	for _, v := range node.values {
		n, err := writeValue(body[p:], v)
		if err != nil {
			return 0, err
		}
		p += n
	}
	return off, nil
}

func releaseArrayNode(node *arrayNode, workspace *encodeWorkspace) {
	if node == nil {
		return
	}
	if node.ownsValues {
		putValueSliceWithWorkspace(node.values, workspace)
	}
	if node.ownsChildren {
		putArrayNodeSliceWithWorkspace(node.children, workspace)
	}
	putArrayNodeWithWorkspace(node, workspace)
}
