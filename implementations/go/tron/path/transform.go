package path

import (
	"fmt"

	tron "tron"
)

type stepKind int

const (
	stepKey stepKind = iota
	stepIndex
)

type pathStep struct {
	kind  stepKind
	key   []byte
	index uint32
}

type match struct {
	path  []pathStep
	value jValue
}

// Transform applies fn to every value matched by the expression and returns a new document.
// Transform only operates on values that directly exist in the TRON document.
func (e *Expr) Transform(doc []byte, fn func(tron.Value) (tron.Value, error)) ([]byte, error) {
	rootVal, docType, _, err := rootTRONValue(doc)
	if err != nil {
		return nil, err
	}
	intr := interpreter{}
	rootMatch := match{path: nil, value: valueFromTRON(doc, rootVal)}
	matches, err := intr.collectMatches(e.root, rootMatch)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return doc, nil
	}

	switch docType {
	case tron.DocScalar:
		if len(matches) != 1 || len(matches[0].path) != 0 {
			return nil, fmt.Errorf("transform expects root scalar match")
		}
		updated, err := fn(rootVal)
		if err != nil {
			return nil, err
		}
		return tron.EncodeScalarDocument(updated)
	case tron.DocTree:
		builder, trailer, err := tron.NewBuilderFromDocument(doc)
		if err != nil {
			return nil, err
		}
		root := rootVal
		for _, m := range matches {
			var err error
			root, err = applyAtPath(builder, root, m.path, fn)
			if err != nil {
				return nil, err
			}
		}
		if root.Type != tron.TypeMap && root.Type != tron.TypeArr {
			return tron.EncodeScalarDocument(root)
		}
		return builder.BytesWithTrailer(root.Offset, trailer.RootOffset), nil
	default:
		return nil, fmt.Errorf("unknown document type")
	}
}

