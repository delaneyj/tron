package tron

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/delaneyj/toolbelt/bytebufferpool"
)

// EncodeMapBranchNode encodes a map branch node.
func EncodeMapBranchNode(n MapBranchNode) ([]byte, error) {
	if len(n.Children) != popcount16(n.Bitmap) {
		return nil, fmt.Errorf("children length does not match bitmap")
	}
	entryCount := uint32(len(n.Children))
	body := bytebufferpool.Get()
	defer bytebufferpool.Put(body)
	var tmp [4]byte
	binary.LittleEndian.PutUint16(tmp[:2], n.Bitmap)
	body.Write(tmp[:2])
	tmp[0] = 0
	tmp[1] = 0
	body.Write(tmp[:2])
	for _, off := range n.Children {
		binary.LittleEndian.PutUint32(tmp[:], off)
		body.Write(tmp[:])
	}
	return encodeNode(NodeBranch, KeyMap, entryCount, body.Bytes())
}

// EncodeMapLeafNode encodes a map leaf node.
func EncodeMapLeafNode(n MapLeafNode) ([]byte, error) {
	entries := make([]MapLeafEntry, len(n.Entries))
	copy(entries, n.Entries)
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].Key, entries[j].Key) < 0
	})
	for i := 1; i < len(entries); i++ {
		if bytes.Equal(entries[i-1].Key, entries[i].Key) {
			return nil, fmt.Errorf("duplicate map key: %q", entries[i].Key)
		}
	}
	body := bytebufferpool.Get()
	defer bytebufferpool.Put(body)
	for _, entry := range entries {
		if err := encodeBytesToBuffer(body, TypeTxt, entry.Key); err != nil {
			return nil, err
		}
		if err := encodeValueToBuffer(body, entry.Value); err != nil {
			return nil, err
		}
	}
	return encodeNode(NodeLeaf, KeyMap, uint32(len(entries)), body.Bytes())
}

func appendMapBranchNode(builder *Builder, n MapBranchNode) (uint32, error) {
	if len(n.Children) != popcount16(n.Bitmap) {
		return 0, fmt.Errorf("children length does not match bitmap")
	}
	entryCount := uint32(len(n.Children))
	bodyLen := 4 + 4*len(n.Children)
	body, off := appendNodeWithBodyLen(builder, NodeBranch, KeyMap, entryCount, bodyLen)
	binary.LittleEndian.PutUint16(body[0:2], n.Bitmap)
	body[2] = 0
	body[3] = 0
	p := 4
	for _, child := range n.Children {
		binary.LittleEndian.PutUint32(body[p:p+4], child)
		p += 4
	}
	return off, nil
}

func appendMapLeafNodeSorted(builder *Builder, entries []MapLeafEntry) (uint32, error) {
	bodyLen := 0
	for _, entry := range entries {
		keyLen, err := encodedBytesLen(len(entry.Key))
		if err != nil {
			return 0, err
		}
		valLen, err := encodedValueLen(entry.Value)
		if err != nil {
			return 0, err
		}
		bodyLen += keyLen + valLen
	}
	body, off := appendNodeWithBodyLen(builder, NodeLeaf, KeyMap, uint32(len(entries)), bodyLen)
	p := 0
	for _, entry := range entries {
		n, err := writeBytesValue(body[p:], TypeTxt, entry.Key)
		if err != nil {
			return 0, err
		}
		p += n
		n, err = writeValue(body[p:], entry.Value)
		if err != nil {
			return 0, err
		}
		p += n
	}
	return off, nil
}

func appendMapLeafNodeSortedEntries(builder *Builder, entries []mapEntry) (uint32, error) {
	bodyLen := 0
	for _, entry := range entries {
		keyLen, err := encodedBytesLen(len(entry.Key))
		if err != nil {
			return 0, err
		}
		valLen, err := encodedValueLen(entry.Value)
		if err != nil {
			return 0, err
		}
		bodyLen += keyLen + valLen
	}
	return appendMapLeafNodeSortedEntriesWithLen(builder, entries, bodyLen)
}

