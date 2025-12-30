package tron

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// AsInt64 returns the value as int64 when it can be reasonably converted.
// Numeric and text values are converted using best-effort parsing.
func (v Value) AsInt64() (int64, bool) {
	switch v.Type {
	case TypeI64:
		return v.I64, true
	case TypeF64:
		if math.IsNaN(v.F64) || math.IsInf(v.F64, 0) {
			return 0, false
		}
		if v.F64 < math.MinInt64 || v.F64 > math.MaxInt64 {
			return 0, false
		}
		return int64(v.F64), true
	case TypeTxt:
		return parseInt64(string(v.Bytes))
	case TypeBit:
		if v.Bool {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// AsFloat64 returns the value as float64 when it can be reasonably converted.
// Numeric and text values are converted using best-effort parsing.
func (v Value) AsFloat64() (float64, bool) {
	switch v.Type {
	case TypeF64:
		return v.F64, true
	case TypeI64:
		return float64(v.I64), true
	case TypeTxt:
		return parseFloat64(string(v.Bytes))
	case TypeBit:
		if v.Bool {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// AsString returns the value as string when it can be reasonably converted.
// Numeric and boolean values are formatted as their scalar representations.
func (v Value) AsString() (string, bool) {
	switch v.Type {
	case TypeTxt:
		return string(v.Bytes), true
	case TypeBin:
		return string(v.Bytes), true
	case TypeI64:
		return strconv.FormatInt(v.I64, 10), true
	case TypeF64:
		return strconv.FormatFloat(v.F64, 'g', -1, 64), true
	case TypeBit:
		if v.Bool {
			return "1", true
		}
		return "0", true
	default:
		return "", false
	}
}

// AsBytes returns the value as bytes when it can be reasonably converted.
// Text and binary values are returned directly; other scalars are formatted.
func (v Value) AsBytes() ([]byte, bool) {
	switch v.Type {
	case TypeBin, TypeTxt:
		return v.Bytes, true
	default:
		str, ok := v.AsString()
		if !ok {
			return nil, false
		}
		return []byte(str), true
	}
}

// AsArray converts a TypeArr value into a []any using the provided document.
func (v Value) AsArray(doc []byte) ([]any, error) {
	if v.Type != TypeArr {
		return nil, fmt.Errorf("value is not array")
	}
	return valueArrayToAny(doc, v.Offset)
}

// AsObject converts a TypeMap value into a map[string]any using the provided document.
func (v Value) AsObject(doc []byte) (map[string]any, error) {
	if v.Type != TypeMap {
		return nil, fmt.Errorf("value is not map")
	}
	return valueMapToAny(doc, v.Offset)
}

func valueToAny(doc []byte, v Value) (any, error) {
	switch v.Type {
	case TypeNil:
		return nil, nil
	case TypeBit:
		return v.Bool, nil
	case TypeI64:
		return v.I64, nil
	case TypeF64:
		return v.F64, nil
	case TypeTxt:
		return string(v.Bytes), nil
	case TypeBin:
		return v.Bytes, nil
	case TypeArr:
		return valueArrayToAny(doc, v.Offset)
	case TypeMap:
		return valueMapToAny(doc, v.Offset)
	default:
		return nil, fmt.Errorf("unknown value type %d", v.Type)
	}
}

func parseInt64(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		if f < math.MinInt64 || f > math.MaxInt64 {
			return 0, false
		}
		return int64(f), true
	}
	return 0, false
}

func parseFloat64(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, true
	}
	return 0, false
}

func valueMapToAny(doc []byte, off uint32) (map[string]any, error) {
	out := make(map[string]any)
	if err := valueMapFill(doc, off, out); err != nil {
		return nil, err
	}
	return out, nil
}

func valueMapFill(doc []byte, off uint32, out map[string]any) error {
	h, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != KeyMap {
		return fmt.Errorf("node is not a map")
	}
	if h.Kind == NodeLeaf {
		leaf, err := ParseMapLeafNode(node)
		if err != nil {
			return err
		}
		defer ReleaseMapLeafNode(&leaf)
		for _, entry := range leaf.Entries {
			val, err := valueToAny(doc, entry.Value)
			if err != nil {
				return err
			}
			out[string(entry.Key)] = val
		}
		return nil
	}
	branch, err := ParseMapBranchNode(node)
	if err != nil {
		return err
	}
	defer ReleaseMapBranchNode(&branch)
	for _, child := range branch.Children {
		if err := valueMapFill(doc, child, out); err != nil {
			return err
		}
	}
	return nil
}

func valueArrayToAny(doc []byte, off uint32) ([]any, error) {
	length, err := ArrayRootLength(doc, off)
	if err != nil {
		return nil, err
	}
	out := make([]any, length)
	for i := uint32(0); i < length; i++ {
		val, ok, err := ArrGet(doc, off, i)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("array index missing: %d", i)
		}
		conv, err := valueToAny(doc, val)
		if err != nil {
			return nil, err
		}
		out[i] = conv
	}
	return out, nil
}
