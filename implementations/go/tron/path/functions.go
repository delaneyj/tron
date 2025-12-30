package path

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	tron "tron"
)

type jpType string

const (
	jpUnknown     jpType = "unknown"
	jpNumber      jpType = "number"
	jpString      jpType = "string"
	jpArray       jpType = "array"
	jpObject      jpType = "object"
	jpArrayNumber jpType = "array[number]"
	jpArrayString jpType = "array[string]"
	jpExpRef      jpType = "expref"
	jpAny         jpType = "any"
)

type functionEntry struct {
	name      string
	arguments []argSpec
	handler   func([]jValue, *interpreter) (jValue, error)
}

type argSpec struct {
	types    []jpType
	variadic bool
}

type functionCaller struct {
	functionTable map[string]functionEntry
}

var (
	defaultFunctionTable     map[string]functionEntry
	defaultFunctionTableOnce sync.Once
)

func (f *functionCaller) CallFunction(name string, arguments []jValue, intr *interpreter) (jValue, error) {
	if f.functionTable == nil {
		defaultFunctionTableOnce.Do(func() {
			defaultFunctionTable = newFunctionCaller().functionTable
		})
		f.functionTable = defaultFunctionTable
	}
	entry, ok := f.functionTable[name]
	if !ok {
		return nullValue(), errors.New("unknown function: " + name)
	}
	resolved, err := entry.resolveArgs(arguments)
	if err != nil {
		return nullValue(), err
	}
	return entry.handler(resolved, intr)
}

func newFunctionCaller() *functionCaller {
	return &functionCaller{
		functionTable: map[string]functionEntry{
			"length": {
				name: "length",
				arguments: []argSpec{
					{types: []jpType{jpString, jpArray, jpObject}},
				},
				handler: jpfLength,
			},
			"starts_with": {
				name: "starts_with",
				arguments: []argSpec{
					{types: []jpType{jpString}},
					{types: []jpType{jpString}},
				},
				handler: jpfStartsWith,
			},
			"abs": {
				name: "abs",
				arguments: []argSpec{
					{types: []jpType{jpNumber}},
				},
				handler: jpfAbs,
			},
			"avg": {
				name: "avg",
				arguments: []argSpec{
					{types: []jpType{jpArrayNumber}},
				},
				handler: jpfAvg,
			},
			"ceil": {
				name: "ceil",
				arguments: []argSpec{
					{types: []jpType{jpNumber}},
				},
				handler: jpfCeil,
			},
			"contains": {
				name: "contains",
				arguments: []argSpec{
					{types: []jpType{jpArray, jpString}},
					{types: []jpType{jpAny}},
				},
				handler: jpfContains,
			},
			"ends_with": {
				name: "ends_with",
				arguments: []argSpec{
					{types: []jpType{jpString}},
					{types: []jpType{jpString}},
				},
				handler: jpfEndsWith,
			},
			"floor": {
				name: "floor",
				arguments: []argSpec{
					{types: []jpType{jpNumber}},
				},
				handler: jpfFloor,
			},
			"map": {
				name: "map",
				arguments: []argSpec{
					{types: []jpType{jpExpRef}},
					{types: []jpType{jpArray}},
				},
				handler: jpfMap,
			},
			"max": {
				name: "max",
				arguments: []argSpec{
					{types: []jpType{jpArrayNumber, jpArrayString}},
				},
				handler: jpfMax,
			},
			"merge": {
				name: "merge",
				arguments: []argSpec{
					{types: []jpType{jpObject}, variadic: true},
				},
				handler: jpfMerge,
			},
			"max_by": {
				name: "max_by",
				arguments: []argSpec{
					{types: []jpType{jpArray}},
					{types: []jpType{jpExpRef}},
				},
				handler: jpfMaxBy,
			},
			"sum": {
				name: "sum",
				arguments: []argSpec{
					{types: []jpType{jpArrayNumber}},
				},
				handler: jpfSum,
			},
			"min": {
				name: "min",
				arguments: []argSpec{
					{types: []jpType{jpArrayNumber, jpArrayString}},
				},
				handler: jpfMin,
			},
			"min_by": {
				name: "min_by",
				arguments: []argSpec{
					{types: []jpType{jpArray}},
					{types: []jpType{jpExpRef}},
				},
				handler: jpfMinBy,
			},
			"type": {
				name: "type",
				arguments: []argSpec{
					{types: []jpType{jpAny}},
				},
				handler: jpfType,
			},
			"keys": {
				name: "keys",
				arguments: []argSpec{
					{types: []jpType{jpObject}},
				},
				handler: jpfKeys,
			},
			"values": {
				name: "values",
				arguments: []argSpec{
					{types: []jpType{jpObject}},
				},
				handler: jpfValues,
			},
			"sort": {
				name: "sort",
				arguments: []argSpec{
					{types: []jpType{jpArrayString, jpArrayNumber}},
				},
				handler: jpfSort,
			},
			"sort_by": {
				name: "sort_by",
				arguments: []argSpec{
					{types: []jpType{jpArray}},
					{types: []jpType{jpExpRef}},
				},
				handler: jpfSortBy,
			},
			"join": {
				name: "join",
				arguments: []argSpec{
					{types: []jpType{jpString}},
					{types: []jpType{jpArrayString}},
				},
				handler: jpfJoin,
			},
			"reverse": {
				name: "reverse",
				arguments: []argSpec{
					{types: []jpType{jpArray, jpString}},
				},
				handler: jpfReverse,
			},
			"to_array": {
				name: "to_array",
				arguments: []argSpec{
					{types: []jpType{jpAny}},
				},
				handler: jpfToArray,
			},
			"to_string": {
				name: "to_string",
				arguments: []argSpec{
					{types: []jpType{jpAny}},
				},
				handler: jpfToString,
			},
			"to_number": {
				name: "to_number",
				arguments: []argSpec{
					{types: []jpType{jpAny}},
				},
				handler: jpfToNumber,
			},
			"not_null": {
				name: "not_null",
				arguments: []argSpec{
					{types: []jpType{jpAny}, variadic: true},
				},
				handler: jpfNotNull,
			},
		},
	}
}

