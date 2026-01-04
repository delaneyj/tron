package tron

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/minio/simdjson-go"
)

// FromJSON parses JSON using simdjson-go and returns a TRON document.
func FromJSON(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("json input is empty")
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		val, err := scalarValueFromJSON(trimmed)
		if err != nil {
			return nil, err
		}
		return EncodeScalarDocument(val)
	}
	parsed, err := simdjson.Parse(data, nil)
	if err != nil {
		return nil, err
	}
	it := parsed.Iter()
	if it.Advance() != simdjson.TypeRoot {
		return nil, fmt.Errorf("json root not found")
	}
	typ, root, err := it.Root(nil)
	if err != nil {
		return nil, err
	}
	builder := NewBuilder()
	workspace := newEncodeWorkspace()
	val, err := valueFromJSONIter(typ, root, builder, workspace)
	if err != nil {
		return nil, err
	}
	switch val.Type {
	case TypeArr, TypeMap:
		return builder.BytesWithTrailer(val.Offset, 0), nil
	default:
		return EncodeScalarDocument(val)
	}
}

func scalarValueFromJSON(data []byte) (Value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return Value{}, err
	}
	if _, err := dec.Token(); err == nil || err != io.EOF {
		return Value{}, fmt.Errorf("invalid character after top-level value")
	}
	switch val := v.(type) {
	case nil:
		return Value{Type: TypeNil}, nil
	case bool:
		return Value{Type: TypeBit, Bool: val}, nil
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return Value{Type: TypeI64, I64: i}, nil
		}
		if f, err := val.Float64(); err == nil {
			return Value{Type: TypeF64, F64: f}, nil
		}
		return Value{}, fmt.Errorf("invalid json number: %s", val)
	case float64:
		if val >= math.MinInt64 && val <= math.MaxInt64 {
			if math.Trunc(val) == val {
				return Value{Type: TypeI64, I64: int64(val)}, nil
			}
		}
		return Value{Type: TypeF64, F64: val}, nil
	case string:
		if len(val) >= 4 && val[0] == 'b' && val[1] == '6' && val[2] == '4' && val[3] == ':' {
			rest := val[4:]
			if decoded, err := base64.StdEncoding.DecodeString(rest); err == nil {
				return Value{Type: TypeBin, Bytes: decoded}, nil
			}
		}
		return Value{Type: TypeTxt, Bytes: []byte(val)}, nil
	default:
		return Value{}, fmt.Errorf("unsupported scalar json type %T", v)
	}
}

func valueFromJSONIter(typ simdjson.Type, it *simdjson.Iter, builder *Builder, workspace *encodeWorkspace) (Value, error) {
	switch typ {
	case simdjson.TypeNull:
		return Value{Type: TypeNil}, nil
	case simdjson.TypeBool:
		v, err := it.Bool()
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeBit, Bool: v}, nil
	case simdjson.TypeInt:
		v, err := it.Int()
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeI64, I64: v}, nil
	case simdjson.TypeUint:
		v, err := it.Uint()
		if err != nil {
			return Value{}, err
		}
		if v > math.MaxInt64 {
			return Value{Type: TypeF64, F64: float64(v)}, nil
		}
		return Value{Type: TypeI64, I64: int64(v)}, nil
	case simdjson.TypeFloat:
		v, err := it.Float()
		if err != nil {
			return Value{}, err
		}
		if v >= math.MinInt64 && v <= math.MaxInt64 {
			if math.Trunc(v) == v {
				return Value{Type: TypeI64, I64: int64(v)}, nil
			}
		}
		return Value{Type: TypeF64, F64: v}, nil
	case simdjson.TypeString:
		b, err := it.StringBytes()
		if err != nil {
			return Value{}, err
		}
		if len(b) >= 4 && string(b[:4]) == "b64:" {
			if decoded, err := base64.StdEncoding.DecodeString(string(b[4:])); err == nil {
				return Value{Type: TypeBin, Bytes: decoded}, nil
			}
		}
		cpy := append([]byte{}, b...)
		return Value{Type: TypeTxt, Bytes: cpy}, nil
	case simdjson.TypeObject:
		obj, err := it.Object(nil)
		if err != nil {
			return Value{}, err
		}
		mb := newMapBuilderWithWorkspace(workspace)
		var parseErr error
		err = obj.ForEach(func(key []byte, elem simdjson.Iter) {
			if parseErr != nil {
				return
			}
			val, err := valueFromJSONIter(elem.Type(), &elem, builder, workspace)
			if err != nil {
				parseErr = err
				return
			}
			mb.Set(key, val)
		}, nil)
		if err != nil {
			return Value{}, err
		}
		if parseErr != nil {
			return Value{}, parseErr
		}
		off, err := mb.Build(builder)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeMap, Offset: off}, nil
	case simdjson.TypeArray:
		arr, err := it.Array(nil)
		if err != nil {
			return Value{}, err
		}
		ab := newArrayBuilderWithWorkspace(workspace)
		iter := arr.Iter()
		for {
			t := iter.Advance()
			if t == simdjson.TypeNone {
				break
			}
			elem := iter
			val, err := valueFromJSONIter(t, &elem, builder, workspace)
			if err != nil {
				return Value{}, err
			}
			ab.Append(val)
		}
		off, err := ab.Build(builder)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeArr, Offset: off}, nil
	default:
		return Value{}, fmt.Errorf("unsupported json type: %v", typ)
	}
}

