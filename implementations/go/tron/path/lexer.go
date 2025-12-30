package path

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type jpToken struct {
	tokenType jpTokType
	value     string
	position  int
	length    int
}

type jpTokType int

const jpEOF = -1

// jpLexer contains information about the expression being tokenized.
type jpLexer struct {
	expression string       // The expression provided by the user.
	currentPos int          // The current position in the string.
	lastWidth  int          // The width of the current rune.  This
	buf        bytes.Buffer // Internal buffer used for building up values.
	tokens     []jpToken
}

// jpSyntaxError is the main error used whenever a lexing or parsing error occurs.
type jpSyntaxError struct {
	msg        string // Error message displayed to user
	Expression string // Expression that generated a jpSyntaxError
	Offset     int    // The location in the string where the error occurred
}

func (e jpSyntaxError) Error() string {
	// In the future, it would be good to underline the specific
	// location where the error occurred.
	return "jpSyntaxError: " + e.msg
}

// HighlightLocation will show where the syntax error occurred.
// It will place a "^" character on a line below the expression
// at the point where the syntax error occurred.
func (e jpSyntaxError) HighlightLocation() string {
	return e.Expression + "\n" + strings.Repeat(" ", e.Offset) + "^"
}

//go:generate stringer -type=jpTokType
const (
	jpTUnknown jpTokType = iota
	jpTStar
	jpTDot
	jpTFilter
	jpTFlatten
	jpTLparen
	jpTRparen
	jpTLbracket
	jpTRbracket
	jpTLbrace
	jpTRbrace
	jpTOr
	jpTPipe
	jpTNumber
	jpTUnquotedIdentifier
	jpTQuotedIdentifier
	jpTComma
	jpTColon
	jpTLT
	jpTLTE
	jpTGT
	jpTGTE
	jpTEQ
	jpTNE
	jpTJSONLiteral
	jpTStringLiteral
	jpTCurrent
	jpTExpref
	jpTAnd
	jpTNot
	jpTEOF
)

var basicTokens = map[rune]jpTokType{
	'.': jpTDot,
	'*': jpTStar,
	',': jpTComma,
	':': jpTColon,
	'{': jpTLbrace,
	'}': jpTRbrace,
	']': jpTRbracket, // jpTLbracket not included because it could be "[]"
	'(': jpTLparen,
	')': jpTRparen,
	'@': jpTCurrent,
}

// Bit mask for [a-zA-Z_] shifted down 64 bits to fit in a single uint64.
// When using this bitmask just be sure to shift the rune down 64 bits
// before checking against identifierStartBits.
const identifierStartBits uint64 = 576460745995190270

// Bit mask for [a-zA-Z0-9], 128 bits -> 2 uint64s.
var identifierTrailingBits = [2]uint64{287948901175001088, 576460745995190270}

var whiteSpace = map[rune]bool{
	' ': true, '\t': true, '\n': true, '\r': true,
}

func (t jpToken) String() string {
	return fmt.Sprintf("Token{%+v, %s, %d, %d}",
		t.tokenType, t.value, t.position, t.length)
}

// NewLexer creates a new JMESPath lexer.
func NewLexer() *jpLexer {
	lexer := jpLexer{}
	return &lexer
}

func (lexer *jpLexer) reset() {
	lexer.expression = ""
	lexer.currentPos = 0
	lexer.lastWidth = 0
	lexer.buf.Reset()
	if len(lexer.tokens) > 0 {
		clear(lexer.tokens)
		lexer.tokens = lexer.tokens[:0]
	}
}

func (lexer *jpLexer) next() rune {
	if lexer.currentPos >= len(lexer.expression) {
		lexer.lastWidth = 0
		return jpEOF
	}
	r, w := utf8.DecodeRuneInString(lexer.expression[lexer.currentPos:])
	lexer.lastWidth = w
	lexer.currentPos += w
	return r
}

func (lexer *jpLexer) back() {
	lexer.currentPos -= lexer.lastWidth
}

func (lexer *jpLexer) peek() rune {
	t := lexer.next()
	lexer.back()
	return t
}