func (e *functionEntry) resolveArgs(arguments []jValue) ([]jValue, error) {
	if len(e.arguments) == 0 {
		return arguments, nil
	}
	if !e.arguments[len(e.arguments)-1].variadic {
		if len(e.arguments) != len(arguments) {
			return nil, errors.New("incorrect number of args")
		}
		for i, spec := range e.arguments {
			if err := spec.typeCheck(arguments[i]); err != nil {
				return nil, err
			}
		}
		return arguments, nil
	}
	if len(arguments) < len(e.arguments) {
		return nil, errors.New("invalid arity")
	}
	for i := 0; i < len(e.arguments); i++ {
		if err := e.arguments[i].typeCheck(arguments[i]); err != nil {
			return nil, err
		}
	}
	return arguments, nil
}

func (a *argSpec) typeCheck(arg jValue) error {
	for _, t := range a.types {
		switch t {
		case jpNumber:
			if arg.kind == kindNumber {
				return nil
			}
		case jpString:
			if arg.kind == kindString {
				return nil
			}
		case jpArray:
			if arg.kind == kindArray || arg.kind == kindTRONArr {
				return nil
			}
		case jpObject:
			if arg.kind == kindObject || arg.kind == kindTRONMap {
				return nil
			}
		case jpArrayNumber:
			if isArrayNum(arg) {
				return nil
			}
		case jpArrayString:
			if isArrayStr(arg) {
				return nil
			}
		case jpAny:
			return nil
		case jpExpRef:
			if arg.kind == kindExpRef {
				return nil
			}
		}
	}
	return fmt.Errorf("invalid type for function arg: %v", a.types)
}

