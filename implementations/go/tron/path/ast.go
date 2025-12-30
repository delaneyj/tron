package path

import (
	"reflect"
	"unsafe"

	tron "tron"
)

type nodeType int

const (
	astEmpty nodeType = iota
	astComparator
	astCurrentNode
	astExpRef
	astFunctionExpression
	astField
	astFilterProjection
	astFlatten
	astIdentity
	astIndex
	astIndexExpression
	astKeyValPair
	astLiteral
	astMultiSelectHash
	astMultiSelectList
	astOrExpression
	astAndExpression
	astNotExpression
	astPipe
	astProjection
	astSubexpression
	astSlice
	astValueProjection
)

type tokType int

const (
	tUnknown tokType = iota
	tStar
	tDot
	tFilter
	tFlatten
	tLparen
	tRparen
	tLbracket
	tRbracket
	tLbrace
	tRbrace
	tOr
	tPipe
	tNumber
	tUnquotedIdentifier
	tQuotedIdentifier
	tComma
	tColon
	tLT
	tLTE
	tGT
	tGTE
	tEQ
	tNE
	tJSONLiteral
	tStringLiteral
	tCurrent
	tExpref
	tAnd
	tNot
	tEOF
)

type node struct {
	typ      nodeType
	value    any
	children []*node
}

type fieldValue struct {
	key      string
	keyBytes []byte
	hash     uint32
}

func convertAST(ast jpASTNode) (*node, error) {
	return convertASTFast(ast)
}

type rawASTNode struct {
	nodeType int
	value    any
	children []jpASTNode
}

func convertASTFast(ast jpASTNode) (*node, error) {
	raw := *(*rawASTNode)(unsafe.Pointer(&ast))
	typ := nodeType(raw.nodeType)
	val := raw.value
	if typ == astField {
		if key, ok := val.(string); ok {
			keyBytes := []byte(key)
			val = fieldValue{
				key:      key,
				keyBytes: keyBytes,
				hash:     tron.XXH32(keyBytes, 0),
			}
		}
	}
	if typ == astComparator && val != nil {
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Int {
			val = tokType(rv.Int())
		}
	}
	children := make([]*node, 0, len(raw.children))
	for i := range raw.children {
		childNode, err := convertASTFast(raw.children[i])
		if err != nil {
			return nil, err
		}
		children = append(children, childNode)
	}
	return &node{
		typ:      typ,
		value:    val,
		children: children,
	}, nil
}
