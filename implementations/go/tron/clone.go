package tron

import "fmt"

func cloneValueFromDoc(doc []byte, v Value, builder *Builder) (Value, error) {
	switch v.Type {
	case TypeTxt, TypeBin:
		return Value{Type: v.Type, Bytes: v.Bytes}, nil
	case TypeArr:
		off, err := cloneArrayNode(doc, v.Offset, builder)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeArr, Offset: off}, nil
	case TypeMap:
		off, err := cloneMapNode(doc, v.Offset, builder)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeMap, Offset: off}, nil
	default:
		return v, nil
	}
}

// CloneValueFromDoc clones a value from a source document into builder storage.
func CloneValueFromDoc(doc []byte, v Value, builder *Builder) (Value, error) {
	return cloneValueFromDoc(doc, v, builder)
}

func cloneMapNode(doc []byte, off uint32, builder *Builder) (uint32, error) {
	header, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return 0, err
	}
	if header.KeyType != KeyMap {
		return 0, fmt.Errorf("clone expects map node")
	}
	if header.Kind == NodeLeaf {
		leaf, err := ParseMapLeafNode(node)
		if err != nil {
			return 0, err
		}
		defer releaseMapLeafNode(&leaf)
		for i := range leaf.Entries {
			val, err := cloneValueFromDoc(doc, leaf.Entries[i].Value, builder)
			if err != nil {
				return 0, err
			}
			leaf.Entries[i].Value = val
		}
		off, err := appendMapLeafNodeSorted(builder, leaf.Entries)
		if err != nil {
			return 0, err
		}
		return off, nil
	}

	branch, err := ParseMapBranchNode(node)
	if err != nil {
		return 0, err
	}
	defer releaseMapBranchNode(&branch)
	for i, child := range branch.Children {
		cloneOff, err := cloneMapNode(doc, child, builder)
		if err != nil {
			return 0, err
		}
		branch.Children[i] = cloneOff
	}
	newOff, err := appendMapBranchNode(builder, branch)
	if err != nil {
		return 0, err
	}
	return newOff, nil
}

// CloneMapNode clones a map subtree from doc into builder storage.
func CloneMapNode(doc []byte, off uint32, builder *Builder) (uint32, error) {
	return cloneMapNode(doc, off, builder)
}

func cloneArrayNode(doc []byte, off uint32, builder *Builder) (uint32, error) {
	header, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return 0, err
	}
	if header.KeyType != KeyArr {
		return 0, fmt.Errorf("clone expects array node")
	}
	if header.Kind == NodeLeaf {
		leaf, err := ParseArrayLeafNode(node)
		if err != nil {
			return 0, err
		}
		defer releaseArrayLeafNode(&leaf)
		for i := range leaf.Values {
			cloned, err := cloneValueFromDoc(doc, leaf.Values[i], builder)
			if err != nil {
				return 0, err
			}
			leaf.Values[i] = cloned
		}
		newOff, err := appendArrayLeafNode(builder, leaf)
		if err != nil {
			return 0, err
		}
		return newOff, nil
	}

	branch, err := ParseArrayBranchNode(node)
	if err != nil {
		return 0, err
	}
	defer releaseArrayBranchNode(&branch)
	for i, child := range branch.Children {
		cloneOff, err := cloneArrayNode(doc, child, builder)
		if err != nil {
			return 0, err
		}
		branch.Children[i] = cloneOff
	}
	newOff, err := appendArrayBranchNode(builder, branch)
	if err != nil {
		return 0, err
	}
	return newOff, nil
}

// CloneArrayNode clones an array subtree from doc into builder storage.
func CloneArrayNode(doc []byte, off uint32, builder *Builder) (uint32, error) {
	return cloneArrayNode(doc, off, builder)
}
