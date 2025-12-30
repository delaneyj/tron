package tron

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// DocType indicates the top-level document kind.
type DocType uint8

const (
	DocUnknown DocType = iota
	DocScalar
	DocTree
)

// DetectDocType inspects the tail to determine document type.
func DetectDocType(b []byte) (DocType, error) {
	if len(b) < len(ScalarMagic) {
		return DocUnknown, fmt.Errorf("document too short")
	}
	tail := b[len(b)-len(ScalarMagic):]
	switch {
	case bytes.Equal(tail, ScalarMagic[:]):
		return DocScalar, nil
	case bytes.Equal(tail, TrailerMagic[:]):
		if len(b) < TrailerSize {
			return DocUnknown, fmt.Errorf("tree trailer too short")
		}
		return DocTree, nil
	default:
		return DocUnknown, fmt.Errorf("unknown document trailer")
	}
}

// DecodeScalarDocument decodes a scalar document into a value.
func DecodeScalarDocument(b []byte) (Value, error) {
	if len(b) < len(ScalarMagic) {
		return Value{}, fmt.Errorf("document too short")
	}
	if !bytes.Equal(b[len(b)-len(ScalarMagic):], ScalarMagic[:]) {
		return Value{}, fmt.Errorf("missing NORT terminator")
	}
	payload := b[:len(b)-len(ScalarMagic)]
	val, n, err := DecodeValue(payload)
	if err != nil {
		return Value{}, err
	}
	if n != len(payload) {
		return Value{}, fmt.Errorf("extra bytes after scalar value")
	}
	return val, nil
}

// EncodeScalarDocument encodes a scalar value into a document.
func EncodeScalarDocument(v Value) ([]byte, error) {
	val, err := EncodeValue(v)
	if err != nil {
		return nil, err
	}
	out := append([]byte{}, val...)
	out = append(out, ScalarMagic[:]...)
	return out, nil
}

// ParseRootHeader returns the trailer and root node header.
func ParseRootHeader(b []byte) (Trailer, NodeHeader, error) {
	tr, err := ParseTrailer(b)
	if err != nil {
		return Trailer{}, NodeHeader{}, err
	}
	h, _, err := NodeSliceAt(b, tr.RootOffset)
	return tr, h, err
}

// NodeSliceAt returns the node header and the full node bytes at offset.
func NodeSliceAt(b []byte, off uint32) (NodeHeader, []byte, error) {
	if int(off) < 0 || int(off) >= len(b) {
		return NodeHeader{}, nil, fmt.Errorf("node offset out of range: %d", off)
	}
	h, err := ParseNodeHeader(b[off:])
	if err != nil {
		return NodeHeader{}, nil, err
	}
	end := int(off) + int(h.NodeLen)
	if end > len(b) {
		return NodeHeader{}, nil, fmt.Errorf("node length out of range: %d", h.NodeLen)
	}
	return h, b[off:end], nil
}

// Builder helps assemble a tree document by appending nodes.
type Builder struct {
	buf []byte
}

// NewBuilder creates an empty builder.
func NewBuilder() *Builder {
	return &Builder{buf: make([]byte, 0, 1024)}
}

// NewBuilderWithCapacity creates an empty builder with a given capacity.
func NewBuilderWithCapacity(capacity int) *Builder {
	if capacity <= 0 {
		return NewBuilder()
	}
	return &Builder{buf: make([]byte, 0, capacity)}
}

// NewBuilderFromDocument copies a tree document into a builder and returns its trailer.
func NewBuilderFromDocument(doc []byte) (*Builder, Trailer, error) {
	tr, err := ParseTrailer(doc)
	if err != nil {
		return nil, Trailer{}, err
	}
	buf := append([]byte{}, doc[:len(doc)-TrailerSize]...)
	return &Builder{buf: buf}, tr, nil
}

// AppendNode appends an encoded node and returns its offset.
func (b *Builder) AppendNode(node []byte) uint32 {
	off := uint32(len(b.buf))
	b.buf = append(b.buf, node...)
	return off
}

// Buffer returns the current builder buffer (without trailer).
func (b *Builder) Buffer() []byte {
	return b.buf
}

// Reset clears the builder buffer while retaining its capacity.
func (b *Builder) Reset() {
	b.buf = b.buf[:0]
}

// BytesWithTrailer returns the document with a trailer appended.
func (b *Builder) BytesWithTrailer(rootOffset, prevRootOffset uint32) []byte {
	out := append([]byte{}, b.buf...)
	out = AppendTrailer(out, Trailer{
		RootOffset:     rootOffset,
		PrevRootOffset: prevRootOffset,
	})
	return out
}

// BytesWithTrailerInPlace appends a trailer to the builder buffer and returns it.
// The builder buffer is modified to include the trailer.
func (b *Builder) BytesWithTrailerInPlace(rootOffset, prevRootOffset uint32) []byte {
	start := len(b.buf)
	if cap(b.buf) < start+TrailerSize {
		b.buf = append(b.buf, make([]byte, TrailerSize)...)
	} else {
		b.buf = b.buf[:start+TrailerSize]
	}
	binary.LittleEndian.PutUint32(b.buf[start:start+4], rootOffset)
	binary.LittleEndian.PutUint32(b.buf[start+4:start+8], prevRootOffset)
	copy(b.buf[start+8:start+12], TrailerMagic[:])
	return b.buf
}
