package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Semantic Analysis
// ---------------------------------------------------------------------------

type TypeKind int

const (
	TK_Void TypeKind = iota
	TK_Bool
	TK_Char
	TK_Int
	TK_Float
	TK_String
	TK_Pointer
	TK_Struct
	TK_Array
	TK_Custom
	TK_Enum
)

type Type struct {
	Kind     TypeKind
	Name     string
	BaseType *Type
	ElemType *Type
	Size     int
}

func (t *Type) String() string {
	switch t.Kind {
	case TK_Void:
		return "void"
	case TK_Bool:
		return "bool"
	case TK_Char:
		return "char"
	case TK_Int:
		return "int"
	case TK_Float:
		return "float"
	case TK_String:
		return "string"
	case TK_Pointer:
		return "*" + t.BaseType.String()
	case TK_Struct:
		return t.Name
	case TK_Array:
		return fmt.Sprintf("%s[%d]", t.BaseType.String(), t.Size)
	case TK_Custom:
		return t.Name
	case TK_Enum:
		return t.Name
	default:
		return "unknown"
	}
}

type Symbol struct {
	Name         string
	Type         *Type
	DeclaredType string
	Throws       bool
	IsConst      bool
}

type TypeChecker struct {
	symbolTable        map[string]*Symbol
	moduleSymbols      map[string]map[string]*Symbol
	typeTable          map[string]*Type
	structMethods      map[string]map[string]*Type
	structMethodThrows map[string]map[string]bool
	structFields       map[string]map[string]*Type
	structDefModule    map[string]string
	structPrivateFields map[string]map[string]bool
	enumValues         map[string]map[string]int64
	currentFunc        *FuncDecl
	imported           map[string]bool
	requiredLibs       map[string]bool
	libraryPaths       map[string]bool
	templateDefs       map[string]*TemplateDecl
	moduleTemplates    map[string]map[string]*TemplateDecl
	funcReturnTypes     map[string][]*Type
	funcDecls           map[string]*FuncDecl
	structMethodDecls   map[string]map[string]*FuncDecl
	ConcreteDecls       []Node
	tryDepth            int
	currentFile         string
	errors              []string
}

func NewTypeChecker() *TypeChecker {
	return &TypeChecker{
		symbolTable:         make(map[string]*Symbol),
		moduleSymbols:       make(map[string]map[string]*Symbol),
		typeTable:           make(map[string]*Type),
		structMethods:       make(map[string]map[string]*Type),
		structMethodThrows:  make(map[string]map[string]bool),
		structFields:        make(map[string]map[string]*Type),
		structDefModule:     make(map[string]string),
		structPrivateFields: map[string]map[string]bool{},
		enumValues:          make(map[string]map[string]int64),
		imported:            make(map[string]bool),
		requiredLibs:        make(map[string]bool),
		libraryPaths:        make(map[string]bool),
		templateDefs:        make(map[string]*TemplateDecl),
		moduleTemplates:     make(map[string]map[string]*TemplateDecl),
		funcReturnTypes:     make(map[string][]*Type),
		funcDecls:           make(map[string]*FuncDecl),
		structMethodDecls:   make(map[string]map[string]*FuncDecl),
	}
}

func (tc *TypeChecker) Check(prog *Program, filePath string, registerGlobals bool) {
	if tc.imported[filePath] {
		return
	}
	tc.imported[filePath] = true
	savedFile := tc.currentFile
	tc.currentFile = filePath
	defer func() { tc.currentFile = savedFile }()
	tc.initPrimitives()

	for _, imp := range prog.Imports {
		tc.resolveImport(imp.Tok, imp.Path, filePath, imp.Global)
	}

	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *TypeAliasDecl:
			tc.registerTypeAlias(n)
		case *StructDecl:
			tc.registerStruct(n)
		case *EnumDecl:
			tc.registerEnum(n)
		case *FuncDecl:
			tc.registerFunc(n)
		case *ExternalFuncDecl:
			tc.registerExternalFunc(n)
		case *TemplateDecl:
			if registerGlobals {
				tc.templateDefs[n.TemplateName()] = n
			}
		}
	}

	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *FuncDecl:
			tc.currentFunc = n
			funcScope := make(map[string]*Symbol)
			for k, v := range tc.symbolTable {
				funcScope[k] = v
			}
			for _, p := range n.Params {
				t := tc.resolveType(p.TypeAnnot)
				if t == nil {
					tc.error(p.Tok, fmt.Sprintf("unknown parameter type '%s'", p.TypeAnnot))
				}
				if p.IsPointer {
					t = &Type{Kind: TK_Pointer, BaseType: t}
				}
				funcScope[p.Name] = &Symbol{Name: p.Name, Type: t, DeclaredType: p.TypeAnnot}
			}
			for _, s := range n.Body.Statements {
				tc.checkStatement(s, funcScope, false)
			}
			tc.currentFunc = nil
		case *TemplateDecl:
		default:
			tc.checkStatement(stmt, tc.symbolTable, false)
		}
	}
}

