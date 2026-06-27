package expr

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenType int

const (
	tokEOF tokenType = iota
	tokNumber
	tokString
	tokIdent
	tokTrue
	tokFalse
	tokNull

	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokPercent
	tokStarStar

	tokEq
	tokNeq
	tokStrictEq
	tokStrictNeq
	tokLt
	tokGt
	tokLte
	tokGte

	tokAnd
	tokOr
	tokNot

	tokQuestion
	tokColon
	tokDot
	tokComma
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokLBrace
	tokRBrace
)

type token struct {
	typ tokenType
	val string
	pos int
}

type lexer struct {
	input []rune
	pos   int
}

func newLexer(input string) *lexer {
	return &lexer{input: []rune(input), pos: 0}
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *lexer) advance() rune {
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
}

func (l *lexer) tokenize() ([]token, error) {
	var tokens []token
	for {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			tokens = append(tokens, token{typ: tokEOF, pos: l.pos})
			return tokens, nil
		}

		start := l.pos
		ch := l.peek()

		switch {
		case ch == '+':
			l.advance()
			tokens = append(tokens, token{typ: tokPlus, val: "+", pos: start})
		case ch == '-':
			l.advance()
			tokens = append(tokens, token{typ: tokMinus, val: "-", pos: start})
		case ch == '*':
			l.advance()
			if l.peek() == '*' {
				l.advance()
				tokens = append(tokens, token{typ: tokStarStar, val: "**", pos: start})
			} else {
				tokens = append(tokens, token{typ: tokStar, val: "*", pos: start})
			}
		case ch == '/':
			l.advance()
			tokens = append(tokens, token{typ: tokSlash, val: "/", pos: start})
		case ch == '%':
			l.advance()
			tokens = append(tokens, token{typ: tokPercent, val: "%", pos: start})
		case ch == '=' && l.lookAhead(1) == '=' && l.lookAhead(2) == '=':
			l.pos += 3
			tokens = append(tokens, token{typ: tokStrictEq, val: "===", pos: start})
		case ch == '=' && l.lookAhead(1) == '=':
			l.pos += 2
			tokens = append(tokens, token{typ: tokEq, val: "==", pos: start})
		case ch == '!' && l.lookAhead(1) == '=' && l.lookAhead(2) == '=':
			l.pos += 3
			tokens = append(tokens, token{typ: tokStrictNeq, val: "!==", pos: start})
		case ch == '!' && l.lookAhead(1) == '=':
			l.pos += 2
			tokens = append(tokens, token{typ: tokNeq, val: "!=", pos: start})
		case ch == '!':
			l.advance()
			tokens = append(tokens, token{typ: tokNot, val: "!", pos: start})
		case ch == '<' && l.lookAhead(1) == '=':
			l.pos += 2
			tokens = append(tokens, token{typ: tokLte, val: "<=", pos: start})
		case ch == '<':
			l.advance()
			tokens = append(tokens, token{typ: tokLt, val: "<", pos: start})
		case ch == '>' && l.lookAhead(1) == '=':
			l.pos += 2
			tokens = append(tokens, token{typ: tokGte, val: ">=", pos: start})
		case ch == '>':
			l.advance()
			tokens = append(tokens, token{typ: tokGt, val: ">", pos: start})
		case ch == '&' && l.lookAhead(1) == '&':
			l.pos += 2
			tokens = append(tokens, token{typ: tokAnd, val: "&&", pos: start})
		case ch == '|' && l.lookAhead(1) == '|':
			l.pos += 2
			tokens = append(tokens, token{typ: tokOr, val: "||", pos: start})
		case ch == '?':
			l.advance()
			tokens = append(tokens, token{typ: tokQuestion, val: "?", pos: start})
		case ch == ':':
			l.advance()
			tokens = append(tokens, token{typ: tokColon, val: ":", pos: start})
		case ch == '.':
			l.advance()
			tokens = append(tokens, token{typ: tokDot, val: ".", pos: start})
		case ch == ',':
			l.advance()
			tokens = append(tokens, token{typ: tokComma, val: ",", pos: start})
		case ch == '(':
			l.advance()
			tokens = append(tokens, token{typ: tokLParen, val: "(", pos: start})
		case ch == ')':
			l.advance()
			tokens = append(tokens, token{typ: tokRParen, val: ")", pos: start})
		case ch == '[':
			l.advance()
			tokens = append(tokens, token{typ: tokLBracket, val: "[", pos: start})
		case ch == ']':
			l.advance()
			tokens = append(tokens, token{typ: tokRBracket, val: "]", pos: start})
		case ch == '{':
			l.advance()
			tokens = append(tokens, token{typ: tokLBrace, val: "{", pos: start})
		case ch == '}':
			l.advance()
			tokens = append(tokens, token{typ: tokRBrace, val: "}", pos: start})
		case ch == '"' || ch == '\'':
			tok, err := l.readString(ch)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		case isDigit(ch):
			tokens = append(tokens, l.readNumber())
		case isIdentStart(ch):
			tok := l.readIdent()
			tokens = append(tokens, tok)
		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", ch, l.pos)
		}
	}
}

func (l *lexer) lookAhead(n int) rune {
	pos := l.pos + n
	if pos >= len(l.input) {
		return 0
	}
	return l.input[pos]
}

func (l *lexer) readString(quote rune) (token, error) {
	start := l.pos
	l.advance() // skip opening quote
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.advance()
		if ch == quote {
			return token{typ: tokString, val: sb.String(), pos: start}, nil
		}
		if ch == '\\' {
			if l.pos >= len(l.input) {
				return token{}, fmt.Errorf("unterminated string escape at position %d", l.pos)
			}
			esc := l.advance()
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\':
				sb.WriteByte('\\')
			case '\'':
				sb.WriteByte('\'')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteByte('\\')
				sb.WriteRune(esc)
			}
		} else {
			sb.WriteRune(ch)
		}
	}
	return token{}, fmt.Errorf("unterminated string starting at position %d", start)
}

func (l *lexer) readNumber() token {
	start := l.pos
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		l.pos++
	}
	if l.pos < len(l.input) && (l.input[l.pos] == 'e' || l.input[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	return token{typ: tokNumber, val: string(l.input[start:l.pos]), pos: start}
}

func (l *lexer) readIdent() token {
	start := l.pos
	for l.pos < len(l.input) && isIdentPart(l.input[l.pos]) {
		l.pos++
	}
	val := string(l.input[start:l.pos])
	switch val {
	case "true":
		return token{typ: tokTrue, val: val, pos: start}
	case "false":
		return token{typ: tokFalse, val: val, pos: start}
	case "null":
		return token{typ: tokNull, val: val, pos: start}
	default:
		return token{typ: tokIdent, val: val, pos: start}
	}
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentStart(ch rune) bool {
	return ch == '_' || ch == '$' || unicode.IsLetter(ch)
}

func isIdentPart(ch rune) bool {
	return isIdentStart(ch) || isDigit(ch)
}
