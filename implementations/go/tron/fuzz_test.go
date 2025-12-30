package tron

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"
)

func FuzzEncodeDecodeValue(f *testing.F) {
	seeds := [][]byte{
		{0x00},
		{0x01},
		{0x02, 0x01},
		{0x03, 0x01, 0x02, 0x03, 0x04},
		{0x04, 'h', 'i'},
		{0x05, 0xff, 0x00, 0x7f},
		{0x06, 0x10, 0x00, 0x00},
		{0x07, 0xff, 0xff, 0xff, 0xff},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		v := valueFromFuzzBytes(data)
		enc, err := EncodeValue(v)
		if err != nil {
			t.Fatalf("encode value: %v", err)
		}
		dec, n, err := DecodeValue(enc)
		if err != nil {
			t.Fatalf("decode value: %v", err)
		}
		if n != len(enc) {
			t.Fatalf("decoded length %d != encoded length %d", n, len(enc))
		}
		if !valueEqual(v, dec) {
			t.Fatalf("roundtrip mismatch: %#v != %#v", v, dec)
		}
		doc, err := EncodeScalarDocument(v)
		if err != nil {
			t.Fatalf("encode scalar doc: %v", err)
		}
		got, err := DecodeScalarDocument(doc)
		if err != nil {
			t.Fatalf("decode scalar doc: %v", err)
		}
		if !valueEqual(v, got) {
			t.Fatalf("scalar roundtrip mismatch: %#v != %#v", v, got)
		}
	})
}

func FuzzJSONRoundTrip(f *testing.F) {
	seeds := [][]byte{
		[]byte("null"),
		[]byte("true"),
		[]byte("false"),
		[]byte("1"),
		[]byte("1.5"),
		[]byte(`"hi"`),
		[]byte(`"b64:AA=="`),
		[]byte("[]"),
		[]byte("{}"),
		[]byte(`{"a":1,"b":[true,false],"c":{"d":"x"}}`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		doc, err := FromJSON(data)
		if err != nil {
			return
		}
		out, err := ToJSON(doc)
		if err != nil {
			t.Fatalf("tojson: %v", err)
		}
		var v any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("json unmarshal: %v", err)
		}
		if _, err := FromJSON([]byte(out)); err != nil {
			t.Fatalf("fromjson roundtrip: %v", err)
		}
	})
}

func valueFromFuzzBytes(data []byte) Value {
	typ := ValueType(data[0] & 0x7)
	payload := data[1:]
	switch typ {
	case TypeNil:
		return Value{Type: TypeNil}
	case TypeBit:
		return Value{Type: TypeBit, Bool: len(payload) > 0 && (payload[0]&0x1) == 1}
	case TypeI64:
		return Value{Type: TypeI64, I64: int64(readUint64(payload))}
	case TypeF64:
		return Value{Type: TypeF64, F64: math.Float64frombits(readUint64(payload))}
	case TypeTxt:
		return Value{Type: TypeTxt, Bytes: payload}
	case TypeBin:
		return Value{Type: TypeBin, Bytes: payload}
	case TypeArr:
		return Value{Type: TypeArr, Offset: readUint32(payload)}
	case TypeMap:
		return Value{Type: TypeMap, Offset: readUint32(payload)}
	default:
		return Value{Type: TypeNil}
	}
}

func readUint32(b []byte) uint32 {
	var tmp [4]byte
	copy(tmp[:], b)
	return binary.LittleEndian.Uint32(tmp[:])
}

func readUint64(b []byte) uint64 {
	var tmp [8]byte
	copy(tmp[:], b)
	return binary.LittleEndian.Uint64(tmp[:])
}