func (tc *TypeChecker) resolveImport(tok Token, path string, currentFile string, isGlobal bool) {
	moduleName := path
	if idx := strings.Index(path, "/"); idx != -1 {
		moduleName = path[idx+1:]
	}
	moduleName = strings.TrimSuffix(moduleName, ".mrl")

	searchPaths := []string{
		fmt.Sprintf("%s/%s.mrl", filepath.Dir(currentFile), path),
		fmt.Sprintf("std/%s.mrl", path),
		fmt.Sprintf("packages/%s.mrl", path),
	}

	var foundPath string
	var isPackage bool
	var pkgDir string

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			foundPath = p
			break
		}
	}

	if foundPath == "" {
		pkgSearchPaths := []string{
			filepath.Join(filepath.Dir(currentFile), path),
			filepath.Join("std", path),
			filepath.Join("packages", path),
		}
		for _, dir := range pkgSearchPaths {
			mrlPath := filepath.Join(dir, moduleName+".mrl")
			if _, err := os.Stat(mrlPath); err == nil {
				foundPath = mrlPath
				isPackage = true
				pkgDir = dir
				break
			}
		}
	}

	if foundPath == "" {
		tc.error(tok, fmt.Sprintf("import \"%s\" not found", path))
		return
	}

	if isPackage {
		soPath := filepath.Join(pkgDir, "lib"+moduleName+".so")
		if _, err := os.Stat(soPath); err == nil {
			tc.libraryPaths[pkgDir] = true
			tc.requiredLibs[moduleName] = true
		}
	}

	src, err := os.ReadFile(foundPath)
	if err != nil {
		tc.error(tok, fmt.Sprintf("cannot read import %s: %v", foundPath, err))
		return
	}

	lexer := NewLexerWithFile(string(src), foundPath)
	tokens := lexer.Tokenize()
	parser := NewParserWithFile(tokens, string(src), foundPath)
	prog := parser.ParseProgram()

	modSymbols := make(map[string]*Symbol)
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *TypeAliasDecl:
			if n.IsPrivate {
				continue
			}
			t := tc.resolveType(n.BaseType)
			if t == nil {
				tc.error(n.Tok, fmt.Sprintf("unknown base type '%s' in type alias '%s'", n.BaseType, n.Name))
			}
			modSymbols[n.Name] = &Symbol{Name: n.Name, Type: &Type{Kind: TK_Custom, Name: n.Name, BaseType: t}}
		case *StructDecl:
			if n.IsPrivate {
				continue
			}
			t := &Type{Kind: TK_Struct, Name: n.Name}
			tc.typeTable[n.Name] = t
			modSymbols[n.Name] = &Symbol{Name: n.Name, Type: t}
		case *EnumDecl:
			if n.IsPrivate {
				continue
			}
			t := &Type{Kind: TK_Enum, Name: n.Name}
			tc.typeTable[n.Name] = t
			modSymbols[n.Name] = &Symbol{Name: n.Name, Type: t}
		case *FuncDecl:
			if n.IsPrivate {
				continue
			}
			retType := tc.resolveType(n.ReturnType)
			if retType == nil {
				tc.error(n.Tok, fmt.Sprintf("unknown return type '%s' for function '%s'", n.ReturnType, n.Name))
			}
			if n.RetPointer {
				retType = &Type{Kind: TK_Pointer, BaseType: retType}
			}
			modSymbols[n.Name] = &Symbol{Name: n.Name, Type: &Type{Kind: TK_Custom, Name: "func", BaseType: retType}, Throws: n.Throws}
		case *ExternalFuncDecl:
			if n.IsPrivate {
				continue
			}
			retType := tc.resolveType(n.ReturnType)
			if retType == nil {
				tc.error(n.Tok, fmt.Sprintf("unknown return type '%s' for external function '%s'", n.ReturnType, n.Name))
			}
			if n.RetPointer {
				retType = &Type{Kind: TK_Pointer, BaseType: retType}
			}
			modSymbols[n.Name] = &Symbol{Name: n.Name, Type: &Type{Kind: TK_Custom, Name: "func", BaseType: retType}}
		case *VarDecl:
			if n.IsPrivate {
				continue
			}
			t := tc.resolveType(n.TypeAnnot)
			if t == nil {
				tc.error(n.Tok, fmt.Sprintf("unknown type '%s' for variable '%s'", n.TypeAnnot, n.Name))
			}
			if n.IsPointer {
				t = &Type{Kind: TK_Pointer, BaseType: t}
			}
			modSymbols[n.Name] = &Symbol{Name: n.Name, Type: t}
		}
	}
	if isGlobal {
		tc.Check(prog, foundPath, true)
	} else {
		tc.moduleSymbols[moduleName] = modSymbols

		// Save global table state before Check
		savedTypeTable := cloneMap(tc.typeTable)
		savedSymbolTable := cloneMapSyms(tc.symbolTable)
		savedFuncRet := cloneMapRet(tc.funcReturnTypes)
		savedStructFields := cloneMapStructFields(tc.structFields)
		savedStructPriv := cloneMapStructPriv(tc.structPrivateFields)
		savedStructMethods := cloneMapStructMethods(tc.structMethods)
		savedStructMThrows := cloneMapStructMThrows(tc.structMethodThrows)
		savedStructDefMod := cloneMapString(tc.structDefModule)
		savedEnumValues := cloneMapEnum(tc.enumValues)

		tc.Check(prog, foundPath, false)

		// Remove everything that Check added to global tables
		for k := range tc.typeTable {
			if _, ok := savedTypeTable[k]; !ok {
				delete(tc.typeTable, k)
			}
		}
		for k := range tc.symbolTable {
			if _, ok := savedSymbolTable[k]; !ok {
				delete(tc.symbolTable, k)
			}
		}
		for k := range tc.funcReturnTypes {
			if _, ok := savedFuncRet[k]; !ok {
				delete(tc.funcReturnTypes, k)
			}
		}
		for k := range tc.structFields {
			if _, ok := savedStructFields[k]; !ok {
				delete(tc.structFields, k)
			}
		}
		for k := range tc.structPrivateFields {
			if _, ok := savedStructPriv[k]; !ok {
				delete(tc.structPrivateFields, k)
			}
		}
		for k := range tc.structMethods {
			if _, ok := savedStructMethods[k]; !ok {
				delete(tc.structMethods, k)
			}
		}
		for k := range tc.structMethodThrows {
			if _, ok := savedStructMThrows[k]; !ok {
				delete(tc.structMethodThrows, k)
			}
		}
		for k := range tc.structDefModule {
			if _, ok := savedStructDefMod[k]; !ok {
				delete(tc.structDefModule, k)
			}
		}
		for k := range tc.enumValues {
			if _, ok := savedEnumValues[k]; !ok {
				delete(tc.enumValues, k)
			}
		}

		// Also remove struct/enum types added to typeTable during modSymbols building
		for _, stmt := range prog.Statements {
			switch n := stmt.(type) {
			case *StructDecl:
				delete(tc.typeTable, n.Name)
			case *EnumDecl:
				delete(tc.typeTable, n.Name)
			}
		}

		for _, stmt := range prog.Statements {
			if td, ok := stmt.(*TemplateDecl); ok {
				name := td.TemplateName()
				if tc.templateDefs[name] == td {
					delete(tc.templateDefs, name)
				}
				if tc.moduleTemplates[moduleName] == nil {
					tc.moduleTemplates[moduleName] = make(map[string]*TemplateDecl)
				}
				tc.moduleTemplates[moduleName][name] = td
			}
		}
	}
	// Remove private symbols from global symbol table so importing
	// modules cannot access them directly (covers @import case).
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *FuncDecl:
			if n.IsPrivate {
				delete(tc.symbolTable, n.Name)
			}
		case *ExternalFuncDecl:
			if n.IsPrivate {
				delete(tc.symbolTable, n.Name)
			}
		case *TemplateDecl:
			if fd, ok := n.Declaration.(*FuncDecl); ok && fd.IsPrivate {
				name := n.TemplateName()
				delete(tc.symbolTable, name)
			}
		case *EnumDecl:
			if n.IsPrivate {
				for _, ev := range n.Values {
					delete(tc.symbolTable, ev.Name)
				}
			} else {
				for _, ev := range n.Values {
					if ev.IsPrivate {
						delete(tc.symbolTable, ev.Name)
					}
				}
			}
		}
	}
}

func (tc *TypeChecker) initPrimitives() {
	tc.typeTable["void"] = &Type{Kind: TK_Void, Name: "void"}
	tc.typeTable["bool"] = &Type{Kind: TK_Bool, Name: "bool"}
	tc.typeTable["char"] = &Type{Kind: TK_Char, Name: "char"}
	tc.typeTable["int"] = &Type{Kind: TK_Int, Name: "int"}
	tc.typeTable["int8"] = &Type{Kind: TK_Int, Name: "int8"}
	tc.typeTable["int16"] = &Type{Kind: TK_Int, Name: "int16"}
	tc.typeTable["int32"] = &Type{Kind: TK_Int, Name: "int32"}
	tc.typeTable["int64"] = &Type{Kind: TK_Int, Name: "int64"}
	tc.typeTable["uint8"] = &Type{Kind: TK_Int, Name: "uint8"}
	tc.typeTable["uint16"] = &Type{Kind: TK_Int, Name: "uint16"}
	tc.typeTable["uint32"] = &Type{Kind: TK_Int, Name: "uint32"}
	tc.typeTable["uint64"] = &Type{Kind: TK_Int, Name: "uint64"}
	tc.typeTable["float"] = &Type{Kind: TK_Float, Name: "float"}
	tc.typeTable["float32"] = &Type{Kind: TK_Float, Name: "float32"}
	tc.typeTable["float64"] = &Type{Kind: TK_Float, Name: "float64"}
	tc.typeTable["string"] = &Type{Kind: TK_String, Name: "string"}
}

