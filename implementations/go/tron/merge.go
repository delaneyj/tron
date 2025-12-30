package tron

import "fmt"

// MergeMapDocuments merges two map tree documents with right-biased semantics.
// The resulting document reuses left document nodes where possible.
func MergeMapDocuments(left, right []byte) ([]byte, error) {
	leftTrailer, err := ParseTrailer(left)
	if err != nil {
		return nil, err
	}
	rightTrailer, err := ParseTrailer(right)
	if err != nil {
		return nil, err
	}
	leftHeader, _, err := NodeSliceAt(left, leftTrailer.RootOffset)
	if err != nil {
		return nil, err
	}
	rightHeader, _, err := NodeSliceAt(right, rightTrailer.RootOffset)
	if err != nil {
		return nil, err
	}
	if leftHeader.KeyType != KeyMap || rightHeader.KeyType != KeyMap {
		return nil, fmt.Errorf("merge expects map roots")
	}
	base := &Builder{buf: append([]byte{}, left[:len(left)-TrailerSize]...)}
	merger := mapMerger{
		left:    left,
		right:   right,
		builder: base,
	}
	root, _, err := merger.mergeNodes(leftTrailer.RootOffset, rightTrailer.RootOffset, 0)
	if err != nil {
		return nil, err
	}
	return base.BytesWithTrailer(root, leftTrailer.RootOffset), nil
}

type mapMerger struct {
	left    []byte
	right   []byte
	builder *Builder
}

func (m *mapMerger) mergeNodes(leftOff, rightOff uint32, depth int) (uint32, bool, error) {
	leftHeader, leftNode, err := NodeSliceAt(m.left, leftOff)
	if err != nil {
		return 0, false, err
	}
	rightHeader, rightNode, err := NodeSliceAt(m.right, rightOff)
	if err != nil {
		return 0, false, err
	}
	if leftHeader.KeyType != KeyMap || rightHeader.KeyType != KeyMap {
		return 0, false, fmt.Errorf("merge expects map nodes")
	}

	if rightHeader.Kind == NodeLeaf {
		rightLeaf, err := ParseMapLeafNode(rightNode)
		if err != nil {
			return 0, false, err
		}
		defer releaseMapLeafNode(&rightLeaf)
		off := leftOff
		changed := false
		for _, entry := range rightLeaf.Entries {
			val, err := cloneValueFromDoc(m.right, entry.Value, m.builder)
			if err != nil {
				return 0, false, err
			}
			newOff, didChange, err := mapSet(m.builder.buf, off, entry.Key, val, depth, m.builder)
			if err != nil {
				return 0, false, err
			}
			if didChange {
				changed = true
			}
			off = newOff
		}
		return off, changed, nil
	}

	if leftHeader.Kind == NodeLeaf {
		cloneOff, err := cloneMapNode(m.right, rightOff, m.builder)
		if err != nil {
			return 0, false, err
		}
		leftLeaf, err := ParseMapLeafNode(leftNode)
		if err != nil {
			return 0, false, err
		}
		defer releaseMapLeafNode(&leftLeaf)
		off := cloneOff
		for _, entry := range leftLeaf.Entries {
			exists, err := mapHas(m.builder.buf, off, entry.Key, depth)
			if err != nil {
				return 0, false, err
			}
			if exists {
				continue
			}
			newOff, _, err := mapSet(m.builder.buf, off, entry.Key, entry.Value, depth, m.builder)
			if err != nil {
				return 0, false, err
			}
			off = newOff
		}
		return off, true, nil
	}

	leftBranch, err := ParseMapBranchNode(leftNode)
	if err != nil {
		return 0, false, err
	}
	defer releaseMapBranchNode(&leftBranch)
	rightBranch, err := ParseMapBranchNode(rightNode)
	if err != nil {
		return 0, false, err
	}
	defer releaseMapBranchNode(&rightBranch)

	union := leftBranch.Bitmap | rightBranch.Bitmap
	children := make([]uint32, 0, popcount16(union))
	changed := false
	leftIdx := 0
	rightIdx := 0

	for slot := 0; slot < 16; slot++ {
		lHas := ((leftBranch.Bitmap >> uint(slot)) & 1) == 1
		rHas := ((rightBranch.Bitmap >> uint(slot)) & 1) == 1
		if !lHas && !rHas {
			continue
		}
		var child uint32
		if lHas && rHas {
			lChild := leftBranch.Children[leftIdx]
			rChild := rightBranch.Children[rightIdx]
			mergedOff, childChanged, err := m.mergeNodes(lChild, rChild, depth+1)
			if err != nil {
				return 0, false, err
			}
			child = mergedOff
			if childChanged {
				changed = true
			}
		} else if lHas {
			child = leftBranch.Children[leftIdx]
		} else {
			var err error
			child, err = cloneMapNode(m.right, rightBranch.Children[rightIdx], m.builder)
			if err != nil {
				return 0, false, err
			}
			changed = true
		}
		children = append(children, child)
		if lHas {
			leftIdx++
		}
		if rHas {
			rightIdx++
		}
	}

	if !changed && union == leftBranch.Bitmap && slicesEqual(children, leftBranch.Children) {
		return leftOff, false, nil
	}
	branch := MapBranchNode{
		Header:   NodeHeader{Kind: NodeBranch, KeyType: KeyMap},
		Bitmap:   union,
		Children: children,
	}
	off, err := appendMapBranchNode(m.builder, branch)
	if err != nil {
		return 0, false, err
	}
	return off, true, nil
}

func slicesEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
