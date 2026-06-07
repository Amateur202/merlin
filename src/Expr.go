package main

import "strings"

// ---------------------------------------------------------------------------
// Expression Parsing (Pratt Parser)
// ---------------------------------------------------------------------------

type precedence int

const (
	PREC_LOWEST precedence = iota
	PREC_OR
	PREC_AND
	PREC_BIT_OR
	PREC_BIT_XOR
	PREC_BIT_AND
	PREC_EQUALS
	PREC_LESS_GREATER
	PREC_SHIFT
	PREC_SUM
	PREC_PRODUCT
	PREC_PREFIX
	PREC_CALL
)

var precedences = map[TokenType]precedence{
	TOKEN_OR:        PREC_OR,
	TOKEN_AND:       PREC_AND,
	TOKEN_PIPE:      PREC_BIT_OR,
	TOKEN_CARET:     PREC_BIT_XOR,
	TOKEN_AMPERSAND: PREC_BIT_AND,
	TOKEN_EQ:        PREC_EQUALS,
	TOKEN_NEQ:       PREC_EQUALS,
	TOKEN_LT:        PREC_LESS_GREATER,
	TOKEN_GT:        PREC_LESS_GREATER,
	TOKEN_LTE:       PREC_LESS_GREATER,
	TOKEN_GTE:       PREC_LESS_GREATER,
	TOKEN_LSHIFT:    PREC_SHIFT,
	TOKEN_RSHIFT:    PREC_SHIFT,
	TOKEN_PLUS:      PREC_SUM,
	TOKEN_MINUS:     PREC_SUM,
	TOKEN_STAR:      PREC_PRODUCT,
	TOKEN_SLASH:     PREC_PRODUCT,
	TOKEN_PERCENT:   PREC_PRODUCT,
	TOKEN_LPAREN:    PREC_CALL,
	TOKEN_LBRACKET:  PREC_CALL,
	TOKEN_DOT:       PREC_CALL,
	TOKEN_IN:        PREC_LESS_GREATER,
}

func (p *Parser) parseExpression() Node {
	return p.parseExpressionPrecedence(PREC_LOWEST)
}

func (p *Parser) parseExpressionPrecedence(prec precedence) Node {
	tok := p.cur()
	var left Node
	if prefixFn := p.getPrefixFn(tok.Type); prefixFn != nil {
		left = prefixFn()
	} else {
		p.parseError("unexpected token '" + tok.Literal + "' as prefix")
		return nil
	}

	for !p.isEOF() && p.cur().Type != TOKEN_NEWLINE && p.cur().Type != TOKEN_DEDENT {
		infixTok := p.cur()
		infixPrec := precedences[infixTok.Type]
		if infixPrec <= prec {
			break
		}
		if infixFn := p.getInfixFn(infixTok.Type); infixFn != nil {
			left = infixFn(left)
		} else {
			break
		}
	}
	return left
}

