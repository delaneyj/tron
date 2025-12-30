package path

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type jpASTNodeType int

//go:generate stringer -type jpASTNodeType
const (
	ASTEmpty jpASTNodeType = iota
	ASTComparator
	ASTCurrentNode
	ASTExpRef
	ASTFunctionExpression
	ASTField
	ASTFilterProjection
	ASTFlatten
	ASTIdentity
	ASTIndex
	ASTIndexExpression
	ASTKeyValPair
	ASTLiteral
	ASTMultiSelectHash
	ASTMultiSelectList
	ASTOrExpression
	ASTAndExpression
	ASTNotExpression
	ASTPipe
	ASTProjection
	ASTSubexpression
	ASTSlice
	ASTValueProjection
)

// jpASTNode represents the abstract syntax tree of a JMESPath expression.
type jpASTNode struct {
	nodeType jpASTNodeType
	value    any
	children []jpASTNode
}

func (node jpASTNode) String() string {
	return node.PrettyPrint(0)
}

// PrettyPrint will pretty print the parsed AST.
// The AST is an implementation detail and this pretty print
// function is provided as a convenience method to help with
// debugging.  You should not rely on its output as the internal
// structure of the AST may change at any time.
func (node jpASTNode) PrettyPrint(indent int) string {
	spaces := strings.Repeat(" ", indent)
	output := fmt.Sprintf("%s%s {\n", spaces, node.nodeType)
	nextIndent := indent + 2
	if node.value != nil {
		if converted, ok := node.value.(fmt.Stringer); ok {
			// Account for things like comparator nodes
			// that are enums with a String() method.
			output += fmt.Sprintf("%svalue: %s\n", strings.Repeat(" ", nextIndent), converted.String())
		} else {
			output += fmt.Sprintf("%svalue: %#v\n", strings.Repeat(" ", nextIndent), node.value)
		}
	}
	lastIndex := len(node.children)
	if lastIndex > 0 {
		output += fmt.Sprintf("%schildren: {\n", strings.Repeat(" ", nextIndent))
		childIndent := nextIndent + 2
		for _, elem := range node.children {
			output += elem.PrettyPrint(childIndent)
		}
	}
	output += fmt.Sprintf("%s}\n", spaces)
	return output
}

var bindingPowers = map[jpTokType]int{
	jpTEOF:                0,
	jpTUnquotedIdentifier: 0,
	jpTQuotedIdentifier:   0,
	jpTRbracket:           0,
	jpTRparen:             0,
	jpTComma:              0,
	jpTRbrace:             0,
	jpTNumber:             0,
	jpTCurrent:            0,
	jpTExpref:             0,
	jpTColon:              0,
	jpTPipe:               1,
	jpTOr:                 2,
	jpTAnd:                3,
	jpTEQ:                 5,
	jpTLT:                 5,
	jpTLTE:                5,
	jpTGT:                 5,
	jpTGTE:                5,
	jpTNE:                 5,
	jpTFlatten:            9,
	jpTStar:               20,
	jpTFilter:             21,
	jpTDot:                40,
	jpTNot:                45,
	jpTLbrace:             50,
	jpTLbracket:           55,
	jpTLparen:             60,
}

var lexerPool = sync.Pool{
	New: func() any {
		return NewLexer()
	},
}

func acquireLexer() *jpLexer {
	return lexerPool.Get().(*jpLexer)
}

func releaseLexer(lexer *jpLexer) {
	lexer.reset()
	lexerPool.Put(lexer)
}

// jpParser holds state about the current expression being parsed.
type jpParser struct {
	expression string
	tokens     []jpToken
	index      int
}

// newJPParser creates a new JMESPath parser.
func newJPParser() *jpParser {
	p := jpParser{}
	return &p
}

