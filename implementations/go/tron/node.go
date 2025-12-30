package tron

import (
	"encoding/binary"
	"fmt"
)

// NodeKind indicates whether a node is a branch or leaf.
type NodeKind uint8

const (
	NodeBranch NodeKind = 0
	NodeLeaf   NodeKind = 1
)

// KeyType indicates whether a node is for arrays or maps.
type KeyType uint8

const (
	KeyArr KeyType = 0
	KeyMap KeyType = 1
)

// NodeHeader is the common header for all nodes.
type NodeHeader struct {
	NodeLen    uint32
	Kind       NodeKind
	KeyType    KeyType
	EntryCount uint32
}

// ParseNodeHeader parses an 8-byte node header from b.
func ParseNodeHeader(b []byte) (NodeHeader, error) {
	if len(b) < 8 {
		return NodeHeader{}, fmt.Errorf("node header too short: %d", len(b))
	}
	raw := binary.LittleEndian.Uint32(b[0:4])
	kind := NodeKind(raw & 0x1)
	key := KeyType((raw >> 1) & 0x1)
	nodeLen := raw &^ 0x3
	if nodeLen < 8 || nodeLen%4 != 0 {
		return NodeHeader{}, fmt.Errorf("invalid node length: %d", nodeLen)
	}
	count := binary.LittleEndian.Uint32(b[4:8])
	return NodeHeader{
		NodeLen:    nodeLen,
		Kind:       kind,
		KeyType:    key,
		EntryCount: count,
	}, nil
}

// MapBranchNode represents a map branch node.
type MapBranchNode struct {
	Header   NodeHeader
	Bitmap   uint16
	Children []uint32
}

// MapLeafEntry is a single map leaf entry.
type MapLeafEntry struct {
	Key   []byte
	Value Value
}

// MapLeafNode represents a map leaf node.
type MapLeafNode struct {
	Header  NodeHeader
	Entries []MapLeafEntry
}

// ArrayBranchNode represents an array branch node.
type ArrayBranchNode struct {
	Header   NodeHeader
	Shift    uint8
	Bitmap   uint16
	Length   uint32
	Children []uint32
}

// ArrayLeafNode represents an array leaf node.
type ArrayLeafNode struct {
	Header NodeHeader
	Shift  uint8
	Bitmap uint16
	Length uint32
	Values []Value
}

// ParseMapBranchNode parses a map branch node from b.
func ParseMapBranchNode(b []byte) (MapBranchNode, error) {
	h, err := ParseNodeHeader(b)
	if err != nil {
		return MapBranchNode{}, err
	}
	if h.KeyType != KeyMap || h.Kind != NodeBranch {
		return MapBranchNode{}, fmt.Errorf("node is not a map branch")
	}
	if len(b) < int(h.NodeLen) {
		return MapBranchNode{}, fmt.Errorf("node truncated: have %d need %d", len(b), h.NodeLen)
	}
	if int(h.NodeLen) < 12 {
		return MapBranchNode{}, fmt.Errorf("map branch node too small: %d", h.NodeLen)
	}
	p := 8
	bitmap := binary.LittleEndian.Uint16(b[p : p+2])
	p += 2
	reserved := binary.LittleEndian.Uint16(b[p : p+2])
	p += 2
	if reserved != 0 {
		return MapBranchNode{}, fmt.Errorf("map branch reserved must be 0")
	}
	if h.EntryCount != uint32(popcount16(bitmap)) {
		return MapBranchNode{}, fmt.Errorf("entry_count mismatch with bitmap")
	}
	children := getUint32Slice(int(h.EntryCount))
	for i := uint32(0); i < h.EntryCount; i++ {
		if p+4 > int(h.NodeLen) {
			putUint32Slice(children)
			return MapBranchNode{}, fmt.Errorf("child offset truncated")
		}
		off := binary.LittleEndian.Uint32(b[p : p+4])
		p += 4
		children[i] = off
	}
	return MapBranchNode{
		Header:   h,
		Bitmap:   bitmap,
		Children: children,
	}, nil
}

// ParseMapLeafNode parses a map leaf node from b.
func ParseMapLeafNode(b []byte) (MapLeafNode, error) {
	h, err := ParseNodeHeader(b)
	if err != nil {
		return MapLeafNode{}, err
	}
	if h.KeyType != KeyMap || h.Kind != NodeLeaf {
		return MapLeafNode{}, fmt.Errorf("node is not a map leaf")
	}
	if len(b) < int(h.NodeLen) {
		return MapLeafNode{}, fmt.Errorf("node truncated: have %d need %d", len(b), h.NodeLen)
	}
	p := 8
	entries := getEntrySlice(int(h.EntryCount))
	for i := uint32(0); i < h.EntryCount; i++ {
		keyVal, n, err := DecodeValue(b[p:int(h.NodeLen)])
		if err != nil {
			putEntrySlice(entries)
			return MapLeafNode{}, fmt.Errorf("key decode failed: %w", err)
		}
		if keyVal.Type != TypeTxt {
			putEntrySlice(entries)
			return MapLeafNode{}, fmt.Errorf("map leaf key must be txt")
		}
		p += n
		val, m, err := DecodeValue(b[p:int(h.NodeLen)])
		if err != nil {
			putEntrySlice(entries)
			return MapLeafNode{}, fmt.Errorf("value decode failed: %w", err)
		}
		p += m
		entries[i] = MapLeafEntry{
			Key:   keyVal.Bytes,
			Value: val,
		}
	}
	return MapLeafNode{
		Header:  h,
		Entries: entries,
	}, nil
}

