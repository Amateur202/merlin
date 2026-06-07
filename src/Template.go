package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Template instantiation (monomorphization)
// ---------------------------------------------------------------------------

func (tc *TypeChecker) instantiateTemplate(tmpl *TemplateDecl, fullName string) {
	if _, ok := tc.typeTable[fullName]; ok {
		return
	}
	if _, ok := tc.symbolTable[fullName]; ok {
		return
	}

	typeArgs := extractTypeArgs(fullName)

	switch decl := tmpl.Declaration.(type) {
	case *StructDecl:
		tc.instantiateStructTemplate(tmpl, decl, fullName, typeArgs)
	case *FuncDecl:
		tc.instantiateFuncTemplate(tmpl, decl, fullName, typeArgs)
	}
}

func buildSubstMap(params []*TypeParam, typeArgs []string) map[string]string {
	m := make(map[string]string)
	for i, p := range params {
		if i < len(typeArgs) {
			m[p.Name] = typeArgs[i]
		}
	}
	return m
}

func substituteType(typeStr string, subst map[string]string) string {
	if s, ok := subst[typeStr]; ok {
		return s
	}
	if strings.HasSuffix(typeStr, "[]") {
		base := typeStr[:len(typeStr)-2]
		if s, ok := subst[base]; ok {
			return s + "[]"
		}
	}
	if strings.HasPrefix(typeStr, "&") {
		inner := typeStr[1:]
		if s, ok := subst[inner]; ok {
			return "&" + s
		}
	}
	if idx := strings.Index(typeStr, "<"); idx != -1 {
		base := typeStr[:idx]
		inner := typeStr[idx+1 : len(typeStr)-1]
		rawArgs := splitCommasAtDepth(inner)
		var newArgs []string
		for _, arg := range rawArgs {
			newArgs = append(newArgs, substituteType(strings.TrimSpace(arg), subst))
		}
		return base + "<" + strings.Join(newArgs, ", ") + ">"
	}
	return typeStr
}

func substituteFieldType(typeAnnot string, subst map[string]string) string {
	return substituteType(typeAnnot, subst)
}

func cloneStructWithSubst(decl *StructDecl, fullName string, tmpl *TemplateDecl, subst map[string]string) *StructDecl {
	concrete := &StructDecl{
		Tok:  decl.Tok,
		Name: fullName,
	}
	for _, f := range decl.Fields {
		concrete.Fields = append(concrete.Fields, &FieldDecl{
			Tok:        f.Tok,
			Name:       f.Name,
			TypeAnnot:  substituteFieldType(f.TypeAnnot, subst),
			IsPointer:  f.IsPointer,
			IsVolatile: f.IsVolatile,
		})
	}
	for _, m := range decl.Methods {
		if len(m.TypeParams) > 0 {
			continue
		}
		concreteMethod := &FuncDecl{
			Tok:        m.Tok,
			ReturnType: substituteType(m.ReturnType, subst),
			RetPointer: m.RetPointer,
			Name:       m.Name,
			Params:     cloneParamsWithSubst(m.Params, subst),
			Body:       substituteBlock(m.Body, subst),
			IsMethod:   true,
			Throws:     m.Throws,
		}
		concrete.Methods = append(concrete.Methods, concreteMethod)
	}
	return concrete
}

func cloneParamsWithSubst(params []*Param, subst map[string]string) []*Param {
	var result []*Param
	for _, p := range params {
		cp := &Param{
			Tok:        p.Tok,
			Name:       p.Name,
			TypeAnnot:  substituteType(p.TypeAnnot, subst),
			IsPointer:  p.IsPointer,
			IsVolatile: p.IsVolatile,
		}
		if p.Default != nil {
			cp.Default = substituteNode(p.Default, subst)
		}
		result = append(result, cp)
	}
	return result
}