func jpfLength(arguments []jValue, _ *interpreter) (jValue, error) {
	arg := arguments[0]
	switch arg.kind {
	case kindString:
		return jValue{kind: kindNumber, n: float64(utf8.RuneCountInString(arg.s))}, nil
	case kindArray:
		return jValue{kind: kindNumber, n: float64(len(arg.arr))}, nil
	case kindObject:
		return jValue{kind: kindNumber, n: float64(len(arg.obj))}, nil
	case kindTRONArr:
		length, err := arrayLength(arg.doc, arg.off)
		if err != nil {
			return nullValue(), err
		}
		return jValue{kind: kindNumber, n: float64(length)}, nil
	case kindTRONMap:
		count := 0
		if err := mapIterEntries(arg.doc, arg.off, func(_ []byte, _ tron.Value) error {
			count++
			return nil
		}); err != nil {
			return nullValue(), err
		}
		return jValue{kind: kindNumber, n: float64(count)}, nil
	default:
		return nullValue(), errors.New("could not compute length()")
	}
}

func jpfStartsWith(arguments []jValue, _ *interpreter) (jValue, error) {
	return jValue{kind: kindBool, b: strings.HasPrefix(arguments[0].s, arguments[1].s)}, nil
}

func jpfAbs(arguments []jValue, _ *interpreter) (jValue, error) {
	return jValue{kind: kindNumber, n: math.Abs(arguments[0].n)}, nil
}

func jpfAvg(arguments []jValue, _ *interpreter) (jValue, error) {
	items, err := arrayValues(arguments[0])
	if err != nil || len(items) == 0 {
		return nullValue(), nil
	}
	sum := 0.0
	for _, item := range items {
		if item.kind != kindNumber {
			return nullValue(), nil
		}
		sum += item.n
	}
	return jValue{kind: kindNumber, n: sum / float64(len(items))}, nil
}

func jpfCeil(arguments []jValue, _ *interpreter) (jValue, error) {
	return jValue{kind: kindNumber, n: math.Ceil(arguments[0].n)}, nil
}

func jpfContains(arguments []jValue, _ *interpreter) (jValue, error) {
	search := arguments[0]
	el := arguments[1]
	if search.kind == kindString {
		if el.kind != kindString {
			return jValue{kind: kindBool, b: false}, nil
		}
		return jValue{kind: kindBool, b: strings.Contains(search.s, el.s)}, nil
	}
	items, err := arrayValues(search)
	if err != nil {
		return jValue{kind: kindBool, b: false}, nil
	}
	for _, item := range items {
		if valuesEqual(item, el) {
			return jValue{kind: kindBool, b: true}, nil
		}
	}
	return jValue{kind: kindBool, b: false}, nil
}

func jpfEndsWith(arguments []jValue, _ *interpreter) (jValue, error) {
	return jValue{kind: kindBool, b: strings.HasSuffix(arguments[0].s, arguments[1].s)}, nil
}

func jpfFloor(arguments []jValue, _ *interpreter) (jValue, error) {
	return jValue{kind: kindNumber, n: math.Floor(arguments[0].n)}, nil
}

func jpfMap(arguments []jValue, intr *interpreter) (jValue, error) {
	exp := arguments[0]
	items, err := arrayValues(arguments[1])
	if err != nil {
		return nullValue(), err
	}
	mapped := make([]jValue, 0, len(items))
	for _, item := range items {
		current, err := intr.eval(exp.ref, item)
		if err != nil {
			return nullValue(), err
		}
		mapped = append(mapped, current)
	}
	return jValue{kind: kindArray, arr: mapped}, nil
}