func (tc *TypeChecker) registerTypeAlias(n *TypeAliasDecl) {
	base := tc.resolveType(n.BaseType)
	if base == nil {
		tc.error(n.Tok, fmt.Sprintf("unknown base type '%s' in type alias '%s'", n.BaseType, n.Name))
	}
	tc.typeTable[n.Name] = &Type{Kind: TK_Custom, Name: n.Name, BaseType: base}
}

func (tc *TypeChecker) registerStruct(n *StructDecl) {
	tc.typeTable[n.Name] = &Type{Kind: TK_Struct, Name: n.Name}

	if tc.currentFile != "" {
		modName := strings.TrimSuffix(filepath.Base(tc.currentFile), ".mrl")
		tc.structDefModule[n.Name] = modName
	}

	fields := make(map[string]*Type)
	for _, f := range n.Fields {
		ft := tc.resolveType(f.TypeAnnot)
		if ft == nil {
			tc.error(f.Tok, fmt.Sprintf("unknown field type '%s' for field '%s'", f.TypeAnnot, f.Name))
		}
		if f.IsPointer {
			ft = &Type{Kind: TK_Pointer, BaseType: ft}
		}
		fields[f.Name] = ft
		if f.IsPrivate {
			if tc.structPrivateFields[n.Name] == nil {
				tc.structPrivateFields[n.Name] = make(map[string]bool)
			}
			tc.structPrivateFields[n.Name][f.Name] = true
		}
	}
	tc.structFields[n.Name] = fields

	methods := make(map[string]*Type)
	methodThrows := make(map[string]bool)
	methodDecls := make(map[string]*FuncDecl)
	for _, m := range n.Methods {
		retType := tc.resolveType(m.ReturnType)
		if retType == nil {
			tc.error(m.Tok, fmt.Sprintf("unknown return type '%s' for method '%s'", m.ReturnType, m.Name))
		}
		if m.RetPointer {
			retType = &Type{Kind: TK_Pointer, BaseType: retType}
		}
		methods[m.Name] = retType
		methodThrows[m.Name] = m.Throws
		methodDecls[m.Name] = m
	}
	tc.structMethods[n.Name] = methods
	tc.structMethodThrows[n.Name] = methodThrows
	tc.structMethodDecls[n.Name] = methodDecls
}

func (tc *TypeChecker) registerEnum(n *EnumDecl) {
	enumName := n.Name
	tc.typeTable[enumName] = &Type{Kind: TK_Enum, Name: enumName}

	values := make(map[string]int64)
	nextVal := int64(0)
	for _, ev := range n.Values {
		if ev.Value != nil {
			if lit, ok := ev.Value.(*IntLiteral); ok {
				nextVal = lit.Value
			}
		}
		values[ev.Name] = nextVal
		nextVal++
	}
	tc.enumValues[enumName] = values
}

func (tc *TypeChecker) registerFunc(n *FuncDecl) {
	retType := tc.resolveType(n.ReturnType)
	if retType == nil {
		tc.error(n.Tok, fmt.Sprintf("unknown return type '%s' for function '%s'", n.ReturnType, n.Name))
	}
	if n.RetPointer {
		retType = &Type{Kind: TK_Pointer, BaseType: retType}
	}
	tc.symbolTable[n.Name] = &Symbol{Name: n.Name, Type: &Type{Kind: TK_Custom, Name: "func", BaseType: retType}, Throws: n.Throws}
	var retTypes []*Type
	if len(n.ReturnTypes) > 0 {
		for _, rt := range n.ReturnTypes {
			t := tc.resolveType(rt)
			if t == nil {
				tc.error(n.Tok, fmt.Sprintf("unknown return type '%s' for function '%s'", rt, n.Name))
			}
			retTypes = append(retTypes, t)
		}
	} else {
		retTypes = []*Type{retType}
	}
	tc.funcReturnTypes[n.Name] = retTypes
	tc.funcDecls[n.Name] = n

	// Validate default parameter order — all params with defaults must come after
	// params without defaults.
	foundDefault := false
	for _, p := range n.Params {
		if p.Default != nil {
			foundDefault = true
		} else if foundDefault {
			tc.error(p.Tok, fmt.Sprintf("parameter '%s' without default value follows a parameter with a default value", p.Name))
		}
	}
}

func (tc *TypeChecker) registerExternalFunc(n *ExternalFuncDecl) {
	retType := tc.resolveType(n.ReturnType)
	if retType == nil {
		tc.error(n.Tok, fmt.Sprintf("unknown return type '%s' for external function '%s'", n.ReturnType, n.Name))
	}
	if n.RetPointer {
		retType = &Type{Kind: TK_Pointer, BaseType: retType}
	}
	tc.symbolTable[n.Name] = &Symbol{Name: n.Name, Type: &Type{Kind: TK_Custom, Name: "func", BaseType: retType}}
	if n.LinkLib != "" {
		tc.requiredLibs[n.LinkLib] = true
	}
}

func findDotOutsideAngles(s string) int {
	depth := 0
	for i, c := range s {
		if c == '<' {
			depth++
		}
		if c == '>' {
			depth--
		}
		if c == '.' && depth == 0 {
			return i
		}
	}
	return -1
}

func (tc *TypeChecker) resolveType(name string) *Type {
	if t, ok := tc.typeTable[name]; ok {
		return t
	}

	if idx := strings.Index(name, "<"); idx != -1 {
		baseName := name[:idx]
		if dotIdx := findDotOutsideAngles(baseName); dotIdx != -1 {
			moduleName := baseName[:dotIdx]
			typeName := baseName[dotIdx+1:]
			if templates, ok := tc.moduleTemplates[moduleName]; ok {
				if tmpl, ok := templates[typeName]; ok {
					tc.instantiateTemplate(tmpl, name)
					if t, ok := tc.typeTable[name]; ok {
						return t
					}
				}
			}
		} else {
			if tmpl, ok := tc.templateDefs[baseName]; ok {
				tc.instantiateTemplate(tmpl, name)
				if t, ok := tc.typeTable[name]; ok {
					return t
				}
			}
		}
		return nil
	}

	// Handle module-qualified non-template type: module.Type
	if dotIdx := strings.Index(name, "."); dotIdx != -1 {
		moduleName := name[:dotIdx]
		typeName := name[dotIdx+1:]
		if modSymbols, ok := tc.moduleSymbols[moduleName]; ok {
			if sym, ok := modSymbols[typeName]; ok {
				return sym.Type
			}
		}
	}

	return nil
}