func appendMapLeafNodeSortedEntriesWithLen(builder *Builder, entries []mapEntry, bodyLen int) (uint32, error) {
	body, off := appendNodeWithBodyLen(builder, NodeLeaf, KeyMap, uint32(len(entries)), bodyLen)
	p := 0
	for _, entry := range entries {
		n, err := writeBytesValue(body[p:], TypeTxt, entry.Key)
		if err != nil {
			return 0, err
		}
		p += n
		n, err = writeValue(body[p:], entry.Value)
		if err != nil {
			return 0, err
		}
		p += n
	}
	return off, nil
}

// EncodeArrayBranchNode encodes an array branch node.
func EncodeArrayBranchNode(n ArrayBranchNode) ([]byte, error) {
	if n.Shift%4 != 0 {
		return nil, fmt.Errorf("array branch shift must be multiple of 4")
	}
	if len(n.Children) != popcount16(n.Bitmap) {
		return nil, fmt.Errorf("children length does not match bitmap")
	}
	body := bytebufferpool.Get()
	defer bytebufferpool.Put(body)
	body.WriteByte(n.Shift)
	body.WriteByte(0)
	var tmp [4]byte
	binary.LittleEndian.PutUint16(tmp[:2], n.Bitmap)
	body.Write(tmp[:2])
	binary.LittleEndian.PutUint32(tmp[:], n.Length)
	body.Write(tmp[:])
	for _, off := range n.Children {
		binary.LittleEndian.PutUint32(tmp[:], off)
		body.Write(tmp[:])
	}
	return encodeNode(NodeBranch, KeyArr, uint32(len(n.Children)), body.Bytes())
}

// EncodeArrayLeafNode encodes an array leaf node.
func EncodeArrayLeafNode(n ArrayLeafNode) ([]byte, error) {
	if n.Shift != 0 {
		return nil, fmt.Errorf("array leaf shift must be 0")
	}
	if len(n.Values) != popcount16(n.Bitmap) {
		return nil, fmt.Errorf("values length does not match bitmap")
	}
	body := bytebufferpool.Get()
	defer bytebufferpool.Put(body)
	body.WriteByte(n.Shift)
	body.WriteByte(0)
	var tmp [4]byte
	binary.LittleEndian.PutUint16(tmp[:2], n.Bitmap)
	body.Write(tmp[:2])
	binary.LittleEndian.PutUint32(tmp[:], n.Length)
	body.Write(tmp[:])
	for _, v := range n.Values {
		if err := encodeValueToBuffer(body, v); err != nil {
			return nil, err
		}
	}
	return encodeNode(NodeLeaf, KeyArr, uint32(len(n.Values)), body.Bytes())
}

func appendArrayBranchNode(builder *Builder, n ArrayBranchNode) (uint32, error) {
	if n.Shift%4 != 0 {
		return 0, fmt.Errorf("array branch shift must be multiple of 4")
	}
	if len(n.Children) != popcount16(n.Bitmap) {
		return 0, fmt.Errorf("children length does not match bitmap")
	}
	bodyLen := 8 + 4*len(n.Children)
	body, off := appendNodeWithBodyLen(builder, NodeBranch, KeyArr, uint32(len(n.Children)), bodyLen)
	body[0] = n.Shift
	body[1] = 0
	binary.LittleEndian.PutUint16(body[2:4], n.Bitmap)
	binary.LittleEndian.PutUint32(body[4:8], n.Length)
	p := 8
	for _, child := range n.Children {
		binary.LittleEndian.PutUint32(body[p:p+4], child)
		p += 4
	}
	return off, nil
}

func appendArrayLeafNode(builder *Builder, n ArrayLeafNode) (uint32, error) {
	if n.Shift != 0 {
		return 0, fmt.Errorf("array leaf shift must be 0")
	}
	if len(n.Values) != popcount16(n.Bitmap) {
		return 0, fmt.Errorf("values length does not match bitmap")
	}
	bodyLen := 8
	for _, v := range n.Values {
		valLen, err := encodedValueLen(v)
		if err != nil {
			return 0, err
		}
		bodyLen += valLen
	}
	body, off := appendNodeWithBodyLen(builder, NodeLeaf, KeyArr, uint32(len(n.Values)), bodyLen)
	body[0] = n.Shift
	body[1] = 0
	binary.LittleEndian.PutUint16(body[2:4], n.Bitmap)
	binary.LittleEndian.PutUint32(body[4:8], n.Length)
	p := 8
	for _, v := range n.Values {
		n, err := writeValue(body[p:], v)
		if err != nil {
			return 0, err
		}
		p += n
	}
	return off, nil
}