func (i *interpreter) collectMatches(node *node, cur match) ([]match, error) {
	switch node.typ {
	case astCurrentNode, astIdentity:
		return []match{cur}, nil
	case astField:
		if cur.value.kind != kindTRONMap {
			return nil, fmt.Errorf("transform requires map traversal")
		}
		fv := node.value.(fieldValue)
		val, ok, err := mapGetBytesHashed(cur.value.doc, cur.value.off, fv.keyBytes, fv.hash, 0)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		next := match{
			path:  appendStep(cur.path, pathStep{kind: stepKey, key: fv.keyBytes}),
			value: valueFromTRON(cur.value.doc, val),
		}
		return []match{next}, nil
	case astIndex:
		if cur.value.kind != kindTRONArr {
			return nil, fmt.Errorf("transform requires array traversal")
		}
		idx := node.value.(int)
		length, err := arrayLength(cur.value.doc, cur.value.off)
		if err != nil {
			return nil, err
		}
		if idx < 0 {
			idx += int(length)
		}
		if idx < 0 || idx >= int(length) {
			return nil, nil
		}
		val, ok, err := arrGetRaw(cur.value.doc, cur.value.off, uint32(idx))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		next := match{
			path:  appendStep(cur.path, pathStep{kind: stepIndex, index: uint32(idx)}),
			value: valueFromTRON(cur.value.doc, val),
		}
		return []match{next}, nil
	case astSlice:
		if cur.value.kind != kindTRONArr {
			return nil, fmt.Errorf("transform requires array traversal")
		}
		parts := node.value.([]*int)
		params := make([]sliceParam, 3)
		for i, part := range parts {
			if part != nil {
				params[i].Specified = true
				params[i].N = *part
			}
		}
		selected, err := collectSliceMatches(cur.value, cur.path, params)
		if err != nil {
			return nil, err
		}
		return selected, nil
	case astProjection:
		if len(node.children) != 2 {
			return nil, fmt.Errorf("projection expects 2 children")
		}
		left, err := i.collectMatches(node.children[0], cur)
		if err != nil {
			return nil, err
		}
		var results []match
		for _, parent := range left {
			if parent.value.kind != kindTRONArr {
				return nil, fmt.Errorf("transform requires array projection")
			}
			children, err := collectArrayMatches(parent.value, parent.path)
			if err != nil {
				return nil, err
			}
			for _, child := range children {
				sub, err := i.collectMatches(node.children[1], child)
				if err != nil {
					return nil, err
				}
				results = append(results, sub...)
			}
		}
		return results, nil
	case astFilterProjection:
		if len(node.children) != 3 {
			return nil, fmt.Errorf("filter projection expects 3 children")
		}
		left, err := i.collectMatches(node.children[0], cur)
		if err != nil {
			return nil, err
		}
		var results []match
		for _, parent := range left {
			if parent.value.kind != kindTRONArr {
				return nil, fmt.Errorf("transform requires array filter")
			}
			children, err := collectArrayMatches(parent.value, parent.path)
			if err != nil {
				return nil, err
			}
			for _, child := range children {
				passed, err := i.eval(node.children[2], child.value)
				if err != nil {
					return nil, err
				}
				if isFalse(passed) {
					continue
				}
				sub, err := i.collectMatches(node.children[1], child)
				if err != nil {
					return nil, err
				}
				results = append(results, sub...)
			}
		}
		return results, nil
	case astValueProjection:
		if len(node.children) != 2 {
			return nil, fmt.Errorf("value projection expects 2 children")
		}
		left, err := i.collectMatches(node.children[0], cur)
		if err != nil {
			return nil, err
		}
		var results []match
		for _, parent := range left {
			if parent.value.kind != kindTRONMap {
				return nil, fmt.Errorf("transform requires map projection")
			}
			children, err := collectMapMatches(parent.value, parent.path)
			if err != nil {
				return nil, err
			}
			for _, child := range children {
				sub, err := i.collectMatches(node.children[1], child)
				if err != nil {
					return nil, err
				}
				results = append(results, sub...)
			}
		}
		return results, nil
	case astFlatten:
		if len(node.children) != 1 {
			return nil, fmt.Errorf("flatten expects 1 child")
		}
		left, err := i.collectMatches(node.children[0], cur)
		if err != nil {
			return nil, err
		}
		var results []match
		for _, parent := range left {
			if parent.value.kind != kindTRONArr {
				return nil, fmt.Errorf("transform requires array flatten")
			}
			children, err := collectArrayMatches(parent.value, parent.path)
			if err != nil {
				return nil, err
			}
			for _, child := range children {
				if child.value.kind == kindTRONArr {
					grand, err := collectArrayMatches(child.value, child.path)
					if err != nil {
						return nil, err
					}
					results = append(results, grand...)
					continue
				}
				results = append(results, child)
			}
		}
		return results, nil
	case astSubexpression, astIndexExpression:
		left, err := i.collectMatches(node.children[0], cur)
		if err != nil {
			return nil, err
		}
		var results []match
		for _, parent := range left {
			sub, err := i.collectMatches(node.children[1], parent)
			if err != nil {
				return nil, err
			}
			results = append(results, sub...)
		}
		return results, nil
	case astPipe:
		results := []match{cur}
		for _, child := range node.children {
			var next []match
			for _, r := range results {
				sub, err := i.collectMatches(child, r)
				if err != nil {
					return nil, err
				}
				next = append(next, sub...)
			}
			results = next
		}
		return results, nil
	default:
		return nil, fmt.Errorf("transform does not support AST node %v", node.typ)
	}
}

func applyAtPath(builder *tron.Builder, root tron.Value, steps []pathStep, fn func(tron.Value) (tron.Value, error)) (tron.Value, error) {
	if len(steps) == 0 {
		return fn(root)
	}
	step := steps[0]
	switch step.kind {
	case stepKey:
		if root.Type != tron.TypeMap {
			return tron.Value{}, fmt.Errorf("expected map at %q", step.key)
		}
		val, ok, err := mapGetBytes(builder.Buffer(), root.Offset, step.key, 0)
		if err != nil {
			return tron.Value{}, err
		}
		if !ok {
			return tron.Value{}, fmt.Errorf("missing map key %q", step.key)
		}
		child, err := applyAtPath(builder, val, steps[1:], fn)
		if err != nil {
			return tron.Value{}, err
		}
		newOff, _, err := tron.MapSetNode(builder, root.Offset, step.key, child)
		if err != nil {
			return tron.Value{}, err
		}
		return tron.Value{Type: tron.TypeMap, Offset: newOff}, nil
	case stepIndex:
		if root.Type != tron.TypeArr {
			return tron.Value{}, fmt.Errorf("expected array at index %d", step.index)
		}
		val, ok, err := arrGetRaw(builder.Buffer(), root.Offset, step.index)
		if err != nil {
			return tron.Value{}, err
		}
		if !ok {
			return tron.Value{}, fmt.Errorf("missing array index %d", step.index)
		}
		child, err := applyAtPath(builder, val, steps[1:], fn)
		if err != nil {
			return tron.Value{}, err
		}
		length, err := tron.ArrayRootLength(builder.Buffer(), root.Offset)
		if err != nil {
			return tron.Value{}, err
		}
		newOff, err := tron.ArraySetNode(builder, root.Offset, step.index, child, length)
		if err != nil {
			return tron.Value{}, err
		}
		return tron.Value{Type: tron.TypeArr, Offset: newOff}, nil
	default:
		return tron.Value{}, fmt.Errorf("unknown path step")
	}
}

