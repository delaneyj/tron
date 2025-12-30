package tron

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"

	_ "embed"

	"github.com/fxamacker/cbor/v2"
)

var (
	//go:embed testdata/geojson_large.json
	benchSampleJSON   []byte
	benchSampleCBOR   []byte
	benchSampleTRON    []byte
	benchSampleAny    any
	benchSampleAnyTRON any
	benchCBORDec      cbor.DecMode

	keyFeatures    = []byte("features")
	keyGeometry    = []byte("geometry")
	keyCoordinates = []byte("coordinates")
	keyProperties  = []byte("properties")
	keyElevation   = []byte("elevation")

	benchStringCache = newStringCache()
)

var sinkBytes []byte
var sinkAny any
var sinkValue Value

func init() {
	dm, err := cbor.DecOptions{
		DefaultMapType: reflect.TypeOf(map[string]any{}),
	}.DecMode()
	if err != nil {
		panic(err)
	}
	benchCBORDec = dm

	var obj map[string]any
	if err := json.Unmarshal(benchSampleJSON, &obj); err != nil {
		panic(err)
	}
	benchSampleAny = obj
	benchSampleAnyTRON = internAny(obj, benchStringCache)
	encoded, err := cbor.Marshal(obj)
	if err != nil {
		panic(err)
	}
	benchSampleCBOR = encoded
	tronDoc, err := FromJSON(benchSampleJSON)
	if err != nil {
		panic(err)
	}
	benchSampleTRON = tronDoc
}

func BenchmarkTRONDecodeRead(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleTRON)))
	for i := 0; i < b.N; i++ {
		tr, err := ParseTrailer(benchSampleTRON)
		if err != nil {
			b.Fatal(err)
		}
		features, ok, err := MapGet(benchSampleTRON, tr.RootOffset, keyFeatures)
		if err != nil || !ok {
			b.Fatalf("missing features: %v", err)
		}
		feature0, ok, err := arrGet(benchSampleTRON, features.Offset, 0)
		if err != nil || !ok {
			b.Fatalf("missing feature0: %v", err)
		}
		geom, ok, err := MapGet(benchSampleTRON, feature0.Offset, keyGeometry)
		if err != nil || !ok {
			b.Fatalf("missing geometry: %v", err)
		}
		coords, ok, err := MapGet(benchSampleTRON, geom.Offset, keyCoordinates)
		if err != nil || !ok {
			b.Fatalf("missing coordinates: %v", err)
		}
		first, ok, err := arrGet(benchSampleTRON, coords.Offset, 0)
		if err != nil || !ok {
			b.Fatalf("missing coordinate[0]: %v", err)
		}
		sinkValue = first
	}
}

func BenchmarkTRONDecodeFullClone(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleTRON)))
	for i := 0; i < b.N; i++ {
		out, err := cloneDocument(benchSampleTRON)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkTRONDecodeModifyEncode(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleTRON)))
	for i := 0; i < b.N; i++ {
		out, err := tronUpdateElevation(benchSampleTRON, 1500)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkJSONDecodeRead(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleJSON)))
	for i := 0; i < b.N; i++ {
		var obj map[string]any
		if err := json.Unmarshal(benchSampleJSON, &obj); err != nil {
			b.Fatal(err)
		}
		features := obj["features"].([]any)
		feature0 := features[0].(map[string]any)
		geom := feature0["geometry"].(map[string]any)
		coords := geom["coordinates"].([]any)
		sinkAny = coords[0]
	}
}

func BenchmarkJSONEncodeOnly(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleJSON)))
	for i := 0; i < b.N; i++ {
		out, err := json.Marshal(benchSampleAny)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkJSONDecodeEncode(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleJSON)))
	for i := 0; i < b.N; i++ {
		var obj map[string]any
		if err := json.Unmarshal(benchSampleJSON, &obj); err != nil {
			b.Fatal(err)
		}
		out, err := json.Marshal(obj)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkJSONDecodeModifyEncode(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleJSON)))
	for i := 0; i < b.N; i++ {
		var obj map[string]any
		if err := json.Unmarshal(benchSampleJSON, &obj); err != nil {
			b.Fatal(err)
		}
		features := obj["features"].([]any)
		feature0 := features[0].(map[string]any)
		props := feature0["properties"].(map[string]any)
		props["elevation"] = float64(1500)
		out, err := json.Marshal(obj)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkCBORDecodeRead(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleCBOR)))
	for i := 0; i < b.N; i++ {
		var obj map[string]any
		if err := benchCBORDec.Unmarshal(benchSampleCBOR, &obj); err != nil {
			b.Fatal(err)
		}
		features := obj["features"].([]any)
		feature0 := features[0].(map[string]any)
		geom := feature0["geometry"].(map[string]any)
		coords := geom["coordinates"].([]any)
		sinkAny = coords[0]
	}
}