func mergeScope(parent map[string]*Symbol) map[string]*Symbol {
	s := make(map[string]*Symbol, len(parent))
	for k, v := range parent {
		s[k] = v
	}
	return s
}

func (tc *TypeChecker) checkStatement(n Node, scope map[string]*Symbol, inLoop bool) {
	switch stmt := n.(type) {
	case *VarDecl:
		t := tc.resolveType(stmt.TypeAnnot)
		if t == nil {
			tc.error(stmt.Tok, fmt.Sprintf("unknown type '%s' for variable '%s'", stmt.TypeAnnot, stmt.Name))
		}
		// If resolved via module prefix, replace type annot with short name
		if t != nil && t.Name != "" {
			if dotIdx := strings.Index(stmt.TypeAnnot, "."); dotIdx != -1 {
				stmt.TypeAnnot = t.Name
			}
		}
		if stmt.IsPointer {
			t = &Type{Kind: TK_Pointer, BaseType: t}
		}
		if stmt.ArraySize != "" {
			size, _ := strconv.Atoi(stmt.ArraySize)
			t = &Type{Kind: TK_Array, BaseType: t, Size: size}
		}
		if stmt.Value != nil {
			valType := tc.inferType(stmt.Value, scope)
			if stmt.ArraySize == "" && !stmt.IsPointer && valType.Kind == TK_Array && valType.BaseType == t {
				t = valType
				stmt.ArraySize = strconv.Itoa(valType.Size)
			}
			tc.checkAssignment(t, valType, stmt.Tok)
		}
		declType := stmt.TypeAnnot
		if stmt.ArraySize != "" {
			declType = declType + "[" + stmt.ArraySize + "]"
		}
		scope[stmt.Name] = &Symbol{Name: stmt.Name, Type: t, DeclaredType: declType, IsConst: stmt.IsConst}
	case *ShortVarDecl:
		t := tc.inferType(stmt.Value, scope)
		if t == nil {
			tc.error(stmt.Tok, "cannot infer type of expression")
		}
		if _, ok := scope[stmt.Name]; ok {
			tc.error(stmt.Tok, fmt.Sprintf("variable '%s' already declared", stmt.Name))
		}
		declType := typeToAnnotString(t)
		stmt.TypeAnnot = declType
		scope[stmt.Name] = &Symbol{Name: stmt.Name, Type: t, DeclaredType: declType, IsConst: stmt.IsConst}
	case *CompoundAssignStmt:
		if id, ok := stmt.Target.(*Identifier); ok {
			if sym, ok := scope[id.Value]; ok && sym.IsConst {
				tc.error(stmt.Tok, fmt.Sprintf("cannot assign to const variable '%s'", id.Value))
			}
		}
		lhsType := tc.inferType(stmt.Target, scope)
		rhsType := tc.inferType(stmt.Value, scope)
		if lhsType.Kind == TK_Pointer && rhsType.Kind == TK_Int {
			if stmt.Op != "+=" && stmt.Op != "-=" {
				tc.error(stmt.Tok, fmt.Sprintf("invalid operation: %s %s %s", lhsType.String(), stmt.Op, rhsType.String()))
			}
		} else {
			tc.checkAssignment(lhsType, rhsType, stmt.Tok)
		}
	case *AssignStmt:
		if id, ok := stmt.Target.(*Identifier); ok {
			if sym, ok := scope[id.Value]; ok && sym.IsConst {
				tc.error(stmt.Tok, fmt.Sprintf("cannot assign to const variable '%s'", id.Value))
			}
		}
		lhsType := tc.inferType(stmt.Target, scope)
		rhsType := tc.inferType(stmt.Value, scope)
		tc.checkAssignment(lhsType, rhsType, stmt.Tok)
	case *ReturnStmt:
		if tc.currentFunc != nil {
			retTypes := tc.funcReturnTypes[tc.currentFunc.Name]
			if len(retTypes) == 0 {
				retTypes = []*Type{tc.resolveType(tc.currentFunc.ReturnType)}
			}
			if len(stmt.Values) == 0 && len(retTypes) == 1 && retTypes[0] != nil && (retTypes[0].Kind == TK_Void || retTypes[0].Name == "void") {
				// void return with no value is fine
			} else if len(stmt.Values) != len(retTypes) {
				tc.error(stmt.Tok, fmt.Sprintf("return %d values but function expects %d", len(stmt.Values), len(retTypes)))
			} else {
				for i, val := range stmt.Values {
					valType := tc.inferType(val, scope)
					if i < len(retTypes) {
						expected := retTypes[i]
						if tc.currentFunc.RetPointer && i == 0 {
							expected = &Type{Kind: TK_Pointer, BaseType: expected}
						}
						tc.checkAssignment(expected, valType, stmt.Tok)
					}
				}
			}
		}
	case *MultiAssignStmt:
		// Check if single value is a multi-return call
		if len(stmt.Values) == 1 {
			if call, ok := stmt.Values[0].(*CallExpr); ok {
				funcRetTypes := tc.getCallReturnTypes(call, scope)
				if len(funcRetTypes) == len(stmt.Targets) {
					for i, target := range stmt.Targets {
						if id, ok := target.(*Identifier); ok {
							if sym, ok := scope[id.Value]; ok && sym.IsConst {
								tc.error(stmt.Tok, fmt.Sprintf("cannot assign to const variable '%s'", id.Value))
							}
						}
						tgtType := tc.inferType(target, scope)
						tc.checkAssignment(tgtType, funcRetTypes[i], stmt.Tok)
					}
					break
				}
			}
		}
		if len(stmt.Targets) != len(stmt.Values) {
			tc.error(stmt.Tok, fmt.Sprintf("mismatch: %d targets but %d values", len(stmt.Targets), len(stmt.Values)))
		} else {
			for i, target := range stmt.Targets {
				if id, ok := target.(*Identifier); ok {
					if sym, ok := scope[id.Value]; ok && sym.IsConst {
						tc.error(stmt.Tok, fmt.Sprintf("cannot assign to const variable '%s'", id.Value))
					}
				}
				tgtType := tc.inferType(target, scope)
				valType := tc.inferType(stmt.Values[i], scope)
				tc.checkAssignment(tgtType, valType, stmt.Tok)
			}
		}
	case *MultiShortVarDecl:
		// Check if single value is a multi-return call
		if len(stmt.Values) == 1 {
			if call, ok := stmt.Values[0].(*CallExpr); ok {
				funcRetTypes := tc.getCallReturnTypes(call, scope)
				if len(funcRetTypes) == len(stmt.Names) {
					for i, name := range stmt.Names {
						if _, ok := scope[name]; ok {
							tc.error(stmt.Tok, fmt.Sprintf("variable '%s' already declared", name))
						}
						t := funcRetTypes[i]
						declType := typeToAnnotString(t)
						scope[name] = &Symbol{Name: name, Type: t, DeclaredType: declType}
					}
					break
				}
			}
		}
		for i, name := range stmt.Names {
			if _, ok := scope[name]; ok {
				tc.error(stmt.Tok, fmt.Sprintf("variable '%s' already declared", name))
			}
			if i < len(stmt.Values) {
				t := tc.inferType(stmt.Values[i], scope)
				if t == nil {
					tc.error(stmt.Tok, fmt.Sprintf("cannot infer type of value for '%s'", name))
				}
				declType := typeToAnnotString(t)
				scope[name] = &Symbol{Name: name, Type: t, DeclaredType: declType}
			}
		}
	case *BreakStmt:
		if !inLoop {
			tc.error(stmt.Tok, "'break' used outside of loop")
		}
	case *ContinueStmt:
		if !inLoop {
			tc.error(stmt.Tok, "'continue' used outside of loop")
		}
	case *PassStmt:
		if tc.currentFunc == nil {
			tc.error(stmt.Tok, "'pass' used outside of function")
		}
	case *DeferStmt:
		tc.inferType(stmt.Call, scope)
	case *IfStmt:
		tc.inferType(stmt.Condition, scope)
		bodyScope := mergeScope(scope)
		for _, s := range stmt.Consequence.Statements {
			tc.checkStatement(s, bodyScope, inLoop)
		}
		for _, alt := range stmt.Alternatives {
			tc.inferType(alt.Condition, scope)
			altScope := mergeScope(scope)
			for _, s := range alt.Body.Statements {
				tc.checkStatement(s, altScope, inLoop)
			}
		}
		if stmt.Else != nil {
			elseScope := mergeScope(scope)
			for _, s := range stmt.Else.Statements {
				tc.checkStatement(s, elseScope, inLoop)
			}
		}
	case *ForRangeStmt:
		tc.inferType(stmt.From, scope)
		tc.inferType(stmt.To, scope)
		loopScope := mergeScope(scope)
		loopScope[stmt.Var] = &Symbol{Name: stmt.Var, Type: tc.typeTable["int"], DeclaredType: "int"}
		for _, s := range stmt.Body.Statements {
			tc.checkStatement(s, loopScope, true)
		}
	case *ForCondStmt:
		tc.inferType(stmt.Condition, scope)
		loopScope := mergeScope(scope)
		for _, s := range stmt.Body.Statements {
			tc.checkStatement(s, loopScope, true)
		}
	case *ForInfiniteStmt:
		loopScope := mergeScope(scope)
		for _, s := range stmt.Body.Statements {
			tc.checkStatement(s, loopScope, true)
		}
	case *MatchStmt:
		tc.inferType(stmt.Subject, scope)
		for _, c := range stmt.Cases {
			if c.Value != nil {
				tc.inferType(c.Value, scope)
			}
			caseScope := mergeScope(scope)
			for _, s := range c.Body.Statements {
				tc.checkStatement(s, caseScope, inLoop)
			}
		}
	case *RaiseStmt:
		if tc.currentFunc == nil || !tc.currentFunc.Throws {
			tc.error(stmt.Tok, "'raise' used outside of 'throws' function")
		}
		valType := tc.inferType(stmt.Value, scope)
		if valType.Kind != TK_Int {
			tc.error(stmt.Tok, "'raise' value must be of type 'int'")
		}
	case *TryCatchStmt:
		tc.tryDepth++
		for _, s := range stmt.Body.Statements {
			tc.checkStatement(s, scope, inLoop)
		}
		for _, cc := range stmt.Catches {
			if cc.Value != nil {
				tc.inferType(cc.Value, scope)
			}
			for _, s := range cc.Body.Statements {
				tc.checkStatement(s, scope, inLoop)
			}
		}
		tc.tryDepth--
	case *ExprStmt:
		tc.inferType(stmt.Expression, scope)
	}
}