// tokenize takes an expression and returns corresponding tokens.
func (lexer *jpLexer) tokenize(expression string) ([]jpToken, error) {
	tokens := lexer.tokens[:0]
	lexer.expression = expression
	lexer.currentPos = 0
	lexer.lastWidth = 0
	lexer.buf.Reset()
	defer func() {
		lexer.tokens = tokens
	}()
loop:
	for {
		r := lexer.next()
		if identifierStartBits&(1<<(uint64(r)-64)) > 0 {
			t := lexer.consumeUnquotedIdentifier()
			tokens = append(tokens, t)
		} else if val, ok := basicTokens[r]; ok {
			// Basic single char jpToken.
			t := jpToken{
				tokenType: val,
				value:     string(r),
				position:  lexer.currentPos - lexer.lastWidth,
				length:    1,
			}
			tokens = append(tokens, t)
		} else if r == '-' || (r >= '0' && r <= '9') {
			t := lexer.consumeNumber()
			tokens = append(tokens, t)
		} else if r == '[' {
			t := lexer.consumeLBracket()
			tokens = append(tokens, t)
		} else if r == '"' {
			t, err := lexer.consumeQuotedIdentifier()
			if err != nil {
				return tokens, err
			}
			tokens = append(tokens, t)
		} else if r == '\'' {
			t, err := lexer.consumeRawStringLiteral()
			if err != nil {
				return tokens, err
			}
			tokens = append(tokens, t)
		} else if r == '`' {
			t, err := lexer.consumeLiteral()
			if err != nil {
				return tokens, err
			}
			tokens = append(tokens, t)
		} else if r == '|' {
			t := lexer.matchOrElse(r, '|', jpTOr, jpTPipe)
			tokens = append(tokens, t)
		} else if r == '<' {
			t := lexer.matchOrElse(r, '=', jpTLTE, jpTLT)
			tokens = append(tokens, t)
		} else if r == '>' {
			t := lexer.matchOrElse(r, '=', jpTGTE, jpTGT)
			tokens = append(tokens, t)
		} else if r == '!' {
			t := lexer.matchOrElse(r, '=', jpTNE, jpTNot)
			tokens = append(tokens, t)
		} else if r == '=' {
			t := lexer.matchOrElse(r, '=', jpTEQ, jpTUnknown)
			tokens = append(tokens, t)
		} else if r == '&' {
			t := lexer.matchOrElse(r, '&', jpTAnd, jpTExpref)
			tokens = append(tokens, t)
		} else if r == jpEOF {
			break loop
		} else if _, ok := whiteSpace[r]; ok {
			// Ignore whitespace
		} else {
			return tokens, lexer.syntaxError(fmt.Sprintf("Unknown char: %s", strconv.QuoteRuneToASCII(r)))
		}
	}
	tokens = append(tokens, jpToken{jpTEOF, "", len(lexer.expression), 0})
	return tokens, nil
}

// Consume characters until the ending rune "r" is reached.
// If the end of the expression is reached before seeing the
// terminating rune "r", then an error is returned.
// If no error occurs then the matching substring is returned.
// The returned string will not include the ending rune.
func (lexer *jpLexer) consumeUntil(end rune) (string, error) {
	start := lexer.currentPos
	current := lexer.next()
	for current != end && current != jpEOF {
		if current == '\\' && lexer.peek() != jpEOF {
			lexer.next()
		}
		current = lexer.next()
	}
	if lexer.lastWidth == 0 {
		// Then we hit an EOF so we never reached the closing
		// delimiter.
		return "", jpSyntaxError{
			msg:        "Unclosed delimiter: " + string(end),
			Expression: lexer.expression,
			Offset:     len(lexer.expression),
		}
	}
	return lexer.expression[start : lexer.currentPos-lexer.lastWidth], nil
}

func (lexer *jpLexer) consumeLiteral() (jpToken, error) {
	start := lexer.currentPos
	value, err := lexer.consumeUntil('`')
	if err != nil {
		return jpToken{}, err
	}
	value = strings.Replace(value, "\\`", "`", -1)
	return jpToken{
		tokenType: jpTJSONLiteral,
		value:     value,
		position:  start,
		length:    len(value),
	}, nil
}