func BenchmarkCBOREncodeOnly(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleCBOR)))
	for i := 0; i < b.N; i++ {
		out, err := cbor.Marshal(benchSampleAny)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkCBORDecodeEncode(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleCBOR)))
	for i := 0; i < b.N; i++ {
		var obj map[string]any
		if err := benchCBORDec.Unmarshal(benchSampleCBOR, &obj); err != nil {
			b.Fatal(err)
		}
		out, err := cbor.Marshal(obj)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkCBORDecodeModifyEncode(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleCBOR)))
	for i := 0; i < b.N; i++ {
		var obj map[string]any
		if err := benchCBORDec.Unmarshal(benchSampleCBOR, &obj); err != nil {
			b.Fatal(err)
		}
		features := obj["features"].([]any)
		feature0 := features[0].(map[string]any)
		props := feature0["properties"].(map[string]any)
		props["elevation"] = int64(1500)
		out, err := cbor.Marshal(obj)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkTRONEncodeOnly(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSampleTRON)))
	builderPool := sync.Pool{
		New: func() any {
			return NewBuilderWithCapacity(len(benchSampleTRON))
		},
	}
	workspace := newEncodeWorkspace()
	for i := 0; i < b.N; i++ {
		builder := builderPool.Get().(*Builder)
		builder.Reset()
		val, err := valueFromAny(benchSampleAnyTRON, builder, benchStringCache, workspace)
		if err != nil {
			b.Fatal(err)
		}
		var out []byte
		switch val.Type {
		case TypeArr, TypeMap:
			out = builder.BytesWithTrailerInPlace(val.Offset, 0)
		default:
			out, err = EncodeScalarDocument(val)
			if err != nil {
				b.Fatal(err)
			}
		}
		sinkBytes = out
		builderPool.Put(builder)
	}
}

func cloneDocument(doc []byte) ([]byte, error) {
	docType, err := DetectDocType(doc)
	if err != nil {
		return nil, err
	}
	switch docType {
	case DocScalar:
		val, err := DecodeScalarDocument(doc)
		if err != nil {
			return nil, err
		}
		return EncodeScalarDocument(val)
	case DocTree:
		tr, err := ParseTrailer(doc)
		if err != nil {
			return nil, err
		}
		h, _, err := NodeSliceAt(doc, tr.RootOffset)
		if err != nil {
			return nil, err
		}
		builder := NewBuilder()
		switch h.KeyType {
		case KeyMap:
			root, err := cloneMapNode(doc, tr.RootOffset, builder)
			if err != nil {
				return nil, err
			}
			return builder.BytesWithTrailer(root, 0), nil
		case KeyArr:
			root, err := cloneArrayNode(doc, tr.RootOffset, builder)
			if err != nil {
				return nil, err
			}
			return builder.BytesWithTrailer(root, 0), nil
		default:
			return nil, fmt.Errorf("unknown root key type")
		}
	default:
		return nil, fmt.Errorf("unknown document type")
	}
}