func (p *Parser) getPrefixFn(tt TokenType) func() Node {
	switch tt {
	case TOKEN_INT:
		return func() Node {
			tok := p.cur()
			p.advance()
			return &IntLiteral{Tok: tok, Value: parseInt(tok.Literal)}
		}
	case TOKEN_FLOAT:
		return func() Node {
			tok := p.cur()
			p.advance()
			return &FloatLiteral{Tok: tok, Value: parseFloat(tok.Literal)}
		}
	case TOKEN_STRING:
		return func() Node {
			tok := p.cur()
			p.advance()
			return &StringLiteral{Tok: tok, Value: tok.Literal}
		}
	case TOKEN_CHAR:
		return func() Node {
			tok := p.cur()
			p.advance()
			return &CharLiteral{Tok: tok, Value: rune(tok.Literal[0])}
		}
	case TOKEN_TRUE:
		return func() Node {
			tok := p.cur()
			p.advance()
			return &BoolLiteral{Tok: tok, Value: true}
		}
	case TOKEN_FALSE:
		return func() Node {
			tok := p.cur()
			p.advance()
			return &BoolLiteral{Tok: tok, Value: false}
		}
	case TOKEN_IDENT, TOKEN_SELF:
		return func() Node {
			tok := p.cur()
			p.advance()

			// Template instantiation: Type<Args> or func<Args>
			if p.cur().Type == TOKEN_LT {
				shouldTry := p.isTemplateName(tok.Literal)
				if !shouldTry && p.pos+1 < len(p.tokens) {
					next := p.tokens[p.pos+1]
					shouldTry = next.Type == TOKEN_GT || IsTypeToken(next.Type)
				}
				if shouldTry {
					if typeArgs := p.tryParseTemplateArgTypes(); typeArgs != nil {
						inst := &TemplateInstantiation{Tok: tok, Name: tok.Literal, TypeArgs: typeArgs}

						// Struct literal with template type: Box<int>{...}
						if p.cur().Type == TOKEN_LBRACE {
							p.advance()
							var fields []*FieldInit
							for p.cur().Type != TOKEN_RBRACE && !p.isEOF() {
								fTok := p.cur()
								fName := p.expect(TOKEN_IDENT).Literal
								p.expect(TOKEN_COLON)
								val := p.parseExpression()
								fields = append(fields, &FieldInit{Tok: fTok, Name: fName, Value: val})
								if p.cur().Type == TOKEN_COMMA {
									p.advance()
								}
							}
							p.expect(TOKEN_RBRACE)
							fullType := inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"
							return &StructLiteral{Tok: tok, TypeName: fullType, Fields: fields}
						}

						return inst
					}
				}
			}

			if p.cur().Type == TOKEN_LBRACE {
				p.advance()
				var fields []*FieldInit
				for p.cur().Type != TOKEN_RBRACE && !p.isEOF() {
					fTok := p.cur()
					name := p.expect(TOKEN_IDENT).Literal
					p.expect(TOKEN_COLON)
					val := p.parseExpression()
					fields = append(fields, &FieldInit{Tok: fTok, Name: name, Value: val})
					if p.cur().Type == TOKEN_COMMA {
						p.advance()
					}
				}
				p.expect(TOKEN_RBRACE)
				return &StructLiteral{Tok: tok, TypeName: tok.Literal, Fields: fields}
			}
			return &Identifier{Tok: tok, Value: tok.Literal}
		}
	case TOKEN_LPAREN:
		return func() Node {
			p.advance()
			isVolatile := false
			if p.cur().Type == TOKEN_VOLATILE {
				isVolatile = true
				p.advance()
			}
			if IsTypeToken(p.cur().Type) || isVolatile || p.cur().Type == TOKEN_AMPERSAND {
				if !isVolatile && p.cur().Type == TOKEN_IDENT && p.tokens[p.pos+1].Type != TOKEN_RPAREN {
					expr := p.parseExpression()
					p.expect(TOKEN_RPAREN)
					return expr
				}
				tok := p.cur()
				isPtr := false
				if tok.Type == TOKEN_AMPERSAND {
					isPtr = true
					p.advance()
					if !IsTypeToken(p.cur().Type) {
						p.parseError("expected type after & in cast")
					}
				}
				typeName := p.parseTypeName()
				p.expect(TOKEN_RPAREN)
				operand := p.parseExpressionPrecedence(PREC_PREFIX)
				return &CastExpr{Tok: tok, TargetType: typeName, IsPointer: isPtr, IsVolatile: isVolatile, Operand: operand}
			}
			expr := p.parseExpression()
			p.expect(TOKEN_RPAREN)
			return expr
		}
	case TOKEN_NOT, TOKEN_MINUS:
		return func() Node {
			tok := p.cur()
			p.advance()
			operand := p.parseExpressionPrecedence(PREC_PREFIX)
			return &UnaryExpr{Tok: tok, Op: tok.Literal, Operand: operand}
		}
	case TOKEN_AMPERSAND:
		return func() Node {
			tok := p.cur()
			p.advance()
			operand := p.parseExpressionPrecedence(PREC_PREFIX)
			return &AddressOf{Tok: tok, Operand: operand}
		}
	case TOKEN_STAR:
		return func() Node {
			tok := p.cur()
			p.advance()
			operand := p.parseExpressionPrecedence(PREC_PREFIX)
			return &Deref{Tok: tok, Operand: operand}
		}
	case TOKEN_INT_KW, TOKEN_INT8, TOKEN_INT16, TOKEN_INT32, TOKEN_INT64,
		TOKEN_UINT8, TOKEN_UINT16, TOKEN_UINT32, TOKEN_UINT64,
		TOKEN_FLOAT_KW, TOKEN_FLOAT32, TOKEN_FLOAT64,
		TOKEN_STRING_KW, TOKEN_CHAR_KW, TOKEN_BOOL:
		return func() Node {
			tok := p.cur()
			p.advance()
			if p.cur().Type == TOKEN_LPAREN {
				p.advance()
				operand := p.parseExpression()
				p.expect(TOKEN_RPAREN)
				return &ConvExpr{Tok: tok, TargetType: tok.Literal, Operand: operand}
			}
			return &Identifier{Tok: tok, Value: tok.Literal}
		}
	case TOKEN_LBRACKET:
		return func() Node { return p.parseArrayLiteralNode() }
	case TOKEN_LBRACE:
		return func() Node {
			p.parseError("standalone { literal is not supported (use TypeName{...} for structs)")
			return nil
		}
	}
	return nil
}