// Parse will compile a JMESPath expression.
func (p *jpParser) Parse(expression string) (jpASTNode, error) {
	lexer := acquireLexer()
	defer releaseLexer(lexer)
	p.expression = expression
	p.index = 0
	tokens, err := lexer.tokenize(expression)
	if err != nil {
		p.tokens = nil
		p.expression = ""
		return jpASTNode{}, err
	}
	p.tokens = tokens
	parsed, err := p.parseExpression(0)
	if err != nil {
		p.tokens = nil
		p.expression = ""
		return jpASTNode{}, err
	}
	if p.current() != jpTEOF {
		err := p.syntaxError(fmt.Sprintf(
			"Unexpected jpToken at the end of the expression: %s", p.current()))
		p.tokens = nil
		p.expression = ""
		return jpASTNode{}, err
	}
	p.tokens = nil
	p.expression = ""
	return parsed, nil
}

func (p *jpParser) parseExpression(bindingPower int) (jpASTNode, error) {
	var err error
	leftToken := p.lookaheadToken(0)
	p.advance()
	leftNode, err := p.nud(leftToken)
	if err != nil {
		return jpASTNode{}, err
	}
	currentToken := p.current()
	for bindingPower < bindingPowers[currentToken] {
		p.advance()
		leftNode, err = p.led(currentToken, leftNode)
		if err != nil {
			return jpASTNode{}, err
		}
		currentToken = p.current()
	}
	return leftNode, nil
}

func (p *jpParser) parseIndexExpression() (jpASTNode, error) {
	if p.lookahead(0) == jpTColon || p.lookahead(1) == jpTColon {
		return p.parseSliceExpression()
	}
	indexStr := p.lookaheadToken(0).value
	parsedInt, err := strconv.Atoi(indexStr)
	if err != nil {
		return jpASTNode{}, err
	}
	indexNode := jpASTNode{nodeType: ASTIndex, value: parsedInt}
	p.advance()
	if err := p.match(jpTRbracket); err != nil {
		return jpASTNode{}, err
	}
	return indexNode, nil
}

func (p *jpParser) parseSliceExpression() (jpASTNode, error) {
	parts := []*int{nil, nil, nil}
	index := 0
	current := p.current()
	for current != jpTRbracket && index < 3 {
		if current == jpTColon {
			index++
			p.advance()
		} else if current == jpTNumber {
			parsedInt, err := strconv.Atoi(p.lookaheadToken(0).value)
			if err != nil {
				return jpASTNode{}, err
			}
			parts[index] = &parsedInt
			p.advance()
		} else {
			return jpASTNode{}, p.syntaxError(
				"Expected jpTColon or jpTNumber" + ", received: " + p.current().String())
		}
		current = p.current()
	}
	if err := p.match(jpTRbracket); err != nil {
		return jpASTNode{}, err
	}
	return jpASTNode{
		nodeType: ASTSlice,
		value:    parts,
	}, nil
}

func (p *jpParser) match(tokenType jpTokType) error {
	if p.current() == tokenType {
		p.advance()
		return nil
	}
	return p.syntaxError("Expected " + tokenType.String() + ", received: " + p.current().String())
}

