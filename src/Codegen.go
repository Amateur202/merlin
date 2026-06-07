package main

import (
	"fmt"
	"strings"
)

type CodeGen struct {
	output          *strings.Builder
	indent          int
	importedModules map[string]bool
	allProgs        map[string]*Program
	concreteDecls   []Node
	throwsFuncs     map[string]bool
	inThrowsFunc    bool
	tryErrVar       string
	tryEndLabel     string
	tryCounter      int
	tryOutCounter   int
		varTypes             map[string]string
		pointerVars          map[string]bool
		funcTypes            map[string][]string
		needsStdio           bool
		needsStdlib          bool
		inputCounter         int
		arrayLenVars         map[string]string            // var name → hidden len var name for T[] params
		funcTArrayPositions  map[string][]int             // concrete func name → indices of T[] params
		templateSubst        map[string]map[string]string // mangled struct/func name → {T: int, ...}
		currentSubst         map[string]string            // currently active substitution for body emission
		currentFuncName      string                       // name of the function currently being generated
		deferStack           []string                     // deferred cleanup code (LIFO)
		structFields         map[string]map[string]string // struct name → {field name → type annot}
	}

func NewCodeGen() *CodeGen {
	return &CodeGen{
		output:              &strings.Builder{},
		importedModules:     make(map[string]bool),
		throwsFuncs:         make(map[string]bool),
		varTypes:            make(map[string]string),
		pointerVars:         make(map[string]bool),
		funcTypes:           make(map[string][]string),
		arrayLenVars:        make(map[string]string),
		funcTArrayPositions: make(map[string][]int),
		templateSubst:       make(map[string]map[string]string),
		structFields:        make(map[string]map[string]string),
	}
}

func (cg *CodeGen) SetConcreteDecls(decls []Node) {
	cg.concreteDecls = decls
}

func (cg *CodeGen) emitStringData(n Node, prog *Program) {
	if lit, ok := n.(*StringLiteral); ok {
		cg.output.WriteString(cStringLiteral(lit.Value))
	} else {
		cg.genNode(n, prog)
		cg.output.WriteString(".data")
	}
}

func (cg *CodeGen) getOutCType(funcName string) string {
	retTypes, ok := cg.funcTypes[funcName]
	if !ok || len(retTypes) == 0 {
		return "intptr_t"
	}
	retType := retTypes[0]
	if retType == "" || retType == "void" {
		return "void"
	}
	if retType == "string" {
		return "MerlinString"
	}
	if strings.HasPrefix(retType, "&") {
		inner := cg.resolvePrimitiveType(mangleTemplateName(retType[1:]))
		return inner + "*"
	}
	return cg.resolvePrimitiveType(mangleTemplateName(retType))
}

func (cg *CodeGen) resolvePrimitiveType(t string) string {
	mapping := map[string]string{
		"int":     "intptr_t",
		"int8":    "int8_t",
		"int16":   "int16_t",
		"int32":   "int32_t",
		"int64":   "int64_t",
		"uint8":   "uint8_t",
		"uint16":  "uint16_t",
		"uint32":  "uint32_t",
		"uint64":  "uint64_t",
		"float":   "double",
		"float32": "float",
	"float64": "double",
		"bool":    "uint8_t",
		"char":    "char",
	}
	if val, ok := mapping[t]; ok {
		return val
	}
	if t == "string" {
		return "MerlinString"
	}
	return t
}

func (cg *CodeGen) isPrimitiveTypeName(t string) bool {
	switch t {
	case "int", "int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64",
		"float", "float32", "float64",
		"bool", "char", "string":
		return true
	}
	return false
}

func (cg *CodeGen) scanForPrint(prog *Program) {
	if prog == nil { return }
	var walk func(n Node)
	walk = func(n Node) {
		if n == nil { return }
		switch node := n.(type) {
		case *Program:
			for _, s := range node.Statements { walk(s) }
		case *BlockStmt:
			for _, s := range node.Statements { walk(s) }
		case *FuncDecl:
			for _, p := range node.Params { walk(p) }
			if node.Body != nil { walk(node.Body) }
		case *ExternalFuncDecl:
		case *StructDecl:
			for _, f := range node.Fields { walk(f) }
			for _, m := range node.Methods { walk(m) }
		case *ExprStmt:
			if call, ok := node.Expression.(*CallExpr); ok {
				fn := cg.getCallFuncName(call)
				if fn == "print" || fn == "input" {
					cg.needsStdio = true
				}
			}
		case *AssignStmt:
			walk(node.Target)
			walk(node.Value)
		case *MultiAssignStmt:
			for _, t := range node.Targets { walk(t) }
			for _, v := range node.Values { walk(v) }
		case *MultiShortVarDecl:
			for _, v := range node.Values { walk(v) }
		case *VarDecl:
			walk(node.Value)
		case *IfStmt:
			walk(node.Condition)
			walk(node.Consequence)
			for _, a := range node.Alternatives {
				walk(a.Condition)
				walk(a.Body)
			}
			if node.Else != nil { walk(node.Else) }
		case *ForRangeStmt:
			walk(node.From)
			walk(node.To)
			walk(node.Body)
		case *ForCondStmt:
			walk(node.Condition)
			walk(node.Body)
		case *ForInfiniteStmt:
			walk(node.Body)
		case *MatchStmt:
			walk(node.Subject)
			for _, c := range node.Cases {
				walk(c.Value)
				walk(c.Body)
			}
		case *TryCatchStmt:
			walk(node.Body)
			for _, c := range node.Catches {
				walk(c.Value)
				walk(c.Body)
			}
		case *ReturnStmt:
			for _, v := range node.Values { walk(v) }
		case *DeferStmt:
			walk(node.Call)
		case *RaiseStmt:
			walk(node.Value)
		case *TemplateDecl:
			walk(node.Declaration)
		case *CallExpr:
			fn := cg.getCallFuncName(node)
			if fn == "print" || fn == "input" {
				cg.needsStdio = true
			}
		}
	}
	walk(prog)
}

func (cg *CodeGen) registerThrowsFuncs(prog *Program) {
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*FuncDecl); ok && fd.Throws {
			cg.throwsFuncs[fd.Name] = true
		}
		if td, ok := stmt.(*TemplateDecl); ok {
			if fd, ok := td.Declaration.(*FuncDecl); ok && fd.Throws {
				cg.throwsFuncs[fd.Name] = true
			}
		}
	}
}

func (cg *CodeGen) registerFuncTypes(prog *Program, allProgs map[string]*Program) {
	// Collect template parameter names from all template declarations
	tmplParams := map[string][]*TypeParam{} // template name → param list
	for _, p := range allProgs {
		for _, stmt := range p.Statements {
			if td, ok := stmt.(*TemplateDecl); ok {
				tmplParams[td.TemplateName()] = td.TypeParams
			}
			if fd, ok := stmt.(*FuncDecl); ok {
				if len(fd.ReturnTypes) > 0 {
					cg.funcTypes[fd.Name] = fd.ReturnTypes
				} else {
					cg.funcTypes[fd.Name] = []string{fd.ReturnType}
				}
			}
			if ed, ok := stmt.(*ExternalFuncDecl); ok {
				cg.funcTypes[ed.Name] = []string{ed.ReturnType}
			}
		}
	}
	for _, stmt := range prog.Statements {
		if td, ok := stmt.(*TemplateDecl); ok {
			tmplParams[td.TemplateName()] = td.TypeParams
		}
		if ed, ok := stmt.(*ExternalFuncDecl); ok {
			cg.funcTypes[ed.Name] = []string{ed.ReturnType}
		}
	}
	for _, d := range cg.concreteDecls {
		if fd, ok := d.(*FuncDecl); ok {
			if len(fd.ReturnTypes) > 0 {
				cg.funcTypes[fd.Name] = fd.ReturnTypes
			} else {
				cg.funcTypes[fd.Name] = []string{fd.ReturnType}
			}
			// Record T[] param positions for call site length injection
			for i, p := range fd.Params {
				if strings.HasSuffix(p.TypeAnnot, "[]") {
					cg.funcTArrayPositions[fd.Name] = append(cg.funcTArrayPositions[fd.Name], i)
				}
			}
		}
		if sd, ok := d.(*StructDecl); ok {
			structName := mangleTemplateName(sd.Name)
			fields := make(map[string]string)
			for _, f := range sd.Fields {
				ft := f.TypeAnnot
				if f.IsPointer {
					ft = ft + "*"
				}
				fields[f.Name] = ft
			}
			cg.structFields[structName] = fields
			for _, m := range sd.Methods {
				if len(m.ReturnTypes) > 0 {
					cg.funcTypes[structName+"_"+m.Name] = m.ReturnTypes
				} else {
					cg.funcTypes[structName+"_"+m.Name] = []string{m.ReturnType}
				}
			}
			if idx := strings.Index(sd.Name, "<"); idx != -1 {
				baseName := sd.Name[:idx]
				if params, ok := tmplParams[baseName]; ok {
					inner := sd.Name[idx+1 : len(sd.Name)-1]
					typeArgs := strings.Split(inner, ", ")
					subst := map[string]string{}
					for i, p := range params {
						if i < len(typeArgs) {
							subst[p.Name] = typeArgs[i]
						}
					}
					cg.templateSubst[mangleTemplateName(sd.Name)] = subst
				}
			}
		}
	}
	// Also register method return types and struct fields for regular structs from all programs
	for _, p := range allProgs {
		for _, stmt := range p.Statements {
			if sd, ok := stmt.(*StructDecl); ok {
				structName := mangleTemplateName(sd.Name)
				for _, m := range sd.Methods {
					if len(m.ReturnTypes) > 0 {
						cg.funcTypes[structName+"_"+m.Name] = m.ReturnTypes
					} else {
						cg.funcTypes[structName+"_"+m.Name] = []string{m.ReturnType}
					}
				}
				if _, exists := cg.structFields[structName]; !exists {
					fields := make(map[string]string)
					for _, f := range sd.Fields {
						ft := f.TypeAnnot
						if f.IsPointer {
							ft = ft + "*"
						}
						fields[f.Name] = ft
					}
					cg.structFields[structName] = fields
				}
			}
		}
	}
	// Second pass: build substitutions for concrete template function instantiations
	for _, d := range cg.concreteDecls {
		if fd, ok := d.(*FuncDecl); ok {
			if idx := strings.Index(fd.Name, "<"); idx != -1 {
				baseName := fd.Name[:idx]
				if params, ok := tmplParams[baseName]; ok {
					inner := fd.Name[idx+1 : len(fd.Name)-1]
					typeArgs := strings.Split(inner, ", ")
					subst := map[string]string{}
					for i, p := range params {
						if i < len(typeArgs) {
							subst[p.Name] = typeArgs[i]
						}
					}
					cg.templateSubst[mangleTemplateName(fd.Name)] = subst
				}
			}
		}
	}
}

