package path

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

func normalizeExpression(expression string) (string, error) {
	if strings.IndexByte(expression, '\'') == -1 {
		if strings.IndexByte(expression, '@') == -1 && strings.IndexByte(expression, ':') == -1 {
			return expression, nil
		}
		if hasInvalidAtFunction(expression) {
			return "", fmt.Errorf("syntax error: invalid function name")
		}
		if hasInvalidSliceSyntax(expression) {
			return "", fmt.Errorf("syntax error: invalid slice")
		}
		return expression, nil
	}
	rewritten, err := rewriteRawStringLiterals(expression)
	if err != nil {
		return "", err
	}
	if hasInvalidAtFunction(rewritten) {
		return "", fmt.Errorf("syntax error: invalid function name")
	}
	if hasInvalidSliceSyntax(rewritten) {
		return "", fmt.Errorf("syntax error: invalid slice")
	}
	return rewritten, nil
}

func rewriteRawStringLiterals(expression string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(expression); {
		switch expression[i] {
		case '`':
			start := i
			i++
			for i < len(expression) {
				if expression[i] == '\\' && i+1 < len(expression) && expression[i+1] == '`' {
					i += 2
					continue
				}
				if expression[i] == '`' {
					i++
					break
				}
				i++
			}
			if i > len(expression) {
				return "", fmt.Errorf("syntax error: unclosed literal")
			}
			out.WriteString(expression[start:i])
		case '\'':
			i++
			var raw strings.Builder
			closed := false
			for i < len(expression) {
				if expression[i] == '\'' {
					i++
					closed = true
					break
				}
				if expression[i] != '\\' {
					raw.WriteByte(expression[i])
					i++
					continue
				}
				start := i
				for i < len(expression) && expression[i] == '\\' {
					i++
				}
				count := i - start
				if i < len(expression) && expression[i] == '\'' {
					if count%2 == 1 {
						for j := 0; j < count-1; j++ {
							raw.WriteByte('\\')
						}
						raw.WriteByte('\'')
						i++
						continue
					}
					for j := 0; j < count; j++ {
						raw.WriteByte('\\')
					}
					i++
					closed = true
					break
				}
				for j := 0; j < count; j++ {
					raw.WriteByte('\\')
				}
			}
			if !closed {
				return "", fmt.Errorf("syntax error: unclosed raw string")
			}
			encoded, err := json.Marshal(raw.String())
			if err != nil {
				return "", err
			}
			out.WriteByte('`')
			out.Write(encoded)
			out.WriteByte('`')
		default:
			out.WriteByte(expression[i])
			i++
		}
	}
	return out.String(), nil
}

func hasInvalidAtFunction(expression string) bool {
	for i := 0; i < len(expression); {
		switch expression[i] {
		case '`':
			i = skipLiteral(expression, i)
		case '"':
			i = skipQuotedIdentifier(expression, i)
		case '@':
			j := i + 1
			for j < len(expression) && unicode.IsSpace(rune(expression[j])) {
				j++
			}
			if j < len(expression) && expression[j] == '(' {
				return true
			}
			i++
		default:
			i++
		}
	}
	return false
}

func hasInvalidSliceSyntax(expression string) bool {
	depth := 0
	for i := 0; i < len(expression); i++ {
		switch expression[i] {
		case '`':
			i = skipLiteral(expression, i) - 1
		case '"':
			i = skipQuotedIdentifier(expression, i) - 1
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 1 {
				count := 1
				for j := i + 1; j < len(expression) && expression[j] == ':'; j++ {
					count++
				}
				if count > 2 {
					return true
				}
			}
		}
	}
	return false
}

func skipLiteral(expression string, start int) int {
	i := start + 1
	for i < len(expression) {
		if expression[i] == '\\' && i+1 < len(expression) && expression[i+1] == '`' {
			i += 2
			continue
		}
		if expression[i] == '`' {
			return i + 1
		}
		i++
	}
	return len(expression)
}

func skipQuotedIdentifier(expression string, start int) int {
	i := start + 1
	for i < len(expression) {
		if expression[i] == '\\' && i+1 < len(expression) && expression[i+1] == '"' {
			i += 2
			continue
		}
		if expression[i] == '"' {
			return i + 1
		}
		i++
	}
	return len(expression)
}