func unwrapType(t *Type) *Type {
	for t != nil && t.Kind == TK_Custom && t.BaseType != nil {
		t = t.BaseType
	}
	return t
}

func isScalar(t *Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case TK_Int, TK_Float, TK_Bool, TK_Char, TK_Pointer:
		return true
	}
	return false
}

func typeToAnnotString(t *Type) string {
	if t == nil {
		return "void"
	}
	switch t.Kind {
	case TK_Pointer:
		return "&" + typeToAnnotString(t.BaseType)
	case TK_Array:
		return typeToAnnotString(t.BaseType) + "[" + strconv.Itoa(t.Size) + "]"
	default:
		if t.Name != "" {
			return t.Name
		}
		switch t.Kind {
		case TK_Int:
			return "int"
		case TK_Float:
			return "float"
		case TK_Bool:
			return "bool"
		case TK_Char:
			return "char"
		case TK_String:
			return "string"
		case TK_Void:
			return "void"
		case TK_Struct:
			return t.Name
		case TK_Enum:
			return t.Name
		case TK_Custom:
			return t.Name
		default:
			return "int"
		}
	}
}

func (tc *TypeChecker) inferType(n Node, scope map[string]*Symbol) *Type {
	switch expr := n.(type) {
	case *IntLiteral:
		return tc.typeTable["int"]
	case *FloatLiteral:
		return tc.typeTable["float"]
	case *BoolLiteral:
		return tc.typeTable["bool"]
	case *StringLiteral:
		return tc.typeTable["string"]
	case *CharLiteral:
		return tc.typeTable["char"]
	case *Identifier:
		if sym, ok := scope[expr.Value]; ok {
			return sym.Type
		}
		tc.error(expr.Tok, fmt.Sprintf("undeclared identifier '%s'", expr.Value))
		return tc.typeTable["void"]
	case *StructLiteral:
		t := tc.resolveType(expr.TypeName)
		if t == nil {
			tc.error(expr.Tok, fmt.Sprintf("unknown struct type '%s'", expr.TypeName))
			return tc.typeTable["void"]
		}
		return t
	case *ArrayLiteral:
		bt := tc.typeTable["int"]
		if len(expr.Elements) > 0 {
			bt = tc.inferType(expr.Elements[0], scope)
		}
		return &Type{Kind: TK_Array, BaseType: bt, Size: len(expr.Elements)}
	case *SelectorExpr:
		if ident, ok := expr.Object.(*Identifier); ok {
			if modSymbols, ok := tc.moduleSymbols[ident.Value]; ok {
				if sym, ok := modSymbols[expr.Field]; ok {
					return sym.Type
				}
				tc.error(expr.Tok, fmt.Sprintf("module '%s' has no symbol '%s'", ident.Value, expr.Field))
			}
			if _, inScope := scope[ident.Value]; !inScope {
				if tc.typeTable[ident.Value] != nil && tc.typeTable[ident.Value].Kind == TK_Enum {
					if _, ok := tc.enumValues[ident.Value][expr.Field]; ok {
						expr.IsEnumValue = true
						return tc.typeTable["int"]
					}
					tc.error(expr.Tok, fmt.Sprintf("enum '%s' has no value '%s'", ident.Value, expr.Field))
					return tc.typeTable["void"]
				}
			}
		}
		objType := tc.inferType(expr.Object, scope)
		if objType.Kind == TK_String {
			return tc.typeTable["int"]
		}
		structType := objType
		if structType.Kind == TK_Pointer && structType.BaseType != nil {
			structType = structType.BaseType
		}
		if structType.Kind == TK_Struct {
			if fields, ok := tc.structFields[structType.Name]; ok {
				if ft, ok := fields[expr.Field]; ok {
					if tc.structPrivateFields[structType.Name][expr.Field] {
						defModule := tc.structDefModule[structType.Name]
						if defModule != "" {
							curModule := strings.TrimSuffix(filepath.Base(tc.currentFile), ".mrl")
							if defModule != curModule {
								tc.error(expr.Tok, fmt.Sprintf("field '%s' is private in struct '%s'", expr.Field, structType.Name))
							}
						}
					}
					return ft
				}
			}
			return tc.typeTable["int"]
		}
		return tc.typeTable["void"]
	case *IndexExpr:
		collType := tc.inferType(expr.Collection, scope)
		tc.inferType(expr.Index, scope)
		if collType.Kind == TK_Array && collType.BaseType != nil {
			return collType.BaseType
		}
		if collType.Kind == TK_Pointer && collType.BaseType != nil {
			return collType.BaseType
		}
		return tc.typeTable["int"]
	case *SliceExpr:
		collType := tc.inferType(expr.Collection, scope)
		if expr.Low != nil {
			tc.inferType(expr.Low, scope)
		}
		if expr.High != nil {
			tc.inferType(expr.High, scope)
		}
		if collType.Kind == TK_String {
			return tc.typeTable["string"]
		}
		if collType.Kind == TK_Array && collType.BaseType != nil {
			lowVal := int64(0)
			highVal := int64(-1)
			if expr.Low != nil {
				if lit, ok := expr.Low.(*IntLiteral); ok {
					lowVal = lit.Value
				}
			}
			if expr.High != nil {
				if lit, ok := expr.High.(*IntLiteral); ok {
					highVal = lit.Value
				}
			}
			if lowVal == 0 && highVal == -1 && expr.Low == nil && expr.High == nil {
				expr.SliceSize = collType.Size
			} else if expr.Low != nil && expr.High != nil {
				if _, ok := expr.Low.(*IntLiteral); ok {
					if _, ok := expr.High.(*IntLiteral); ok {
						expr.SliceSize = int(highVal - lowVal)
					}
				}
			}
			if expr.SliceSize <= 0 {
				tc.error(expr.Tok, "array slice bounds must be integer literals")
				return tc.typeTable["void"]
			}
			return &Type{Kind: TK_Array, BaseType: collType.BaseType, Size: expr.SliceSize}
		}
		tc.error(expr.Tok, "cannot slice non-array, non-string type")
		return tc.typeTable["void"]
	case *BinaryExpr:
		l := tc.inferType(expr.Left, scope)
		r := tc.inferType(expr.Right, scope)
		if expr.Op == "in" {
			if r == nil || l == nil {
				return tc.typeTable["bool"]
			}
			baseR := unwrapType(r)
			if baseR.Kind == TK_String {
				unwrapL := unwrapType(l)
				if unwrapL.Kind != TK_String && unwrapL.Kind != TK_Char {
					tc.error(expr.Tok, fmt.Sprintf("cannot use 'in' on string with '%s'", l.String()))
				}
			} else if baseR.Kind == TK_Array {
				if baseR.BaseType == nil || baseR.BaseType.String() != unwrapType(l).String() {
					tc.error(expr.Tok, fmt.Sprintf("type mismatch in 'in': '%s' not in '%s'", l.String(), r.String()))
				}
			} else {
				tc.error(expr.Tok, fmt.Sprintf("cannot use 'in' on type '%s'", r.String()))
			}
			return tc.typeTable["bool"]
		}
		if l.Kind == TK_Float || r.Kind == TK_Float {
			return tc.typeTable["float"]
		}

		if l.Kind == TK_String && r.Kind == TK_String {
			if expr.Op == "+" {
				return tc.typeTable["string"]
			}
			if expr.Op == "==" || expr.Op == "!=" || expr.Op == "<" || expr.Op == ">" || expr.Op == "<=" || expr.Op == ">=" {
				return tc.typeTable["bool"]
			}
			tc.error(expr.Tok, fmt.Sprintf("invalid operation: %s %s %s", l.String(), expr.Op, r.String()))
		}

		if l.Kind == TK_Pointer && r.Kind == TK_Int {
			if expr.Op == "+" || expr.Op == "-" || expr.Op == "==" || expr.Op == "!=" {
				if expr.Op == "==" || expr.Op == "!=" {
					return tc.typeTable["bool"]
				}
				return l
			}
			tc.error(expr.Tok, fmt.Sprintf("invalid operation: %s %s %s", l.String(), expr.Op, r.String()))
		}
		if l.Kind == TK_Int && r.Kind == TK_Pointer && expr.Op == "+" {
			return r
		}
		if l.Kind == TK_Pointer && r.Kind == TK_Pointer {
			if expr.Op == "-" {
				return tc.typeTable["int"]
			}
			if expr.Op == "==" || expr.Op == "!=" || expr.Op == "<" || expr.Op == ">" || expr.Op == "<=" || expr.Op == ">=" {
				return tc.typeTable["bool"]
			}
			tc.error(expr.Tok, fmt.Sprintf("invalid operation: %s %s %s", l.String(), expr.Op, r.String()))
		}

		return tc.typeTable["int"]
	case *UnaryExpr:
		operandType := tc.inferType(expr.Operand, scope)
		if expr.Op == "not" {
			return tc.typeTable["bool"]
		}
		return operandType
	case *AddressOf:
		base := tc.inferType(expr.Operand, scope)
		return &Type{Kind: TK_Pointer, BaseType: base}
	case *Deref:
		ptrType := tc.inferType(expr.Operand, scope)
		if ptrType.Kind == TK_Pointer && ptrType.BaseType != nil {
			return ptrType.BaseType
		}
		return tc.typeTable["void"]
	case *CastExpr:
		t := tc.resolveType(expr.TargetType)
		if t == nil {
			tc.error(expr.Tok, fmt.Sprintf("unknown cast type '%s'", expr.TargetType))
		}
		if expr.IsPointer {
			t = &Type{Kind: TK_Pointer, BaseType: t}
		}
		opType := tc.inferType(expr.Operand, scope)
		if opType != nil {
			baseOp := unwrapType(opType)
			baseTgt := unwrapType(t)
			if baseTgt != nil && baseTgt.Kind == TK_Pointer {
				if !isScalar(baseOp) {
					tc.error(expr.Tok, fmt.Sprintf("cannot cast '%s' to '%s'", opType.String(), t.String()))
				}
			} else if !isScalar(baseOp) || !isScalar(baseTgt) {
				tc.error(expr.Tok, fmt.Sprintf("cannot cast '%s' to '%s'", opType.String(), t.String()))
			}
		}
		return t
	case *ConvExpr:
		t := tc.resolveType(expr.TargetType)
		if t == nil {
			tc.error(expr.Tok, fmt.Sprintf("unknown conversion type '%s'", expr.TargetType))
		}
		return t
	case *TemplateInstantiation:
		if tmpl, ok := tc.templateDefs[expr.Name]; ok {
			fullName := expr.Name + "<" + strings.Join(expr.TypeArgs, ", ") + ">"
			tc.instantiateTemplate(tmpl, fullName)
			if t, ok := tc.typeTable[fullName]; ok {
				return t
			}
			if sym, ok := tc.symbolTable[fullName]; ok {
				return sym.Type
			}
		}
		if expr.Object != nil {
			if objIdent, ok := expr.Object.(*Identifier); ok {
				if tmpl, ok := tc.moduleTemplates[objIdent.Value][expr.Name]; ok {
					fullName := objIdent.Value + "." + expr.Name + "<" + strings.Join(expr.TypeArgs, ", ") + ">"
					tc.instantiateTemplate(tmpl, fullName)
					if t, ok := tc.typeTable[fullName]; ok {
						return t
					}
					if sym, ok := tc.symbolTable[fullName]; ok {
						return sym.Type
					}
				}
			}
		}
		return tc.typeTable["void"]
	case *CallExpr:
		if inst, ok := expr.Function.(*TemplateInstantiation); ok {
			if inst.Object != nil {
				if objIdent, ok := inst.Object.(*Identifier); ok {
					if _, ok := tc.moduleSymbols[objIdent.Value]; ok {
						fullName := inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"
						if tmpl, ok := tc.templateDefs[inst.Name]; ok {
							inst.ResolvedName = fullName
							tc.instantiateTemplate(tmpl, fullName)
							if sym, ok := tc.symbolTable[fullName]; ok {
								if sym.Type.Kind == TK_Custom && sym.Type.Name == "func" {
									if sym.Throws {
										tc.checkThrowsCall(expr.Tok, fullName)
									}
									if fd, ok := tc.funcDecls[fullName]; ok {
										tc.fillDefaultArgs(expr, fd.Params)
									}
									return sym.Type.BaseType
								}
							}
						}
						if tmpl, ok := tc.moduleTemplates[objIdent.Value][inst.Name]; ok {
							fullName = objIdent.Value + "." + inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"
							inst.ResolvedName = fullName
							tc.instantiateTemplate(tmpl, fullName)
							if sym, ok := tc.symbolTable[fullName]; ok {
								if sym.Type.Kind == TK_Custom && sym.Type.Name == "func" {
									if sym.Throws {
										tc.checkThrowsCall(expr.Tok, fullName)
									}
									if fd, ok := tc.funcDecls[fullName]; ok {
										tc.fillDefaultArgs(expr, fd.Params)
									}
									return sym.Type.BaseType
								}
							}
						}
						return tc.typeTable["int"]
					}
				}
				retType := tc.inferTemplateMethodCall(inst, expr.Args, scope)
				return retType
			}
			fullName := inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"
			inst.ResolvedName = fullName
			if tmpl, ok := tc.templateDefs[inst.Name]; ok {
				tc.instantiateTemplate(tmpl, fullName)
				if sym, ok := tc.symbolTable[fullName]; ok {
					if sym.Type.Kind == TK_Custom && sym.Type.Name == "func" {
						if sym.Throws {
							tc.checkThrowsCall(expr.Tok, fullName)
						}
						if fd, ok := tc.funcDecls[fullName]; ok {
							tc.fillDefaultArgs(expr, fd.Params)
						}
						return sym.Type.BaseType
					}
				}
			}
			return tc.typeTable["int"]
		}
		if sel, ok := expr.Function.(*SelectorExpr); ok {
			if ident, ok := sel.Object.(*Identifier); ok {
				if modSymbols, ok := tc.moduleSymbols[ident.Value]; ok {
					if sym, ok := modSymbols[sel.Field]; ok {
						if sym.Type.Kind == TK_Custom && sym.Type.Name == "func" {
							if sym.Throws {
								tc.checkThrowsCall(expr.Tok, sel.Field)
							}
							if fd, ok := tc.funcDecls[sel.Field]; ok {
								tc.fillDefaultArgs(expr, fd.Params)
							}
							return sym.Type.BaseType
						}
						tc.error(expr.Tok, fmt.Sprintf("symbol '%s' in module '%s' is not a function", sel.Field, ident.Value))
					}
					tc.error(expr.Tok, fmt.Sprintf("module '%s' has no symbol '%s'", ident.Value, sel.Field))
				}

			if sym, ok := scope[ident.Value]; ok && sym.Type != nil {
				baseType := sym.Type
				if baseType.Kind == TK_Pointer && baseType.BaseType != nil && baseType.BaseType.Kind == TK_Struct {
						baseType = baseType.BaseType
					}
					if baseType.Kind == TK_Struct {
						structName := baseType.Name
						if methods, ok := tc.structMethods[structName]; ok {
							if retType, ok := methods[sel.Field]; ok {
								if tc.structMethodThrows[structName][sel.Field] {
									tc.checkThrowsCall(expr.Tok, sel.Field)
								}
								if mDecls, ok := tc.structMethodDecls[structName]; ok {
									if md, ok := mDecls[sel.Field]; ok {
										tc.fillDefaultArgs(expr, md.Params)
									}
								}
								return retType
							}
							tc.error(expr.Tok, fmt.Sprintf("struct '%s' has no method '%s'", structName, sel.Field))
						}
					}
				}

				tc.error(expr.Tok, fmt.Sprintf("undeclared module or struct instance '%s'", ident.Value))
			}
		}
		if ident, ok := expr.Function.(*Identifier); ok {
			if ident.Value == "print" {
				for _, arg := range expr.Args {
					tc.inferType(arg, scope)
				}
				return tc.typeTable["void"]
			}
			if ident.Value == "input" {
				if len(expr.Args) > 1 {
					tc.error(expr.Tok, "input() accepts at most one argument (prompt string)")
				}
				if len(expr.Args) == 1 {
					argType := tc.inferType(expr.Args[0], scope)
					if argType == nil || argType.Kind != TK_String {
						tc.error(expr.Tok, "input() prompt argument must be a string")
					}
				}
				return tc.typeTable["string"]
			}
			if ident.Value == "len" {
				if len(expr.Args) != 1 {
					tc.error(expr.Tok, "len() requires exactly one argument")
				}
				argType := tc.inferType(expr.Args[0], scope)
				if argType.Kind != TK_String && argType.Kind != TK_Array {
					tc.error(expr.Tok, fmt.Sprintf("len() not supported for type '%s'", argType.String()))
				}
				return tc.typeTable["int"]
			}
			if ident.Value == "sizeof" {
				if len(expr.Args) != 1 {
					tc.error(expr.Tok, "sizeof() requires exactly one argument")
				}
				tc.inferType(expr.Args[0], scope)
				return tc.typeTable["int"]
			}
			if ident.Value == "typeof" {
				if len(expr.Args) != 1 {
					tc.error(expr.Tok, "typeof() requires exactly one argument")
				}
				_ = tc.inferType(expr.Args[0], scope)
				typeName := tc.typeNameString(expr.Args[0], scope)
				expr.TypeResult = typeName
				strType := tc.resolveType("string")
				if len(typeName)+1 > strType.Size {
					strType = &Type{Kind: TK_String, Size: len(typeName) + 1}
				}
				return strType
			}
			if sym, ok := scope[ident.Value]; ok {
				if sym.Type.Kind == TK_Custom && sym.Type.Name == "func" {
					if sym.Throws {
						tc.checkThrowsCall(expr.Tok, ident.Value)
					}
					if fd, ok := tc.funcDecls[ident.Value]; ok {
						tc.fillDefaultArgs(expr, fd.Params)
					}
					return sym.Type.BaseType
				}
				tc.error(expr.Tok, fmt.Sprintf("identifier '%s' is not a function", ident.Value))
			}
			tc.error(expr.Tok, fmt.Sprintf("undeclared function '%s'", ident.Value))
		}
		return tc.typeTable["void"]
	}
	return tc.typeTable["void"]
}

