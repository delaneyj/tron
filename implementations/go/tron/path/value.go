package path

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"

	tron "tron"
)

type jKind int

const (
	kindNull jKind = iota
	kindBool
	kindNumber
	kindString
	kindArray
	kindObject
	kindTRONMap
	kindTRONArr
	kindExpRef
)

type jValue struct {
	kind jKind
	b    bool
	n    float64
	s    string
	arr  []jValue
	obj  map[string]jValue
	doc  []byte
	off  uint32
	ref  *node
	tv   tron.Value
	tvOK bool
}

func nullValue() jValue { return jValue{kind: kindNull} }

func valueFromTRON(doc []byte, v tron.Value) jValue {
	switch v.Type {
	case tron.TypeNil:
		return jValue{kind: kindNull, tv: v, tvOK: true}
	case tron.TypeBit:
		return jValue{kind: kindBool, b: v.Bool, tv: v, tvOK: true}
	case tron.TypeI64:
		return jValue{kind: kindNumber, n: float64(v.I64), tv: v, tvOK: true}
	case tron.TypeF64:
		return jValue{kind: kindNumber, n: v.F64, tv: v, tvOK: true}
	case tron.TypeTxt:
		return jValue{kind: kindString, s: string(v.Bytes), tv: v, tvOK: true}
	case tron.TypeBin:
		return jValue{kind: kindString, s: "b64:" + base64.StdEncoding.EncodeToString(v.Bytes), tv: v, tvOK: true}
	case tron.TypeArr:
		return jValue{kind: kindTRONArr, doc: doc, off: v.Offset, tv: v, tvOK: true}
	case tron.TypeMap:
		return jValue{kind: kindTRONMap, doc: doc, off: v.Offset, tv: v, tvOK: true}
	default:
		return nullValue()
	}
}

func valueFromLiteral(lit any) jValue {
	switch v := lit.(type) {
	case nil:
		return nullValue()
	case bool:
		return jValue{kind: kindBool, b: v}
	case float64:
		return jValue{kind: kindNumber, n: v}
	case string:
		return jValue{kind: kindString, s: v}
	case []any:
		out := make([]jValue, 0, len(v))
		for _, item := range v {
			out = append(out, valueFromLiteral(item))
		}
		return jValue{kind: kindArray, arr: out}
	case map[string]any:
		out := make(map[string]jValue, len(v))
		for k, item := range v {
			out[k] = valueFromLiteral(item)
		}
		return jValue{kind: kindObject, obj: out}
	default:
		return nullValue()
	}
}

func (v jValue) isNull() bool {
	return v.kind == kindNull
}

func (v jValue) toInterface() (any, error) {
	switch v.kind {
	case kindNull:
		return nil, nil
	case kindBool:
		return v.b, nil
	case kindNumber:
		return v.n, nil
	case kindString:
		return v.s, nil
	case kindArray:
		out := make([]any, len(v.arr))
		for i, item := range v.arr {
			val, err := item.toInterface()
			if err != nil {
				return nil, err
			}
			out[i] = val
		}
		return out, nil
	case kindObject:
		out := make(map[string]any, len(v.obj))
		for k, item := range v.obj {
			val, err := item.toInterface()
			if err != nil {
				return nil, err
			}
			out[k] = val
		}
		return out, nil
	case kindTRONArr:
		length, err := arrayLength(v.doc, v.off)
		if err != nil {
			return nil, err
		}
		values := make([]tron.Value, length)
		present := make([]bool, length)
		if err := arrCollectValues(v.doc, v.off, 0, values, present); err != nil {
			return nil, err
		}
		out := make([]any, length)
		for i := range values {
			if !present[i] {
				out[i] = nil
				continue
			}
			val, err := valueFromTRON(v.doc, values[i]).toInterface()
			if err != nil {
				return nil, err
			}
			out[i] = val
		}
		return out, nil
	case kindTRONMap:
		out := map[string]any{}
		if err := mapIterEntries(v.doc, v.off, func(key []byte, val tron.Value) error {
			iv, err := valueFromTRON(v.doc, val).toInterface()
			if err != nil {
				return err
			}
			out[string(key)] = iv
			return nil
		}); err != nil {
			return nil, err
		}
		return out, nil
	case kindExpRef:
		return nil, fmt.Errorf("cannot convert expref to interface")
	default:
		return nil, fmt.Errorf("unknown value kind")
	}
}

func (v jValue) toTRONValue() (tron.Value, error) {
	if v.tvOK {
		return v.tv, nil
	}
	switch v.kind {
	case kindNull:
		return tron.Value{Type: tron.TypeNil}, nil
	case kindBool:
		return tron.Value{Type: tron.TypeBit, Bool: v.b}, nil
	case kindNumber:
		if math.IsNaN(v.n) || math.IsInf(v.n, 0) {
			return tron.Value{}, fmt.Errorf("invalid number")
		}
		if v.n == math.Trunc(v.n) && v.n >= math.MinInt64 && v.n <= math.MaxInt64 {
			return tron.Value{Type: tron.TypeI64, I64: int64(v.n)}, nil
		}
		return tron.Value{Type: tron.TypeF64, F64: v.n}, nil
	case kindString:
		return tron.Value{Type: tron.TypeTxt, Bytes: []byte(v.s)}, nil
	case kindTRONArr:
		return tron.Value{Type: tron.TypeArr, Offset: v.off}, nil
	case kindTRONMap:
		return tron.Value{Type: tron.TypeMap, Offset: v.off}, nil
	case kindArray, kindObject:
		return tron.Value{}, fmt.Errorf("expression result is not a TRON-backed value")
	case kindExpRef:
		return tron.Value{}, fmt.Errorf("expref cannot be returned as a value")
	default:
		return tron.Value{}, fmt.Errorf("unknown value kind")
	}
}

func (v jValue) toJSON() (string, error) {
	raw, err := v.toInterface()
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