func tronUpdateElevation(doc []byte, elevation int64) ([]byte, error) {
	tr, err := ParseTrailer(doc)
	if err != nil {
		return nil, err
	}
	features, ok, err := MapGet(doc, tr.RootOffset, keyFeatures)
	if err != nil || !ok || features.Type != TypeArr {
		return nil, fmt.Errorf("missing features array")
	}
	feature0, ok, err := arrGet(doc, features.Offset, 0)
	if err != nil || !ok || feature0.Type != TypeMap {
		return nil, fmt.Errorf("missing feature0 map")
	}
	props, ok, err := MapGet(doc, feature0.Offset, keyProperties)
	if err != nil || !ok || props.Type != TypeMap {
		return nil, fmt.Errorf("missing properties map")
	}
	builder := &Builder{buf: append([]byte{}, doc[:len(doc)-TrailerSize]...)}
	newProps, _, err := mapSet(builder.buf, props.Offset, keyElevation, Value{Type: TypeI64, I64: elevation}, 0, builder)
	if err != nil {
		return nil, err
	}
	newFeature0, _, err := mapSet(builder.buf, feature0.Offset, keyProperties, Value{Type: TypeMap, Offset: newProps}, 0, builder)
	if err != nil {
		return nil, err
	}
	featuresLen, err := arrayRootLength(builder.buf, features.Offset)
	if err != nil {
		return nil, err
	}
	newFeatures, err := arrSet(builder, features.Offset, 0, Value{Type: TypeMap, Offset: newFeature0}, featuresLen)
	if err != nil {
		return nil, err
	}
	newRoot, _, err := mapSet(builder.buf, tr.RootOffset, keyFeatures, Value{Type: TypeArr, Offset: newFeatures}, 0, builder)
	if err != nil {
		return nil, err
	}
	return builder.BytesWithTrailer(newRoot, tr.RootOffset), nil
}

type stringCache struct {
	txt  map[string][]byte
	bin  map[string][]byte
	hash map[string]uint32
}

type mapAnyEntry struct {
	key   []byte
	hash  uint32
	value any
}

type mapAny []mapAnyEntry

func newStringCache() *stringCache {
	return &stringCache{
		txt:  make(map[string][]byte),
		bin:  make(map[string][]byte),
		hash: make(map[string]uint32),
	}
}

func (c *stringCache) bytesForText(s string) []byte {
	if b, ok := c.txt[s]; ok {
		return b
	}
	b := []byte(s)
	c.txt[s] = b
	return b
}

func (c *stringCache) bytesAndHashForText(s string) ([]byte, uint32) {
	if b, ok := c.txt[s]; ok {
		if h, ok := c.hash[s]; ok {
			return b, h
		}
		h := XXH32(b, 0)
		c.hash[s] = h
		return b, h
	}
	b := []byte(s)
	h := XXH32(b, 0)
	c.txt[s] = b
	c.hash[s] = h
	return b, h
}

func (c *stringCache) bytesForBin(s string) ([]byte, bool) {
	if b, ok := c.bin[s]; ok {
		return b, true
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	c.bin[s] = decoded
	return decoded, true
}

type textBytes []byte

func internAny(v any, cache *stringCache) any {
	switch val := v.(type) {
	case nil:
		return nil
	case bool:
		return val
	case string:
		if len(val) >= 4 && val[0] == 'b' && val[1] == '6' && val[2] == '4' && val[3] == ':' {
			rest := val[4:]
			if cache != nil {
				if decoded, ok := cache.bytesForBin(rest); ok {
					return decoded
				}
			} else if decoded, err := base64.StdEncoding.DecodeString(rest); err == nil {
				return decoded
			}
		}
		if cache != nil {
			return textBytes(cache.bytesForText(val))
		}
		return textBytes([]byte(val))
	case []byte:
		return val
	case float64:
		return val
	case json.Number:
		return val
	case int:
		return val
	case int64:
		return val
	case uint64:
		return val
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = internAny(elem, cache)
		}
		return out
	case map[string]any:
		out := make(mapAny, 0, len(val))
		for k, elem := range val {
			var keyBytes []byte
			var keyHash uint32
			if cache != nil {
				keyBytes, keyHash = cache.bytesAndHashForText(k)
			} else {
				keyBytes = []byte(k)
				keyHash = XXH32(keyBytes, 0)
			}
			out = append(out, mapAnyEntry{
				key:   keyBytes,
				hash:  keyHash,
				value: internAny(elem, cache),
			})
		}
		return out
	default:
		return val
	}
}