func (tc *TypeChecker) getCallReturnTypes(expr *CallExpr, scope map[string]*Symbol) []*Type {
	if inst, ok := expr.Function.(*TemplateInstantiation); ok {
		fullName := inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"
		if rt, ok := tc.funcReturnTypes[fullName]; ok {
			if fd, ok := tc.funcDecls[fullName]; ok {
				tc.fillDefaultArgs(expr, fd.Params)
			}
			return rt
		}
		if inst.ResolvedName != "" {
			if rt, ok := tc.funcReturnTypes[inst.ResolvedName]; ok {
				if fd, ok := tc.funcDecls[inst.ResolvedName]; ok {
					tc.fillDefaultArgs(expr, fd.Params)
				}
				return rt
			}
		}
	}
	if ident, ok := expr.Function.(*Identifier); ok {
		if rt, ok := tc.funcReturnTypes[ident.Value]; ok {
			if fd, ok := tc.funcDecls[ident.Value]; ok {
				tc.fillDefaultArgs(expr, fd.Params)
			}
			return rt
		}
	}
	if sym, ok := scope[getCallIdentName(expr)]; ok {
		if sym.Type != nil && sym.Type.Kind == TK_Custom && sym.Type.Name == "func" {
			if fd, ok := tc.funcDecls[getCallIdentName(expr)]; ok {
				tc.fillDefaultArgs(expr, fd.Params)
			}
			return []*Type{sym.Type.BaseType}
		}
	}
	return nil
}