func (p *jpParser) led(tokenType jpTokType, node jpASTNode) (jpASTNode, error) {
	switch tokenType {
	case jpTDot:
		if p.current() != jpTStar {
			right, err := p.parseDotRHS(bindingPowers[jpTDot])
			return jpASTNode{
				nodeType: ASTSubexpression,
				children: []jpASTNode{node, right},
			}, err
		}
		p.advance()
		right, err := p.parseProjectionRHS(bindingPowers[jpTDot])
		return jpASTNode{
			nodeType: ASTValueProjection,
			children: []jpASTNode{node, right},
		}, err
	case jpTPipe:
		right, err := p.parseExpression(bindingPowers[jpTPipe])
		return jpASTNode{nodeType: ASTPipe, children: []jpASTNode{node, right}}, err
	case jpTOr:
		right, err := p.parseExpression(bindingPowers[jpTOr])
		return jpASTNode{nodeType: ASTOrExpression, children: []jpASTNode{node, right}}, err
	case jpTAnd:
		right, err := p.parseExpression(bindingPowers[jpTAnd])
		return jpASTNode{nodeType: ASTAndExpression, children: []jpASTNode{node, right}}, err
	case jpTLparen:
		name := node.value
		var args []jpASTNode
		for p.current() != jpTRparen {
			expression, err := p.parseExpression(0)
			if err != nil {
				return jpASTNode{}, err
			}
			if p.current() == jpTComma {
				if err := p.match(jpTComma); err != nil {
					return jpASTNode{}, err
				}
			}
			args = append(args, expression)
		}
		if err := p.match(jpTRparen); err != nil {
			return jpASTNode{}, err
		}
		return jpASTNode{
			nodeType: ASTFunctionExpression,
			value:    name,
			children: args,
		}, nil
	case jpTFilter:
		return p.parseFilter(node)
	case jpTFlatten:
		left := jpASTNode{nodeType: ASTFlatten, children: []jpASTNode{node}}
		right, err := p.parseProjectionRHS(bindingPowers[jpTFlatten])
		return jpASTNode{
			nodeType: ASTProjection,
			children: []jpASTNode{left, right},
		}, err
	case jpTEQ, jpTNE, jpTGT, jpTGTE, jpTLT, jpTLTE:
		right, err := p.parseExpression(bindingPowers[tokenType])
		if err != nil {
			return jpASTNode{}, err
		}
		return jpASTNode{
			nodeType: ASTComparator,
			value:    tokenType,
			children: []jpASTNode{node, right},
		}, nil
	case jpTLbracket:
		tokenType := p.current()
		var right jpASTNode
		var err error
		if tokenType == jpTNumber || tokenType == jpTColon {
			right, err = p.parseIndexExpression()
			if err != nil {
				return jpASTNode{}, err
			}
			return p.projectIfSlice(node, right)
		}
		// Otherwise this is a projection.
		if err := p.match(jpTStar); err != nil {
			return jpASTNode{}, err
		}
		if err := p.match(jpTRbracket); err != nil {
			return jpASTNode{}, err
		}
		right, err = p.parseProjectionRHS(bindingPowers[jpTStar])
		if err != nil {
			return jpASTNode{}, err
		}
		return jpASTNode{
			nodeType: ASTProjection,
			children: []jpASTNode{node, right},
		}, nil
	}
	return jpASTNode{}, p.syntaxError("Unexpected jpToken: " + tokenType.String())
}