func (cg *CodeGen) Emit(prog *Program, allProgs map[string]*Program) string {
	cg.allProgs = allProgs

	// Pre-scan function return types
	cg.registerFuncTypes(prog, allProgs)

	// Pre-scan for print calls to set needsStdio before header emission
	cg.scanForPrint(prog)
	for _, p := range allProgs {
		if p != prog {
			cg.scanForPrint(p)
		}
	}

	cg.emitHeader()
	
	// Pre-register throws functions
	cg.registerThrowsFuncs(prog)
	for _, p := range allProgs {
		if p != prog {
			cg.registerThrowsFuncs(p)
		}
	}
	for _, d := range cg.concreteDecls {
		if fd, ok := d.(*FuncDecl); ok && fd.Throws {
			cg.throwsFuncs[fd.Name] = true
		}
	}

	// Rule B: String typedefs (main program + all imports)
	allStringProgs := []*Program{prog}
	for _, p := range allProgs {
		if p != prog {
			allStringProgs = append(allStringProgs, p)
		}
	}
	cg.emitStringTypedefs(allStringProgs...)
	cg.emitStringHelpers()
	
	// External functions (main program + all imports)
	cg.emitExternalFuncs(prog)
	for _, p := range allProgs {
		if p != prog {
			cg.emitExternalFuncs(p)
		}
	}

	// Rule L: Emit all imported programs first
	for _, p := range allProgs {
		if p != prog {
			for _, stmt := range p.Statements {
				cg.genNode(stmt, p)
			}
			cg.output.WriteString("\n")
		}
	}

	// Emit type aliases first (before template instantiations that may reference them)
	for _, stmt := range prog.Statements {
		if _, ok := stmt.(*TypeAliasDecl); ok {
			cg.genNode(stmt, prog)
		}
	}
	for _, p := range allProgs {
		if p != prog {
			for _, stmt := range p.Statements {
				if _, ok := stmt.(*TypeAliasDecl); ok {
					cg.genNode(stmt, p)
				}
			}
		}
	}

	// Emit concrete template instantiations (structs, functions)
	for _, d := range cg.concreteDecls {
		switch d.(type) {
		case *StructDecl:
			cg.genNode(d, prog)
		}
	}
	for _, d := range cg.concreteDecls {
		switch d.(type) {
		case *FuncDecl:
			cg.genNode(d, prog)
		}
	}

	// Separate top-level declarations from code statements.
	// Declarations (FuncDecl, VarDecl, StructDecl, etc.) go at file scope.
	// Code statements (ExprStmt, AssignStmt, etc.) get auto-wrapped in main().
	var codeStmts []Node
	for _, stmt := range prog.Statements {
		if _, ok := stmt.(*TemplateDecl); ok {
			continue
		}
		switch s := stmt.(type) {
		case *FuncDecl, *StructDecl, *EnumDecl, *ExternalFuncDecl, *CBlock, *TemplateInstantiation:
			cg.genNode(stmt, prog)
		case *VarDecl:
			if isConstantInit(s.Value) {
				cg.genNode(s, prog)
			} else {
				declCopy := *s
				declCopy.Value = nil
				cg.genNode(&declCopy, prog)
				codeStmts = append(codeStmts, &AssignStmt{
					Tok:    s.Tok,
					Target: &Identifier{Tok: s.Tok, Value: s.Name},
					Value:  s.Value,
				})
			}
		default:
			codeStmts = append(codeStmts, stmt)
		}
	}
	if len(codeStmts) > 0 {
		cg.output.WriteString("int main() {\n")
		cg.pushIndent()
		for _, s := range codeStmts {
			cg.genNode(s, prog)
		}
		cg.emitDeferStack()
		cg.output.WriteString("\treturn 0;\n")
		cg.popIndent()
		cg.output.WriteString("}\n")
	}

	return cg.output.String()
}

func (cg *CodeGen) emitStringTypedefs(progs ...*Program) {
	cg.output.WriteString("typedef struct { char* data; int length; int capacity; } MerlinString;\n\n")
}

func (cg *CodeGen) emitStringHelpers() {
	cg.output.WriteString(
		"#define ms_new_len(s, len) ({\\\n" +
		"    const char* _ms_s = (s);\\\n" +
		"    int _ms_n = (len);\\\n" +
		"    char* _ms_d = (char*)malloc(_ms_n + 1);\\\n" +
		"    if (_ms_d) {\\\n" +
		"        if (_ms_s && _ms_n > 0) memcpy(_ms_d, _ms_s, _ms_n);\\\n" +
		"        _ms_d[_ms_n] = 0;\\\n" +
		"    }\\\n" +
		"    (MerlinString){_ms_d, _ms_n, _ms_n + 1};\\\n" +
		"})\n\n" +
		"#define ms_new_empty() ({\\\n" +
		"    char* _ms_d = (char*)malloc(1);\\\n" +
		"    if (_ms_d) _ms_d[0] = 0;\\\n" +
		"    (MerlinString){_ms_d, 0, 1};\\\n" +
		"})\n\n" +
		"static inline void ms_free(MerlinString* s) {\n" +
		"    if (s && s->data) {\n" +
		"        free(s->data);\n" +
		"        s->data = NULL;\n" +
		"        s->length = 0;\n" +
		"        s->capacity = 0;\n" +
		"    }\n" +
		"}\n\n")
}

func (cg *CodeGen) emitHeader() {

	cg.output.WriteString("#include <stdint.h>\n")
	cg.output.WriteString("#include <stdbool.h>\n")
	cg.output.WriteString("#include <string.h>\n")
	cg.output.WriteString("#include <stdio.h>\n")
	cg.output.WriteString("#include <stdlib.h>\n")
	cg.output.WriteString("\n")
}

func (cg *CodeGen) getExprType(n Node) string {
	switch expr := n.(type) {
	case *IntLiteral:
		return "int"
	case *FloatLiteral:
		return "float"
	case *StringLiteral:
		return "string"
	case *BoolLiteral:
		return "bool"
	case *CharLiteral:
		return "char"
	case *Identifier:
		if t, ok := cg.varTypes[expr.Value]; ok {
			return t
		}
		// Not a variable — treat as a type name (e.g. sizeof(Entry))
		return expr.Value
	case *BinaryExpr:
		if expr.Op == "in" {
			return "bool"
		}
		lt := cg.getExprType(expr.Left)
		rt := cg.getExprType(expr.Right)
		if lt == "string" && rt == "string" {
			if expr.Op == "+" {
				return "string"
			}
			if expr.Op == "==" || expr.Op == "!=" {
				return "bool"
			}
		}
		if lt == "float" || rt == "float" {
			return "float"
		}
		return "int"
	case *UnaryExpr:
		if expr.Op == "not" {
			return "bool"
		}
		return cg.getExprType(expr.Operand)
	case *CastExpr:
		return expr.TargetType
	case *ConvExpr:
		return expr.TargetType
	case *CallExpr:
		if inst, ok := expr.Function.(*TemplateInstantiation); ok {
			fullName := inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"
			if t, ok := cg.funcTypes[fullName]; ok {
				if len(t) > 0 {
					return t[0]
				}
			}
			if inst.ResolvedName != "" {
				if t, ok := cg.funcTypes[inst.ResolvedName]; ok {
					if len(t) > 0 {
						return t[0]
					}
				}
			}
			return "int"
		}
		if ident, ok := expr.Function.(*Identifier); ok {
			if ident.Value == "input" || ident.Value == "typeof" {
				return "string"
			}
			if t, ok := cg.funcTypes[ident.Value]; ok {
				if len(t) > 0 {
					return t[0]
				}
			}
			if ident.Value == "len" || ident.Value == "sizeof" {
				return "int"
			}
		}
		if sel, ok := expr.Function.(*SelectorExpr); ok {
			if t, ok := cg.funcTypes[sel.Field]; ok {
				if len(t) > 0 {
					return t[0]
				}
			}
			if id, ok := sel.Object.(*Identifier); ok {
				if varType, ok := cg.varTypes[id.Value]; ok {
					structName := mangleTemplateName(varType)
					if t, ok := cg.funcTypes[structName+"_"+sel.Field]; ok {
						if len(t) > 0 {
							return t[0]
						}
					}
				}
			}
		}
		return "int"
	case *SelectorExpr:
		if ident, ok := expr.Object.(*Identifier); ok {
			if t, ok := cg.varTypes[ident.Value]; ok {
				structName := mangleTemplateName(t)
				if fields, ok := cg.structFields[structName]; ok {
					if ft, ok := fields[expr.Field]; ok {
						return ft
					}
				}
			}
		}
		return "int"
	case *IndexExpr:
		if ident, ok := expr.Collection.(*Identifier); ok {
			if t, ok := cg.varTypes[ident.Value]; ok {
				if idx := strings.Index(t, "["); idx != -1 {
					return t[:idx]
				}
			}
		}
		collType := cg.getExprType(expr.Collection)
		if strings.HasSuffix(collType, "*") {
			return collType[:len(collType)-1]
		}
		return "int"
	case *SliceExpr:
		collType := cg.getExprType(expr.Collection)
		if collType == "string" {
			return "string"
		}
		if idx := strings.Index(collType, "["); idx != -1 {
			baseType := collType[:idx]
			if expr.SliceSize > 0 {
				return fmt.Sprintf("%s[%d]", baseType, expr.SliceSize)
			}
			return baseType + "[0]"
		}
		return "int"
	default:
		return "int"
	}
}