func getCallIdentName(expr *CallExpr) string {
	if ident, ok := expr.Function.(*Identifier); ok {
		return ident.Value
	}
	if sel, ok := expr.Function.(*SelectorExpr); ok {
		if ident, ok := sel.Object.(*Identifier); ok {
			return ident.Value + "." + sel.Field
		}
	}
	return ""
}

func (tc *TypeChecker) checkThrowsCall(tok Token, name string) {
	if tc.currentFunc != nil && tc.currentFunc.Throws {
		return
	}
	if tc.tryDepth > 0 {
		return
	}
	tc.error(tok, fmt.Sprintf("throws function '%s' must be called inside try/catch block or throws function", name))
}

func (tc *TypeChecker) typeNameString(n Node, scope map[string]*Symbol) string {
	switch expr := n.(type) {
	case *IntLiteral:
		return "int"
	case *FloatLiteral:
		return "float"
	case *BoolLiteral:
		return "bool"
	case *CharLiteral:
		return "char"
	case *StringLiteral:
		return "string"
	case *Identifier:
		if sym, ok := scope[expr.Value]; ok && sym.DeclaredType != "" {
			if sym.Type.Kind == TK_Pointer {
				return "&" + sym.DeclaredType
			}
			return sym.DeclaredType
		}
	case *AddressOf:
		return "&" + tc.typeNameString(expr.Operand, scope)
	case *Deref:
		inner := tc.typeNameString(expr.Operand, scope)
		if strings.HasPrefix(inner, "&") {
			return inner[1:]
		}
		return inner
	case *CastExpr:
		if expr.IsPointer {
			return "&" + expr.TargetType
		}
		return expr.TargetType
	case *ConvExpr:
		return expr.TargetType
	case *UnaryExpr:
		return tc.typeNameString(expr.Operand, scope)
	case *BinaryExpr:
		lt := tc.inferType(expr.Left, scope)
		if lt != nil && lt.Kind == TK_Float {
			return "float"
		}
		return "int"
	case *SelectorExpr:
		t := tc.inferType(expr, scope)
		if t != nil {
			return t.String()
		}
	case *ArrayLiteral:
		if len(expr.Elements) > 0 {
			elem := tc.typeNameString(expr.Elements[0], scope)
			return elem + "[]"
		}
		return "int[]"
	case *SliceExpr:
		t := tc.inferType(expr, scope)
		if t != nil {
			return t.String()
		}
	case *TemplateInstantiation:
		return expr.Name + "<" + strings.Join(expr.TypeArgs, ", ") + ">"
	}
	t := tc.inferType(n, scope)
	if t != nil {
		return t.String()
	}
	return "unknown"
}