// ToJSON encodes a TRON document into a JSON string.
func ToJSON(doc []byte) (string, error) {
	var sb strings.Builder
	if err := WriteJSON(&sb, doc); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// WriteJSON appends JSON for doc to sb.
func WriteJSON(sb *strings.Builder, doc []byte) error {
	docType, err := DetectDocType(doc)
	if err != nil {
		return err
	}
	switch docType {
	case DocScalar:
		val, err := DecodeScalarDocument(doc)
		if err != nil {
			return err
		}
		return writeJSONValue(sb, doc, val)
	case DocTree:
		tr, err := ParseTrailer(doc)
		if err != nil {
			return err
		}
		header, _, err := NodeSliceAt(doc, tr.RootOffset)
		if err != nil {
			return err
		}
		switch header.KeyType {
		case KeyMap:
			return writeJSONValue(sb, doc, Value{Type: TypeMap, Offset: tr.RootOffset})
		case KeyArr:
			return writeJSONValue(sb, doc, Value{Type: TypeArr, Offset: tr.RootOffset})
		default:
			return fmt.Errorf("unknown root node type")
		}
	default:
		return fmt.Errorf("unsupported document type")
	}
}

func writeJSONValue(sb *strings.Builder, doc []byte, v Value) error {
	switch v.Type {
	case TypeNil:
		sb.WriteString("null")
	case TypeBit:
		if v.Bool {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case TypeI64:
		sb.WriteString(strconv.FormatInt(v.I64, 10))
	case TypeF64:
		sb.WriteString(strconv.FormatFloat(v.F64, 'g', -1, 64))
	case TypeTxt:
		writeJSONStringBytes(sb, v.Bytes)
	case TypeBin:
		sb.WriteByte('"')
		sb.WriteString("b64:")
		sb.WriteString(base64.StdEncoding.EncodeToString(v.Bytes))
		sb.WriteByte('"')
	case TypeArr:
		return writeJSONArray(sb, doc, v.Offset)
	case TypeMap:
		return writeJSONObject(sb, doc, v.Offset)
	default:
		return fmt.Errorf("unknown value type %d", v.Type)
	}
	return nil
}

func writeJSONObject(sb *strings.Builder, doc []byte, off uint32) error {
	sb.WriteByte('{')
	state := mapWriteState{sb: sb, first: true}
	if err := writeMapNode(&state, doc, off); err != nil {
		return err
	}
	sb.WriteByte('}')
	return nil
}

type mapWriteState struct {
	sb    *strings.Builder
	first bool
}

func writeMapNode(state *mapWriteState, doc []byte, off uint32) error {
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
		defer releaseMapLeafNode(&leaf)
		for _, entry := range leaf.Entries {
			if err := writeMapEntry(state, entry.Key, entry.Value, doc); err != nil {
				return err
			}
		}
		return nil
	}
	branch, err := ParseMapBranchNode(node)
	if err != nil {
		return err
	}
	defer releaseMapBranchNode(&branch)
	for _, child := range branch.Children {
		if err := writeMapNode(state, doc, child); err != nil {
			return err
		}
	}
	return nil
}

func writeMapEntry(state *mapWriteState, key []byte, val Value, doc []byte) error {
	if !state.first {
		state.sb.WriteByte(',')
	}
	state.first = false
	writeJSONStringBytes(state.sb, key)
	state.sb.WriteByte(':')
	return writeJSONValue(state.sb, doc, val)
}

func writeJSONArray(sb *strings.Builder, doc []byte, off uint32) error {
	h, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != KeyArr {
		return fmt.Errorf("node is not an array")
	}
	var length uint32
	switch h.Kind {
	case NodeLeaf:
		leaf, err := ParseArrayLeafNode(node)
		if err != nil {
			return err
		}
		defer releaseArrayLeafNode(&leaf)
		length = leaf.Length
	case NodeBranch:
		branch, err := ParseArrayBranchNode(node)
		if err != nil {
			return err
		}
		defer releaseArrayBranchNode(&branch)
		length = branch.Length
	default:
		return fmt.Errorf("unknown array node kind")
	}
	if length == 0 {
		sb.WriteString("[]")
		return nil
	}
	values := make([]Value, length)
	present := make([]bool, length)
	if err := collectArrayValues(doc, off, 0, values, present); err != nil {
		return err
	}

	sb.WriteByte('[')
	for i := range values {
		if i > 0 {
			sb.WriteByte(',')
		}
		if !present[i] {
			sb.WriteString("null")
			continue
		}
		if err := writeJSONValue(sb, doc, values[i]); err != nil {
			return err
		}
	}
	sb.WriteByte(']')
	return nil
}

func collectArrayValues(doc []byte, off uint32, base uint32, values []Value, present []bool) error {
	h, node, err := NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != KeyArr {
		return fmt.Errorf("node is not an array")
	}
	if h.Kind == NodeLeaf {
		leaf, err := ParseArrayLeafNode(node)
		if err != nil {
			return err
		}
		defer releaseArrayLeafNode(&leaf)
		if leaf.Shift != 0 {
			return fmt.Errorf("array leaf shift must be 0")
		}
		idx := 0
		for slot := 0; slot < 16; slot++ {
			if ((leaf.Bitmap >> uint(slot)) & 1) == 0 {
				continue
			}
			index := base + uint32(slot)
			if index >= uint32(len(values)) {
				return fmt.Errorf("array index out of range: %d", index)
			}
			values[index] = leaf.Values[idx]
			present[index] = true
			idx++
		}
		return nil
	}

	branch, err := ParseArrayBranchNode(node)
	if err != nil {
		return err
	}
	defer releaseArrayBranchNode(&branch)
	idx := 0
	for slot := 0; slot < 16; slot++ {
		if ((branch.Bitmap >> uint(slot)) & 1) == 0 {
			continue
		}
		child := branch.Children[idx]
		childBase := base + (uint32(slot) << branch.Shift)
		if err := collectArrayValues(doc, child, childBase, values, present); err != nil {
			return err
		}
		idx++
	}
	return nil
}

func writeJSONStringBytes(sb *strings.Builder, b []byte) {
	sb.WriteByte('"')
	for _, c := range b {
		switch c {
		case '"', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if c < 0x20 {
				sb.WriteString(`\u00`)
				sb.WriteByte(hexDigit(c >> 4))
				sb.WriteByte(hexDigit(c & 0xF))
			} else {
				sb.WriteByte(c)
			}
		}
	}
	sb.WriteByte('"')
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'A' + (n - 10)
}
