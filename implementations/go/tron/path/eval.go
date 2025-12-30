package path

import (
	"errors"
	"fmt"

	tron "tron"
)

// Search compiles and evaluates a JMESPath expression against a TRON document.
// The returned value is backed by the provided document when possible.
// Computed arrays or objects return an error.
func Search(expression string, doc []byte) (tron.Value, error) {
	expr, err := Compile(expression)
	if err != nil {
		return tron.Value{}, err
	}
	return expr.Search(doc)
}

// Search evaluates a compiled expression against a TRON document.
// The returned value is backed by the provided document when possible.
// Computed arrays or objects return an error.
func (e *Expr) Search(doc []byte) (tron.Value, error) {
	root, _, err := rootValue(doc)
	if err != nil {
		return tron.Value{}, err
	}
	intr := interpreter{}
	out, err := intr.eval(e.root, root)
	if err != nil {
		return tron.Value{}, err
	}
	if out.kind == kindExpRef {
		return tron.Value{}, fmt.Errorf("expref cannot be returned as a value")
	}
	return out.toTRONValue()
}

type interpreter struct {
	funcs functionCaller
}

func (i *interpreter) eval(node *node, current jValue) (jValue, error) {
	switch node.typ {
	case astEmpty:
		return nullValue(), nil
	case astCurrentNode, astIdentity:
		return current, nil
	case astLiteral:
		return valueFromLiteral(node.value), nil
	case astKeyValPair:
		if len(node.children) != 1 {
			return nullValue(), fmt.Errorf("key/value pair expects 1 child")
		}
		return i.eval(node.children[0], current)
	case astExpRef:
		if len(node.children) != 1 {
			return nullValue(), fmt.Errorf("expref missing child")
		}
		return jValue{kind: kindExpRef, ref: node.children[0]}, nil
	case astField:
		return evalFieldValue(current, node.value.(fieldValue))
	case astIndex:
		index := node.value.(int)
		return indexValue(current, index)
	case astSlice:
		return sliceValue(current, node.value)
	case astIndexExpression, astSubexpression:
		if len(node.children) != 2 {
			return nullValue(), fmt.Errorf("subexpression expects 2 children")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		if left.isNull() {
			return nullValue(), nil
		}
		if node.children[1].typ == astField {
			return evalFieldValue(left, node.children[1].value.(fieldValue))
		}
		return i.eval(node.children[1], left)
	case astPipe:
		result := current
		var err error
		for _, child := range node.children {
			result, err = i.eval(child, result)
			if err != nil {
				return nullValue(), err
			}
		}
		return result, nil
	case astOrExpression:
		if len(node.children) != 2 {
			return nullValue(), fmt.Errorf("or expression expects 2 children")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		if isFalse(left) {
			return i.eval(node.children[1], current)
		}
		return left, nil
	case astAndExpression:
		if len(node.children) != 2 {
			return nullValue(), fmt.Errorf("and expression expects 2 children")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		if isFalse(left) {
			return left, nil
		}
		return i.eval(node.children[1], current)
	case astNotExpression:
		if len(node.children) != 1 {
			return nullValue(), fmt.Errorf("not expression expects 1 child")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		return jValue{kind: kindBool, b: isFalse(left)}, nil
	case astComparator:
		if len(node.children) != 2 {
			return nullValue(), fmt.Errorf("comparator expects 2 children")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		right, err := i.eval(node.children[1], current)
		if err != nil {
			return nullValue(), err
		}
		return compareValues(node.value.(tokType), left, right), nil
	case astMultiSelectList:
		if current.isNull() {
			return nullValue(), nil
		}
		out := make([]jValue, 0, len(node.children))
		for _, child := range node.children {
			val, err := i.eval(child, current)
			if err != nil {
				return nullValue(), err
			}
			out = append(out, val)
		}
		return jValue{kind: kindArray, arr: out}, nil
	case astMultiSelectHash:
		if current.isNull() {
			return nullValue(), nil
		}
		out := make(map[string]jValue, len(node.children))
		for _, child := range node.children {
			key := child.value.(string)
			val, err := i.eval(child.children[0], current)
			if err != nil {
				return nullValue(), err
			}
			out[key] = val
		}
		return jValue{kind: kindObject, obj: out}, nil
	case astProjection:
		if len(node.children) != 2 {
			return nullValue(), fmt.Errorf("projection expects 2 children")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		items, err := arrayValues(left)
		if err != nil {
			return nullValue(), nil
		}
		collected := make([]jValue, 0, len(items))
		for _, item := range items {
			val, err := i.eval(node.children[1], item)
			if err != nil {
				return nullValue(), err
			}
			if !val.isNull() {
				collected = append(collected, val)
			}
		}
		return jValue{kind: kindArray, arr: collected}, nil
	case astFilterProjection:
		if len(node.children) != 3 {
			return nullValue(), fmt.Errorf("filter projection expects 3 children")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		items, err := arrayValues(left)
		if err != nil {
			return nullValue(), nil
		}
		collected := make([]jValue, 0, len(items))
		for _, item := range items {
			passed, err := i.eval(node.children[2], item)
			if err != nil {
				return nullValue(), err
			}
			if !isFalse(passed) {
				val, err := i.eval(node.children[1], item)
				if err != nil {
					return nullValue(), err
				}
				if !val.isNull() {
					collected = append(collected, val)
				}
			}
		}
		return jValue{kind: kindArray, arr: collected}, nil
	case astValueProjection:
		if len(node.children) != 2 {
			return nullValue(), fmt.Errorf("value projection expects 2 children")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		values, err := objectValues(left)
		if err != nil {
			return nullValue(), nil
		}
		collected := make([]jValue, 0, len(values))
		for _, item := range values {
			val, err := i.eval(node.children[1], item)
			if err != nil {
				return nullValue(), err
			}
			if !val.isNull() {
				collected = append(collected, val)
			}
		}
		return jValue{kind: kindArray, arr: collected}, nil
	case astFlatten:
		if len(node.children) != 1 {
			return nullValue(), fmt.Errorf("flatten expects 1 child")
		}
		left, err := i.eval(node.children[0], current)
		if err != nil {
			return nullValue(), err
		}
		items, err := arrayValues(left)
		if err != nil {
			return nullValue(), nil
		}
		flattened := make([]jValue, 0, len(items))
		for _, item := range items {
			sub, err := arrayValues(item)
			if err == nil {
				flattened = append(flattened, sub...)
				continue
			}
			flattened = append(flattened, item)
		}
		return jValue{kind: kindArray, arr: flattened}, nil
	case astFunctionExpression:
		args := make([]jValue, 0, len(node.children))
		for _, child := range node.children {
			val, err := i.eval(child, current)
			if err != nil {
				return nullValue(), err
			}
			args = append(args, val)
		}
		return i.funcs.CallFunction(node.value.(string), args, i)
	default:
		return nullValue(), fmt.Errorf("unsupported AST node %v", node.typ)
	}
}

func rootValue(doc []byte) (jValue, tron.DocType, error) {
	docType, err := tron.DetectDocType(doc)
	if err != nil {
		return nullValue(), tron.DocUnknown, err
	}
	switch docType {
	case tron.DocScalar:
		val, err := tron.DecodeScalarDocument(doc)
		if err != nil {
			return nullValue(), docType, err
		}
		return valueFromTRON(doc, val), docType, nil
	case tron.DocTree:
		tr, err := tron.ParseTrailer(doc)
		if err != nil {
			return nullValue(), docType, err
		}
		h, _, err := tron.NodeSliceAt(doc, tr.RootOffset)
		if err != nil {
			return nullValue(), docType, err
		}
		switch h.KeyType {
		case tron.KeyMap:
			return jValue{
				kind: kindTRONMap,
				doc:  doc,
				off:  tr.RootOffset,
				tv:   tron.Value{Type: tron.TypeMap, Offset: tr.RootOffset},
				tvOK: true,
			}, docType, nil
		case tron.KeyArr:
			return jValue{
				kind: kindTRONArr,
				doc:  doc,
				off:  tr.RootOffset,
				tv:   tron.Value{Type: tron.TypeArr, Offset: tr.RootOffset},
				tvOK: true,
			}, docType, nil
		default:
			return nullValue(), docType, fmt.Errorf("unknown root node type")
		}
	default:
		return nullValue(), docType, fmt.Errorf("unknown document type")
	}
}

func indexValue(current jValue, index int) (jValue, error) {
	items, err := arrayValues(current)
	if err != nil {
		return nullValue(), nil
	}
	if index < 0 {
		index += len(items)
	}
	if index < 0 || index >= len(items) {
		return nullValue(), nil
	}
	return items[index], nil
}

func sliceValue(current jValue, raw any) (jValue, error) {
	items, err := arrayValues(current)
	if err != nil {
		return nullValue(), nil
	}
	parts := raw.([]*int)
	params := make([]sliceParam, 3)
	for i, part := range parts {
		if part != nil {
			params[i].Specified = true
			params[i].N = *part
		}
	}
	result, err := slice(items, params)
	if err != nil {
		return nullValue(), err
	}
	return jValue{kind: kindArray, arr: result}, nil
}

const arrayLinearThreshold = 16

func arrayValues(v jValue) ([]jValue, error) {
	switch v.kind {
	case kindArray:
		return v.arr, nil
	case kindTRONArr:
		length, err := arrayLength(v.doc, v.off)
		if err != nil {
			return nil, err
		}
		if length <= arrayLinearThreshold {
			out := make([]jValue, length)
			for i := 0; i < int(length); i++ {
				val, ok, err := arrGetRaw(v.doc, v.off, uint32(i))
				if err != nil {
					return nil, err
				}
				if !ok {
					out[i] = nullValue()
					continue
				}
				out[i] = valueFromTRON(v.doc, val)
			}
			return out, nil
		}
		values := getValueSlice(int(length))
		present := getBoolSlice(int(length))
		defer putValueSlice(values)
		defer putBoolSlice(present)
		if err := arrCollectValues(v.doc, v.off, 0, values, present); err != nil {
			return nil, err
		}
		out := make([]jValue, length)
		for i := range values {
			if !present[i] {
				out[i] = nullValue()
				continue
			}
			out[i] = valueFromTRON(v.doc, values[i])
		}
		return out, nil
	default:
		return nil, fmt.Errorf("not an array")
	}
}

func objectValues(v jValue) ([]jValue, error) {
	switch v.kind {
	case kindObject:
		out := make([]jValue, 0, len(v.obj))
		for _, val := range v.obj {
			out = append(out, val)
		}
		return out, nil
	case kindTRONMap:
		out := make([]jValue, 0, 8)
		if err := mapIterValues(v.doc, v.off, func(val tron.Value) error {
			out = append(out, valueFromTRON(v.doc, val))
			return nil
		}); err != nil {
			return nil, err
		}
		return out, nil
	default:
		return nil, fmt.Errorf("not an object")
	}
}

func evalFieldValue(current jValue, fv fieldValue) (jValue, error) {
	switch current.kind {
	case kindObject:
		if val, ok := current.obj[fv.key]; ok {
			return val, nil
		}
		return nullValue(), nil
	case kindTRONMap:
		val, ok, err := mapGetBytesHashed(current.doc, current.off, fv.keyBytes, fv.hash, 0)
		if err != nil {
			return nullValue(), err
		}
		if !ok {
			return nullValue(), nil
		}
		return valueFromTRON(current.doc, val), nil
	default:
		return nullValue(), nil
	}
}

func isFalse(v jValue) bool {
	switch v.kind {
	case kindNull:
		return true
	case kindBool:
		return !v.b
	case kindString:
		return len(v.s) == 0
	case kindArray:
		return len(v.arr) == 0
	case kindObject:
		return len(v.obj) == 0
	case kindTRONArr:
		length, err := arrayLength(v.doc, v.off)
		return err == nil && length == 0
	case kindTRONMap:
		return mapIsEmpty(v.doc, v.off)
	default:
		return false
	}
}

func mapIsEmpty(doc []byte, off uint32) bool {
	h, _, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return false
	}
	if h.KeyType != tron.KeyMap {
		return false
	}
	if h.Kind == tron.NodeLeaf {
		return h.EntryCount == 0
	}
	if h.Kind == tron.NodeBranch {
		return h.EntryCount == 0
	}
	return false
}

func compareValues(op tokType, left, right jValue) jValue {
	switch op {
	case tEQ:
		return jValue{kind: kindBool, b: valuesEqual(left, right)}
	case tNE:
		return jValue{kind: kindBool, b: !valuesEqual(left, right)}
	default:
		ln, lok := numberValue(left)
		rn, rok := numberValue(right)
		if !lok || !rok {
			return nullValue()
		}
		switch op {
		case tGT:
			return jValue{kind: kindBool, b: ln > rn}
		case tGTE:
			return jValue{kind: kindBool, b: ln >= rn}
		case tLT:
			return jValue{kind: kindBool, b: ln < rn}
		case tLTE:
			return jValue{kind: kindBool, b: ln <= rn}
		}
	}
	return nullValue()
}

func valuesEqual(a, b jValue) bool {
	if a.kind == b.kind {
		switch a.kind {
		case kindNull:
			return true
		case kindBool:
			return a.b == b.b
		case kindNumber:
			return a.n == b.n
		case kindString:
			return a.s == b.s
		case kindArray:
			return arrayValuesEqual(a.arr, b.arr)
		case kindObject:
			return objectMapEqual(a.obj, b.obj)
		case kindTRONArr:
			return arraysEqual(a, b)
		case kindTRONMap:
			return tronMapsEqual(a, b)
		case kindExpRef:
			return false
		default:
			return false
		}
	}
	if isArrayKind(a) && isArrayKind(b) {
		return arraysEqual(a, b)
	}
	if isObjectKind(a) && isObjectKind(b) {
		return objectsEqual(a, b)
	}
	return false
}

func numberValue(v jValue) (float64, bool) {
	if v.kind == kindNumber {
		return v.n, true
	}
	return 0, false
}

func isArrayKind(v jValue) bool {
	return v.kind == kindArray || v.kind == kindTRONArr
}

func isObjectKind(v jValue) bool {
	return v.kind == kindObject || v.kind == kindTRONMap
}

func arrayValuesEqual(a, b []jValue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !valuesEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func objectMapEqual(a, b map[string]jValue) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !valuesEqual(av, bv) {
			return false
		}
	}
	return true
}

func arraysEqual(a, b jValue) bool {
	alen, err := arrayLen(a)
	if err != nil {
		return false
	}
	blen, err := arrayLen(b)
	if err != nil {
		return false
	}
	if alen != blen {
		return false
	}
	for i := 0; i < alen; i++ {
		av, err := arrayElem(a, i)
		if err != nil {
			return false
		}
		bv, err := arrayElem(b, i)
		if err != nil {
			return false
		}
		if !valuesEqual(av, bv) {
			return false
		}
	}
	return true
}

func arrayLen(v jValue) (int, error) {
	switch v.kind {
	case kindArray:
		return len(v.arr), nil
	case kindTRONArr:
		length, err := arrayLength(v.doc, v.off)
		if err != nil {
			return 0, err
		}
		return int(length), nil
	default:
		return 0, fmt.Errorf("not an array")
	}
}

func arrayElem(v jValue, index int) (jValue, error) {
	switch v.kind {
	case kindArray:
		return v.arr[index], nil
	case kindTRONArr:
		val, ok, err := arrGetRaw(v.doc, v.off, uint32(index))
		if err != nil {
			return nullValue(), err
		}
		if !ok {
			return nullValue(), nil
		}
		return valueFromTRON(v.doc, val), nil
	default:
		return nullValue(), fmt.Errorf("not an array")
	}
}

func objectsEqual(a, b jValue) bool {
	switch {
	case a.kind == kindObject && b.kind == kindObject:
		return objectMapEqual(a.obj, b.obj)
	case a.kind == kindTRONMap && b.kind == kindTRONMap:
		return tronMapsEqual(a, b)
	case a.kind == kindTRONMap && b.kind == kindObject:
		return tronMapEqualsObject(a, b.obj)
	case a.kind == kindObject && b.kind == kindTRONMap:
		return tronMapEqualsObject(b, a.obj)
	default:
		return false
	}
}

var errMapMismatch = errors.New("map mismatch")

func tronMapEqualsObject(v jValue, obj map[string]jValue) bool {
	count := 0
	err := mapIterEntries(v.doc, v.off, func(key []byte, val tron.Value) error {
		count++
		ov, ok := obj[string(key)]
		if !ok {
			return errMapMismatch
		}
		if !valuesEqual(valueFromTRON(v.doc, val), ov) {
			return errMapMismatch
		}
		return nil
	})
	if err == errMapMismatch {
		return false
	}
	if err != nil {
		return false
	}
	return count == len(obj)
}

func tronMapsEqual(a, b jValue) bool {
	count := 0
	err := mapIterEntries(a.doc, a.off, func(key []byte, val tron.Value) error {
		count++
		hash := tron.XXH32(key, 0)
		otherVal, ok, err := mapGetBytesHashed(b.doc, b.off, key, hash, 0)
		if err != nil {
			return err
		}
		if !ok {
			return errMapMismatch
		}
		if !valuesEqual(valueFromTRON(a.doc, val), valueFromTRON(b.doc, otherVal)) {
			return errMapMismatch
		}
		return nil
	})
	if err == errMapMismatch {
		return false
	}
	if err != nil {
		return false
	}
	otherCount, err := mapEntryCount(b.doc, b.off)
	if err != nil {
		return false
	}
	return count == otherCount
}

func mapEntryCount(doc []byte, off uint32) (int, error) {
	count := 0
	err := mapIterEntries(doc, off, func(key []byte, val tron.Value) error {
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

type sliceParam struct {
	N         int
	Specified bool
}

func slice(items []jValue, parts []sliceParam) ([]jValue, error) {
	computed, err := computeSliceParams(len(items), parts)
	if err != nil {
		return nil, err
	}
	start, stop, step := computed[0], computed[1], computed[2]
	out := []jValue{}
	if step > 0 {
		for i := start; i < stop; i += step {
			out = append(out, items[i])
		}
	} else {
		for i := start; i > stop; i += step {
			out = append(out, items[i])
		}
	}
	return out, nil
}

func computeSliceParams(length int, parts []sliceParam) ([]int, error) {
	var start, stop, step int
	if !parts[2].Specified {
		step = 1
	} else if parts[2].N == 0 {
		return nil, fmt.Errorf("invalid slice, step cannot be 0")
	} else {
		step = parts[2].N
	}
	stepNeg := step < 0
	if !parts[0].Specified {
		if stepNeg {
			start = length - 1
		} else {
			start = 0
		}
	} else {
		start = capSlice(length, parts[0].N, step)
	}
	if !parts[1].Specified {
		if stepNeg {
			stop = -1
		} else {
			stop = length
		}
	} else {
		stop = capSlice(length, parts[1].N, step)
	}
	return []int{start, stop, step}, nil
}

func capSlice(length int, actual int, step int) int {
	if actual < 0 {
		actual += length
		if actual < 0 {
			if step < 0 {
				actual = -1
			} else {
				actual = 0
			}
		}
	} else if actual >= length {
		if step < 0 {
			actual = length - 1
		} else {
			actual = length
		}
	}
	return actual
}