func (tc *TypeChecker) checkAssignment(target, value *Type, tok Token) {
	if target == nil || value == nil {
		return
	}
	if target.Kind != value.Kind {
		if target.Kind == TK_Custom || value.Kind == TK_Custom {
			if target.Name != value.Name {
				tc.error(tok, fmt.Sprintf("cannot assign type '%s' to type '%s'", value.Name, target.Name))
			}
			return
		}
		if (target.Kind == TK_Int || target.Kind == TK_Float) && (value.Kind == TK_Int || value.Kind == TK_Float) {
			return
		}
		if target.Kind == TK_Char && value.Kind == TK_Int {
			return
		}
		if target.Kind == TK_Pointer && value.Kind == TK_Pointer && target.BaseType.String() == value.BaseType.String() {
			return
		}
		if target.Kind == TK_Enum && value.Kind == TK_Int {
			return
		}
		if target.Kind == TK_Int && value.Kind == TK_Enum {
			return
		}
		tc.error(tok, fmt.Sprintf("type mismatch: cannot assign %s to %s", value.String(), target.String()))
	}
}

func (tc *TypeChecker) fillDefaultArgs(call *CallExpr, params []*Param) {
	if len(call.Args) >= len(params) {
		return
	}
	for i := len(call.Args); i < len(params); i++ {
		if params[i].Default != nil {
			call.Args = append(call.Args, cloneNode(params[i].Default))
		} else if len(call.Args) <= i {
			tc.error(call.Tok, fmt.Sprintf("missing argument for parameter '%s' in call to '%s'", params[i].Name, getCallIdentName(call)))
			return
		}
	}
}

// Clone helpers for snapshot/restore during import cleanup

func cloneMap[V any](m map[string]V) map[string]V {
	out := make(map[string]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneMapSyms(m map[string]*Symbol) map[string]*Symbol {
	return cloneMap(m)
}
func cloneMapRet(m map[string][]*Type) map[string][]*Type {
	return cloneMap(m)
}
func cloneMapStructFields(m map[string]map[string]*Type) map[string]map[string]*Type {
	return cloneMap(m)
}
func cloneMapStructPriv(m map[string]map[string]bool) map[string]map[string]bool {
	return cloneMap(m)
}
func cloneMapStructMethods(m map[string]map[string]*Type) map[string]map[string]*Type {
	return cloneMap(m)
}
func cloneMapStructMThrows(m map[string]map[string]bool) map[string]map[string]bool {
	return cloneMap(m)
}
func cloneMapString(m map[string]string) map[string]string {
	return cloneMap(m)
}
func cloneMapEnum(m map[string]map[string]int64) map[string]map[string]int64 {
	return cloneMap(m)
}