func (lexer *jpLexer) consumeRawStringLiteral() (jpToken, error) {
	start := lexer.currentPos
	currentIndex := start
	current := lexer.next()
	for current != '\'' && lexer.peek() != jpEOF {
		if current == '\\' && lexer.peek() == '\'' {
			chunk := lexer.expression[currentIndex : lexer.currentPos-1]
			lexer.buf.WriteString(chunk)
			lexer.buf.WriteString("'")
			lexer.next()
			currentIndex = lexer.currentPos
		}
		current = lexer.next()
	}
	if lexer.lastWidth == 0 {
		// Then we hit an EOF so we never reached the closing
		// delimiter.
		return jpToken{}, jpSyntaxError{
			msg:        "Unclosed delimiter: '",
			Expression: lexer.expression,
			Offset:     len(lexer.expression),
		}
	}
	if currentIndex < lexer.currentPos {
		lexer.buf.WriteString(lexer.expression[currentIndex : lexer.currentPos-1])
	}
	value := lexer.buf.String()
	// Reset the buffer so it can reused again.
	lexer.buf.Reset()
	return jpToken{
		tokenType: jpTStringLiteral,
		value:     value,
		position:  start,
		length:    len(value),
	}, nil
}

func (lexer *jpLexer) syntaxError(msg string) jpSyntaxError {
	return jpSyntaxError{
		msg:        msg,
		Expression: lexer.expression,
		Offset:     lexer.currentPos - 1,
	}
}

// Checks for a two char jpToken, otherwise matches a single character
// jpToken. This is used whenever a two char jpToken overlaps a single
// char jpToken, e.g. "||" -> jpTPipe, "|" -> jpTOr.
func (lexer *jpLexer) matchOrElse(first rune, second rune, matchedType jpTokType, singleCharType jpTokType) jpToken {
	start := lexer.currentPos - lexer.lastWidth
	nextRune := lexer.next()
	var t jpToken
	if nextRune == second {
		t = jpToken{
			tokenType: matchedType,
			value:     string(first) + string(second),
			position:  start,
			length:    2,
		}
	} else {
		lexer.back()
		t = jpToken{
			tokenType: singleCharType,
			value:     string(first),
			position:  start,
			length:    1,
		}
	}
	return t
}

func (lexer *jpLexer) consumeLBracket() jpToken {
	// There's three options here:
	// 1. A filter expression "[?"
	// 2. A flatten operator "[]"
	// 3. A bare rbracket "["
	start := lexer.currentPos - lexer.lastWidth
	nextRune := lexer.next()
	var t jpToken
	if nextRune == '?' {
		t = jpToken{
			tokenType: jpTFilter,
			value:     "[?",
			position:  start,
			length:    2,
		}
	} else if nextRune == ']' {
		t = jpToken{
			tokenType: jpTFlatten,
			value:     "[]",
			position:  start,
			length:    2,
		}
	} else {
		t = jpToken{
			tokenType: jpTLbracket,
			value:     "[",
			position:  start,
			length:    1,
		}
		lexer.back()
	}
	return t
}

func (lexer *jpLexer) consumeQuotedIdentifier() (jpToken, error) {
	start := lexer.currentPos
	value, err := lexer.consumeUntil('"')
	if err != nil {
		return jpToken{}, err
	}
	var decoded string
	asJSON := []byte("\"" + value + "\"")
	if err := json.Unmarshal([]byte(asJSON), &decoded); err != nil {
		return jpToken{}, err
	}
	return jpToken{
		tokenType: jpTQuotedIdentifier,
		value:     decoded,
		position:  start - 1,
		length:    len(decoded),
	}, nil
}

func (lexer *jpLexer) consumeUnquotedIdentifier() jpToken {
	// Consume runes until we reach the end of an unquoted
	// identifier.
	start := lexer.currentPos - lexer.lastWidth
	for {
		r := lexer.next()
		if r < 0 || r > 128 || identifierTrailingBits[uint64(r)/64]&(1<<(uint64(r)%64)) == 0 {
			lexer.back()
			break
		}
	}
	value := lexer.expression[start:lexer.currentPos]
	return jpToken{
		tokenType: jpTUnquotedIdentifier,
		value:     value,
		position:  start,
		length:    lexer.currentPos - start,
	}
}

func (lexer *jpLexer) consumeNumber() jpToken {
	// Consume runes until we reach something that's not a number.
	start := lexer.currentPos - lexer.lastWidth
	for {
		r := lexer.next()
		if r < '0' || r > '9' {
			lexer.back()
			break
		}
	}
	value := lexer.expression[start:lexer.currentPos]
	return jpToken{
		tokenType: jpTNumber,
		value:     value,
		position:  start,
		length:    lexer.currentPos - start,
	}
}
