package main

import (
	"fmt"
	"strings"
)

// ===========================================================================
// Parser
// ===========================================================================

type Parser struct {
	tokens        []Token
	pos           int
	source        string
	templateNames map[string]int // template name -> number of type parameters
	loopDepth     int            // tracks nesting inside for loops
	errors        []string
	filePath      string
}

func NewParser(tokens []Token, source string) *Parser {
	return &Parser{tokens: tokens, pos: 0, source: source, templateNames: make(map[string]int)}
}

func NewParserWithFile(tokens []Token, source string, filePath string) *Parser {
	return &Parser{tokens: tokens, pos: 0, source: source, templateNames: make(map[string]int), filePath: filePath}
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

func (p *Parser) ParseProgram() *Program {
	prog := &Program{}
	p.skipNewlines()

	for !p.isEOF() {
		switch p.cur().Type {
		case TOKEN_AT:
			p.advance()
			imp := p.parseImport()
			imp.Global = true
			prog.Imports = append(prog.Imports, imp)
		case TOKEN_IMPORT:
			prog.Imports = append(prog.Imports, p.parseImport())
		case TOKEN_EXTERNAL:
			prog.Statements = append(prog.Statements, p.parseExternalFunc())
		case TOKEN_TYPE:
			prog.Statements = append(prog.Statements, p.parseTypeAlias())
		case TOKEN_STRUCT:
			prog.Statements = append(prog.Statements, p.parseStruct())
		case TOKEN_ENUM:
			prog.Statements = append(prog.Statements, p.parseEnumDecl())
		case TOKEN_TEMPLATE:
			prog.Statements = append(prog.Statements, p.parseTemplateDecl())
		case TOKEN_CBLOCK:
			prog.Statements = append(prog.Statements, p.parseCBlock())
		case TOKEN_FOR:
			prog.Statements = append(prog.Statements, p.parseFor())
		case TOKEN_IF:
			prog.Statements = append(prog.Statements, p.parseIf())
		case TOKEN_MATCH:
			prog.Statements = append(prog.Statements, p.parseMatch())
		case TOKEN_RETURN:
			prog.Statements = append(prog.Statements, p.parseReturn())
		case TOKEN_BREAK:
			tok := p.cur(); p.advance(); p.expectNewline()
			prog.Statements = append(prog.Statements, &BreakStmt{Tok: tok})
		case TOKEN_CONTINUE:
			tok := p.cur(); p.advance(); p.expectNewline()
			prog.Statements = append(prog.Statements, &ContinueStmt{Tok: tok})
		case TOKEN_PASS:
			tok := p.cur(); p.advance(); p.expectNewline()
			prog.Statements = append(prog.Statements, &PassStmt{Tok: tok})
		case TOKEN_DEFER:
			tok := p.cur(); p.advance()
			call := p.parseExpression()
			p.expectNewline()
			prog.Statements = append(prog.Statements, &DeferStmt{Tok: tok, Call: call})
		case TOKEN_RAISE:
			prog.Statements = append(prog.Statements, p.parseRaise())
		case TOKEN_TRY:
			prog.Statements = append(prog.Statements, p.parseTryCatch())
		case TOKEN_ASM:
			prog.Statements = append(prog.Statements, p.parseAsm())
		case TOKEN_PRIVATE:
			p.advance()
			switch p.cur().Type {
			case TOKEN_EXTERNAL:
				d := p.parseExternalFunc()
				d.IsPrivate = true
				prog.Statements = append(prog.Statements, d)
			case TOKEN_TYPE:
				d := p.parseTypeAlias()
				d.IsPrivate = true
				prog.Statements = append(prog.Statements, d)
			case TOKEN_STRUCT:
				d := p.parseStruct()
				d.IsPrivate = true
				prog.Statements = append(prog.Statements, d)
			case TOKEN_ENUM:
				d := p.parseEnumDecl()
				d.IsPrivate = true
				prog.Statements = append(prog.Statements, d)
			case TOKEN_TEMPLATE:
				d := p.parseTemplateDecl()
				if fd, ok := d.Declaration.(*FuncDecl); ok {
					fd.IsPrivate = true
				} else if sd, ok := d.Declaration.(*StructDecl); ok {
					sd.IsPrivate = true
				}
				prog.Statements = append(prog.Statements, d)
			default:
				if p.isFuncDecl() {
					d := p.parseFuncDecl(false)
					d.IsPrivate = true
					prog.Statements = append(prog.Statements, d)
				} else if p.isVarDecl() {
					d := p.parseVarDecl()
					d.IsPrivate = true
					prog.Statements = append(prog.Statements, d)
				} else {
					p.parseError("expected declaration after 'private'")
				}
			}
		default:
			if p.cur().Type == TOKEN_CONST {
				if p.isVarDecl() {
					prog.Statements = append(prog.Statements, p.parseVarDecl())
				} else if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TOKEN_IDENT {
					constTok := p.cur()
					p.advance()
					decl := p.parseShortVarDecl()
					decl.Tok = constTok
					decl.IsConst = true
					prog.Statements = append(prog.Statements, decl)
				} else {
					prog.Statements = append(prog.Statements, p.parseAssignOrExpr())
				}
			} else if p.cur().Type == TOKEN_IDENT && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TOKEN_COMMA {
				prog.Statements = append(prog.Statements, p.parseMultiAssignOrShortDecl())
			} else if p.isFuncDecl() {
				prog.Statements = append(prog.Statements, p.parseFuncDecl(false))
			} else if p.cur().Type == TOKEN_IDENT && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TOKEN_DEFINE {
				prog.Statements = append(prog.Statements, p.parseShortVarDecl())
			} else if p.isVarDecl() {
				prog.Statements = append(prog.Statements, p.parseVarDecl())
			} else {
				prog.Statements = append(prog.Statements, p.parseAssignOrExpr())
			}
		}
		p.skipNewlines()
	}
	return prog
}

func (p *Parser) parseShortVarDecl() *ShortVarDecl {
	tok := p.cur()
	name := p.expect(TOKEN_IDENT)
	p.expect(TOKEN_DEFINE)
	value := p.parseExpression()
	p.expectNewline()
	return &ShortVarDecl{Tok: tok, Name: name.Literal, Value: value}
}

func (p *Parser) parseMultiAssignOrShortDecl() Node {
	var names []string
	tok := p.cur()
	names = append(names, p.expect(TOKEN_IDENT).Literal)
	for p.cur().Type == TOKEN_COMMA {
		p.advance()
		names = append(names, p.expect(TOKEN_IDENT).Literal)
	}
	if p.cur().Type == TOKEN_DEFINE {
		p.advance()
		var vals []Node
		vals = append(vals, p.parseExpression())
		for p.cur().Type == TOKEN_COMMA {
			p.advance()
			vals = append(vals, p.parseExpression())
		}
		p.expectNewline()
		return &MultiShortVarDecl{Tok: tok, Names: names, Values: vals}
	}
	if p.cur().Type == TOKEN_ASSIGN {
		p.advance()
		var targets []Node
		for _, name := range names {
			targets = append(targets, &Identifier{Tok: tok, Value: name})
		}
		names = nil
		var vals []Node
		vals = append(vals, p.parseExpression())
		for p.cur().Type == TOKEN_COMMA {
			p.advance()
			vals = append(vals, p.parseExpression())
		}
		p.expectNewline()
		return &MultiAssignStmt{Tok: tok, Targets: targets, Values: vals}
	}
	p.parseError("expected ':=' or '=' after identifier list")
	return nil
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

func (p *Parser) parseImport() *ImportDecl {
	tok := p.expect(TOKEN_IMPORT)
	pathTok := p.expect(TOKEN_STRING)
	p.expectNewline()
	return &ImportDecl{Tok: tok, Path: pathTok.Literal}
}

func (p *Parser) parseExternalFunc() *ExternalFuncDecl {
	tok := p.expect(TOKEN_EXTERNAL)
	retPtr := false
	if p.cur().Type == TOKEN_AMPERSAND {
		retPtr = true
		p.advance()
	}
	retType := p.parseTypeName()
	name := p.expect(TOKEN_IDENT)
	p.expect(TOKEN_LPAREN)
	params := p.parseParams()
	p.expect(TOKEN_RPAREN)
	
	linkLib := ""
	if p.cur().Type == TOKEN_LINK {
		p.advance()
		libTok := p.expect(TOKEN_STRING)
		linkLib = libTok.Literal
	}
	
	p.expectNewline()

	return &ExternalFuncDecl{
		Tok:        tok,
		ReturnType: retType,
		RetPointer: retPtr,
		Name:       name.Literal,
		Params:     params,
		LinkLib:    linkLib,
	}
}

// ---------------------------------------------------------------------------
// Type alias
// ---------------------------------------------------------------------------

func (p *Parser) parseTypeAlias() *TypeAliasDecl {
	tok := p.expect(TOKEN_TYPE)
	name := p.expect(TOKEN_IDENT)
	baseType := p.parseTypeName()
	decl := &TypeAliasDecl{Tok: tok, Name: name.Literal, BaseType: baseType}
	p.expectNewline()
	return decl
}

// ---------------------------------------------------------------------------
// Struct
// ---------------------------------------------------------------------------

func (p *Parser) parseStruct() *StructDecl {
	tok := p.expect(TOKEN_STRUCT)
	name := p.expect(TOKEN_IDENT)
	p.expect(TOKEN_COLON)
	p.expectNewline()
	p.expect(TOKEN_INDENT)

	decl := &StructDecl{Tok: tok, Name: name.Literal}

	for !p.isEOF() && p.cur().Type != TOKEN_DEDENT {
		p.skipNewlines()
		if p.cur().Type == TOKEN_DEDENT {
			break
		}
		// Template method: template<U> retType name(...):
		if p.cur().Type == TOKEN_TEMPLATE {
			tmpl := p.parseTemplateDecl()
			if fd, ok := tmpl.Declaration.(*FuncDecl); ok {
				fd.IsMethod = true
				decl.Methods = append(decl.Methods, fd)
			} else {
				p.parseError("expected method after template<...> inside struct")
			}
			continue
		}
		// Method declaration inside struct
		if p.isFuncDecl() {
			m := p.parseFuncDecl(true)
			decl.Methods = append(decl.Methods, m)
		} else {
			// Field declaration
			f := p.parseFieldDecl()
			decl.Fields = append(decl.Fields, f)
		}
	}

	p.expect(TOKEN_DEDENT)
	return decl
}

func (p *Parser) parseEnumDecl() *EnumDecl {
	tok := p.expect(TOKEN_ENUM)
	name := p.expect(TOKEN_IDENT)
	p.expect(TOKEN_COLON)
	p.expectNewline()
	p.expect(TOKEN_INDENT)

	decl := &EnumDecl{Tok: tok, Name: name.Literal}

	for !p.isEOF() && p.cur().Type != TOKEN_DEDENT {
		p.skipNewlines()
		if p.cur().Type == TOKEN_DEDENT {
			break
		}
		isPrivate := false
		if p.cur().Type == TOKEN_PRIVATE {
			isPrivate = true
			p.advance()
		}
		valTok := p.expect(TOKEN_IDENT)
		ev := &EnumValueDecl{Tok: valTok, Name: valTok.Literal, IsPrivate: isPrivate}
		if p.cur().Type == TOKEN_ASSIGN {
			p.advance()
			ev.Value = p.parseExpression()
		}
		p.expectNewline()
		decl.Values = append(decl.Values, ev)
	}

	p.expect(TOKEN_DEDENT)
	return decl
}

func (p *Parser) parseFieldDecl() *FieldDecl {
	tok := p.cur()
	volatile := false
	isPrivate := false
	if p.cur().Type == TOKEN_PRIVATE {
		isPrivate = true
		p.advance()
	}
	if p.cur().Type == TOKEN_VOLATILE {
		volatile = true
		p.advance()
	}
	isPtr := false
	if p.cur().Type == TOKEN_AMPERSAND {
		isPtr = true
		p.advance()
	}
	typeName := p.parseTypeName()
	name := p.expect(TOKEN_IDENT)
	p.expectNewline()
	return &FieldDecl{
		Tok:        tok,
		Name:       name.Literal,
		TypeAnnot:  typeName,
		IsPointer:  isPtr,
		IsVolatile: volatile,
		IsPrivate:  isPrivate,
	}
}

// ---------------------------------------------------------------------------
// Template declaration
// ---------------------------------------------------------------------------

// parseTemplateDecl handles: template<T>  (then struct/func on next line)
func (p *Parser) parseTemplateDecl() *TemplateDecl {
	tok := p.expect(TOKEN_TEMPLATE)
	p.expect(TOKEN_LT)
	var params []*TypeParam
	for !p.isEOF() && p.cur().Type != TOKEN_GT {
		paramTok := p.expect(TOKEN_IDENT)
		params = append(params, &TypeParam{Tok: paramTok, Name: paramTok.Literal})
		if p.cur().Type == TOKEN_COMMA {
			p.advance()
		}
	}
	p.expect(TOKEN_GT)
	p.expectNewline()

	// Register template name for disambiguation later
	var decl Node
	var templName string
	if p.cur().Type == TOKEN_STRUCT {
		s := p.parseStruct()
		templName = s.Name
		decl = s
	} else if p.isFuncDecl() {
		f := p.parseFuncDecl(false)
		f.TypeParams = params // attach template params to the function
		templName = f.Name
		decl = f
	} else {
		p.parseError("expected struct or function declaration after template<...>")
	}
	p.templateNames[templName] = len(params)

	return &TemplateDecl{Tok: tok, TypeParams: params, Declaration: decl}
}

// parseTemplateArgsInType parses <Type1, Type2, ...> in a type position
// and returns the full type string like "Box<int>"
func (p *Parser) parseTemplateArgsInType(baseName string) string {
	p.expect(TOKEN_LT)
	var args []string
	for !p.isEOF() && p.cur().Type != TOKEN_GT && p.cur().Type != TOKEN_RSHIFT {
		args = append(args, p.parseTypeName())
		if p.cur().Type == TOKEN_COMMA {
			p.advance()
		}
	}
	if p.cur().Type == TOKEN_RSHIFT {
		p.advance()
		// RSHIFT provides two > tokens. Insert a synthetic GT
		// so outer template levels also see a closing >.
		gt := Token{Type: TOKEN_GT, Literal: ">"}
		p.tokens = append(p.tokens[:p.pos], append([]Token{gt}, p.tokens[p.pos:]...)...)
	} else {
		p.expect(TOKEN_GT)
	}
	return baseName + "<" + strings.Join(args, ", ") + ">"
}

// parseTemplateArgTypes parses <Type1, Type2, ...> for expression-context
// template instantiation and returns the type argument strings.
func (p *Parser) parseTemplateArgTypes() []string {
	p.expect(TOKEN_LT)
	var args []string
	for !p.isEOF() && p.cur().Type != TOKEN_GT && p.cur().Type != TOKEN_RSHIFT {
		args = append(args, p.parseTypeName())
		if p.cur().Type == TOKEN_COMMA {
			p.advance()
		}
	}
	if p.cur().Type == TOKEN_RSHIFT {
		p.advance()
		// RSHIFT provides two > tokens. Insert a synthetic GT
		// so outer template levels also see a closing >.
		gt := Token{Type: TOKEN_GT, Literal: ">"}
		p.tokens = append(p.tokens[:p.pos], append([]Token{gt}, p.tokens[p.pos:]...)...)
	} else {
		p.expect(TOKEN_GT)
	}
	return args
}

// tryParseTemplateArgTypes speculatively tries to parse <Type1, Type2, ...>
// in expression context. Returns nil if the tokens don't look like template args.
// Unlike parseTemplateArgTypes, this does NOT call parseError/exit on failure.
func (p *Parser) tryParseTemplateArgTypes() []string {
	if p.cur().Type != TOKEN_LT { return nil }
	save := p.pos
	p.advance() // consume <

	// The first token must be a type keyword (or > for closing immediately)
	if p.cur().Type == TOKEN_GT {
		p.advance()
		return []string{}
	}
	if !IsTypeToken(p.cur().Type) {
		p.pos = save
		return nil
	}

	var args []string
	for !p.isEOF() {
		typeName, ok := p.tryParseTypeName()
		if !ok {
			p.pos = save
			return nil
		}
		args = append(args, typeName)
		if p.cur().Type == TOKEN_COMMA {
			p.advance()
		} else if p.cur().Type == TOKEN_GT {
			p.advance()
			return args
		} else {
			// Unexpected token — not template args
			p.pos = save
			return nil
		}
	}
	p.pos = save
	return nil
}

// tryParseTypeName attempts to parse a single type name without calling parseError.
// Returns the type name string and true on success, or ("", false) on failure.
func (p *Parser) tryParseTypeName() (string, bool) {
	save := p.pos
	tok := p.cur()
	if !IsTypeToken(tok.Type) {
		return "", false
	}
	p.advance()

	// string[N] sizing
	if tok.Type == TOKEN_STRING_KW && p.cur().Type == TOKEN_LBRACKET {
		p.advance()
		if p.cur().Type != TOKEN_INT {
			p.pos = save
			return "", false
		}
		size := p.cur().Literal
		p.advance()
		if p.cur().Type != TOKEN_RBRACKET {
			p.pos = save
			return "", false
		}
		p.advance()
		return fmt.Sprintf("string[%s]", size), true
	}

	// Template instantiation: Type<Arg1, Arg2>
	if p.cur().Type == TOKEN_LT {
		p.advance()
		name := tok.Literal + "<"
		depth := 1
		for !p.isEOF() && depth > 0 {
			if p.cur().Type == TOKEN_LT {
				depth++
				name += "<"
				p.advance()
			} else if p.cur().Type == TOKEN_GT {
				depth--
				if depth > 0 {
					name += ">"
				}
				p.advance()
			} else if p.cur().Type == TOKEN_RSHIFT && depth >= 1 {
				depth -= 2
				name += ">"
				if depth >= 0 {
					name += ">"
				}
				p.advance()
			} else if p.cur().Type == TOKEN_COMMA {
				name += ", "
				p.advance()
			} else if IsTypeToken(p.cur().Type) {
				arg, ok := p.tryParseTypeName()
				if !ok {
					p.pos = save
					return "", false
				}
				name += arg
			} else {
				p.pos = save
				return "", false
			}
		}
		if depth != 0 {
			p.pos = save
			return "", false
		}
		return name, true
	}

	// Module-qualified type: module.Type
	if p.cur().Type == TOKEN_DOT {
		p.advance()
		if !IsTypeToken(p.cur().Type) {
			p.pos = save
			return "", false
		}
		name := tok.Literal + "." + p.cur().Literal
		p.advance()
		return name, true
	}

	return tok.Literal, true
}

// isTemplateName checks if the given identifier is a known template declaration.
func (p *Parser) isTemplateName(name string) bool {
	_, ok := p.templateNames[name]
	return ok
}

// ---------------------------------------------------------------------------
// Function / method declaration
// ---------------------------------------------------------------------------

// isFuncDecl peeks ahead to decide whether the current position is a function
// declaration. A function declaration begins with a return type followed by an
// identifier followed by '('.
func (p *Parser) isFuncDecl() bool {
	save := p.pos
	defer func() { p.pos = save }()

	// Skip optional volatile / pointer prefix (either order)
	if p.cur().Type == TOKEN_VOLATILE {
		p.advance()
		if p.cur().Type == TOKEN_AMPERSAND {
			p.advance()
		}
	} else if p.cur().Type == TOKEN_AMPERSAND {
		p.advance()
		if p.cur().Type == TOKEN_VOLATILE {
			p.advance()
		}
	}
	if !IsTypeToken(p.cur().Type) {
		return false
	}
	p.advance()
	// Skip optional [N] on string
	if p.cur().Type == TOKEN_LBRACKET {
		for !p.isEOF() && p.cur().Type != TOKEN_RBRACKET {
			p.advance()
		}
		if !p.isEOF() {
			p.advance()
		}
	}
	// Skip optional template type args: <Type1, Type2> with nesting
	if p.cur().Type == TOKEN_LT {
		depth := 1
		p.advance()
		for !p.isEOF() && depth > 0 {
			if p.cur().Type == TOKEN_LT {
				depth++
			} else if p.cur().Type == TOKEN_GT {
				depth--
			} else if p.cur().Type == TOKEN_RSHIFT && depth >= 1 {
				depth -= 2
			}
			if depth > 0 {
				p.advance()
			} else {
				p.advance()
				break
			}
		}
	}
	// Multi-return: int, string name(
	for p.cur().Type == TOKEN_COMMA {
		p.advance()
		if !IsTypeToken(p.cur().Type) {
			return false
		}
		p.advance()
		// Skip optional [N]
		if p.cur().Type == TOKEN_LBRACKET {
			for !p.isEOF() && p.cur().Type != TOKEN_RBRACKET {
				p.advance()
			}
			if !p.isEOF() {
				p.advance()
			}
		}
		// Skip template args on subsequent types
		if p.cur().Type == TOKEN_LT {
			depth := 1
			p.advance()
			for !p.isEOF() && depth > 0 {
				if p.cur().Type == TOKEN_LT {
					depth++
				} else if p.cur().Type == TOKEN_GT {
					depth--
				} else if p.cur().Type == TOKEN_RSHIFT && depth >= 1 {
					depth -= 2
				}
				if depth > 0 {
					p.advance()
				} else {
					p.advance()
					break
				}
			}
		}
	}
	if p.cur().Type != TOKEN_IDENT {
		return false
	}
	p.advance()
	return p.cur().Type == TOKEN_LPAREN
}

func (p *Parser) parseFuncDecl(isMethod bool) *FuncDecl {
	tok := p.cur()
	retPtr := false
	if p.cur().Type == TOKEN_VOLATILE {
		p.advance()
		if p.cur().Type == TOKEN_AMPERSAND {
			retPtr = true
			p.advance()
		}
	} else if p.cur().Type == TOKEN_AMPERSAND {
		retPtr = true
		p.advance()
		if p.cur().Type == TOKEN_VOLATILE {
			p.advance()
		}
	}
	retType := p.parseTypeName()
	var retTypes []string
	retTypes = append(retTypes, retType)
	for p.cur().Type == TOKEN_COMMA {
		p.advance()
		nextType := p.parseTypeName()
		retTypes = append(retTypes, nextType)
	}
	name := p.expect(TOKEN_IDENT)
	p.expect(TOKEN_LPAREN)
	params := p.parseParams()
	p.expect(TOKEN_RPAREN)
	throws := false
	if p.cur().Type == TOKEN_THROWS {
		throws = true
		p.advance()
	}
	p.expect(TOKEN_COLON)
	p.expectNewline()
	body := p.parseBlock()

	return &FuncDecl{
		Tok:         tok,
		ReturnType:  retType,
		ReturnTypes: retTypes,
		RetPointer:  retPtr,
		Name:        name.Literal,
		Params:      params,
		Body:        body,
		IsMethod:    isMethod,
		Throws:      throws,
	}
}

func (p *Parser) parseParams() []*Param {
	var params []*Param
	first := true
	for p.cur().Type != TOKEN_RPAREN && !p.isEOF() {
		if !first {
			p.expect(TOKEN_COMMA)
		}
		first = false
		tok := p.cur()
		volatile := false
		if p.cur().Type == TOKEN_VOLATILE {
			volatile = true
			p.advance()
		}
		isPtr := false
		if p.cur().Type == TOKEN_AMPERSAND {
			isPtr = true
			p.advance()
		}
		typeName := p.parseTypeName()
		name := p.expectAny([]TokenType{TOKEN_IDENT, TOKEN_SELF})
		param := &Param{
			Tok:        tok,
			Name:       name.Literal,
			TypeAnnot:  typeName,
			IsPointer:  isPtr,
			IsVolatile: volatile,
		}
		if p.cur().Type == TOKEN_ASSIGN {
			p.advance()
			param.Default = p.parseExpression()
		}
		params = append(params, param)
	}
	return params
}

// ---------------------------------------------------------------------------
// Variable declaration
// ---------------------------------------------------------------------------

func (p *Parser) parseVarDecl() *VarDecl {
	tok := p.cur()
	isConst := false
	volatile := false
	isPtr := false
	if p.cur().Type == TOKEN_CONST {
		isConst = true
		p.advance()
		tok = p.cur()
	}
	if p.cur().Type == TOKEN_VOLATILE {
		volatile = true
		p.advance()
		if p.cur().Type == TOKEN_AMPERSAND {
			isPtr = true
			p.advance()
		}
	} else if p.cur().Type == TOKEN_AMPERSAND {
		isPtr = true
		p.advance()
		if p.cur().Type == TOKEN_VOLATILE {
			volatile = true
			p.advance()
		}
	}
	typeName := p.parseTypeName()
	arraySize := ""

	// Explicit size annotation: int name[512] = ...
	// (separate from string[N] which is encoded in typeName itself)
	name := p.expect(TOKEN_IDENT)
	if p.cur().Type == TOKEN_LBRACKET {
		p.advance()
		sizeTok := p.expect(TOKEN_INT)
		arraySize = sizeTok.Literal
		p.expect(TOKEN_RBRACKET)
	}

	var value Node
	if p.cur().Type == TOKEN_ASSIGN {
		p.advance()
		value = p.parseExpression()
	}
	p.expectNewline()

	return &VarDecl{
		Tok:        tok,
		Name:       name.Literal,
		TypeAnnot:  typeName,
		IsPointer:  isPtr,
		IsVolatile: volatile,
		IsConst:    isConst,
		ArraySize:  arraySize,
		Value:      value,
	}
}

// ---------------------------------------------------------------------------
// Block
// ---------------------------------------------------------------------------

func (p *Parser) parseBlock() *BlockStmt {
	tok := p.cur()
	p.expect(TOKEN_INDENT)
	block := &BlockStmt{Tok: tok}

	for !p.isEOF() && p.cur().Type != TOKEN_DEDENT {
		p.skipNewlines()
		if p.cur().Type == TOKEN_DEDENT || p.isEOF() {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
	}

	p.expect(TOKEN_DEDENT)
	return block
}

// ---------------------------------------------------------------------------
// Statements
// ---------------------------------------------------------------------------

func (p *Parser) parseStatement() Node {
	switch p.cur().Type {
	case TOKEN_IF:
		return p.parseIf()
	case TOKEN_FOR:
		return p.parseFor()
	case TOKEN_MATCH:
		return p.parseMatch()
	case TOKEN_RETURN:
		return p.parseReturn()
	case TOKEN_BREAK:
		tok := p.cur()
		p.advance()
		p.expectNewline()
		return &BreakStmt{Tok: tok}
	case TOKEN_CONTINUE:
		tok := p.cur()
		p.advance()
		p.expectNewline()
		return &ContinueStmt{Tok: tok}
	case TOKEN_PASS:
		tok := p.cur()
		p.advance()
		p.expectNewline()
		return &PassStmt{Tok: tok}
	case TOKEN_DEFER:
		tok := p.cur()
		p.advance()
		call := p.parseExpression()
		p.expectNewline()
		return &DeferStmt{Tok: tok, Call: call}
	case TOKEN_RAISE:
		return p.parseRaise()
	case TOKEN_TRY:
		return p.parseTryCatch()
	case TOKEN_ASM:
		return p.parseAsm()
	case TOKEN_CBLOCK:
		return p.parseCBlock()
	case TOKEN_PRIVATE:
		p.advance()
		switch p.cur().Type {
		case TOKEN_STRUCT:
			d := p.parseStruct()
			d.IsPrivate = true
			return d
		case TOKEN_ENUM:
			d := p.parseEnumDecl()
			d.IsPrivate = true
			return d
		default:
			if p.isFuncDecl() {
				d := p.parseFuncDecl(false)
				d.IsPrivate = true
				return d
			} else if p.isVarDecl() {
				d := p.parseVarDecl()
				d.IsPrivate = true
				return d
			} else {
				p.parseError("expected declaration after 'private'")
				return nil
			}
		}
	default:
		// Const declaration
		if p.cur().Type == TOKEN_CONST {
			if p.isVarDecl() {
				return p.parseVarDecl()
			}
			if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TOKEN_IDENT {
				constTok := p.cur()
				p.advance()
				decl := p.parseShortVarDecl()
				decl.Tok = constTok
				decl.IsConst = true
				return decl
			}
		}
		// Multi-assign or multi-short-decl: x, s = ... or x, s := ...
		if p.cur().Type == TOKEN_IDENT && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TOKEN_COMMA {
			return p.parseMultiAssignOrShortDecl()
		}
		// Short variable declaration: name := expr
		if p.cur().Type == TOKEN_IDENT && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TOKEN_DEFINE {
			return p.parseShortVarDecl()
		}
		// Variable declaration or expression/assignment statement.
		if p.isFuncDecl() {
			return p.parseFuncDecl(false)
		}
		if p.isVarDecl() {
			return p.parseVarDecl()
		}
		return p.parseAssignOrExpr()
	}
}

func (p *Parser) parseIf() *IfStmt {
	tok := p.expect(TOKEN_IF)
	cond := p.parseExpression()
	p.expect(TOKEN_COLON)
	p.expectNewline()
	body := p.parseBlock()

	stmt := &IfStmt{Tok: tok, Condition: cond, Consequence: body}

	for p.cur().Type == TOKEN_ELIF {
		elifTok := p.cur()
		p.advance()
		elifCond := p.parseExpression()
		p.expect(TOKEN_COLON)
		p.expectNewline()
		elifBody := p.parseBlock()
		stmt.Alternatives = append(stmt.Alternatives, &ElifClause{
			Tok:       elifTok,
			Condition: elifCond,
			Body:      elifBody,
		})
	}

	if p.cur().Type == TOKEN_ELSE {
		p.advance()
		p.expect(TOKEN_COLON)
		p.expectNewline()
		stmt.Else = p.parseBlock()
	}

	return stmt
}

func (p *Parser) parseFor() Node {
	tok := p.expect(TOKEN_FOR)

	// Bare 'for:' — infinite loop
	if p.cur().Type == TOKEN_COLON {
		p.advance()
		p.expectNewline()
		p.loopDepth++
		body := p.parseBlock()
		p.loopDepth--
		return &ForInfiniteStmt{Tok: tok, Body: body}
	}

	// 'for i in range(a, b):'
	if p.cur().Type == TOKEN_IDENT && p.tokens[p.pos+1].Type == TOKEN_IN {
		varTok := p.cur()
		p.advance()          // consume ident
		p.expect(TOKEN_IN)
		p.expect(TOKEN_RANGE)
		p.expect(TOKEN_LPAREN)
		from := p.parseExpression()
		p.expect(TOKEN_COMMA)
		to := p.parseExpression()
		p.expect(TOKEN_RPAREN)
		p.expect(TOKEN_COLON)
		p.expectNewline()
		p.loopDepth++
		body := p.parseBlock()
		p.loopDepth--
		return &ForRangeStmt{Tok: tok, Var: varTok.Literal, From: from, To: to, Body: body}
	}

	// 'for <condition>:'
	cond := p.parseExpression()
	p.expect(TOKEN_COLON)
	p.expectNewline()
	p.loopDepth++
	body := p.parseBlock()
	p.loopDepth--
	return &ForCondStmt{Tok: tok, Condition: cond, Body: body}
}

func (p *Parser) parseMatch() *MatchStmt {
	tok := p.expect(TOKEN_MATCH)
	subject := p.parseExpression()
	p.expect(TOKEN_COLON)
	p.expectNewline()
	p.expect(TOKEN_INDENT)

	stmt := &MatchStmt{Tok: tok, Subject: subject}

	for !p.isEOF() && p.cur().Type != TOKEN_DEDENT {
		p.skipNewlines()
		if p.cur().Type == TOKEN_DEDENT {
			break
		}

		caseTok := p.expect(TOKEN_CASE)
		mc := &MatchCase{Tok: caseTok}

		if p.cur().Type == TOKEN_COLON {
			// catch-all: case:
			mc.IsCatchAll = true
			p.advance()
		} else {
			mc.Value = p.parseExpression()
			p.expect(TOKEN_COLON)
		}
		p.expectNewline()
		mc.Body = p.parseBlock()

		// If the last statement in the body is 'continue' and we're not
		// inside a loop, mark fall-through.
		if p.loopDepth == 0 && len(mc.Body.Statements) > 0 {
			last := mc.Body.Statements[len(mc.Body.Statements)-1]
			if _, ok := last.(*ContinueStmt); ok {
				mc.FallThrough = true
				mc.Body.Statements = mc.Body.Statements[:len(mc.Body.Statements)-1]
			}
		}

		stmt.Cases = append(stmt.Cases, mc)
	}

	p.expect(TOKEN_DEDENT)
	return stmt
}

func (p *Parser) parseReturn() *ReturnStmt {
	tok := p.expect(TOKEN_RETURN)
	if p.cur().Type == TOKEN_NEWLINE || p.cur().Type == TOKEN_DEDENT || p.isEOF() {
		p.expectNewline()
		return &ReturnStmt{Tok: tok}
	}
	var vals []Node
	vals = append(vals, p.parseExpression())
	for p.cur().Type == TOKEN_COMMA {
		p.advance()
		vals = append(vals, p.parseExpression())
	}
	p.expectNewline()
	return &ReturnStmt{Tok: tok, Values: vals}
}

func (p *Parser) parseAsm() *AsmBlock {
	tok := p.expect(TOKEN_ASM)
	p.expect(TOKEN_COLON)
	p.expectNewline()
	p.expect(TOKEN_INDENT)

	var lines []string
	for !p.isEOF() && p.cur().Type != TOKEN_DEDENT {
		// Collect the raw literal text of each token on the line.
		var line string
		for !p.isEOF() && p.cur().Type != TOKEN_NEWLINE && p.cur().Type != TOKEN_DEDENT {
			if line != "" {
				line += " "
			}
			line += p.cur().Literal
			p.advance()
		}
		lines = append(lines, line)
		p.skipNewlines()
	}

	p.expect(TOKEN_DEDENT)
	return &AsmBlock{Tok: tok, Lines: lines}
}

func (p *Parser) parseCBlock() *CBlock {
	tok := p.expect(TOKEN_CBLOCK)
	p.expect(TOKEN_COLON)
	p.expectNewline()
	p.expect(TOKEN_INDENT)

	// Consume the raw block body emitted by the lexer.
	rawTok := p.expect(TOKEN_CBLOCK_RAW)

	// Split into lines (as they were captured by the lexer).
	lines := strings.Split(rawTok.Literal, "\n")

	// Sync the DEDENT token that follows.
	for !p.isEOF() && p.cur().Type != TOKEN_DEDENT {
		p.advance()
	}
	if p.cur().Type == TOKEN_DEDENT {
		p.advance()
	}

	p.skipNewlines()

	return &CBlock{Tok: tok, Lines: lines}
}

func (p *Parser) parseRaise() *RaiseStmt {
	tok := p.expect(TOKEN_RAISE)
	val := p.parseExpression()
	p.expectNewline()
	return &RaiseStmt{Tok: tok, Value: val}
}

func (p *Parser) parseTryCatch() *TryCatchStmt {
	tok := p.expect(TOKEN_TRY)
	p.expect(TOKEN_COLON)
	p.expectNewline()
	body := p.parseBlock()

	stmt := &TryCatchStmt{Tok: tok, Body: body}

	for p.cur().Type == TOKEN_CATCH {
		catchTok := p.cur()
		p.advance()
		cc := &CatchClause{Tok: catchTok}
		if p.cur().Type == TOKEN_COLON {
			cc.IsCatchAll = true
			p.advance()
		} else {
			cc.Value = p.parseExpression()
			p.expect(TOKEN_COLON)
		}
		p.expectNewline()
		cc.Body = p.parseBlock()
		stmt.Catches = append(stmt.Catches, cc)
	}

	return stmt
}

// parseAssignOrExpr handles: assignment, compound assignment, or bare expression.
func (p *Parser) parseAssignOrExpr() Node {
	tok := p.cur()
	lhs := p.parseExpression()

	switch p.cur().Type {
	case TOKEN_ASSIGN:
		p.advance()
		rhs := p.parseExpression()
		p.expectNewline()
		return &AssignStmt{Tok: tok, Target: lhs, Value: rhs}
	case TOKEN_PLUS_ASSIGN, TOKEN_MINUS_ASSIGN, TOKEN_STAR_ASSIGN,
		TOKEN_SLASH_ASSIGN, TOKEN_MOD_ASSIGN:
		op := p.cur().Literal
		p.advance()
		rhs := p.parseExpression()
		p.expectNewline()
		return &CompoundAssignStmt{Tok: tok, Target: lhs, Op: op, Value: rhs}
	default:
		p.expectNewline()
		return &ExprStmt{Tok: tok, Expression: lhs}
	}
}

// ---------------------------------------------------------------------------
// Expression parsing — stub
// Emits a stub node. OpenCode replaces this with a full Pratt parser.
// ---------------------------------------------------------------------------

// parseExpression is implemented in eval.go using a Pratt parser.


// parsePrimary is no longer needed as it's integrated into the Pratt parser in eval.go.

func (p *Parser) parseArrayLiteralNode() *ArrayLiteral {
	tok := p.expect(TOKEN_LBRACKET)
	var elems []Node
	for p.cur().Type != TOKEN_RBRACKET && !p.isEOF() {
		elems = append(elems, p.parseExpression())
		if p.cur().Type == TOKEN_COMMA {
			p.advance()
		}
	}
	p.expect(TOKEN_RBRACKET)
	return &ArrayLiteral{Tok: tok, Elements: elems}
}


// ---------------------------------------------------------------------------
// Type name parsing
// ---------------------------------------------------------------------------

// parseTypeName reads a full type annotation string, including optional pointer
// prefix (already consumed by caller) and string[N] sizing.
// Returns the type as a plain string (e.g. "uint8", "string[512]", "int32").
func (p *Parser) parseTypeName() string {
	tok := p.cur()
	if !IsTypeToken(tok.Type) {
		p.parseError(fmt.Sprintf("expected type name, got '%s'", tok.Literal))
	}
	name := tok.Literal
	p.advance()

	// Template instantiation: Type<Arg1, Arg2>
	if p.cur().Type == TOKEN_LT && (p.isTemplateName(name) || tok.Type == TOKEN_IDENT) {
		name = p.parseTemplateArgsInType(name)
	}

	// Module-qualified type: module.Type or module.Type<Args>
	if p.cur().Type == TOKEN_DOT {
		p.advance()
		fieldTok := p.cur()
		if !IsTypeToken(fieldTok.Type) {
			p.parseError(fmt.Sprintf("expected type name after '.', got '%s'", fieldTok.Literal))
		}
		p.advance()
		name = name + "." + fieldTok.Literal
		if p.cur().Type == TOKEN_LT {
			return p.parseTemplateArgsInType(name)
		}
		return name
	}

	// Array type suffix: T[]
	if p.cur().Type == TOKEN_LBRACKET && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TOKEN_RBRACKET {
		p.advance() // consume [
		p.advance() // consume ]
		return name + "[]"
	}

	return name
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *Parser) isVarDecl() bool {
	save := p.pos
	defer func() { p.pos = save }()

	// Skip optional const prefix
	if p.cur().Type == TOKEN_CONST {
		p.advance()
	}

	// Skip optional volatile / pointer prefix (either order)
	if p.cur().Type == TOKEN_VOLATILE {
		p.advance()
		if p.cur().Type == TOKEN_AMPERSAND {
			p.advance()
		}
	} else if p.cur().Type == TOKEN_AMPERSAND {
		p.advance()
		if p.cur().Type == TOKEN_VOLATILE {
			p.advance()
		}
	}
	if !IsTypeToken(p.cur().Type) {
		return false
	}
	p.advance()
	// Handle module-qualified type: module.Type
	if p.cur().Type == TOKEN_DOT {
		p.advance()
		if !IsTypeToken(p.cur().Type) {
			return false
		}
		p.advance()
	}
	if p.cur().Type == TOKEN_LBRACKET {
		for !p.isEOF() && p.cur().Type != TOKEN_RBRACKET {
			p.advance()
		}
		if !p.isEOF() {
			p.advance()
		}
	}
	// Skip optional template type args: <Type1, Type2> with nesting
	if p.cur().Type == TOKEN_LT {
		depth := 1
		p.advance()
		for !p.isEOF() && depth > 0 {
			if p.cur().Type == TOKEN_LT {
				depth++
			} else if p.cur().Type == TOKEN_GT {
				depth--
			} else if p.cur().Type == TOKEN_RSHIFT && depth >= 1 {
				depth -= 2
			}
			if depth > 0 {
				p.advance()
			} else {
				p.advance()
				break
			}
		}
	}
	return p.cur().Type == TOKEN_IDENT
}

func (p *Parser) cur() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TOKEN_EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

func (p *Parser) isEOF() bool {
	return p.pos >= len(p.tokens) || p.tokens[p.pos].Type == TOKEN_EOF
}

func (p *Parser) expect(tt TokenType) Token {
	tok := p.cur()
	if tok.Type != tt {
		p.parseError(fmt.Sprintf("expected %s, got '%s'", tt, tok.Literal))
		p.advance()
		return Token{Type: tt, Literal: "", Line: tok.Line, Col: tok.Col}
	}
	p.advance()
	return tok
}

func (p *Parser) expectAny(types []TokenType) Token {
	tok := p.cur()
	for _, tt := range types {
		if tok.Type == tt {
			p.advance()
			return tok
		}
	}
	p.parseError(fmt.Sprintf("expected %s, got '%s'", types[0], tok.Literal))
	return tok
}

func (p *Parser) expectNewline() {
	if p.cur().Type == TOKEN_NEWLINE {
		p.advance()
		return
	}
	if p.cur().Type == TOKEN_DEDENT || p.cur().Type == TOKEN_EOF {
		return
	}
	p.parseError(fmt.Sprintf("expected newline, got '%s'", p.cur().Literal))
	if !p.isEOF() {
		p.advance()
	}
}

func (p *Parser) skipNewlines() {
	for p.cur().Type == TOKEN_NEWLINE {
		p.advance()
	}
}

func (p *Parser) parseError(msg string) {
	tok := p.cur()
	err := fmt.Sprintf("[merlin] ERROR (%s, line %d, col %d): %s", p.filePath, tok.Line, tok.Col, msg)
	p.errors = append(p.errors, err)
}