func rootTRONValue(doc []byte) (tron.Value, tron.DocType, tron.Trailer, error) {
	docType, err := tron.DetectDocType(doc)
	if err != nil {
		return tron.Value{}, tron.DocUnknown, tron.Trailer{}, err
	}
	switch docType {
	case tron.DocScalar:
		val, err := tron.DecodeScalarDocument(doc)
		return val, docType, tron.Trailer{}, err
	case tron.DocTree:
		tr, err := tron.ParseTrailer(doc)
		if err != nil {
			return tron.Value{}, docType, tron.Trailer{}, err
		}
		h, _, err := tron.NodeSliceAt(doc, tr.RootOffset)
		if err != nil {
			return tron.Value{}, docType, tron.Trailer{}, err
		}
		switch h.KeyType {
		case tron.KeyMap:
			return tron.Value{Type: tron.TypeMap, Offset: tr.RootOffset}, docType, tr, nil
		case tron.KeyArr:
			return tron.Value{Type: tron.TypeArr, Offset: tr.RootOffset}, docType, tr, nil
		default:
			return tron.Value{}, docType, tron.Trailer{}, fmt.Errorf("unknown root node type")
		}
	default:
		return tron.Value{}, docType, tron.Trailer{}, fmt.Errorf("unknown document type")
	}
}

func collectArrayMatches(v jValue, base []pathStep) ([]match, error) {
	length, err := arrayLength(v.doc, v.off)
	if err != nil {
		return nil, err
	}
	values := make([]tron.Value, length)
	present := make([]bool, length)
	if err := arrCollectValues(v.doc, v.off, 0, values, present); err != nil {
		return nil, err
	}
	matches := make([]match, 0, length)
	for i := range values {
		if !present[i] {
			continue
		}
		step := pathStep{kind: stepIndex, index: uint32(i)}
		matches = append(matches, match{
			path:  appendStep(base, step),
			value: valueFromTRON(v.doc, values[i]),
		})
	}
	return matches, nil
}

func collectMapMatches(v jValue, base []pathStep) ([]match, error) {
	matches := []match{}
	err := mapIterEntries(v.doc, v.off, func(key []byte, val tron.Value) error {
		step := pathStep{kind: stepKey, key: key}
		matches = append(matches, match{
			path:  appendStep(base, step),
			value: valueFromTRON(v.doc, val),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func appendStep(path []pathStep, step pathStep) []pathStep {
	out := make([]pathStep, len(path)+1)
	copy(out, path)
	out[len(path)] = step
	return out
}

func collectSliceMatches(v jValue, base []pathStep, parts []sliceParam) ([]match, error) {
	length, err := arrayLength(v.doc, v.off)
	if err != nil {
		return nil, err
	}
	computed, err := computeSliceParams(int(length), parts)
	if err != nil {
		return nil, err
	}
	start, stop, step := computed[0], computed[1], computed[2]
	out := []match{}
	if step > 0 {
		for i := start; i < stop; i += step {
			if i < 0 || i >= int(length) {
				continue
			}
			val, ok, err := arrGetRaw(v.doc, v.off, uint32(i))
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			out = append(out, match{
				path:  appendStep(base, pathStep{kind: stepIndex, index: uint32(i)}),
				value: valueFromTRON(v.doc, val),
			})
		}
		return out, nil
	}
	for i := start; i > stop; i += step {
		if i < 0 || i >= int(length) {
			continue
		}
		val, ok, err := arrGetRaw(v.doc, v.off, uint32(i))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out = append(out, match{
			path:  appendStep(base, pathStep{kind: stepIndex, index: uint32(i)}),
			value: valueFromTRON(v.doc, val),
		})
	}
	return out, nil
}