func (tc *TypeChecker) instantiateStructTemplate(tmpl *TemplateDecl, decl *StructDecl, fullName string, typeArgs []string) {
	subst := buildSubstMap(tmpl.TypeParams, typeArgs)

	structType := &Type{Kind: TK_Struct, Name: fullName}
	tc.typeTable[fullName] = structType

	concrete := cloneStructWithSubst(decl, fullName, tmpl, subst)

	fields := make(map[string]*Type)
	for _, f := range concrete.Fields {
		ft := tc.resolveType(f.TypeAnnot)
		if ft == nil {
			tc.error(f.Tok, fmt.Sprintf("unknown field type '%s' for field '%s' in template instantiation '%s'", f.TypeAnnot, f.Name, fullName))
		}
		if f.IsPointer {
			ft = &Type{Kind: TK_Pointer, BaseType: ft}
		}
		fields[f.Name] = ft
	}
	tc.structFields[fullName] = fields

	methods := make(map[string]*Type)
	methodThrows := make(map[string]bool)
	methodDecls := make(map[string]*FuncDecl)
	for _, m := range concrete.Methods {
		mRet := tc.resolveType(m.ReturnType)
		if mRet == nil {
			tc.error(m.Tok, fmt.Sprintf("unknown return type '%s' for method '%s' in template instantiation '%s'", m.ReturnType, m.Name, fullName))
		}
		if m.RetPointer {
			mRet = &Type{Kind: TK_Pointer, BaseType: mRet}
		}
		methods[m.Name] = mRet
		methodThrows[m.Name] = m.Throws
		methodDecls[m.Name] = m

		if m.Body != nil {
			savedFunc := tc.currentFunc
			tc.currentFunc = m
			funcScope := make(map[string]*Symbol)
			for k, v := range tc.symbolTable {
				funcScope[k] = v
			}
			selfType := &Type{Kind: TK_Struct, Name: fullName}
			funcScope["self"] = &Symbol{Name: "self", Type: &Type{Kind: TK_Pointer, BaseType: selfType}, DeclaredType: "&" + fullName}
			for _, p := range m.Params {
				t := tc.resolveType(p.TypeAnnot)
				if t == nil {
					tc.error(p.Tok, fmt.Sprintf("unknown parameter type '%s' in method '%s' of template instantiation '%s'", p.TypeAnnot, m.Name, fullName))
				}
				if p.IsPointer {
					t = &Type{Kind: TK_Pointer, BaseType: t}
				}
				funcScope[p.Name] = &Symbol{Name: p.Name, Type: t, DeclaredType: p.TypeAnnot}
			}
			for _, s := range m.Body.Statements {
				tc.checkStatement(s, funcScope, false)
			}
			tc.currentFunc = savedFunc
		}
	}
	tc.structMethods[fullName] = methods
	tc.structMethodThrows[fullName] = methodThrows
	tc.structMethodDecls[fullName] = methodDecls

	tc.ConcreteDecls = append(tc.ConcreteDecls, concrete)
}

func (tc *TypeChecker) instantiateFuncTemplate(tmpl *TemplateDecl, decl *FuncDecl, fullName string, typeArgs []string) {
	subst := buildSubstMap(tmpl.TypeParams, typeArgs)

	var retTypes []string
	for _, rt := range decl.ReturnTypes {
		retTypes = append(retTypes, substituteType(rt, subst))
	}

	concrete := &FuncDecl{
		Tok:         decl.Tok,
		ReturnType:  substituteType(decl.ReturnType, subst),
		ReturnTypes: retTypes,
		RetPointer:  decl.RetPointer,
		Name:        fullName,
		Params:      cloneParamsWithSubst(decl.Params, subst),
		Body:        substituteBlock(decl.Body, subst),
		IsMethod:    decl.IsMethod,
		Throws:      decl.Throws,
	}
	tc.ConcreteDecls = append(tc.ConcreteDecls, concrete)
	tc.funcDecls[fullName] = concrete

	retType := tc.resolveType(concrete.ReturnType)
	if retType == nil {
		tc.error(concrete.Tok, fmt.Sprintf("unknown return type '%s' for template function '%s'", concrete.ReturnType, fullName))
	}
	if concrete.RetPointer {
		retType = &Type{Kind: TK_Pointer, BaseType: retType}
	}

	tc.symbolTable[fullName] = &Symbol{
		Name:   fullName,
		Type:   &Type{Kind: TK_Custom, Name: "func", BaseType: retType},
		Throws: concrete.Throws,
	}

	var retTypesTyped []*Type
	for _, rt := range retTypes {
		t := tc.resolveType(rt)
		if t == nil {
			tc.error(concrete.Tok, fmt.Sprintf("unknown return type '%s' for template function '%s'", rt, fullName))
		}
		retTypesTyped = append(retTypesTyped, t)
	}
	tc.funcReturnTypes[fullName] = retTypesTyped

	savedFunc := tc.currentFunc
	tc.currentFunc = concrete
	funcScope := make(map[string]*Symbol)
	for k, v := range tc.symbolTable {
		funcScope[k] = v
	}
	for _, p := range concrete.Params {
		t := tc.resolveType(p.TypeAnnot)
		if t == nil {
			tc.error(p.Tok, fmt.Sprintf("unknown parameter type '%s'", p.TypeAnnot))
		}
		if p.IsPointer {
			t = &Type{Kind: TK_Pointer, BaseType: t}
		}
		funcScope[p.Name] = &Symbol{Name: p.Name, Type: t, DeclaredType: p.TypeAnnot}
	}
	for _, s := range concrete.Body.Statements {
		tc.checkStatement(s, funcScope, false)
	}
	tc.currentFunc = savedFunc
}

