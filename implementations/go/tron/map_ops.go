package tron

import (
	"bytes"
	"fmt"
	"math"
)

func mapGet(doc []byte, off uint32, key []byte, depth int) (Value, bool, error) {
	h, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return Value{}, false, err
	}
	if h.KeyType != KeyMap {
		return Value{}, false, fmt.Errorf("node is not a map")
	}
	if h.Kind == NodeLeaf {
		leaf, err := ParseMapLeafNode(node)
		if err != nil {
			return Value{}, false, err
		}
		defer releaseMapLeafNode(&leaf)
		for _, entry := range leaf.Entries {
			if bytes.Equal(entry.Key, key) {
				return entry.Value, true, nil
			}
		}
		return Value{}, false, nil
	}

	branch, err := ParseMapBranchNode(node)
	if err != nil {
		return Value{}, false, err
	}
	defer releaseMapBranchNode(&branch)
	slot := uint8((XXH32(key, 0) >> (depth * 4)) & hamtMask)
	if ((branch.Bitmap >> slot) & 1) == 0 {
		return Value{}, false, nil
	}
	mask := uint16((uint32(1) << slot) - 1)
	idx := popcount16(branch.Bitmap & mask)
	child := branch.Children[idx]
	return mapGet(doc, child, key, depth+1)
}

// MapGet returns the value for key under the map node at rootOff.
func MapGet(doc []byte, rootOff uint32, key []byte) (Value, bool, error) {
	return mapGet(doc, rootOff, key, 0)
}

// MapHas reports whether key exists under the map node at rootOff.
func MapHas(doc []byte, rootOff uint32, key []byte) (bool, error) {
	return mapHas(doc, rootOff, key, 0)
}

// MapSetNode updates a map node at rootOff and returns the new root offset.
func MapSetNode(builder *Builder, rootOff uint32, key []byte, val Value) (uint32, bool, error) {
	if builder == nil {
		return 0, false, fmt.Errorf("nil builder")
	}
	return mapSet(builder.buf, rootOff, key, val, 0, builder)
}

// MapDelNode deletes a key from a map node at rootOff and returns the new root offset.
func MapDelNode(builder *Builder, rootOff uint32, key []byte) (uint32, bool, error) {
	if builder == nil {
		return 0, false, fmt.Errorf("nil builder")
	}
	return mapDel(builder.buf, rootOff, key, 0, builder)
}

// EmptyMapRoot returns a new empty map root node offset.
func EmptyMapRoot(builder *Builder) (uint32, error) {
	if builder == nil {
		return 0, fmt.Errorf("nil builder")
	}
	return appendMapLeafNodeSorted(builder, nil)
}

func mapHas(doc []byte, off uint32, key []byte, depth int) (bool, error) {
	_, ok, err := mapGet(doc, off, key, depth)
	return ok, err
}

func mapSet(doc []byte, off uint32, key []byte, val Value, depth int, builder *Builder) (uint32, bool, error) {
	if depth > maxDepth32 {
		return 0, false, fmt.Errorf("map depth exceeds max")
	}
	h, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return 0, false, err
	}
	if h.KeyType != KeyMap {
		return 0, false, fmt.Errorf("node is not a map")
	}
	if h.Kind == NodeLeaf {
		leaf, err := ParseMapLeafNode(node)
		if err != nil {
			return 0, false, err
		}
		defer releaseMapLeafNode(&leaf)
		entries := leaf.Entries
		for i, entry := range entries {
			if bytes.Equal(entry.Key, key) {
				if valueEqual(entry.Value, val) {
					return off, false, nil
				}
				newEntries := make([]MapLeafEntry, len(entries))
				copy(newEntries, entries)
				newEntries[i].Value = val
				newNode := buildMapNodeFromEntries(newEntries, depth, nil)
				newOff, err := encodeMapNode(builder, newNode, nil)
				if err != nil {
					return 0, false, err
				}
				return newOff, true, nil
			}
		}
		newEntries := make([]MapLeafEntry, len(entries)+1)
		copy(newEntries, entries)
		newEntries[len(entries)] = MapLeafEntry{Key: key, Value: val}
		newNode := buildMapNodeFromEntries(newEntries, depth, nil)
		newOff, err := encodeMapNode(builder, newNode, nil)
		if err != nil {
			return 0, false, err
		}
		return newOff, true, nil
	}

	branch, err := ParseMapBranchNode(node)
	if err != nil {
		return 0, false, err
	}
	defer releaseMapBranchNode(&branch)
	slot := uint8((XXH32(key, 0) >> (depth * 4)) & hamtMask)
	mask := uint16((uint32(1) << slot) - 1)
	idx := popcount16(branch.Bitmap & mask)
	hasChild := ((branch.Bitmap >> slot) & 1) == 1

	if hasChild {
		child := branch.Children[idx]
		newChild, changed, err := mapSet(doc, child, key, val, depth+1, builder)
		if err != nil {
			return 0, false, err
		}
		if !changed {
			return off, false, nil
		}
		children := branch.Children
		children[idx] = newChild
		newBranch := MapBranchNode{
			Header:   NodeHeader{Kind: NodeBranch, KeyType: KeyMap},
			Bitmap:   branch.Bitmap,
			Children: children,
		}
		newOff, err := appendMapBranchNode(builder, newBranch)
		if err != nil {
			return 0, false, err
		}
		return newOff, true, nil
	}

	newChildNode := buildMapNodeFromEntries([]MapLeafEntry{{Key: key, Value: val}}, depth+1, nil)
	newChild, err := encodeMapNode(builder, newChildNode, nil)
	if err != nil {
		return 0, false, err
	}
	newBitmap := branch.Bitmap | (1 << slot)
	children := getUint32Slice(len(branch.Children) + 1)
	copy(children, branch.Children[:idx])
	children[idx] = newChild
	copy(children[idx+1:], branch.Children[idx:])
	newBranch := MapBranchNode{
		Header:   NodeHeader{Kind: NodeBranch, KeyType: KeyMap},
		Bitmap:   newBitmap,
		Children: children,
	}
	newOff, err := appendMapBranchNode(builder, newBranch)
	putUint32Slice(children)
	if err != nil {
		return 0, false, err
	}
	return newOff, true, nil
}

