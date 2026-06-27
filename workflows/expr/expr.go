package expr

import (
	"fmt"
	"strings"
)

func Eval(expression string, env *Env) (Value, error) {
	tokens, err := newLexer(expression).tokenize()
	if err != nil {
		return Null, fmt.Errorf("lexer: %w", err)
	}
	ast, err := newParser(tokens).parse()
	if err != nil {
		return Null, fmt.Errorf("parser: %w", err)
	}
	return eval(ast, env)
}

func EvalInterpolated(s string, env *Env) (Value, error) {
	if !strings.Contains(s, "${") {
		return String(s), nil
	}
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") && strings.Count(s, "${") == 1 {
		inner := s[2 : len(s)-1]
		return Eval(inner, env)
	}

	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			end := findMatchingBrace(s, i+2)
			if end < 0 {
				return Null, fmt.Errorf("unterminated expression at position %d", i)
			}
			inner := s[i+2 : end]
			val, err := Eval(inner, env)
			if err != nil {
				return Null, err
			}
			result.WriteString(val.ToString())
			i = end + 1
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return String(result.String()), nil
}

func findMatchingBrace(s string, start int) int {
	depth := 1
	inStr := rune(0)
	for i := start; i < len(s); i++ {
		ch := rune(s[i])
		if inStr != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inStr = ch
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