func substituteBlock(block *BlockStmt, subst map[string]string) *BlockStmt {
	nb := &BlockStmt{Tok: block.Tok}
	for _, s := range block.Statements {
		nb.Statements = append(nb.Statements, substituteNode(s, subst))
	}
	return nb
}

func substituteNode(n Node, subst map[string]string) Node {
	if n == nil {
		return nil
	}
	switch node := n.(type) {
	case *BlockStmt:
		return substituteBlock(node, subst)
	case *ReturnStmt:
		vals := make([]Node, len(node.Values))
		for i, v := range node.Values {
			vals[i] = substituteNode(v, subst)
		}
		return &ReturnStmt{Tok: node.Tok, Values: vals}
	case *ExprStmt:
		return &ExprStmt{Tok: node.Tok, Expression: substituteNode(node.Expression, subst)}
	case *CastExpr:
		return &CastExpr{
			Tok: node.Tok, TargetType: substituteType(node.TargetType, subst),
			IsPointer: node.IsPointer, Operand: substituteNode(node.Operand, subst),
		}
	case *ConvExpr:
		return &ConvExpr{
			Tok: node.Tok, TargetType: substituteType(node.TargetType, subst),
			Operand: substituteNode(node.Operand, subst),
		}
	case *VarDecl:
		return &VarDecl{
			Tok: node.Tok, Name: node.Name,
			TypeAnnot: substituteType(node.TypeAnnot, subst),
			IsPointer: node.IsPointer, IsVolatile: node.IsVolatile, IsConst: node.IsConst,
			ArraySize: node.ArraySize, Value: substituteNode(node.Value, subst),
		}
	case *StructLiteral:
		return &StructLiteral{
			Tok: node.Tok, TypeName: substituteType(node.TypeName, subst),
			Fields: node.Fields,
		}
	case *AssignStmt:
		return &AssignStmt{Tok: node.Tok, Target: substituteNode(node.Target, subst), Value: substituteNode(node.Value, subst)}
	case *MultiAssignStmt:
		targets := make([]Node, len(node.Targets))
		for i, t := range node.Targets {
			targets[i] = substituteNode(t, subst)
		}
		vals := make([]Node, len(node.Values))
		for i, v := range node.Values {
			vals[i] = substituteNode(v, subst)
		}
		return &MultiAssignStmt{Tok: node.Tok, Targets: targets, Values: vals}
	case *MultiShortVarDecl:
		vals := make([]Node, len(node.Values))
		for i, v := range node.Values {
			vals[i] = substituteNode(v, subst)
		}
		names := make([]string, len(node.Names))
		copy(names, node.Names)
		return &MultiShortVarDecl{Tok: node.Tok, Names: names, Values: vals}
	case *IfStmt:
		alts := make([]*ElifClause, len(node.Alternatives))
		for i, a := range node.Alternatives {
			alts[i] = &ElifClause{Tok: a.Tok, Condition: substituteNode(a.Condition, subst), Body: substituteBlock(a.Body, subst)}
		}
		var elseBlk *BlockStmt
		if node.Else != nil {
			elseBlk = substituteBlock(node.Else, subst)
		}
		return &IfStmt{
			Tok: node.Tok, Condition: substituteNode(node.Condition, subst),
			Consequence: substituteBlock(node.Consequence, subst),
			Alternatives: alts, Else: elseBlk,
		}
	case *ForRangeStmt:
		return &ForRangeStmt{Tok: node.Tok, Var: node.Var, From: substituteNode(node.From, subst),
			To: substituteNode(node.To, subst), Body: substituteBlock(node.Body, subst)}
	case *ForCondStmt:
		return &ForCondStmt{Tok: node.Tok, Condition: substituteNode(node.Condition, subst),
			Body: substituteBlock(node.Body, subst)}
	case *ForInfiniteStmt:
		return &ForInfiniteStmt{Tok: node.Tok, Body: substituteBlock(node.Body, subst)}
	case *MatchStmt:
		cases := make([]*MatchCase, len(node.Cases))
		for i, c := range node.Cases {
			cases[i] = &MatchCase{
				Tok: c.Tok, Value: substituteNode(c.Value, subst),
				IsCatchAll: c.IsCatchAll, FallThrough: c.FallThrough,
				Body: substituteBlock(c.Body, subst),
			}
		}
		return &MatchStmt{Tok: node.Tok, Subject: substituteNode(node.Subject, subst), Cases: cases}
	case *CallExpr:
		args := make([]Node, len(node.Args))
		for i, a := range node.Args {
			args[i] = substituteNode(a, subst)
		}
		return &CallExpr{
			Tok: node.Tok, Function: substituteNode(node.Function, subst),
			Args: args, TypeResult: node.TypeResult,
		}
	case *TemplateInstantiation:
		newArgs := make([]string, len(node.TypeArgs))
		for i, a := range node.TypeArgs {
			newArgs[i] = substituteType(a, subst)
		}
		return &TemplateInstantiation{
			Tok: node.Tok, Name: node.Name,
			TypeArgs: newArgs, Object: substituteNode(node.Object, subst),
		}
	case *UnaryExpr:
		return &UnaryExpr{Tok: node.Tok, Op: node.Op, Operand: substituteNode(node.Operand, subst)}
	case *BinaryExpr:
		return &BinaryExpr{Tok: node.Tok, Left: substituteNode(node.Left, subst), Op: node.Op, Right: substituteNode(node.Right, subst)}
	case *SelectorExpr:
		return &SelectorExpr{Tok: node.Tok, Object: substituteNode(node.Object, subst), Field: node.Field}
	case *IndexExpr:
		return &IndexExpr{Tok: node.Tok, Collection: substituteNode(node.Collection, subst), Index: substituteNode(node.Index, subst)}
	case *SliceExpr:
		return &SliceExpr{Tok: node.Tok, Collection: substituteNode(node.Collection, subst), Low: substituteNode(node.Low, subst), High: substituteNode(node.High, subst), SliceSize: node.SliceSize}
	case *AddressOf:
		return &AddressOf{Tok: node.Tok, Operand: substituteNode(node.Operand, subst)}
	case *Deref:
		return &Deref{Tok: node.Tok, Operand: substituteNode(node.Operand, subst)}
	default:
		return n
	}
}