func jpfMax(arguments []jValue, _ *interpreter) (jValue, error) {
	items, err := arrayValues(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	if len(items) == 0 {
		return nullValue(), nil
	}
	switch items[0].kind {
	case kindNumber:
		best := items[0].n
		for _, item := range items[1:] {
			if item.kind != kindNumber {
				return nullValue(), errors.New("max() expects array of numbers or strings")
			}
			if item.n > best {
				best = item.n
			}
		}
		return jValue{kind: kindNumber, n: best}, nil
	case kindString:
		best := items[0].s
		for _, item := range items[1:] {
			if item.kind != kindString {
				return nullValue(), errors.New("max() expects array of numbers or strings")
			}
			if item.s > best {
				best = item.s
			}
		}
		return jValue{kind: kindString, s: best}, nil
	default:
		return nullValue(), errors.New("max() expects array of numbers or strings")
	}
}

func jpfMerge(arguments []jValue, _ *interpreter) (jValue, error) {
	out := map[string]jValue{}
	for _, arg := range arguments {
		obj, err := objectMap(arg)
		if err != nil {
			return nullValue(), err
		}
		for k, v := range obj {
			out[k] = v
		}
	}
	return jValue{kind: kindObject, obj: out}, nil
}

func jpfMaxBy(arguments []jValue, intr *interpreter) (jValue, error) {
	items, err := arrayValues(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	if len(items) == 0 {
		return nullValue(), nil
	}
	expr := arguments[1].ref
	best := items[0]
	bestKey, err := intr.eval(expr, best)
	if err != nil {
		return nullValue(), err
	}
	if bestKey.kind != kindNumber && bestKey.kind != kindString {
		return nullValue(), errors.New("invalid type, must be number or string")
	}
	for _, item := range items[1:] {
		key, err := intr.eval(expr, item)
		if err != nil {
			return nullValue(), err
		}
		if key.kind != bestKey.kind {
			return nullValue(), errors.New("invalid type, must be number or string")
		}
		if compareSortKey(key, bestKey) > 0 {
			best = item
			bestKey = key
		}
	}
	return best, nil
}

func jpfSum(arguments []jValue, _ *interpreter) (jValue, error) {
	items, err := arrayValues(arguments[0])
	if err != nil {
		return jValue{kind: kindNumber, n: 0}, nil
	}
	total := 0.0
	for _, item := range items {
		if item.kind != kindNumber {
			return jValue{kind: kindNumber, n: 0}, nil
		}
		total += item.n
	}
	return jValue{kind: kindNumber, n: total}, nil
}

func jpfMin(arguments []jValue, _ *interpreter) (jValue, error) {
	items, err := arrayValues(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	if len(items) == 0 {
		return nullValue(), nil
	}
	switch items[0].kind {
	case kindNumber:
		best := items[0].n
		for _, item := range items[1:] {
			if item.kind != kindNumber {
				return nullValue(), errors.New("min() expects array of numbers or strings")
			}
			if item.n < best {
				best = item.n
			}
		}
		return jValue{kind: kindNumber, n: best}, nil
	case kindString:
		best := items[0].s
		for _, item := range items[1:] {
			if item.kind != kindString {
				return nullValue(), errors.New("min() expects array of numbers or strings")
			}
			if item.s < best {
				best = item.s
			}
		}
		return jValue{kind: kindString, s: best}, nil
	default:
		return nullValue(), errors.New("min() expects array of numbers or strings")
	}
}

func jpfMinBy(arguments []jValue, intr *interpreter) (jValue, error) {
	items, err := arrayValues(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	if len(items) == 0 {
		return nullValue(), nil
	}
	expr := arguments[1].ref
	best := items[0]
	bestKey, err := intr.eval(expr, best)
	if err != nil {
		return nullValue(), err
	}
	if bestKey.kind != kindNumber && bestKey.kind != kindString {
		return nullValue(), errors.New("invalid type, must be number or string")
	}
	for _, item := range items[1:] {
		key, err := intr.eval(expr, item)
		if err != nil {
			return nullValue(), err
		}
		if key.kind != bestKey.kind {
			return nullValue(), errors.New("invalid type, must be number or string")
		}
		if compareSortKey(key, bestKey) < 0 {
			best = item
			bestKey = key
		}
	}
	return best, nil
}

func jpfType(arguments []jValue, _ *interpreter) (jValue, error) {
	return jValue{kind: kindString, s: typeName(arguments[0])}, nil
}

func jpfKeys(arguments []jValue, _ *interpreter) (jValue, error) {
	obj, err := objectMap(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	out := make([]jValue, 0, len(obj))
	for k := range obj {
		out = append(out, jValue{kind: kindString, s: k})
	}
	return jValue{kind: kindArray, arr: out}, nil
}

func jpfValues(arguments []jValue, _ *interpreter) (jValue, error) {
	obj, err := objectMap(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	out := make([]jValue, 0, len(obj))
	for _, v := range obj {
		out = append(out, v)
	}
	return jValue{kind: kindArray, arr: out}, nil
}

func jpfSort(arguments []jValue, _ *interpreter) (jValue, error) {
	if nums, ok := toArrayNum(arguments[0]); ok {
		sort.SliceStable(nums, func(i, j int) bool { return nums[i] < nums[j] })
		out := make([]jValue, len(nums))
		for i, n := range nums {
			out[i] = jValue{kind: kindNumber, n: n}
		}
		return jValue{kind: kindArray, arr: out}, nil
	}
	if strs, ok := toArrayStr(arguments[0]); ok {
		sort.SliceStable(strs, func(i, j int) bool { return strs[i] < strs[j] })
		out := make([]jValue, len(strs))
		for i, s := range strs {
			out[i] = jValue{kind: kindString, s: s}
		}
		return jValue{kind: kindArray, arr: out}, nil
	}
	return nullValue(), errors.New("sort() expects array of strings or numbers")
}

func jpfSortBy(arguments []jValue, intr *interpreter) (jValue, error) {
	items, err := arrayValues(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	if len(items) == 0 {
		return jValue{kind: kindArray, arr: items}, nil
	}
	expr := arguments[1].ref
	type keyed struct {
		item jValue
		key  jValue
	}
	keyedItems := make([]keyed, 0, len(items))
	expectedKind := jKind(-1)
	for _, item := range items {
		key, err := intr.eval(expr, item)
		if err != nil {
			return nullValue(), err
		}
		if key.kind != kindNumber && key.kind != kindString {
			return nullValue(), errors.New("invalid type, must be number or string")
		}
		if expectedKind == -1 {
			expectedKind = key.kind
		} else if key.kind != expectedKind {
			return nullValue(), errors.New("invalid type, must be number or string")
		}
		keyedItems = append(keyedItems, keyed{item: item, key: key})
	}
	sort.SliceStable(keyedItems, func(i, j int) bool {
		return compareSortKey(keyedItems[i].key, keyedItems[j].key) < 0
	})
	out := make([]jValue, len(keyedItems))
	for i, item := range keyedItems {
		out[i] = item.item
	}
	return jValue{kind: kindArray, arr: out}, nil
}

func jpfJoin(arguments []jValue, _ *interpreter) (jValue, error) {
	sep := arguments[0].s
	strs, _ := toArrayStr(arguments[1])
	return jValue{kind: kindString, s: strings.Join(strs, sep)}, nil
}

func jpfReverse(arguments []jValue, _ *interpreter) (jValue, error) {
	if arguments[0].kind == kindString {
		r := []rune(arguments[0].s)
		for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return jValue{kind: kindString, s: string(r)}, nil
	}
	items, err := arrayValues(arguments[0])
	if err != nil {
		return nullValue(), err
	}
	length := len(items)
	reversed := make([]jValue, length)
	for i, item := range items {
		reversed[length-(i+1)] = item
	}
	return jValue{kind: kindArray, arr: reversed}, nil
}

func jpfToArray(arguments []jValue, _ *interpreter) (jValue, error) {
	if arguments[0].kind == kindArray || arguments[0].kind == kindTRONArr {
		return arguments[0], nil
	}
	return jValue{kind: kindArray, arr: []jValue{arguments[0]}}, nil
}

func jpfToString(arguments []jValue, _ *interpreter) (jValue, error) {
	if arguments[0].kind == kindString {
		return arguments[0], nil
	}
	str, err := arguments[0].toJSON()
	if err != nil {
		return nullValue(), err
	}
	return jValue{kind: kindString, s: str}, nil
}

func jpfToNumber(arguments []jValue, _ *interpreter) (jValue, error) {
	arg := arguments[0]
	if arg.kind == kindNumber {
		return arg, nil
	}
	if arg.kind == kindString {
		conv, err := strconv.ParseFloat(arg.s, 64)
		if err != nil {
			return nullValue(), nil
		}
		return jValue{kind: kindNumber, n: conv}, nil
	}
	if arg.kind == kindArray || arg.kind == kindObject || arg.kind == kindTRONArr || arg.kind == kindTRONMap {
		return nullValue(), nil
	}
	if arg.kind == kindNull || arg.kind == kindBool {
		return nullValue(), nil
	}
	return nullValue(), errors.New("unknown type")
}

func jpfNotNull(arguments []jValue, _ *interpreter) (jValue, error) {
	for _, arg := range arguments {
		if !arg.isNull() {
			return arg, nil
		}
	}
	return nullValue(), nil
}

func isArrayNum(arg jValue) bool {
	switch arg.kind {
	case kindArray:
		for _, item := range arg.arr {
			if item.kind != kindNumber {
				return false
			}
		}
		return true
	case kindTRONArr:
		length, err := arrayLength(arg.doc, arg.off)
		if err != nil {
			return false
		}
		for i := 0; i < int(length); i++ {
			val, ok, err := arrGetRaw(arg.doc, arg.off, uint32(i))
			if err != nil || !ok {
				return false
			}
			if val.Type != tron.TypeI64 && val.Type != tron.TypeF64 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isArrayStr(arg jValue) bool {
	switch arg.kind {
	case kindArray:
		for _, item := range arg.arr {
			if item.kind != kindString {
				return false
			}
		}
		return true
	case kindTRONArr:
		length, err := arrayLength(arg.doc, arg.off)
		if err != nil {
			return false
		}
		for i := 0; i < int(length); i++ {
			val, ok, err := arrGetRaw(arg.doc, arg.off, uint32(i))
			if err != nil || !ok {
				return false
			}
			if val.Type != tron.TypeTxt && val.Type != tron.TypeBin {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func toArrayNum(arg jValue) ([]float64, bool) {
	items, err := arrayValues(arg)
	if err != nil {
		return nil, false
	}
	out := make([]float64, len(items))
	for i, item := range items {
		if item.kind != kindNumber {
			return nil, false
		}
		out[i] = item.n
	}
	return out, true
}

func toArrayStr(arg jValue) ([]string, bool) {
	items, err := arrayValues(arg)
	if err != nil {
		return nil, false
	}
	out := make([]string, len(items))
	for i, item := range items {
		if item.kind != kindString {
			return nil, false
		}
		out[i] = item.s
	}
	return out, true
}

func objectMap(arg jValue) (map[string]jValue, error) {
	switch arg.kind {
	case kindObject:
		return arg.obj, nil
	case kindTRONMap:
		out := map[string]jValue{}
		if err := mapIterEntries(arg.doc, arg.off, func(key []byte, val tron.Value) error {
			out[string(key)] = valueFromTRON(arg.doc, val)
			return nil
		}); err != nil {
			return nil, err
		}
		return out, nil
	default:
		return nil, fmt.Errorf("not an object")
	}
}

func typeName(arg jValue) string {
	switch arg.kind {
	case kindNull:
		return "null"
	case kindBool:
		return "boolean"
	case kindNumber:
		return "number"
	case kindString:
		return "string"
	case kindArray, kindTRONArr:
		return "array"
	case kindObject, kindTRONMap:
		return "object"
	default:
		return string(jpUnknown)
	}
}

func compareSortKey(a, b jValue) int {
	if a.kind == kindNumber && b.kind == kindNumber {
		if a.n < b.n {
			return -1
		}
		if a.n > b.n {
			return 1
		}
		return 0
	}
	if a.kind == kindString && b.kind == kindString {
		if a.s < b.s {
			return -1
		}
		if a.s > b.s {
			return 1
		}
		return 0
	}
	return 0
}
