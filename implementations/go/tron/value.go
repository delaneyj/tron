package tron

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/delaneyj/toolbelt/bytebufferpool"
)

// Value represents a decoded TRON value record.
type Value struct {
	Type   ValueType
	Bool   bool
	I64    int64
	F64    float64
	Bytes  []byte
	Offset uint32
}

// DecodeValue decodes a value record from b and returns the value and bytes read.
func DecodeValue(b []byte) (Value, int, error) {
	if len(b) < 1 {
		return Value{}, 0, fmt.Errorf("value tag missing")
	}
	tag := b[0]
	typ := TypeFromTag(tag)

	switch typ {
	case TypeNil:
		if LowBits(tag) != 0 {
			return Value{}, 0, fmt.Errorf("nil tag has non-zero low bits")
		}
		return Value{Type: TypeNil}, 1, nil
	case TypeBit:
		if (tag & 0x1E) != 0 {
			return Value{}, 0, fmt.Errorf("bit tag has invalid low bits")
		}
		return Value{Type: TypeBit, Bool: (tag & 0x01) == 1}, 1, nil
	case TypeI64:
		v, n, err := DecodeI64Value(b)
		if err != nil {
			return Value{}, 0, err
		}
		return Value{Type: TypeI64, I64: v}, n, nil
	case TypeF64:
		v, n, err := DecodeF64Value(b)
		if err != nil {
			return Value{}, 0, err
		}
		return Value{Type: TypeF64, F64: v}, n, nil
	case TypeTxt, TypeBin, TypeArr, TypeMap:
		length, n, err := decodeLength(tag, b[1:])
		if err != nil {
			return Value{}, 0, err
		}
		if len(b) < 1+n+int(length) {
			return Value{}, 0, fmt.Errorf("payload too short: need %d bytes", length)
		}
		payload := b[1+n : 1+n+int(length)]

		switch typ {
		case TypeTxt, TypeBin:
			return Value{Type: typ, Bytes: payload}, 1 + n + int(length), nil
		case TypeArr, TypeMap:
			if length == 0 || length > 4 {
				return Value{}, 0, fmt.Errorf("node offset length out of range: %d", length)
			}
			var off uint32
			for i := uint64(0); i < length; i++ {
				off |= uint32(payload[i]) << (8 * i)
			}
			return Value{Type: typ, Offset: off}, 1 + n + int(length), nil
		}
	}

	return Value{}, 0, fmt.Errorf("unknown value type %d", typ)
}

// DecodeI64Value decodes an i64 value record without copying payload bytes.
func DecodeI64Value(b []byte) (int64, int, error) {
	if len(b) < 1 {
		return 0, 0, fmt.Errorf("value tag missing")
	}
	tag := b[0]
	if TypeFromTag(tag) != TypeI64 {
		return 0, 0, fmt.Errorf("value is not i64")
	}
	if LowBits(tag) != 0 {
		return 0, 0, fmt.Errorf("i64 tag has non-zero low bits")
	}
	if len(b) < 1+8 {
		return 0, 0, fmt.Errorf("i64 payload too short")
	}
	v := int64(binary.LittleEndian.Uint64(b[1:9]))
	return v, 9, nil
}

// DecodeF64Value decodes an f64 value record without copying payload bytes.
func DecodeF64Value(b []byte) (float64, int, error) {
	if len(b) < 1 {
		return 0, 0, fmt.Errorf("value tag missing")
	}
	tag := b[0]
	if TypeFromTag(tag) != TypeF64 {
		return 0, 0, fmt.Errorf("value is not f64")
	}
	if LowBits(tag) != 0 {
		return 0, 0, fmt.Errorf("f64 tag has non-zero low bits")
	}
	if len(b) < 1+8 {
		return 0, 0, fmt.Errorf("f64 payload too short")
	}
	u := binary.LittleEndian.Uint64(b[1:9])
	return math.Float64frombits(u), 9, nil
}

// EncodeValue encodes v into a value record.
func EncodeValue(v Value) ([]byte, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	if err := encodeValueToBuffer(buf, v); err != nil {
		return nil, err
	}
	out := append([]byte{}, buf.Bytes()...)
	return out, nil
}

func encodeBytesValue(typ ValueType, payload []byte) ([]byte, error) {
	if typ != TypeTxt && typ != TypeBin {
		return nil, fmt.Errorf("invalid type for bytes value: %d", typ)
	}
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	if err := encodeBytesToBuffer(buf, typ, payload); err != nil {
		return nil, err
	}
	out := append([]byte{}, buf.Bytes()...)
	return out, nil
}