func (p *jpParser) nud(jpToken jpToken) (jpASTNode, error) {
	switch jpToken.tokenType {
	case jpTJSONLiteral:
		var parsed interface{}
		err := json.Unmarshal([]byte(jpToken.value), &parsed)
		if err != nil {
			return jpASTNode{}, err
		}
		return jpASTNode{nodeType: ASTLiteral, value: parsed}, nil
	case jpTStringLiteral:
		return jpASTNode{nodeType: ASTLiteral, value: jpToken.value}, nil
	case jpTUnquotedIdentifier:
		return jpASTNode{
			nodeType: ASTField,
			value:    jpToken.value,
		}, nil
	case jpTQuotedIdentifier:
		node := jpASTNode{nodeType: ASTField, value: jpToken.value}
		if p.current() == jpTLparen {
			return jpASTNode{}, p.syntaxErrorToken("Can't have quoted identifier as function name.", jpToken)
		}
		return node, nil
	case jpTStar:
		left := jpASTNode{nodeType: ASTIdentity}
		var right jpASTNode
		var err error
		if p.current() == jpTRbracket {
			right = jpASTNode{nodeType: ASTIdentity}
		} else {
			right, err = p.parseProjectionRHS(bindingPowers[jpTStar])
		}
		return jpASTNode{nodeType: ASTValueProjection, children: []jpASTNode{left, right}}, err
	case jpTFilter:
		return p.parseFilter(jpASTNode{nodeType: ASTIdentity})
	case jpTLbrace:
		return p.parseMultiSelectHash()
	case jpTFlatten:
		left := jpASTNode{
			nodeType: ASTFlatten,
			children: []jpASTNode{{nodeType: ASTIdentity}},
		}
		right, err := p.parseProjectionRHS(bindingPowers[jpTFlatten])
		if err != nil {
			return jpASTNode{}, err
		}
		return jpASTNode{nodeType: ASTProjection, children: []jpASTNode{left, right}}, nil
	case jpTLbracket:
		tokenType := p.current()
		//var right jpASTNode
		if tokenType == jpTNumber || tokenType == jpTColon {
			right, err := p.parseIndexExpression()
			if err != nil {
				return jpASTNode{}, nil
			}
			return p.projectIfSlice(jpASTNode{nodeType: ASTIdentity}, right)
		} else if tokenType == jpTStar && p.lookahead(1) == jpTRbracket {
			p.advance()
			p.advance()
			right, err := p.parseProjectionRHS(bindingPowers[jpTStar])
			if err != nil {
				return jpASTNode{}, err
			}
			return jpASTNode{
				nodeType: ASTProjection,
				children: []jpASTNode{{nodeType: ASTIdentity}, right},
			}, nil
		} else {
			return p.parseMultiSelectList()
		}
	case jpTCurrent:
		return jpASTNode{nodeType: ASTCurrentNode}, nil
	case jpTExpref:
		expression, err := p.parseExpression(bindingPowers[jpTExpref])
		if err != nil {
			return jpASTNode{}, err
		}
		return jpASTNode{nodeType: ASTExpRef, children: []jpASTNode{expression}}, nil
	case jpTNot:
		expression, err := p.parseExpression(bindingPowers[jpTNot])
		if err != nil {
			return jpASTNode{}, err
		}
		return jpASTNode{nodeType: ASTNotExpression, children: []jpASTNode{expression}}, nil
	case jpTLparen:
		expression, err := p.parseExpression(0)
		if err != nil {
			return jpASTNode{}, err
		}
		if err := p.match(jpTRparen); err != nil {
			return jpASTNode{}, err
		}
		return expression, nil
	case jpTEOF:
		return jpASTNode{}, p.syntaxErrorToken("Incomplete expression", jpToken)
	}

	return jpASTNode{}, p.syntaxErrorToken("Invalid jpToken: "+jpToken.tokenType.String(), jpToken)
}

func (p *jpParser) parseMultiSelectList() (jpASTNode, error) {
	var expressions []jpASTNode
	for {
		expression, err := p.parseExpression(0)
		if err != nil {
			return jpASTNode{}, err
		}
		expressions = append(expressions, expression)
		if p.current() == jpTRbracket {
			break
		}
		err = p.match(jpTComma)
		if err != nil {
			return jpASTNode{}, err
		}
	}
	err := p.match(jpTRbracket)
	if err != nil {
		return jpASTNode{}, err
	}
	return jpASTNode{
		nodeType: ASTMultiSelectList,
		children: expressions,
	}, nil
}

func (p *jpParser) parseMultiSelectHash() (jpASTNode, error) {
	var children []jpASTNode
	for {
		keyToken := p.lookaheadToken(0)
		if err := p.match(jpTUnquotedIdentifier); err != nil {
			if err := p.match(jpTQuotedIdentifier); err != nil {
				return jpASTNode{}, p.syntaxError("Expected jpTQuotedIdentifier or jpTUnquotedIdentifier")
			}
		}
		keyName := keyToken.value
		err := p.match(jpTColon)
		if err != nil {
			return jpASTNode{}, err
		}
		value, err := p.parseExpression(0)
		if err != nil {
			return jpASTNode{}, err
		}
		node := jpASTNode{
			nodeType: ASTKeyValPair,
			value:    keyName,
			children: []jpASTNode{value},
		}
		children = append(children, node)
		if p.current() == jpTComma {
			err := p.match(jpTComma)
			if err != nil {
				return jpASTNode{}, nil
			}
		} else if p.current() == jpTRbrace {
			err := p.match(jpTRbrace)
			if err != nil {
				return jpASTNode{}, nil
			}
			break
		}
	}
	return jpASTNode{
		nodeType: ASTMultiSelectHash,
		children: children,
	}, nil
}