func mapDel(doc []byte, off uint32, key []byte, depth int, builder *Builder) (uint32, bool, error) {
	if depth > maxDepth32 {
		return 0, false, fmt.Errorf("map depth exceeds max")
	}
	h, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return 0, false, err
	}
	if h.KeyType != KeyMap {
		return 0, false, fmt.Errorf("node is not a map")
	}
	if h.Kind == NodeLeaf {
		leaf, err := ParseMapLeafNode(node)
		if err != nil {
			return 0, false, err
		}
		defer releaseMapLeafNode(&leaf)
		found := false
		entries := make([]MapLeafEntry, 0, len(leaf.Entries))
		for _, entry := range leaf.Entries {
			if bytes.Equal(entry.Key, key) {
				found = true
				continue
			}
			entries = append(entries, entry)
		}
		if !found {
			return off, false, nil
		}
		if len(entries) == 0 {
			off, err := appendMapLeafNodeSorted(builder, nil)
			if err != nil {
				return 0, false, err
			}
			return off, true, nil
		}
		newNode := buildMapNodeFromEntries(entries, depth, nil)
		newOff, err := encodeMapNode(builder, newNode, nil)
		if err != nil {
			return 0, false, err
		}
		return newOff, true, nil
	}

	branch, err := ParseMapBranchNode(node)
	if err != nil {
		return 0, false, err
	}
	defer releaseMapBranchNode(&branch)
	slot := uint8((XXH32(key, 0) >> (depth * 4)) & hamtMask)
	mask := uint16((uint32(1) << slot) - 1)
	idx := popcount16(branch.Bitmap & mask)
	hasChild := ((branch.Bitmap >> slot) & 1) == 1
	if !hasChild {
		return off, false, nil
	}
	child := branch.Children[idx]
	newChild, changed, err := mapDel(doc, child, key, depth+1, builder)
	if err != nil {
		return 0, false, err
	}
	if !changed {
		return off, false, nil
	}
	emptyChild, err := mapNodeEmpty(builder.buf, newChild)
	if err != nil {
		return 0, false, err
	}
	if emptyChild {
		newBitmap := branch.Bitmap &^ (1 << slot)
		children := getUint32Slice(len(branch.Children) - 1)
		copy(children, branch.Children[:idx])
		copy(children[idx:], branch.Children[idx+1:])
		if len(children) == 0 {
			putUint32Slice(children)
			off, err := appendMapLeafNodeSorted(builder, nil)
			if err != nil {
				return 0, false, err
			}
			return off, true, nil
		}
		newBranch := MapBranchNode{
			Header:   NodeHeader{Kind: NodeBranch, KeyType: KeyMap},
			Bitmap:   newBitmap,
			Children: children,
		}
		newOff, err := appendMapBranchNode(builder, newBranch)
		putUint32Slice(children)
		if err != nil {
			return 0, false, err
		}
		return newOff, true, nil
	}

	children := branch.Children
	children[idx] = newChild
	newBranch := MapBranchNode{
		Header:   NodeHeader{Kind: NodeBranch, KeyType: KeyMap},
		Bitmap:   branch.Bitmap,
		Children: children,
	}
	newOff, err := appendMapBranchNode(builder, newBranch)
	if err != nil {
		return 0, false, err
	}
	return newOff, true, nil
}

func valueEqual(a, b Value) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case TypeNil:
		return true
	case TypeBit:
		return a.Bool == b.Bool
	case TypeI64:
		return a.I64 == b.I64
	case TypeF64:
		return math.Float64bits(a.F64) == math.Float64bits(b.F64)
	case TypeTxt, TypeBin:
		return bytes.Equal(a.Bytes, b.Bytes)
	case TypeArr, TypeMap:
		return a.Offset == b.Offset
	default:
		return false
	}
}

func mapNodeEmpty(doc []byte, off uint32) (bool, error) {
	h, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return false, err
	}
	if h.KeyType != KeyMap {
		return false, fmt.Errorf("node is not a map")
	}
	if h.Kind != NodeLeaf {
		return false, nil
	}
	leaf, err := ParseMapLeafNode(node)
	if err != nil {
		return false, err
	}
	defer releaseMapLeafNode(&leaf)
	return len(leaf.Entries) == 0, nil
}