func encodeOffsetValue(typ ValueType, offset uint32) ([]byte, error) {
	if typ != TypeArr && typ != TypeMap {
		return nil, fmt.Errorf("invalid type for offset value: %d", typ)
	}
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	if err := encodeOffsetToBuffer(buf, typ, offset); err != nil {
		return nil, err
	}
	out := append([]byte{}, buf.Bytes()...)
	return out, nil
}

// decodeLength reads the length encoding for txt/bin/arr/map.
// It returns the payload length and number of length bytes read.
func decodeLength(tag byte, b []byte) (uint64, int, error) {
	if (tag & 0x10) != 0 {
		return uint64(tag & 0x0F), 0, nil
	}
	n := int(tag & 0x0F)
	if n < 1 || n > 8 {
		return 0, 0, fmt.Errorf("invalid length-of-length: %d", n)
	}
	if len(b) < n {
		return 0, 0, fmt.Errorf("length-of-length bytes missing")
	}
	var length uint64
	for i := 0; i < n; i++ {
		length |= uint64(b[i]) << (8 * i)
	}
	return length, n, nil
}

// encodeLength encodes a payload length for txt/bin/arr/map and returns tag and length bytes.
func encodeLength(prefix byte, length int) (byte, []byte, error) {
	if length < 0 {
		return 0, nil, fmt.Errorf("negative length")
	}
	if length <= 15 {
		return prefix | 0x10 | byte(length), nil, nil
	}
	// minimal N such that length fits
	n := 1
	for max := 0xFF; length > max && n < 8; n++ {
		max = (max << 8) | 0xFF
	}
	if n > 8 {
		return 0, nil, fmt.Errorf("length too large")
	}
	lenBytes := make([]byte, n)
	for i := 0; i < n; i++ {
		lenBytes[i] = byte(length >> (8 * i))
	}
	return prefix | byte(n&0x0F), lenBytes, nil
}

func encodeValueToBuffer(buf *bytebufferpool.ByteBuffer, v Value) error {
	switch v.Type {
	case TypeNil:
		buf.WriteByte(TagNil)
		return nil
	case TypeBit:
		if v.Bool {
			buf.WriteByte(TagBitTrue)
		} else {
			buf.WriteByte(TagBitFalse)
		}
		return nil
	case TypeI64:
		buf.WriteByte(TagI64)
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], uint64(v.I64))
		buf.Write(tmp[:])
		return nil
	case TypeF64:
		buf.WriteByte(TagF64)
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(v.F64))
		buf.Write(tmp[:])
		return nil
	case TypeTxt, TypeBin:
		return encodeBytesToBuffer(buf, v.Type, v.Bytes)
	case TypeArr, TypeMap:
		return encodeOffsetToBuffer(buf, v.Type, v.Offset)
	default:
		return fmt.Errorf("unknown value type %d", v.Type)
	}
}

func encodeTextStringToBuffer(buf *bytebufferpool.ByteBuffer, s string) error {
	if err := writeLength(buf, byte(TypeTxt)<<5, len(s)); err != nil {
		return err
	}
	buf.WriteString(s)
	return nil
}

func encodeBytesToBuffer(buf *bytebufferpool.ByteBuffer, typ ValueType, payload []byte) error {
	if typ != TypeTxt && typ != TypeBin {
		return fmt.Errorf("invalid type for bytes value: %d", typ)
	}
	if err := writeLength(buf, byte(typ)<<5, len(payload)); err != nil {
		return err
	}
	buf.Write(payload)
	return nil
}

func encodeOffsetToBuffer(buf *bytebufferpool.ByteBuffer, typ ValueType, offset uint32) error {
	if typ != TypeArr && typ != TypeMap {
		return fmt.Errorf("invalid type for offset value: %d", typ)
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
	if err := writeLength(buf, byte(typ)<<5, length); err != nil {
		return err
	}
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], offset)
	buf.Write(tmp[:length])
	return nil
}

func writeLength(buf *bytebufferpool.ByteBuffer, prefix byte, length int) error {
	if length < 0 {
		return fmt.Errorf("negative length")
	}
	if length <= 15 {
		buf.WriteByte(prefix | 0x10 | byte(length))
		return nil
	}
	n := 1
	for max := 0xFF; length > max && n < 8; n++ {
		max = (max << 8) | 0xFF
	}
	if n > 8 {
		return fmt.Errorf("length too large")
	}
	buf.WriteByte(prefix | byte(n&0x0F))
	var tmp [8]byte
	for i := 0; i < n; i++ {
		tmp[i] = byte(length >> (8 * i))
	}
	buf.Write(tmp[:n])
	return nil
}
