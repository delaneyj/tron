package tron

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// TrailerMagic is the 4-byte terminator for tree documents.
var TrailerMagic = [4]byte{'T', 'R', 'O', 'N'}

// ScalarMagic is the 4-byte terminator for scalar documents.
var ScalarMagic = [4]byte{'N', 'O', 'R', 'T'}

const TrailerSize = 12

// Trailer describes the root record trailer at the end of a tree document.
type Trailer struct {
	RootOffset     uint32
	PrevRootOffset uint32
}

// ParseTrailer parses the last 12 bytes of a tree document.
func ParseTrailer(b []byte) (Trailer, error) {
	if len(b) < TrailerSize {
		return Trailer{}, fmt.Errorf("trailer too short: %d", len(b))
	}
	if !bytes.Equal(b[len(b)-4:], TrailerMagic[:]) {
		return Trailer{}, fmt.Errorf("missing TRON trailer magic")
	}
	start := len(b) - TrailerSize
	return Trailer{
		RootOffset:     binary.LittleEndian.Uint32(b[start : start+4]),
		PrevRootOffset: binary.LittleEndian.Uint32(b[start+4 : start+8]),
	}, nil
}

// AppendTrailer appends a trailer to dst and returns the extended slice.
func AppendTrailer(dst []byte, t Trailer) []byte {
	buf := make([]byte, TrailerSize)
	binary.LittleEndian.PutUint32(buf[0:4], t.RootOffset)
	binary.LittleEndian.PutUint32(buf[4:8], t.PrevRootOffset)
	copy(buf[8:12], TrailerMagic[:])
	return append(dst, buf...)
}