func (p *jpParser) projectIfSlice(left jpASTNode, right jpASTNode) (jpASTNode, error) {
	indexExpr := jpASTNode{
		nodeType: ASTIndexExpression,
		children: []jpASTNode{left, right},
	}
	if right.nodeType == ASTSlice {
		right, err := p.parseProjectionRHS(bindingPowers[jpTStar])
		return jpASTNode{
			nodeType: ASTProjection,
			children: []jpASTNode{indexExpr, right},
		}, err
	}
	return indexExpr, nil
}
func (p *jpParser) parseFilter(node jpASTNode) (jpASTNode, error) {
	var right, condition jpASTNode
	var err error
	condition, err = p.parseExpression(0)
	if err != nil {
		return jpASTNode{}, err
	}
	if err := p.match(jpTRbracket); err != nil {
		return jpASTNode{}, err
	}
	if p.current() == jpTFlatten {
		right = jpASTNode{nodeType: ASTIdentity}
	} else {
		right, err = p.parseProjectionRHS(bindingPowers[jpTFilter])
		if err != nil {
			return jpASTNode{}, err
		}
	}

	return jpASTNode{
		nodeType: ASTFilterProjection,
		children: []jpASTNode{node, right, condition},
	}, nil
}

func (p *jpParser) parseDotRHS(bindingPower int) (jpASTNode, error) {
	lookahead := p.current()
	if tokensOneOf([]jpTokType{jpTQuotedIdentifier, jpTUnquotedIdentifier, jpTStar}, lookahead) {
		return p.parseExpression(bindingPower)
	} else if lookahead == jpTLbracket {
		if err := p.match(jpTLbracket); err != nil {
			return jpASTNode{}, err
		}
		return p.parseMultiSelectList()
	} else if lookahead == jpTLbrace {
		if err := p.match(jpTLbrace); err != nil {
			return jpASTNode{}, err
		}
		return p.parseMultiSelectHash()
	}
	return jpASTNode{}, p.syntaxError("Expected identifier, lbracket, or lbrace")
}

func (p *jpParser) parseProjectionRHS(bindingPower int) (jpASTNode, error) {
	current := p.current()
	if bindingPowers[current] < 10 {
		return jpASTNode{nodeType: ASTIdentity}, nil
	} else if current == jpTLbracket {
		return p.parseExpression(bindingPower)
	} else if current == jpTFilter {
		return p.parseExpression(bindingPower)
	} else if current == jpTDot {
		err := p.match(jpTDot)
		if err != nil {
			return jpASTNode{}, err
		}
		return p.parseDotRHS(bindingPower)
	} else {
		return jpASTNode{}, p.syntaxError("Error")
	}
}

func (p *jpParser) lookahead(number int) jpTokType {
	return p.lookaheadToken(number).tokenType
}

func (p *jpParser) current() jpTokType {
	return p.lookahead(0)
}

func (p *jpParser) lookaheadToken(number int) jpToken {
	return p.tokens[p.index+number]
}

func (p *jpParser) advance() {
	p.index++
}

func tokensOneOf(elements []jpTokType, jpToken jpTokType) bool {
	for _, elem := range elements {
		if elem == jpToken {
			return true
		}
	}
	return false
}

func (p *jpParser) syntaxError(msg string) jpSyntaxError {
	return jpSyntaxError{
		msg:        msg,
		Expression: p.expression,
		Offset:     p.lookaheadToken(0).position,
	}
}

// Create a jpSyntaxError based on the provided jpToken.
// This differs from syntaxError() which creates a jpSyntaxError
// based on the current lookahead jpToken.
func (p *jpParser) syntaxErrorToken(msg string, t jpToken) jpSyntaxError {
	return jpSyntaxError{
		msg:        msg,
		Expression: p.expression,
		Offset:     t.position,
	}
}