func (tc *TypeChecker) inferTemplateMethodCall(inst *TemplateInstantiation, args []Node, scope map[string]*Symbol) *Type {
	objType := tc.inferType(inst.Object, scope)
	if objType.Kind != TK_Struct {
		tc.error(inst.Tok, "template method call on non-struct type")
		return tc.typeTable["void"]
	}

	structFullName := objType.Name
	baseName := structFullName
	if idx := strings.Index(structFullName, "<"); idx != -1 {
		baseName = structFullName[:idx]
	}

	tmpl, ok := tc.templateDefs[baseName]
	if !ok {
		if idx := strings.Index(baseName, "."); idx != -1 {
			modName := baseName[:idx]
			tmplName := baseName[idx+1:]
			if modTmpls, ok := tc.moduleTemplates[modName]; ok {
				tmpl, ok = modTmpls[tmplName]
			}
		}
	}
	if !ok {
		tc.error(inst.Tok, fmt.Sprintf("no template definition found for '%s'", baseName))
		return tc.typeTable["void"]
	}

	structDecl, ok := tmpl.Declaration.(*StructDecl)
	if !ok {
		tc.error(inst.Tok, fmt.Sprintf("'%s' is not a template struct", baseName))
		return tc.typeTable["void"]
	}

	var methodTmpl *FuncDecl
	for _, m := range structDecl.Methods {
		if m.Name == inst.Name {
			methodTmpl = m
			break
		}
	}
	if methodTmpl == nil {
		if methods, ok := tc.structMethods[structFullName]; ok {
			if retType, ok := methods[inst.Name]; ok {
				if tc.structMethodThrows[structFullName][inst.Name] {
					tc.checkThrowsCall(inst.Tok, inst.Name)
				}
				return retType
			}
		}
		tc.error(inst.Tok, fmt.Sprintf("struct '%s' has no method '%s'", baseName, inst.Name))
		return tc.typeTable["void"]
	}

	if len(methodTmpl.TypeParams) == 0 {
		if methods, ok := tc.structMethods[structFullName]; ok {
			if retType, ok := methods[inst.Name]; ok {
				if tc.structMethodThrows[structFullName][inst.Name] {
					tc.checkThrowsCall(inst.Tok, inst.Name)
				}
				return retType
			}
		}
		return tc.typeTable["int"]
	}

	if methodTmpl.Throws {
		tc.checkThrowsCall(inst.Tok, inst.Name)
	}
	methodSubst := buildSubstMap(methodTmpl.TypeParams, inst.TypeArgs)
	methodFullName := structFullName + "_" + inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"

	structSubst := make(map[string]string)
	if idx := strings.Index(structFullName, "<"); idx != -1 {
		structArgs := extractTypeArgs(structFullName)
		structParams := tmpl.TypeParams
		for i, p := range structParams {
			if i < len(structArgs) {
				structSubst[p.Name] = structArgs[i]
			}
		}
	}
	for k, v := range structSubst {
		if _, exists := methodSubst[k]; !exists {
			methodSubst[k] = v
		}
	}

	selfParam := &Param{
		Tok:       methodTmpl.Tok,
		Name:      "self",
		TypeAnnot: structFullName,
		IsPointer: true,
	}
	inst.ResolvedName = methodFullName
	concreteBody := substituteBlock(methodTmpl.Body, methodSubst)

	var retTypes []string
	for _, rt := range methodTmpl.ReturnTypes {
		retTypes = append(retTypes, substituteType(rt, methodSubst))
	}

	concreteMethod := &FuncDecl{
		Tok:         methodTmpl.Tok,
		ReturnType:  substituteType(methodTmpl.ReturnType, methodSubst),
		ReturnTypes: retTypes,
		RetPointer:  methodTmpl.RetPointer,
		Name:        methodFullName,
		Params:      append([]*Param{selfParam}, cloneParamsWithSubst(methodTmpl.Params, methodSubst)...),
		Body:        concreteBody,
		IsMethod:    false,
		Throws:      methodTmpl.Throws,
	}
	tc.ConcreteDecls = append(tc.ConcreteDecls, concreteMethod)

	retType := tc.resolveType(concreteMethod.ReturnType)
	if retType == nil {
		tc.error(concreteMethod.Tok, fmt.Sprintf("unknown return type '%s' for template method '%s'", concreteMethod.ReturnType, methodFullName))
	}
	if concreteMethod.RetPointer {
		retType = &Type{Kind: TK_Pointer, BaseType: retType}
	}
	tc.symbolTable[methodFullName] = &Symbol{
		Name: methodFullName,
		Type: &Type{Kind: TK_Custom, Name: "func", BaseType: retType},
	}

	var retTypesTyped []*Type
	for _, rt := range retTypes {
		t := tc.resolveType(rt)
		if t == nil {
			tc.error(concreteMethod.Tok, fmt.Sprintf("unknown return type '%s' for template method '%s'", rt, methodFullName))
		}
		retTypesTyped = append(retTypesTyped, t)
	}
	tc.funcReturnTypes[methodFullName] = retTypesTyped

	return retType
}

func splitCommasAtDepth(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func extractTypeArgs(fullName string) []string {
	start := strings.Index(fullName, "<")
	end := strings.LastIndex(fullName, ">")
	if start == -1 || end == -1 {
		return nil
	}
	inner := fullName[start+1 : end]
	if inner == "" {
		return nil
	}
	args := splitCommasAtDepth(inner)
	for i := range args {
		args[i] = strings.TrimSpace(args[i])
	}
	return args
}

func (tc *TypeChecker) error(tok Token, msg string) {
	err := fmt.Sprintf("[merlin] ERROR (%s, line %d, col %d): %s", tc.currentFile, tok.Line, tok.Col, msg)
	tc.errors = append(tc.errors, err)
}
