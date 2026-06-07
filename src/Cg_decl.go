package main

import (
	"fmt"
	"strings"
)

func (cg *CodeGen) emitExternalFuncs(prog *Program) {
	for _, stmt := range prog.Statements {
		if ext, ok := stmt.(*ExternalFuncDecl); ok {
			retType := cg.resolvePrimitiveType(ext.ReturnType)
			if ext.RetPointer {
				retType += "*"
			}

			params := []string{}
			for _, p := range ext.Params {
				pType := cg.resolvePrimitiveType(p.TypeAnnot)
				if p.IsPointer {
					pType += "*"
				}
				if p.IsVolatile {
					pType = "volatile " + pType
				}
				params = append(params, fmt.Sprintf("%s %s", pType, p.Name))
			}

			cg.output.WriteString(fmt.Sprintf("extern %s %s(%s);\n", retType, ext.Name, strings.Join(params, ", ")))
		}
	}
	if prog.Statements != nil {
		cg.output.WriteString("\n")
	}
}

func (cg *CodeGen) genFunc(n *FuncDecl, structName string, prog *Program) {
	funcName := mangleTemplateName(n.Name)
	sigStructName := structName
	if structName != "" {
		funcName = fmt.Sprintf("%s_%s", mangleTemplateName(structName), n.Name)
		sigStructName = mangleTemplateName(structName)
	}

	if n.Throws {
		cg.throwsFuncs[funcName] = true
	}

	isMultiReturn := len(n.ReturnTypes) > 1

	var retTypes []string
	if isMultiReturn {
		for _, rt := range n.ReturnTypes {
			t := cg.resolvePrimitiveType(mangleTemplateName(rt))
			if rt == "string" {
				t = "MerlinString"
			}
			retTypes = append(retTypes, t)
		}
	}
	retType := cg.resolvePrimitiveType(mangleTemplateName(n.ReturnType))
	if n.RetPointer {
		retType += "*"
	}

	if n.Throws {
		cg.output.WriteString(fmt.Sprintf("int %s(", funcName))
		if isMultiReturn {
			for i, rt := range retTypes {
				if i > 0 {
					cg.output.WriteString(", ")
				}
				cg.output.WriteString(fmt.Sprintf("%s* _out_%d", rt, i))
			}
		} else if retType != "void" {
			cg.output.WriteString(fmt.Sprintf("%s* _out", retType))
		}
		hasMore := ((isMultiReturn || retType != "void")) && (sigStructName != "" || len(n.Params) > 0)
		if hasMore {
			cg.output.WriteString(", ")
		}
	} else if isMultiReturn {
		cg.output.WriteString(fmt.Sprintf("void %s(", funcName))
		for i, rt := range retTypes {
			if i > 0 {
				cg.output.WriteString(", ")
			}
			cg.output.WriteString(fmt.Sprintf("%s* _out_%d", rt, i))
		}
		if sigStructName != "" || len(n.Params) > 0 {
			cg.output.WriteString(", ")
		}
	} else {
		cg.output.WriteString(fmt.Sprintf("%s %s(", retType, funcName))
	}

	if sigStructName != "" {
		cg.output.WriteString(fmt.Sprintf("%s* self", sigStructName))
		cg.varTypes["self"] = structName
		cg.pointerVars["self"] = true
		if len(n.Params) > 0 {
			cg.output.WriteString(", ")
		}
	}
	for i, p := range n.Params {
		cg.varTypes[p.Name] = p.TypeAnnot
		isArrayType := strings.HasSuffix(p.TypeAnnot, "[]")
		baseTypeAnnot := p.TypeAnnot
		if isArrayType {
			baseTypeAnnot = p.TypeAnnot[:len(p.TypeAnnot)-2]
		}
		pType := cg.resolvePrimitiveType(mangleTemplateName(baseTypeAnnot))
		if p.IsPointer {
			pType += "*"
		}
		if p.IsVolatile {
			pType = "volatile " + pType
		}
		if isArrayType {
			cg.output.WriteString(fmt.Sprintf("%s %s[]", pType, p.Name))
			lenVar := p.Name + "_len"
			cg.arrayLenVars[p.Name] = lenVar
			cg.output.WriteString(fmt.Sprintf(", intptr_t %s", lenVar))
		} else {
			cg.output.WriteString(fmt.Sprintf("%s %s", pType, p.Name))
		}
		if i < len(n.Params)-1 {
			cg.output.WriteString(", ")
		}
	}
	cg.output.WriteString(") ")

	oldSubst := cg.currentSubst
	cg.currentSubst = nil
	if structName != "" {
		if s, ok := cg.templateSubst[mangleTemplateName(structName)]; ok {
			cg.currentSubst = s
		}
	}
	if cg.currentSubst == nil {
		if s, ok := cg.templateSubst[funcName]; ok {
			cg.currentSubst = s
		}
	}

	oldThrows := cg.inThrowsFunc
	oldFuncName := cg.currentFuncName
	cg.inThrowsFunc = n.Throws
	cg.currentFuncName = funcName
	cg.output.WriteString("{\n")
	cg.pushIndent()
	if n.Body != nil {
		for _, s := range n.Body.Statements {
			cg.output.WriteString(strings.Repeat("\t", cg.indent))
			cg.genNode(s, prog)
		}
	}
	if n.Throws {
		cg.output.WriteString(strings.Repeat("\t", cg.indent) + "return 0;\n")
	}
	cg.popIndent()
	cg.output.WriteString(strings.Repeat("\t", cg.indent) + "}")
	cg.inThrowsFunc = oldThrows
	cg.currentFuncName = oldFuncName
	cg.currentSubst = oldSubst
	cg.output.WriteString("\n\n")
}