func (cg *CodeGen) isPointerExpr(n Node) bool {
	switch expr := n.(type) {
	case *Identifier:
		return cg.pointerVars[expr.Value]
	}
	return false
}

func (cg *CodeGen) genNode(n Node, prog *Program) {
	if n == nil {
		return
	}

	switch node := n.(type) {
		case *TypeAliasDecl:
			cg.output.WriteString(fmt.Sprintf("typedef %s %s;\n", mangleTemplateName(cg.resolvePrimitiveType(node.BaseType)), node.Name))
		case *EnumDecl:
			cg.output.WriteString(fmt.Sprintf("typedef enum {\n"))
			for i, ev := range node.Values {
				if i > 0 {
					cg.output.WriteString(",\n")
				}
				cg.output.WriteString(fmt.Sprintf("\t%s", ev.Name))
				if ev.Value != nil {
					if lit, ok := ev.Value.(*IntLiteral); ok {
						cg.output.WriteString(fmt.Sprintf(" = %d", lit.Value))
					}
				}
			}
			cg.output.WriteString("\n")
			cg.output.WriteString(fmt.Sprintf("} %s;\n\n", node.Name))
	case *ExternalFuncDecl:
		// Handled in emitExternalFuncs
	case *StructDecl:
		structName := mangleTemplateName(node.Name)
		cg.output.WriteString(fmt.Sprintf("typedef struct {\n"))

		cg.pushIndent()
		for _, f := range node.Fields {
			typeName := f.TypeAnnot
			if typeName == "string" {
				typeName = "MerlinString"
			} else {
				typeName = cg.resolvePrimitiveType(typeName)
			}
			typeName = mangleTemplateName(typeName)
			if f.IsPointer {
				typeName += "*"
			}
			if f.IsVolatile {
				typeName = "volatile " + typeName
			}
			cg.output.WriteString(fmt.Sprintf("%s %s;\n", typeName, f.Name))
		}
		cg.popIndent()
		cg.output.WriteString(fmt.Sprintf("} %s;\n\n", structName))

		for _, m := range node.Methods {
			cg.genFunc(m, node.Name, prog)
		}

	case *FuncDecl:
		cg.genFunc(node, "", prog)

	case *VarDecl:
		typeName := node.TypeAnnot
		if typeName == "string" {
			typeName = "MerlinString"
		} else {
			typeName = mangleTemplateName(cg.resolvePrimitiveType(typeName))
		}
		if node.IsPointer {
			typeName += "*"
		}
		if node.IsVolatile {
			typeName = "volatile " + typeName
		}
		if node.IsConst && typeName != "MerlinString" {
			typeName = "const " + typeName
		}
		typeAnnot := node.TypeAnnot
		if node.ArraySize != "" {
			typeAnnot = typeAnnot + "[" + node.ArraySize + "]"
		}
		cg.varTypes[node.Name] = typeAnnot
		cg.pointerVars[node.Name] = node.IsPointer

		// Handle throws function call as initializer
		if node.Value != nil {
			if call, ok := node.Value.(*CallExpr); ok {
				funcName := cg.getCallFuncName(call)
				if funcName == "input" {
					cg.needsStdio = true
					lenName := fmt.Sprintf("_inlen_%s", node.Name)
					cg.output.WriteString(fmt.Sprintf("char _inbuf_%s[4096];\n", node.Name))
					cg.output.WriteString(fmt.Sprintf("%s %s;\n", typeName, node.Name))
					if len(call.Args) == 1 {
						cg.emitPrintString(call.Args[0], prog)
						cg.output.WriteString(";\n")
					}
					cg.output.WriteString(fmt.Sprintf("fgets(_inbuf_%s, 4096, stdin);\n", node.Name))
					cg.output.WriteString(fmt.Sprintf("int %s = strlen(_inbuf_%s);\n", lenName, node.Name))
					cg.output.WriteString(fmt.Sprintf("if (%s > 0 && _inbuf_%s[%s-1] == '\\n') { _inbuf_%s[--%s] = '\\0'; }\n",
						lenName, node.Name, lenName, node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("%s.data = malloc(%s + 1);\n", node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("memcpy(%s.data, _inbuf_%s, %s + 1);\n", node.Name, node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("%s.length = %s;\n", node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("%s.capacity = %s;\n", node.Name, lenName))
					break
				}
			if funcName != "" && cg.throwsFuncs[funcName] {
				outType := cg.getOutCType(funcName)
				if outType == "void" {
					if cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("%s = %s(", cg.tryErrVar, funcName))
						for _, arg := range call.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(");\n")
						cg.output.WriteString(fmt.Sprintf("if (%s) goto %s;\n", cg.tryErrVar, cg.tryEndLabel))
					} else {
						cg.output.WriteString(fmt.Sprintf("%s(", funcName))
						for _, arg := range call.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(");\n")
					}
					break
				}
				outNum := cg.tryOutCounter
				cg.tryOutCounter++
				outVar := fmt.Sprintf("_out_%d", outNum)
				cg.output.WriteString(fmt.Sprintf("%s %s;\n", outType, outVar))
				if cg.tryErrVar != "" {
					cg.output.WriteString(fmt.Sprintf("%s = %s(&%s", cg.tryErrVar, funcName, outVar))
			for _, arg := range call.Args {
						cg.output.WriteString(", ")
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(");\n")
					cg.output.WriteString(fmt.Sprintf("if (%s) goto %s;\n", cg.tryErrVar, cg.tryEndLabel))
				} else {
					cg.output.WriteString(fmt.Sprintf("%s(&%s", funcName, outVar))
					for _, arg := range call.Args {
						cg.output.WriteString(", ")
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(");\n")
				}
				cg.output.WriteString(fmt.Sprintf("%s %s = %s;\n", typeName, node.Name, outVar))
				break
			}
		}
	}

		cg.output.WriteString(fmt.Sprintf("%s %s", typeName, node.Name))
		if node.ArraySize != "" {
			cg.output.WriteString(fmt.Sprintf("[%s]", node.ArraySize))
		}
		if node.Value != nil {
			cg.output.WriteString(" = ")
			if typeName == "MerlinString" {
				cg.genNode(node.Value, prog)
			} else {
				cg.genNode(node.Value, prog)
			}
		}
		cg.output.WriteString(";\n")

	case *ShortVarDecl:
		typeName := node.TypeAnnot
		if typeName == "string" {
			typeName = "MerlinString"
		} else {
			typeName = mangleTemplateName(cg.resolvePrimitiveType(typeName))
		}
		if node.IsConst && typeName != "MerlinString" {
			typeName = "const " + typeName
		}
		cg.varTypes[node.Name] = node.TypeAnnot
		cg.pointerVars[node.Name] = false

		if node.Value != nil {
			if call, ok := node.Value.(*CallExpr); ok {
				funcName := cg.getCallFuncName(call)
				if funcName == "input" {
					cg.needsStdio = true
					lenName := fmt.Sprintf("_inlen_%s", node.Name)
					cg.output.WriteString(fmt.Sprintf("char _inbuf_%s[4096];\n", node.Name))
					cg.output.WriteString(fmt.Sprintf("%s %s;\n", typeName, node.Name))
					if len(call.Args) == 1 {
						cg.emitPrintString(call.Args[0], prog)
						cg.output.WriteString(";\n")
					}
					cg.output.WriteString(fmt.Sprintf("fgets(_inbuf_%s, 4096, stdin);\n", node.Name))
					cg.output.WriteString(fmt.Sprintf("int %s = strlen(_inbuf_%s);\n", lenName, node.Name))
					cg.output.WriteString(fmt.Sprintf("if (%s > 0 && _inbuf_%s[%s-1] == '\\n') { _inbuf_%s[--%s] = '\\0'; }\n",
						lenName, node.Name, lenName, node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("%s.data = malloc(%s + 1);\n", node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("memcpy(%s.data, _inbuf_%s, %s + 1);\n", node.Name, node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("%s.length = %s;\n", node.Name, lenName))
					cg.output.WriteString(fmt.Sprintf("%s.capacity = %s;\n", node.Name, lenName))
					break
				}
				if funcName != "" && cg.throwsFuncs[funcName] {
					outType := cg.getOutCType(funcName)
					if outType == "void" {
						if cg.tryErrVar != "" {
							cg.output.WriteString(fmt.Sprintf("%s = %s(", cg.tryErrVar, funcName))
							for _, arg := range call.Args {
								cg.output.WriteString(", ")
								cg.genNode(arg, prog)
							}
							cg.output.WriteString(");\n")
							cg.output.WriteString(fmt.Sprintf("if (%s) goto %s;\n", cg.tryErrVar, cg.tryEndLabel))
						} else {
							cg.output.WriteString(fmt.Sprintf("%s(", funcName))
							for _, arg := range call.Args {
								cg.output.WriteString(", ")
								cg.genNode(arg, prog)
							}
							cg.output.WriteString(");\n")
						}
						break
					}
					outNum := cg.tryOutCounter
					cg.tryOutCounter++
					outVar := fmt.Sprintf("_out_%d", outNum)
					cg.output.WriteString(fmt.Sprintf("%s %s;\n", outType, outVar))
					if cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("%s = %s(&%s", cg.tryErrVar, funcName, outVar))
						for _, arg := range call.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(");\n")
						cg.output.WriteString(fmt.Sprintf("if (%s) goto %s;\n", cg.tryErrVar, cg.tryEndLabel))
					} else {
						cg.output.WriteString(fmt.Sprintf("%s(&%s", funcName, outVar))
						for _, arg := range call.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(");\n")
					}
					cg.output.WriteString(fmt.Sprintf("%s %s = %s;\n", typeName, node.Name, outVar))
					break
				}
			}
		}

		if node.Value != nil {
			if slice, ok := node.Value.(*SliceExpr); ok && slice.SliceSize > 0 {
				if idx := strings.Index(typeName, "["); idx != -1 {
					baseType := typeName[:idx]
					cBaseType := mangleTemplateName(cg.resolvePrimitiveType(baseType))
					sizePart := typeName[idx:]
					cg.output.WriteString(fmt.Sprintf("%s %s%s", cBaseType, node.Name, sizePart))
					cg.output.WriteString(";\n")
					cg.output.WriteString(fmt.Sprintf("memcpy(%s, (", node.Name))
					cg.genNode(slice.Collection, prog)
					cg.output.WriteString(") + ")
					if slice.Low != nil {
						cg.genNode(slice.Low, prog)
					} else {
						cg.output.WriteString("0")
					}
					cg.output.WriteString(fmt.Sprintf(", %d * sizeof(%s));\n", slice.SliceSize, cBaseType))
					return
				}
			}
		}
		if idx := strings.Index(typeName, "["); idx != -1 {
			baseType := typeName[:idx]
			sizePart := typeName[idx:]
			cBaseType := mangleTemplateName(cg.resolvePrimitiveType(baseType))
			cg.output.WriteString(fmt.Sprintf("%s %s%s", cBaseType, node.Name, sizePart))
		} else {
			cg.output.WriteString(fmt.Sprintf("%s %s", typeName, node.Name))
		}
		if node.Value != nil {
			cg.output.WriteString(" = ")
			cg.genNode(node.Value, prog)
		}
		cg.output.WriteString(";\n")

	case *BlockStmt:
		cg.output.WriteString("{\n")
		cg.pushIndent()
		for _, s := range node.Statements {
			cg.genNode(s, prog)
		}
		cg.popIndent()
		cg.output.WriteString("}\n")

	case *AssignStmt:
		if call, ok := node.Value.(*CallExpr); ok {
			funcName := cg.getCallFuncName(call)
			if funcName == "input" {
				cg.needsStdio = true
				targetName := ""
				if id, ok := node.Target.(*Identifier); ok {
					targetName = id.Value
				}
				lenName := fmt.Sprintf("_aslen_%s", targetName)
				cg.output.WriteString(fmt.Sprintf("char _asbuf_%s[4096];\n", targetName))
				if len(call.Args) == 1 {
					cg.emitPrintString(call.Args[0], prog)
					cg.output.WriteString(";\n")
				}
				cg.output.WriteString(fmt.Sprintf("fgets(_asbuf_%s, 4096, stdin);\n", targetName))
				cg.output.WriteString(fmt.Sprintf("int %s = strlen(_asbuf_%s);\n", lenName, targetName))
				cg.output.WriteString(fmt.Sprintf("if (%s > 0 && _asbuf_%s[%s-1] == '\\n') { _asbuf_%s[--%s] = '\\0'; }\n",
					lenName, targetName, lenName, targetName, lenName))
				cg.output.WriteString(fmt.Sprintf("free(%s.data);\n", targetName))
				cg.output.WriteString(fmt.Sprintf("%s.data = malloc(%s + 1);\n", targetName, lenName))
				cg.output.WriteString(fmt.Sprintf("memcpy(%s.data, _asbuf_%s, %s + 1);\n", targetName, targetName, lenName))
				cg.output.WriteString(fmt.Sprintf("%s.length = %s;\n", targetName, lenName))
				cg.output.WriteString(fmt.Sprintf("%s.capacity = %s;\n", targetName, lenName))
				break
			}
			if funcName != "" && cg.throwsFuncs[funcName] {
				outNum := cg.tryOutCounter
				cg.tryOutCounter++
				outVar := fmt.Sprintf("_out_%d", outNum)
				outType := cg.getOutCType(funcName)
				cg.output.WriteString(fmt.Sprintf("%s %s;\n", outType, outVar))
				if cg.tryErrVar != "" {
					cg.output.WriteString(fmt.Sprintf("%s = %s(&%s", cg.tryErrVar, funcName, outVar))
				for _, arg := range call.Args {
						cg.output.WriteString(", ")
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(");\n")
					cg.output.WriteString(fmt.Sprintf("if (%s) goto %s;\n", cg.tryErrVar, cg.tryEndLabel))
				} else {
					cg.output.WriteString(fmt.Sprintf("%s(&%s", funcName, outVar))
					for _, arg := range call.Args {
						cg.output.WriteString(", ")
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(");\n")
				}
				cg.genNode(node.Target, prog)
				cg.output.WriteString(fmt.Sprintf(" = %s;\n", outVar))
				break
			}
		}
	// Handle array element assignment with bounds check
	if idxExpr, ok := node.Target.(*IndexExpr); ok {
		if collId, ok := idxExpr.Collection.(*Identifier); ok {
			collType := cg.getExprType(idxExpr.Collection)
			bound := ""
			doCheck := false
			if collType != "string" {
				if t, ok := cg.varTypes[collId.Value]; ok {
					if start := strings.Index(t, "["); start != -1 {
						end := strings.Index(t, "]")
						if end > start+1 {
							bound = t[start+1 : end]
							doCheck = true
						} else if strings.HasSuffix(t, "[]") {
							if lenVar, ok := cg.arrayLenVars[collId.Value]; ok {
								bound = lenVar
								doCheck = true
							}
						}
					}
				}
			}
			if doCheck {
				cg.output.WriteString("{\n")
				cg.pushIndent()
				cg.output.WriteString("int _idx_ = ")
				cg.genNode(idxExpr.Index, prog)
				cg.output.WriteString(";\n")
				cg.output.WriteString("if (_idx_ < 0 || _idx_ >= ")
				cg.output.WriteString(bound)
				cg.output.WriteString(") { ")
				if cg.inThrowsFunc && cg.tryErrVar != "" {
					cg.output.WriteString(fmt.Sprintf("%s = 1; goto %s; ", cg.tryErrVar, cg.tryEndLabel))
				} else if cg.inThrowsFunc {
					cg.output.WriteString("return 1; ")
				} else {
					cg.output.WriteString("fprintf(stderr, \"Merlin runtime: index %d out of bounds\\n\", _idx_); exit(1); ")
				}
				cg.output.WriteString("}\n")
				cg.genNode(collId, prog)
				cg.output.WriteString("[_idx_] = ")
				cg.genNode(node.Value, prog)
				cg.output.WriteString(";\n")
				cg.popIndent()
				cg.output.WriteString("}\n")
				break
			}
		}
	}
	cg.genNode(node.Target, prog)
	cg.output.WriteString(" = ")
	cg.genNode(node.Value, prog)
	cg.output.WriteString(";\n")
	case *MultiAssignStmt:
		if call, ok := node.Values[0].(*CallExpr); ok && len(node.Values) == 1 {
			funcName := cg.getCallFuncName(call)
			retTypes := cg.funcTypes[funcName]
			if retTypes != nil && len(retTypes) > 1 {
				cg.output.WriteString(funcName)
				cg.output.WriteString("(")
				for i, tgt := range node.Targets {
					if i > 0 {
						cg.output.WriteString(", ")
					}
					cg.output.WriteString("&")
					cg.genNode(tgt, prog)
				}
				for _, arg := range call.Args {
					cg.output.WriteString(", ")
					cg.genNode(arg, prog)
				}
				cg.output.WriteString(");\n")
				break
			}
		}
		for i, tgt := range node.Targets {
			if i < len(node.Values) {
				cg.genNode(tgt, prog)
				cg.output.WriteString(" = ")
				cg.genNode(node.Values[i], prog)
				cg.output.WriteString(";\n")
			}
		}
	case *MultiShortVarDecl:
		if len(node.Values) == 1 {
			if call, ok := node.Values[0].(*CallExpr); ok {
				funcName := cg.getCallFuncName(call)
				retTypes := cg.funcTypes[funcName]
				if retTypes != nil && len(retTypes) > 1 && len(retTypes) == len(node.Names) {
					for i, name := range node.Names {
						typeName := retTypes[i]
						if typeName == "string" {
							typeName = "MerlinString"
						} else {
							typeName = mangleTemplateName(cg.resolvePrimitiveType(typeName))
						}
						cg.output.WriteString(fmt.Sprintf("%s %s", typeName, name))
						cg.varTypes[name] = retTypes[i]
						if i < len(node.Names)-1 {
							cg.output.WriteString(";\n")
						}
					}
					cg.output.WriteString(";\n")
					cg.output.WriteString(funcName)
					cg.output.WriteString("(")
					for i, name := range node.Names {
						if i > 0 {
							cg.output.WriteString(", ")
						}
						cg.output.WriteString("&")
						cg.output.WriteString(name)
					}
					for _, arg := range call.Args {
						cg.output.WriteString(", ")
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(");\n")
					break
				}
			}
		}
		for i, name := range node.Names {
			if i < len(node.Values) {
				t := cg.getExprType(node.Values[i])
				typeName := t
				if typeName == "string" {
					typeName = "MerlinString"
				} else {
					typeName = mangleTemplateName(cg.resolvePrimitiveType(typeName))
				}
				cg.varTypes[name] = t
				if idx := strings.Index(typeName, "["); idx != -1 {
					baseType := typeName[:idx]
					sizePart := typeName[idx:]
					cg.output.WriteString(fmt.Sprintf("%s %s%s", baseType, name, sizePart))
				} else {
					cg.output.WriteString(fmt.Sprintf("%s %s", typeName, name))
				}
				cg.output.WriteString(" = ")
				cg.genNode(node.Values[i], prog)
				cg.output.WriteString(";\n")
			}
		}
	case *ReturnStmt:
		cg.emitDeferStack()
		if cg.inThrowsFunc {
			for i, v := range node.Values {
				if i > 0 || len(cg.funcTypes[cg.currentFuncName]) > 1 {
					outName := fmt.Sprintf("_out_%d", i)
					cg.output.WriteString(fmt.Sprintf("*%s = ", outName))
					cg.genNode(v, prog)
					cg.output.WriteString(";\n")
				} else {
					cg.output.WriteString("*_out = ")
					cg.genNode(v, prog)
					cg.output.WriteString(";\n")
				}
			}
			cg.output.WriteString("return 0;\n")
		} else {
			retTypes := cg.funcTypes[cg.currentFuncName]
			isMulti := len(retTypes) > 1
			if isMulti {
				for i, v := range node.Values {
					outName := fmt.Sprintf("_out_%d", i)
					cg.output.WriteString(fmt.Sprintf("*%s = ", outName))
					cg.genNode(v, prog)
					cg.output.WriteString(";\n")
				}
			} else {
				if len(node.Values) > 0 {
					cg.output.WriteString("return ")
					cg.genNode(node.Values[0], prog)
					cg.output.WriteString(";\n")
				} else {
					isVoid := len(retTypes) == 0 || retTypes[0] == "" || retTypes[0] == "void"
					if isVoid {
						cg.output.WriteString("return;\n")
					} else {
						cg.output.WriteString("return 0;\n")
					}
				}
			}
		}
	case *BreakStmt:
		cg.output.WriteString("break;\n")
	case *CompoundAssignStmt:
		if idxExpr, ok := node.Target.(*IndexExpr); ok {
			if collId, ok := idxExpr.Collection.(*Identifier); ok {
				collType := cg.getExprType(idxExpr.Collection)
				bound := ""
				doCheck := false
				if collType != "string" {
					if t, ok := cg.varTypes[collId.Value]; ok {
						if start := strings.Index(t, "["); start != -1 {
							end := strings.Index(t, "]")
							if end > start+1 {
								bound = t[start+1 : end]
								doCheck = true
							} else if strings.HasSuffix(t, "[]") {
								if lenVar, ok := cg.arrayLenVars[collId.Value]; ok {
									bound = lenVar
									doCheck = true
								}
							}
						}
					}
				}
				if doCheck {
					cg.output.WriteString("{\n")
					cg.pushIndent()
					cg.output.WriteString("int _idx_ = ")
					cg.genNode(idxExpr.Index, prog)
					cg.output.WriteString(";\n")
					cg.output.WriteString("if (_idx_ < 0 || _idx_ >= ")
					cg.output.WriteString(bound)
					cg.output.WriteString(") { ")
					if cg.inThrowsFunc && cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("%s = 1; goto %s; ", cg.tryErrVar, cg.tryEndLabel))
					} else if cg.inThrowsFunc {
						cg.output.WriteString("return 1; ")
					} else {
						cg.output.WriteString("fprintf(stderr, \"Merlin runtime: index %d out of bounds\\n\", _idx_); exit(1); ")
					}
					cg.output.WriteString("}\n")
					cg.genNode(collId, prog)
					cg.output.WriteString(fmt.Sprintf("[_idx_] %s ", node.Op))
					cg.genNode(node.Value, prog)
					cg.output.WriteString(";\n")
					cg.popIndent()
					cg.output.WriteString("}\n")
					break
				}
			}
		}
		cg.genNode(node.Target, prog)
		cg.output.WriteString(fmt.Sprintf(" %s ", node.Op))
		cg.genNode(node.Value, prog)
		cg.output.WriteString(";\n")
	case *ContinueStmt:
		cg.output.WriteString("continue;\n")
	case *PassStmt:
		cg.output.WriteString(";\n")
	case *DeferStmt:
		var buf strings.Builder
		oldOutput := cg.output
		cg.output = &buf
		if call, ok := node.Call.(*CallExpr); ok {
			funcName := cg.getCallFuncName(call)
			if funcName == "print" {
				cg.genPrint(call, prog)
			} else if funcName == "input" {
				cg.genInput(call, prog)
			} else {
				cg.genNode(node.Call, prog)
				cg.output.WriteString(";\n")
			}
		} else {
			cg.genNode(node.Call, prog)
			cg.output.WriteString(";\n")
		}
		cg.output = oldOutput
		cg.deferStack = append(cg.deferStack, buf.String())
	case *RaiseStmt:
		cg.output.WriteString("return ")
		cg.genNode(node.Value, prog)
		cg.output.WriteString(";\n")
	case *TryCatchStmt:
		tryNum := cg.tryCounter
		cg.tryCounter++
		errVar := fmt.Sprintf("_try_err_%d", tryNum)
		endLabel := fmt.Sprintf("_try_end_%d", tryNum)

		cg.output.WriteString("{\n")
		cg.pushIndent()
		cg.output.WriteString(fmt.Sprintf("int %s = 0;\n", errVar))

		oldErrVar := cg.tryErrVar
		oldEndLabel := cg.tryEndLabel
		cg.tryErrVar = errVar
		cg.tryEndLabel = endLabel

		cg.genNode(node.Body, prog)

		cg.output.WriteString(fmt.Sprintf("%s: ;\n", endLabel))

		cg.tryErrVar = oldErrVar
		cg.tryEndLabel = oldEndLabel

		for i, cc := range node.Catches {
			if i == 0 {
				if cc.IsCatchAll {
					cg.output.WriteString(fmt.Sprintf("if (%s) {\n", errVar))
				} else {
					cg.output.WriteString(fmt.Sprintf("if (%s == ", errVar))
					cg.genNode(cc.Value, prog)
					cg.output.WriteString(") {\n")
				}
			} else {
				if cc.IsCatchAll {
					cg.output.WriteString(fmt.Sprintf("} else if (%s) {\n", errVar))
				} else {
					cg.output.WriteString("} else if (")
					cg.output.WriteString(errVar)
					cg.output.WriteString(" == ")
					cg.genNode(cc.Value, prog)
					cg.output.WriteString(") {\n")
				}
			}
			cg.pushIndent()
			for _, s := range cc.Body.Statements {
				cg.genNode(s, prog)
			}
			cg.popIndent()
		}
		cg.output.WriteString("}\n")

		cg.popIndent()
		cg.output.WriteString("}\n")
	case *IfStmt:
		cg.output.WriteString("if (")
		cg.genNode(node.Condition, prog)
		cg.output.WriteString(") ")
		cg.genNode(node.Consequence, prog)
		
		for _, elif := range node.Alternatives {
			cg.output.WriteString("else if (")
			cg.genNode(elif.Condition, prog)
			cg.output.WriteString(") ")
			cg.genNode(elif.Body, prog)
		}
		
		if node.Else != nil {
			cg.output.WriteString("else ")
			cg.genNode(node.Else, prog)
		}

	case *ForRangeStmt:
		cg.varTypes[node.Var] = "int"
		cg.output.WriteString("for (int ")
		cg.output.WriteString(node.Var)
		cg.output.WriteString(" = ")
		cg.genNode(node.From, prog)
		cg.output.WriteString("; ")
		cg.output.WriteString(node.Var)
		cg.output.WriteString(" < ")
		cg.genNode(node.To, prog)
		cg.output.WriteString("; ")
		cg.output.WriteString(node.Var)
		cg.output.WriteString("++) ")
		cg.genNode(node.Body, prog)

	case *ForCondStmt:
		cg.output.WriteString("while (")
		cg.genNode(node.Condition, prog)
		cg.output.WriteString(") ")
		cg.genNode(node.Body, prog)

	case *ForInfiniteStmt:
		cg.output.WriteString("while (1) ")
		cg.genNode(node.Body, prog)

	case *MatchStmt:
		first := true
		hasAnyCondition := false
		for _, c := range node.Cases {
			if !c.IsCatchAll {
				hasAnyCondition = true
				break
			}
		}

		for _, c := range node.Cases {
			if c.IsCatchAll {
				if hasAnyCondition {
					cg.output.WriteString("else ")
				} else {
					cg.output.WriteString("if (true) ")
				}
				cg.genNode(c.Body, prog)
			} else {
				if first {
					cg.output.WriteString("if (")
					first = false
				} else {
					cg.output.WriteString("else if (")
				}
				cg.genNode(node.Subject, prog)
				cg.output.WriteString(" == ")
				cg.genNode(c.Value, prog)
				cg.output.WriteString(") ")
				cg.genNode(c.Body, prog)
			}
		}
	case *AsmBlock:

		cg.output.WriteString("__asm__ volatile(\n")
		for _, line := range node.Lines {
			cg.pushIndent()
			cg.output.WriteString(fmt.Sprintf("%s\n", line))
			cg.popIndent()
		}
		cg.output.WriteString(");\n")

	case *CBlock:
		for _, line := range node.Lines {
			cg.output.WriteString(fmt.Sprintf("%s\n", line))
		}

	case *ExprStmt:
		if call, ok := node.Expression.(*CallExpr); ok {
			funcName := cg.getCallFuncName(call)
			if funcName == "print" {
				cg.genPrint(call, prog)
				break
			}
			if funcName == "input" {
				cg.genInput(call, prog)
				break
			}
			if funcName != "" && cg.throwsFuncs[funcName] {
				outType := cg.getOutCType(funcName)
				if outType == "void" {
					if cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("%s = %s(", cg.tryErrVar, funcName))
						for _, arg := range call.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(");\n")
						cg.output.WriteString(fmt.Sprintf("if (%s) goto %s;\n", cg.tryErrVar, cg.tryEndLabel))
					} else {
						cg.output.WriteString(fmt.Sprintf("%s(", funcName))
						for _, arg := range call.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(");\n")
					}
					break
				}
				outNum := cg.tryOutCounter
				cg.tryOutCounter++
				outVar := fmt.Sprintf("_out_%d", outNum)
				outType = cg.getOutCType(funcName)
				cg.output.WriteString(fmt.Sprintf("%s %s;\n", outType, outVar))
				if cg.tryErrVar != "" {
					cg.output.WriteString(fmt.Sprintf("%s = %s(&%s", cg.tryErrVar, funcName, outVar))
					for _, arg := range call.Args {
						cg.output.WriteString(", ")
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(");\n")
					cg.output.WriteString(fmt.Sprintf("if (%s) goto %s;\n", cg.tryErrVar, cg.tryEndLabel))
				} else {
					cg.output.WriteString(fmt.Sprintf("%s(&%s", funcName, outVar))
					for _, arg := range call.Args {
						cg.output.WriteString(", ")
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(");\n")
				}
				break
			}
		}
		cg.genNode(node.Expression, prog)
		cg.output.WriteString(";\n")
	case *TemplateInstantiation:
		cg.output.WriteString(mangleTemplateName(node.Name + "<" + strings.Join(node.TypeArgs, ", ") + ">"))

	case *Identifier:
		cg.output.WriteString(node.Value)

	case *IntLiteral:
		cg.output.WriteString(fmt.Sprintf("%d", node.Value))

	case *FloatLiteral:
		cg.output.WriteString(fmt.Sprintf("%f", node.Value))

	case *StringLiteral:
		val := node.Value
		lenVal := len(val)
		if lenVal == 0 {
			cg.output.WriteString("ms_new_empty()")
		} else {
			cg.output.WriteString(fmt.Sprintf("ms_new_len(%s, %d)", cStringLiteral(val), lenVal))
		}

	case *BoolLiteral:
		if node.Value {
			cg.output.WriteString("true")
		} else {
			cg.output.WriteString("false")
		}

	case *CharLiteral:
		cg.output.WriteString(charToCLiteral(rune(node.Value)))

	case *BinaryExpr:
		op := node.Op
		if op == "in" {
			rhsType := cg.getExprType(node.Right)
			lhsType := cg.getExprType(node.Left)
			if rhsType == "string" {
				if lhsType == "char" {
					cg.output.WriteString("(strchr(")
					cg.emitStringData(node.Right, prog)
					cg.output.WriteString(", ")
					cg.genNode(node.Left, prog)
					cg.output.WriteString(") != NULL)")
				} else {
					cg.output.WriteString("(strstr(")
					cg.emitStringData(node.Right, prog)
					cg.output.WriteString(", ")
					cg.emitStringData(node.Left, prog)
					cg.output.WriteString(") != NULL)")
				}
			} else {
				cg.output.WriteString("({ int _found = 0; ")
				cg.output.WriteString("int _n = sizeof(")
				cg.genNode(node.Right, prog)
				cg.output.WriteString(") / sizeof(")
				cg.genNode(node.Right, prog)
				cg.output.WriteString("[0]); for (int _i = 0; _i < _n; _i++) { if (")
				cg.genNode(node.Right, prog)
				cg.output.WriteString("[_i] == ")
				cg.genNode(node.Left, prog)
				cg.output.WriteString(") { _found = 1; break; } } ")
				cg.output.WriteString("_found; })")
			}
			break
		}
		// String concatenation
		lt := cg.getExprType(node.Left)
		rt := cg.getExprType(node.Right)
		if lt == "string" && rt == "string" && op == "+" {
			cg.output.WriteString("({ MerlinString _l = ")
			cg.genNode(node.Left, prog)
			cg.output.WriteString("; MerlinString _r = ")
			cg.genNode(node.Right, prog)
			cg.output.WriteString("; int _n = _l.length + _r.length; char* _d = malloc(_n + 1); memcpy(_d, _l.data, _l.length); memcpy(_d + _l.length, _r.data, _r.length); _d[_n] = 0; MerlinString _s = {_d, _n, _n + 1}; _s; })")
			break
		}
		if lt == "string" && rt == "string" && (op == "==" || op == "!=") {
			cg.output.WriteString("(strcmp(")
			cg.emitStringData(node.Left, prog)
			cg.output.WriteString(", ")
			cg.emitStringData(node.Right, prog)
			if op == "==" {
				cg.output.WriteString(") == 0)")
			} else {
				cg.output.WriteString(") != 0)")
			}
			break
		}
		if op == "and" {
			op = "&&"
		} else if op == "or" {
			op = "||"
		}
		cg.output.WriteString("(")
		cg.genNode(node.Left, prog)
		cg.output.WriteString(fmt.Sprintf(" %s ", op))
		cg.genNode(node.Right, prog)
		cg.output.WriteString(")")

	case *UnaryExpr:
		op := node.Op
		if op == "not" {
			op = "!"
		}
		cg.output.WriteString(op)
		cg.output.WriteString("(")
		cg.genNode(node.Operand, prog)
		cg.output.WriteString(")")

	case *IndexExpr:
		collType := cg.getExprType(node.Collection)
		bound := ""
		doCheck := false
		if id, ok := node.Collection.(*Identifier); ok {
			if collType == "string" {
				bound = ".length"
				doCheck = true
			} else if t, ok := cg.varTypes[id.Value]; ok {
				if start := strings.Index(t, "["); start != -1 {
					end := strings.Index(t, "]")
					if end > start+1 {
						bound = t[start+1 : end]
						doCheck = true
					} else if strings.HasSuffix(t, "[]") {
						if lenVar, ok := cg.arrayLenVars[id.Value]; ok {
							bound = lenVar
							doCheck = true
						}
					}
				}
			}
		}
		if doCheck {
			cg.output.WriteString("({ int _idx_ = (")
			cg.genNode(node.Index, prog)
			cg.output.WriteString("); if (_idx_ < 0 || _idx_ >= ")
			if collType == "string" {
				id := node.Collection.(*Identifier)
				cg.output.WriteString(id.Value)
				cg.output.WriteString(bound)
			} else {
				cg.output.WriteString(bound)
			}
			cg.output.WriteString(") { ")
			if cg.inThrowsFunc && cg.tryErrVar != "" {
				cg.output.WriteString(fmt.Sprintf("%s = 1; goto %s; ", cg.tryErrVar, cg.tryEndLabel))
			} else if cg.inThrowsFunc {
				cg.output.WriteString("return 1; ")
			} else {
				cg.output.WriteString("fprintf(stderr, \"Merlin runtime: index %d out of bounds\\n\", _idx_); exit(1); ")
			}
			cg.output.WriteString("} ")
			cg.genNode(node.Collection, prog)
			if collType == "string" {
				cg.output.WriteString(".data[_idx_]; })")
			} else {
				cg.output.WriteString("[_idx_]; })")
			}
		} else {
			cg.genNode(node.Collection, prog)
			if collType == "string" {
				cg.output.WriteString(".data[")
			} else {
				cg.output.WriteString("[")
			}
			cg.genNode(node.Index, prog)
			cg.output.WriteString("]")
		}

	case *SliceExpr:
		collType := cg.getExprType(node.Collection)
		if collType == "string" {
			cg.output.WriteString("({ MerlinString _s = ")
			cg.genNode(node.Collection, prog)
			cg.output.WriteString("; int _l = ")
			if node.Low != nil {
				cg.genNode(node.Low, prog)
			} else {
				cg.output.WriteString("0")
			}
			cg.output.WriteString("; int _h = ")
			if node.High != nil {
				cg.genNode(node.High, prog)
			} else {
				cg.output.WriteString("_s.length")
			}
			cg.output.WriteString("; int _n = _h - _l; char* _d = malloc(_n + 1); if (_l < 0 || _l > _s.length || _h < 0 || _h > _s.length || _l > _h) { ")
			if cg.inThrowsFunc && cg.tryErrVar != "" {
				cg.output.WriteString(fmt.Sprintf("%s = 1; _s; }", cg.tryErrVar))
			} else {
				cg.output.WriteString("fprintf(stderr, \"Merlin runtime: slice bounds out of range\\n\"); exit(1); }")
			}
			cg.output.WriteString(" memcpy(_d, _s.data + _l, _n); _d[_n] = 0; (MerlinString){_d, _n, _n + 1}; })")
		} else {
			elemType := collType
			if idx := strings.Index(collType, "["); idx != -1 {
				elemType = collType[:idx]
			}
			elemType = mangleTemplateName(cg.resolvePrimitiveType(elemType))
			cg.output.WriteString("({ int _l = ")
			if node.Low != nil {
				cg.genNode(node.Low, prog)
			} else {
				cg.output.WriteString("0")
			}
			cg.output.WriteString("; int _h = ")
			if node.High != nil {
				cg.genNode(node.High, prog)
			} else {
				bound := "0"
				if id, ok := node.Collection.(*Identifier); ok {
					if t, ok := cg.varTypes[id.Value]; ok {
						if start := strings.Index(t, "["); start != -1 {
							end := strings.Index(t, "]")
							if end > start+1 {
								bound = t[start+1 : end]
							}
						}
					}
				}
				cg.output.WriteString(bound)
			}
			cg.output.WriteString(fmt.Sprintf("; int _n = _h - _l; %s _r[_n]; memcpy(_r, (", elemType))
			cg.genNode(node.Collection, prog)
			cg.output.WriteString(") + _l, _n * sizeof(")
			cg.output.WriteString(elemType)
			cg.output.WriteString(")); _r; })")
		}

	case *CallExpr:
		// Template method call or module template call: obj.method<Type>(args)
		if inst, ok := node.Function.(*TemplateInstantiation); ok && inst.Object != nil {
			// Check if it's a module template call (no & prefix)
			if objIdent, ok := inst.Object.(*Identifier); ok && cg.isModule(objIdent.Value, prog) {
				funcName := mangleTemplateName(inst.ResolvedName)
				cg.output.WriteString(funcName + "(")
				cg.emitCallArgsWithLenInjection(inst.ResolvedName, node.Args, 0, prog)
				cg.output.WriteString(")")
				return
			}
			// Template method call: obj.method<Type>(args)
			funcName := mangleTemplateName(inst.ResolvedName)
			if cg.throwsFuncs[funcName] {
				outType := cg.getOutCType(funcName)
				if outType == "void" {
					if cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("({ if ((%s = %s(", cg.tryErrVar, funcName))
						if !cg.isPointerExpr(inst.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(inst.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(fmt.Sprintf("))) goto %s; (void)0; })", cg.tryEndLabel))
					} else {
						cg.output.WriteString(fmt.Sprintf("({ %s(", funcName))
						if !cg.isPointerExpr(inst.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(inst.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString("); (void)0; })")
					}
				} else {
					outNum := cg.tryOutCounter
					cg.tryOutCounter++
					outVar := fmt.Sprintf("_out_%d", outNum)
					if cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("({ %s %s; if ((%s = %s(&%s, ", outType, outVar, cg.tryErrVar, funcName, outVar))
						if !cg.isPointerExpr(inst.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(inst.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(fmt.Sprintf("))) goto %s; %s; })", cg.tryEndLabel, outVar))
					} else {
						cg.output.WriteString(fmt.Sprintf("({ %s %s; %s(&%s, ", outType, outVar, funcName, outVar))
						if !cg.isPointerExpr(inst.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(inst.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(fmt.Sprintf("); %s; })", outVar))
					}
				}
			} else {
				cg.output.WriteString(funcName + "(")
				if !cg.isPointerExpr(inst.Object) {
					cg.output.WriteString("&")
				}
				cg.genNode(inst.Object, prog)
				if len(node.Args) > 0 {
					cg.output.WriteString(", ")
				}
				cg.emitCallArgsWithLenInjection(inst.ResolvedName, node.Args, 1, prog)
				cg.output.WriteString(")")
			}
			return
		}
		if sel, ok := node.Function.(*SelectorExpr); ok {
			if ident, ok := sel.Object.(*Identifier); ok {
				// Module function call: module.func() -> func()
				if cg.isModule(ident.Value, prog) {
					cg.output.WriteString(sel.Field)
					cg.output.WriteString("(")
					for i, arg := range node.Args {
						if i > 0 {
							cg.output.WriteString(", ")
						}
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(")")
					return
				}
				
				// Struct method call: instance.method() -> Struct_method(&instance)
				structName := cg.getStructNameForMethod(sel.Field, sel.Object, prog)
				funcName := fmt.Sprintf("%s_%s", mangleTemplateName(structName), sel.Field)
			if cg.throwsFuncs[funcName] {
				outType := cg.getOutCType(funcName)
				if outType == "void" {
					if cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("({ if ((%s = %s(", cg.tryErrVar, funcName))
						if !cg.isPointerExpr(sel.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(sel.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(fmt.Sprintf("))) goto %s; (void)0; })", cg.tryEndLabel))
					} else {
						cg.output.WriteString(fmt.Sprintf("({ %s(", funcName))
						if !cg.isPointerExpr(sel.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(sel.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString("); (void)0; })")
					}
				} else {
					outNum := cg.tryOutCounter
					cg.tryOutCounter++
					outVar := fmt.Sprintf("_out_%d", outNum)
					if cg.tryErrVar != "" {
						cg.output.WriteString(fmt.Sprintf("({ %s %s; if ((%s = %s(&%s, ", outType, outVar, cg.tryErrVar, funcName, outVar))
						if !cg.isPointerExpr(sel.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(sel.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(fmt.Sprintf("))) goto %s; %s; })", cg.tryEndLabel, outVar))
					} else {
						cg.output.WriteString(fmt.Sprintf("({ %s %s; %s(&%s, ", outType, outVar, funcName, outVar))
						if !cg.isPointerExpr(sel.Object) {
							cg.output.WriteString("&")
						}
						cg.genNode(sel.Object, prog)
						for _, arg := range node.Args {
							cg.output.WriteString(", ")
							cg.genNode(arg, prog)
						}
						cg.output.WriteString(fmt.Sprintf("); %s; })", outVar))
					}
				}
				} else {
					cg.output.WriteString(funcName)
					cg.output.WriteString("(")
					if !cg.isPointerExpr(sel.Object) {
						cg.output.WriteString("&")
					}
					cg.genNode(sel.Object, prog)
					if len(node.Args) > 0 {
						cg.output.WriteString(", ")
					}
					for i, arg := range node.Args {
						if i > 0 {
							cg.output.WriteString(", ")
						}
						cg.genNode(arg, prog)
					}
					cg.output.WriteString(")")
				}
				return
			}
		}
		if ident, ok := node.Function.(*Identifier); ok {
			if ident.Value == "len" {
				if len(node.Args) == 0 { cg.output.WriteString("0"); return }
				argType := cg.getExprType(node.Args[0])
				if _, ok := node.Args[0].(*StringLiteral); ok {
					cg.output.WriteString(fmt.Sprintf("%d", len(node.Args[0].(*StringLiteral).Value)))
				} else if argType == "string" {
					cg.genNode(node.Args[0], prog)
					cg.output.WriteString(".length")
				} else if strings.Contains(argType, "[") && !strings.HasPrefix(argType, "string") {
				// Array: extract size from type string e.g. "int[3]" -> 3
				start := strings.Index(argType, "[")
				end := strings.Index(argType, "]")
				if start != -1 && end != -1 && end > start+1 {
					cg.output.WriteString(argType[start+1 : end])
				} else if strings.HasSuffix(argType, "[]") {
					// T[] parameter: use hidden length variable
					if id, ok := node.Args[0].(*Identifier); ok {
						if lenVar, ok := cg.arrayLenVars[id.Value]; ok {
							cg.output.WriteString(lenVar)
						} else {
							cg.output.WriteString("0")
						}
					} else {
						cg.output.WriteString("0")
					}
				} else {
					cg.output.WriteString("0")
				}
				} else {
					cg.genNode(node.Args[0], prog)
				}
				return
			}
			if ident.Value == "sizeof" {
				argType := cg.getExprType(node.Args[0])
				if argType == "string" {
					cg.genNode(node.Args[0], prog)
					cg.output.WriteString(".capacity")
				} else if idx := strings.Index(argType, "["); idx != -1 {
					// Array: sizeof(array) gives total byte size which is element_size * count
					cg.output.WriteString("sizeof(")
					cg.genNode(node.Args[0], prog)
					cg.output.WriteString(")")
				} else {
					// For primitives, resolve to C type; for structs, emit name directly
					cType := argType
					if cg.isPrimitiveTypeName(argType) {
						cType = cg.resolvePrimitiveType(argType)
					}
					cg.output.WriteString("sizeof(")
					cg.output.WriteString(cType)
					cg.output.WriteString(")")
				}
				return
			}
			if ident.Value == "typeof" {
				typeName := node.TypeResult
				if typeName == "" {
					typeName = "unknown"
				}
				cg.output.WriteString("{.data = \"")
				cg.output.WriteString(typeName)
				cg.output.WriteString("\", .length = ")
				cg.output.WriteString(fmt.Sprintf("%d", len(typeName)))
				cg.output.WriteString(", .capacity = 0}")
				return
			}
		}
		// For template instantiation calls, inject length args for T[] params
		if inst, ok := node.Function.(*TemplateInstantiation); ok {
			fullName := inst.Name + "<" + strings.Join(inst.TypeArgs, ", ") + ">"
			cg.output.WriteString(mangleTemplateName(inst.ResolvedName))
			cg.output.WriteString("(")
			cg.emitCallArgsWithLenInjection(fullName, node.Args, 0, prog)
			cg.output.WriteString(")")
		} else {
			cg.genNode(node.Function, prog)
			cg.output.WriteString("(")
			for i, arg := range node.Args {
				cg.genNode(arg, prog)
				if i < len(node.Args)-1 {
					cg.output.WriteString(", ")
				}
			}
			cg.output.WriteString(")")
		}


	case *ArrayLiteral:
		cg.output.WriteString("{")
		for i, elem := range node.Elements {
			if i > 0 {
				cg.output.WriteString(", ")
			}
			cg.genNode(elem, prog)
		}
		cg.output.WriteString("}")
		case *SelectorExpr:
		if node.IsEnumValue {
			cg.output.WriteString(node.Field)
			return
		}
		if ident, ok := node.Object.(*Identifier); ok {
			// Module variable reference: math.PI -> PI
			if cg.isModule(ident.Value, prog) {
				cg.output.WriteString(node.Field)
				return
			}
			// Self reference in methods
			if ident.Value == "self" {
				cg.output.WriteString("self->")
				cg.output.WriteString(node.Field)
				return
			}
		}
		cg.genNode(node.Object, prog)
		if ident, ok := node.Object.(*Identifier); ok && cg.pointerVars[ident.Value] {
			cg.output.WriteString("->")
		} else {
			cg.output.WriteString(".")
		}
		cg.output.WriteString(node.Field)

	case *CastExpr:
		castType := node.TargetType
		if cg.currentSubst != nil {
			if s, ok := cg.currentSubst[castType]; ok {
				castType = s
			}
		}
		cg.output.WriteString("(")
		if node.IsVolatile {
			cg.output.WriteString("volatile ")
		}
		cg.output.WriteString(mangleTemplateName(cg.resolvePrimitiveType(castType)))
		if node.IsPointer {
			cg.output.WriteString("*")
		}
		cg.output.WriteString(")")
		cg.genNode(node.Operand, prog)

	case *ConvExpr:
		if node.TargetType == "int" {
			if id, ok := node.Operand.(*Identifier); ok {
				if t, ok := cg.varTypes[id.Value]; ok && t == "string" {
					cg.needsStdlib = true
					cg.output.WriteString(fmt.Sprintf("atoi(%s.data)", id.Value))
					break
				}
			}
		}
		if node.TargetType == "float" || node.TargetType == "float32" || node.TargetType == "float64" {
			if id, ok := node.Operand.(*Identifier); ok {
				if t, ok := cg.varTypes[id.Value]; ok && t == "string" {
					cg.needsStdlib = true
					cg.output.WriteString(fmt.Sprintf("atof(%s.data)", id.Value))
					break
				}
			}
		}
		if node.TargetType == "string" {
			switch node.Operand.(type) {
			case *CharLiteral:
				cg.output.WriteString("({ char _c = ")
				cg.genNode(node.Operand, prog)
				cg.output.WriteString("; ms_new_len(&_c, 1); })")
				break
			default:
				cg.output.WriteString("({ char _buf[64]; int _n = snprintf(_buf, 64, \"%lld\", (long long)")
				cg.genNode(node.Operand, prog)
				cg.output.WriteString("); ms_new_len(_buf, _n); })")
				break
			}
			break
		}
		cg.output.WriteString(fmt.Sprintf("(%s)", mangleTemplateName(cg.resolvePrimitiveType(node.TargetType))))
		cg.genNode(node.Operand, prog)

	case *AddressOf:
		cg.output.WriteString("&")
		cg.genNode(node.Operand, prog)

	case *Deref:
		cg.output.WriteString("*")
		cg.genNode(node.Operand, prog)

	case *StructLiteral:
		structTypeName := node.TypeName
		if cg.currentSubst != nil {
			for k, v := range cg.currentSubst {
				structTypeName = strings.ReplaceAll(structTypeName, k, v)
			}
		}
		cg.output.WriteString(fmt.Sprintf("(%s){", mangleTemplateName(structTypeName)))
		for i, f := range node.Fields {
			cg.output.WriteString(fmt.Sprintf(".%s = ", f.Name))
			cg.genNode(f.Value, prog)
			if i < len(node.Fields)-1 {
				cg.output.WriteString(", ")
			}
		}
		cg.output.WriteString("}")
	}
}

func (cg *CodeGen) getCallFuncName(call *CallExpr) string {
	switch fn := call.Function.(type) {
	case *Identifier:
		return fn.Value
	case *SelectorExpr:
		return ""
	case *TemplateInstantiation:
		if fn.ResolvedName != "" {
			return mangleTemplateName(fn.ResolvedName)
		}
		fullName := fn.Name + "<" + strings.Join(fn.TypeArgs, ", ") + ">"
		return mangleTemplateName(fullName)
	}
	return ""
}

func (cg *CodeGen) isModule(name string, prog *Program) bool {
	return cg.importedModules[name]
}

func (cg *CodeGen) pushIndent() {
	cg.indent++
}

func (cg *CodeGen) popIndent() {
	cg.indent--
}

func (cg *CodeGen) emitDeferStack() {
	for i := len(cg.deferStack) - 1; i >= 0; i-- {
		cg.output.WriteString(cg.deferStack[i])
	}
	cg.deferStack = nil
}

func (cg *CodeGen) currentFuncIsMethod() bool {
	return true
}

func (cg *CodeGen) getStructNameForMethod(methodName string, receiver Node, prog *Program) string {
	// If receiver is an identifier, look up its declared type to find the exact struct
	if ident, ok := receiver.(*Identifier); ok {
		if typeName, ok := cg.varTypes[ident.Value]; ok {
			for _, d := range cg.concreteDecls {
				if s, ok := d.(*StructDecl); ok && s.Name == typeName {
					for _, m := range s.Methods {
						if m.Name == methodName {
							return s.Name
						}
					}
				}
			}
			for _, stmt := range prog.Statements {
				if s, ok := stmt.(*StructDecl); ok && s.Name == typeName {
					for _, m := range s.Methods {
						if m.Name == methodName {
							return s.Name
						}
					}
				}
			}
		}
	}
	// Fallback: look up method owner via reverse map
	owner := ""
	ambiguous := false
	searchStruct := func(s *StructDecl) {
		for _, m := range s.Methods {
			if m.Name == methodName {
				if owner == "" {
					owner = s.Name
				} else if owner != s.Name {
					ambiguous = true
				}
			}
		}
	}
	for _, d := range cg.concreteDecls {
		if s, ok := d.(*StructDecl); ok {
			searchStruct(s)
		}
	}
	for _, stmt := range prog.Statements {
		if s, ok := stmt.(*StructDecl); ok {
			searchStruct(s)
		}
		if tmpl, ok := stmt.(*TemplateDecl); ok {
			if s, ok := tmpl.Declaration.(*StructDecl); ok {
				searchStruct(s)
			}
		}
	}
	// Also search imported programs' struct declarations
	for _, p := range cg.allProgs {
		if p == prog { continue }
		for _, stmt := range p.Statements {
			if s, ok := stmt.(*StructDecl); ok {
				searchStruct(s)
			}
			if tmpl, ok := stmt.(*TemplateDecl); ok {
				if s, ok := tmpl.Declaration.(*StructDecl); ok {
					searchStruct(s)
				}
			}
		}
	}
	if !ambiguous {
		return owner
	}
	return ""
}

// emitCallArgsWithLenInjection emits function arguments with automatic
// length injection for T[] parameters at the call site.
// funcFullName is the concrete function name (e.g. "first<int>") to look up funcTArrayPositions.
// paramOffset is the number of leading params (e.g., self for methods) before the emitted args.
func (cg *CodeGen) emitCallArgsWithLenInjection(funcFullName string, args []Node, paramOffset int, prog *Program) {
	positions := cg.funcTArrayPositions[funcFullName]
	for i, arg := range args {
		if i > 0 {
			cg.output.WriteString(", ")
		}
		// Capture arg text once to avoid regenerating for sizeof.
	// sizeof is compile-time in C so runtime evaluation only happens once,
	// but repeating complex arg expressions in the C source is cleaner this way.
	tmp := &strings.Builder{}
	saved := cg.output
	cg.output = tmp
	cg.genNode(arg, prog)
	cg.output = saved
	argText := tmp.String()
	cg.output.WriteString(argText)
	absIdx := paramOffset + i
	for _, pos := range positions {
		if pos == absIdx {
			cg.output.WriteString(", sizeof(")
			cg.genNode(arg, prog)
			cg.output.WriteString(")/sizeof((")
			cg.genNode(arg, prog)
			cg.output.WriteString(")[0])")
			break
		}
	}
	}
}


