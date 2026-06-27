package expr

import (
	"fmt"
	"strconv"
)

type parser struct {
	tokens []token
	pos    int
}

func newParser(tokens []token) *parser {
	return &parser{tokens: tokens}
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.peek()
	p.pos++
	return t
}

func (p *parser) expect(typ tokenType) (token, error) {
	t := p.advance()
	if t.typ != typ {
		return t, fmt.Errorf("expected %d but got %q at position %d", typ, t.val, t.pos)
	}
	return t, nil
}

func (p *parser) parse() (node, error) {
	n, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.peek().typ != tokEOF {
		return nil, fmt.Errorf("unexpected token %q at position %d", p.peek().val, p.peek().pos)
	}
	return n, nil
}

func (p *parser) parseExpr(minPrec int) (node, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		prec := infixPrecedence(t.typ)
		if prec < minPrec {
			break
		}

		if t.typ == tokQuestion {
			left, err = p.parseTernary(left)
			if err != nil {
				return nil, err
			}
			continue
		}

		if t.typ == tokLParen {
			left, err = p.parseCall(left)
			if err != nil {
				return nil, err
			}
			continue
		}

		if t.typ == tokDot {
			p.advance()
			ident, err := p.expect(tokIdent)
			if err != nil {
				return nil, fmt.Errorf("expected property name after '.'")
			}
			left = &memberNode{object: left, property: ident.val}
			continue
		}

		if t.typ == tokLBracket {
			p.advance()
			idx, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tokRBracket); err != nil {
				return nil, fmt.Errorf("expected ']'")
			}
			left = &indexNode{object: left, index: idx}
			continue
		}

		p.advance()
		nextPrec := prec + 1
		if isRightAssoc(t.typ) {
			nextPrec = prec
		}
		right, err := p.parseExpr(nextPrec)
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: t.val, left: left, right: right}
	}

	return left, nil
}

func (p *parser) parsePrefix() (node, error) {
	t := p.peek()

	switch t.typ {
	case tokNumber:
		p.advance()
		n, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", t.val)
		}
		return &literalNode{value: Number(n)}, nil

	case tokString:
		p.advance()
		return &literalNode{value: String(t.val)}, nil

	case tokTrue:
		p.advance()
		return &literalNode{value: Bool(true)}, nil

	case tokFalse:
		p.advance()
		return &literalNode{value: Bool(false)}, nil

	case tokNull:
		p.advance()
		return &literalNode{value: Null}, nil

	case tokIdent:
		p.advance()
		return &identNode{name: t.val}, nil

	case tokLParen:
		p.advance()
		inner, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, fmt.Errorf("expected ')'")
		}
		return inner, nil

	case tokLBracket:
		return p.parseArrayLiteral()

	case tokLBrace:
		return p.parseObjectLiteral()

	case tokMinus, tokPlus, tokNot:
		p.advance()
		operand, err := p.parseExpr(prefixPrecedence)
		if err != nil {
			return nil, err
		}
		return &unaryNode{op: t.val, operand: operand}, nil

	default:
		return nil, fmt.Errorf("unexpected token %q at position %d", t.val, t.pos)
	}
}

func (p *parser) parseTernary(cond node) (node, error) {
	p.advance() // skip '?'
	consequent, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokColon); err != nil {
		return nil, fmt.Errorf("expected ':' in ternary expression")
	}
	alternate, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	return &ternaryNode{cond: cond, consequent: consequent, alternate: alternate}, nil
}

func (p *parser) parseCall(callee node) (node, error) {
	p.advance() // skip '('
	var args []node
	if p.peek().typ != tokRParen {
		for {
			arg, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().typ != tokComma {
				break
			}
			p.advance()
		}
	}
	if _, err := p.expect(tokRParen); err != nil {
		return nil, fmt.Errorf("expected ')'")
	}
	return &callNode{callee: callee, args: args}, nil
}

func (p *parser) parseArrayLiteral() (node, error) {
	p.advance() // skip '['
	var elements []node
	if p.peek().typ != tokRBracket {
		for {
			elem, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			elements = append(elements, elem)
			if p.peek().typ != tokComma {
				break
			}
			p.advance()
		}
	}
	if _, err := p.expect(tokRBracket); err != nil {
		return nil, fmt.Errorf("expected ']'")
	}
	return &arrayNode{elements: elements}, nil
}

func (p *parser) parseObjectLiteral() (node, error) {
	p.advance() // skip '{'
	var keys []string
	var values []node
	if p.peek().typ != tokRBrace {
		for {
			var key string
			t := p.peek()
			switch t.typ {
			case tokIdent:
				p.advance()
				key = t.val
			case tokString:
				p.advance()
				key = t.val
			default:
				return nil, fmt.Errorf("expected property name, got %q", t.val)
			}
			if _, err := p.expect(tokColon); err != nil {
				return nil, fmt.Errorf("expected ':' after property name")
			}
			val, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			keys = append(keys, key)
			values = append(values, val)
			if p.peek().typ != tokComma {
				break
			}
			p.advance()
		}
	}
	if _, err := p.expect(tokRBrace); err != nil {
		return nil, fmt.Errorf("expected '}'")
	}
	return &objectNode{keys: keys, values: values}, nil
}

const prefixPrecedence = 14

func infixPrecedence(typ tokenType) int {
	switch typ {
	case tokOr:
		return 2
	case tokAnd:
		return 3
	case tokEq, tokNeq, tokStrictEq, tokStrictNeq:
		return 6
	case tokLt, tokGt, tokLte, tokGte:
		return 7
	case tokPlus, tokMinus:
		return 9
	case tokStar, tokSlash, tokPercent:
		return 10
	case tokStarStar:
		return 11
	case tokDot:
		return 16
	case tokLBracket:
		return 16
	case tokLParen:
		return 16
	case tokQuestion:
		return 1
	default:
		return -1
	}
}

func isRightAssoc(typ tokenType) bool {
	return typ == tokStarStar
}
