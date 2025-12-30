package path

import (
	"fmt"
	"sync"
)

// Expr is a compiled JMESPath expression.
type Expr struct {
	root *node
}

var parserPool = sync.Pool{
	New: func() any {
		return newJPParser()
	},
}

// Compile parses a JMESPath expression.
func Compile(expression string) (*Expr, error) {
	if expr, ok := compileCache.get(expression); ok {
		return expr, nil
	}
	normalized, err := normalizeExpression(expression)
	if err != nil {
		return nil, err
	}
	parser := parserPool.Get().(*jpParser)
	defer parserPool.Put(parser)
	ast, err := parser.Parse(normalized)
	if err != nil {
		return nil, err
	}
	root, err := convertAST(ast)
	if err != nil {
		return nil, err
	}
	expr := &Expr{root: root}
	compileCache.add(expression, expr)
	return expr, nil
}

// MustCompile is like Compile but panics on error.
func MustCompile(expression string) *Expr {
	expr, err := Compile(expression)
	if err != nil {
		panic(fmt.Sprintf("tron/path: Compile(%q): %v", expression, err))
	}
	return expr
}

// Parse is an alias for Compile.
func Parse(expression string) (*Expr, error) {
	return Compile(expression)
}