func encodeNode(kind NodeKind, key KeyType, entryCount uint32, body []byte) ([]byte, error) {
	nodeLen := uint32(8 + len(body))
	pad := int((4 - (nodeLen % 4)) % 4)
	nodeLen += uint32(pad)
	flags := nodeLen &^ 0x3
	flags |= uint32(kind & 0x1)
	flags |= uint32(key&0x1) << 1

	out := make([]byte, nodeLen)
	binary.LittleEndian.PutUint32(out[0:4], flags)
	binary.LittleEndian.PutUint32(out[4:8], entryCount)
	copy(out[8:], body)
	return out, nil
}

func appendNodeWithBodyLen(builder *Builder, kind NodeKind, key KeyType, entryCount uint32, bodyLen int) ([]byte, uint32) {
	if bodyLen < 0 {
		return nil, 0
	}
	nodeLen := uint32(8 + bodyLen)
	pad := int((4 - (nodeLen % 4)) % 4)
	nodeLen += uint32(pad)
	flags := nodeLen &^ 0x3
	flags |= uint32(kind & 0x1)
	flags |= uint32(key&0x1) << 1

	off := uint32(len(builder.buf))
	newLen := int(off) + int(nodeLen)
	if newLen > cap(builder.buf) {
		builder.buf = append(builder.buf, make([]byte, newLen-len(builder.buf))...)
	} else {
		builder.buf = builder.buf[:newLen]
	}
	out := builder.buf[off:newLen]
	binary.LittleEndian.PutUint32(out[0:4], flags)
	binary.LittleEndian.PutUint32(out[4:8], entryCount)
	if pad != 0 {
		clear(out[8+bodyLen:])
	}
	return out[8 : 8+bodyLen], off
}

func appendNode(builder *Builder, kind NodeKind, key KeyType, entryCount uint32, body []byte) uint32 {
	nodeLen := uint32(8 + len(body))
	pad := int((4 - (nodeLen % 4)) % 4)
	nodeLen += uint32(pad)
	flags := nodeLen &^ 0x3
	flags |= uint32(kind & 0x1)
	flags |= uint32(key&0x1) << 1

	off := uint32(len(builder.buf))
	newLen := int(off) + int(nodeLen)
	if newLen > cap(builder.buf) {
		builder.buf = append(builder.buf, make([]byte, newLen-len(builder.buf))...)
	} else {
		builder.buf = builder.buf[:newLen]
	}
	out := builder.buf[off:newLen]
	binary.LittleEndian.PutUint32(out[0:4], flags)
	binary.LittleEndian.PutUint32(out[4:8], entryCount)
	copy(out[8:], body)
	if pad != 0 {
		clear(out[8+len(body):])
	}
	return off
}

func lengthBytes(length int) (int, error) {
	if length < 0 {
		return 0, fmt.Errorf("negative length")
	}
	if length <= 15 {
		return 0, nil
	}
	l := uint64(length)
	switch {
	case l <= 0xFF:
		return 1, nil
	case l <= 0xFFFF:
		return 2, nil
	case l <= 0xFFFFFF:
		return 3, nil
	case l <= 0xFFFFFFFF:
		return 4, nil
	case l <= 0xFFFFFFFFFF:
		return 5, nil
	case l <= 0xFFFFFFFFFFFF:
		return 6, nil
	case l <= 0xFFFFFFFFFFFFFF:
		return 7, nil
	case l <= 0xFFFFFFFFFFFFFFFF:
		return 8, nil
	default:
		return 0, fmt.Errorf("length too large")
	}
}

func lengthBytesNoErr(length int) int {
	if length < 0 {
		return 0
	}
	if length <= 15 {
		return 0
	}
	l := uint64(length)
	switch {
	case l <= 0xFF:
		return 1
	case l <= 0xFFFF:
		return 2
	case l <= 0xFFFFFF:
		return 3
	case l <= 0xFFFFFFFF:
		return 4
	case l <= 0xFFFFFFFFFF:
		return 5
	case l <= 0xFFFFFFFFFFFF:
		return 6
	case l <= 0xFFFFFFFFFFFFFF:
		return 7
	default:
		return 8
	}
}