// ParseArrayBranchNode parses an array branch node from b.
func ParseArrayBranchNode(b []byte) (ArrayBranchNode, error) {
	h, err := ParseNodeHeader(b)
	if err != nil {
		return ArrayBranchNode{}, err
	}
	if h.KeyType != KeyArr || h.Kind != NodeBranch {
		return ArrayBranchNode{}, fmt.Errorf("node is not an array branch")
	}
	if len(b) < int(h.NodeLen) {
		return ArrayBranchNode{}, fmt.Errorf("node truncated: have %d need %d", len(b), h.NodeLen)
	}
	if int(h.NodeLen) < 16 {
		return ArrayBranchNode{}, fmt.Errorf("array branch node too small: %d", h.NodeLen)
	}
	p := 8
	shift := b[p]
	p++
	reserved := b[p]
	p++
	if reserved != 0 {
		return ArrayBranchNode{}, fmt.Errorf("array branch reserved must be 0")
	}
	bitmap := binary.LittleEndian.Uint16(b[p : p+2])
	p += 2
	length := binary.LittleEndian.Uint32(b[p : p+4])
	p += 4
	if h.EntryCount != uint32(popcount16(bitmap)) {
		return ArrayBranchNode{}, fmt.Errorf("entry_count mismatch with bitmap")
	}
	children := getUint32Slice(int(h.EntryCount))
	for i := uint32(0); i < h.EntryCount; i++ {
		if p+4 > int(h.NodeLen) {
			putUint32Slice(children)
			return ArrayBranchNode{}, fmt.Errorf("child offset truncated")
		}
		off := binary.LittleEndian.Uint32(b[p : p+4])
		p += 4
		children[i] = off
	}
	return ArrayBranchNode{
		Header:   h,
		Shift:    shift,
		Bitmap:   bitmap,
		Length:   length,
		Children: children,
	}, nil
}

// ParseArrayLeafNode parses an array leaf node from b.
func ParseArrayLeafNode(b []byte) (ArrayLeafNode, error) {
	h, err := ParseNodeHeader(b)
	if err != nil {
		return ArrayLeafNode{}, err
	}
	if h.KeyType != KeyArr || h.Kind != NodeLeaf {
		return ArrayLeafNode{}, fmt.Errorf("node is not an array leaf")
	}
	if len(b) < int(h.NodeLen) {
		return ArrayLeafNode{}, fmt.Errorf("node truncated: have %d need %d", len(b), h.NodeLen)
	}
	if int(h.NodeLen) < 16 {
		return ArrayLeafNode{}, fmt.Errorf("array leaf node too small: %d", h.NodeLen)
	}
	p := 8
	shift := b[p]
	p++
	reserved := b[p]
	p++
	if reserved != 0 {
		return ArrayLeafNode{}, fmt.Errorf("array leaf reserved must be 0")
	}
	bitmap := binary.LittleEndian.Uint16(b[p : p+2])
	p += 2
	length := binary.LittleEndian.Uint32(b[p : p+4])
	p += 4
	if h.EntryCount != uint32(popcount16(bitmap)) {
		return ArrayLeafNode{}, fmt.Errorf("entry_count mismatch with bitmap")
	}
	values := getValueSlice(int(h.EntryCount))
	for i := uint32(0); i < h.EntryCount; i++ {
		val, n, err := DecodeValue(b[p:int(h.NodeLen)])
		if err != nil {
			putValueSlice(values)
			return ArrayLeafNode{}, fmt.Errorf("value decode failed: %w", err)
		}
		p += n
		values[i] = val
	}
	return ArrayLeafNode{
		Header: h,
		Shift:  shift,
		Bitmap: bitmap,
		Length: length,
		Values: values,
	}, nil
}

func popcount16(x uint16) int {
	// simple popcount for 16 bits
	x = x - ((x >> 1) & 0x5555)
	x = (x & 0x3333) + ((x >> 2) & 0x3333)
	x = (x + (x >> 4)) & 0x0F0F
	x = x + (x >> 8)
	return int(x & 0x1F)
}

func releaseMapBranchNode(n *MapBranchNode) {
	putUint32Slice(n.Children)
	n.Children = nil
}

func releaseMapLeafNode(n *MapLeafNode) {
	putEntrySlice(n.Entries)
	n.Entries = nil
}

func releaseArrayBranchNode(n *ArrayBranchNode) {
	putUint32Slice(n.Children)
	n.Children = nil
}

func releaseArrayLeafNode(n *ArrayLeafNode) {
	putValueSlice(n.Values)
	n.Values = nil
}

// ReleaseMapBranchNode releases pooled slices held by a map branch node.
func ReleaseMapBranchNode(n *MapBranchNode) {
	releaseMapBranchNode(n)
}

// ReleaseMapLeafNode releases pooled slices held by a map leaf node.
func ReleaseMapLeafNode(n *MapLeafNode) {
	releaseMapLeafNode(n)
}

// ReleaseArrayBranchNode releases pooled slices held by an array branch node.
func ReleaseArrayBranchNode(n *ArrayBranchNode) {
	releaseArrayBranchNode(n)
}

// ReleaseArrayLeafNode releases pooled slices held by an array leaf node.
func ReleaseArrayLeafNode(n *ArrayLeafNode) {
	releaseArrayLeafNode(n)
}