func valueFromAny(v any, builder *Builder, cache *stringCache, workspace *encodeWorkspace) (Value, error) {
	switch val := v.(type) {
	case nil:
		return Value{Type: TypeNil}, nil
	case bool:
		return Value{Type: TypeBit, Bool: val}, nil
	case string:
		if len(val) >= 4 && val[0] == 'b' && val[1] == '6' && val[2] == '4' && val[3] == ':' {
			rest := val[4:]
			if cache != nil {
				if decoded, ok := cache.bytesForBin(rest); ok {
					return Value{Type: TypeBin, Bytes: decoded}, nil
				}
			} else if decoded, err := base64.StdEncoding.DecodeString(rest); err == nil {
				return Value{Type: TypeBin, Bytes: decoded}, nil
			}
		}
		if cache != nil {
			return Value{Type: TypeTxt, Bytes: cache.bytesForText(val)}, nil
		}
		return Value{Type: TypeTxt, Bytes: []byte(val)}, nil
	case textBytes:
		return Value{Type: TypeTxt, Bytes: []byte(val)}, nil
	case []byte:
		return Value{Type: TypeBin, Bytes: val}, nil
	case float64:
		if val >= math.MinInt64 && val <= math.MaxInt64 {
			if math.Trunc(val) == val {
				return Value{Type: TypeI64, I64: int64(val)}, nil
			}
		}
		return Value{Type: TypeF64, F64: val}, nil
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return Value{Type: TypeI64, I64: i}, nil
		}
		if f, err := val.Float64(); err == nil {
			return Value{Type: TypeF64, F64: f}, nil
		}
		return Value{}, fmt.Errorf("invalid json number: %s", val)
	case int:
		return Value{Type: TypeI64, I64: int64(val)}, nil
	case int64:
		return Value{Type: TypeI64, I64: val}, nil
	case uint64:
		if val > math.MaxInt64 {
			return Value{Type: TypeF64, F64: float64(val)}, nil
		}
		return Value{Type: TypeI64, I64: int64(val)}, nil
	case []any:
		if len(val) == 0 {
			leaf := ArrayLeafNode{
				Header: NodeHeader{Kind: NodeLeaf, KeyType: KeyArr},
				Shift:  0,
				Bitmap: 0,
				Length: 0,
				Values: nil,
			}
			off, err := appendArrayLeafNode(builder, leaf)
			if err != nil {
				return Value{}, err
			}
			return Value{Type: TypeArr, Offset: off}, nil
		}
		entries := getArrayEntrySliceWithWorkspace(len(val), workspace)
		for i, elem := range val {
			child, err := valueFromAny(elem, builder, cache, workspace)
			if err != nil {
				putArrayEntrySliceWithWorkspace(entries, workspace)
				return Value{}, err
			}
			entries[i] = arrayEntry{index: uint32(i), value: child}
		}
		length := uint32(len(val))
		shift := arrayRootShift(length)
		root, err := buildArrayNode(entries, shift, length, workspace)
		putArrayEntrySliceWithWorkspace(entries, workspace)
		if err != nil {
			return Value{}, err
		}
		off, err := encodeArrayNode(builder, root, workspace)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeArr, Offset: off}, nil
	case map[string]any:
		if len(val) == 0 {
			off, err := appendMapLeafNodeSorted(builder, nil)
			if err != nil {
				return Value{}, err
			}
			return Value{Type: TypeMap, Offset: off}, nil
		}
		entries := getMapEntrySliceWithWorkspace(len(val), workspace)
		i := 0
		for k, elem := range val {
			child, err := valueFromAny(elem, builder, cache, workspace)
			if err != nil {
				putMapEntrySliceWithWorkspace(entries, workspace)
				return Value{}, err
			}
			var keyBytes []byte
			var keyHash uint32
			if cache != nil {
				keyBytes, keyHash = cache.bytesAndHashForText(k)
			} else {
				keyBytes = []byte(k)
				keyHash = XXH32(keyBytes, 0)
			}
			entries[i] = mapEntry{Key: keyBytes, Value: child, Hash: keyHash}
			i++
		}
		root := buildMapNode(entries, 0, true, workspace)
		off, err := encodeMapNode(builder, root, workspace)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeMap, Offset: off}, nil
	case mapAny:
		if len(val) == 0 {
			off, err := appendMapLeafNodeSorted(builder, nil)
			if err != nil {
				return Value{}, err
			}
			return Value{Type: TypeMap, Offset: off}, nil
		}
		entries := getMapEntrySliceWithWorkspace(len(val), workspace)
		for i, entry := range val {
			child, err := valueFromAny(entry.value, builder, cache, workspace)
			if err != nil {
				putMapEntrySliceWithWorkspace(entries, workspace)
				return Value{}, err
			}
			entries[i] = mapEntry{Key: entry.key, Value: child, Hash: entry.hash}
		}
		root := buildMapNode(entries, 0, true, workspace)
		off, err := encodeMapNode(builder, root, workspace)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeMap, Offset: off}, nil
	default:
		return Value{}, fmt.Errorf("unsupported type %T", v)
	}
}