func encodedBytesLen(length int) (int, error) {
	n, err := lengthBytes(length)
	if err != nil {
		return 0, err
	}
	return 1 + n + length, nil
}

func encodedBytesLenNoErr(length int) int {
	return 1 + lengthBytesNoErr(length) + length
}

func encodedValueLen(v Value) (int, error) {
	switch v.Type {
	case TypeNil, TypeBit:
		return 1, nil
	case TypeI64, TypeF64:
		return 9, nil
	case TypeTxt, TypeBin:
		return encodedBytesLen(len(v.Bytes))
	case TypeArr, TypeMap:
		offsetLen := 1
		if v.Offset > 0xFF {
			offsetLen = 2
		}
		if v.Offset > 0xFFFF {
			offsetLen = 3
		}
		if v.Offset > 0xFFFFFF {
			offsetLen = 4
		}
		return 1 + offsetLen, nil
	default:
		return 0, fmt.Errorf("unknown value type %d", v.Type)
	}
}

func encodedValueLenNoErr(v Value) int {
	switch v.Type {
	case TypeNil, TypeBit:
		return 1
	case TypeI64, TypeF64:
		return 9
	case TypeTxt, TypeBin:
		return encodedBytesLenNoErr(len(v.Bytes))
	case TypeArr, TypeMap:
		offsetLen := 1
		if v.Offset > 0xFF {
			offsetLen = 2
		}
		if v.Offset > 0xFFFF {
			offsetLen = 3
		}
		if v.Offset > 0xFFFFFF {
			offsetLen = 4
		}
		return 1 + offsetLen
	default:
		return 0
	}
}

func writeLengthToSlice(dst []byte, prefix byte, length int) (int, error) {
	n, err := lengthBytes(length)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		dst[0] = prefix | 0x10 | byte(length)
		return 1, nil
	}
	dst[0] = prefix | byte(n&0x0F)
	for i := 0; i < n; i++ {
		dst[1+i] = byte(length >> (8 * i))
	}
	return 1 + n, nil
}

func writeBytesValue(dst []byte, typ ValueType, payload []byte) (int, error) {
	if typ != TypeTxt && typ != TypeBin {
		return 0, fmt.Errorf("invalid bytes value type %d", typ)
	}
	n, err := writeLengthToSlice(dst, byte(typ)<<5, len(payload))
	if err != nil {
		return 0, err
	}
	copy(dst[n:], payload)
	return n + len(payload), nil
}

func writeOffsetValue(dst []byte, typ ValueType, offset uint32) (int, error) {
	if typ != TypeArr && typ != TypeMap {
		return 0, fmt.Errorf("invalid offset value type %d", typ)
	}
	length := 1
	if offset > 0xFF {
		length = 2
	}
	if offset > 0xFFFF {
		length = 3
	}
	if offset > 0xFFFFFF {
		length = 4
	}
	n, err := writeLengthToSlice(dst, byte(typ)<<5, length)
	if err != nil {
		return 0, err
	}
	dst[n+0] = byte(offset)
	if length > 1 {
		dst[n+1] = byte(offset >> 8)
	}
	if length > 2 {
		dst[n+2] = byte(offset >> 16)
	}
	if length > 3 {
		dst[n+3] = byte(offset >> 24)
	}
	return n + length, nil
}

func writeValue(dst []byte, v Value) (int, error) {
	switch v.Type {
	case TypeNil:
		dst[0] = TagNil
		return 1, nil
	case TypeBit:
		if v.Bool {
			dst[0] = TagBitTrue
		} else {
			dst[0] = TagBitFalse
		}
		return 1, nil
	case TypeI64:
		dst[0] = TagI64
		binary.LittleEndian.PutUint64(dst[1:9], uint64(v.I64))
		return 9, nil
	case TypeF64:
		dst[0] = TagF64
		binary.LittleEndian.PutUint64(dst[1:9], math.Float64bits(v.F64))
		return 9, nil
	case TypeTxt, TypeBin:
		return writeBytesValue(dst, v.Type, v.Bytes)
	case TypeArr, TypeMap:
		return writeOffsetValue(dst, v.Type, v.Offset)
	default:
		return 0, fmt.Errorf("unknown value type %d", v.Type)
	}
}