func (p *Parser) getInfixFn(tt TokenType) func(Node) Node {
	switch tt {
	case TOKEN_PLUS, TOKEN_MINUS, TOKEN_STAR, TOKEN_SLASH, TOKEN_PERCENT,
		TOKEN_AMPERSAND, TOKEN_PIPE, TOKEN_CARET, TOKEN_EQ, TOKEN_NEQ,
		TOKEN_LT, TOKEN_GT, TOKEN_LTE, TOKEN_GTE, TOKEN_LSHIFT, TOKEN_RSHIFT,
		TOKEN_AND, TOKEN_OR, TOKEN_IN:
		return func(left Node) Node {
			tok := p.cur()
			p.advance()
			prec := precedences[tok.Type]
			right := p.parseExpressionPrecedence(prec)
			return &BinaryExpr{Tok: tok, Left: left, Op: tok.Literal, Right: right}
		}
	case TOKEN_LBRACKET:
		return func(left Node) Node {
			tok := p.cur()
			p.advance()
			if p.cur().Type == TOKEN_COLON {
				p.advance()
				var high Node
				if p.cur().Type != TOKEN_RBRACKET {
					high = p.parseExpression()
				}
				p.expect(TOKEN_RBRACKET)
				return &SliceExpr{Tok: tok, Collection: left, Low: nil, High: high}
			}
			idx := p.parseExpression()
			if p.cur().Type == TOKEN_COLON {
				p.advance()
				var high Node
				if p.cur().Type != TOKEN_RBRACKET {
					high = p.parseExpression()
				}
				p.expect(TOKEN_RBRACKET)
				return &SliceExpr{Tok: tok, Collection: left, Low: idx, High: high}
			}
			p.expect(TOKEN_RBRACKET)
			return &IndexExpr{Tok: tok, Collection: left, Index: idx}
		}
	case TOKEN_DOT:
		return func(left Node) Node {
			tok := p.cur()
			p.advance()
			field := p.expect(TOKEN_IDENT)
			if p.cur().Type == TOKEN_LT && p.isTemplateName(field.Literal) {
				save := p.pos
				p.advance()
				if p.cur().Type == TOKEN_GT || IsTypeToken(p.cur().Type) {
					p.pos = save
					typeArgs := p.parseTemplateArgTypes()
					return &TemplateInstantiation{
						Tok:      tok,
						Name:     field.Literal,
						TypeArgs: typeArgs,
						Object:   left,
					}
				}
				p.pos = save
			}
			return &SelectorExpr{Tok: tok, Object: left, Field: field.Literal}
		}
	case TOKEN_LPAREN:
		return func(left Node) Node {
			tok := p.cur()
			p.advance()
			var args []Node
			if p.cur().Type != TOKEN_RPAREN {
				for {
					args = append(args, p.parseExpression())
					if p.cur().Type == TOKEN_RPAREN {
						break
					}
					p.expect(TOKEN_COMMA)
				}
			}
			p.expect(TOKEN_RPAREN)
			return &CallExpr{Tok: tok, Function: left, Args: args}
		}
	}
	return nil
}
