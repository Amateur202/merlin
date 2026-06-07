package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// printAST marshals the program to indented JSON and writes it to stdout.
func printAST(prog *Program) {
	tree := buildTree(prog)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(tree); err != nil {
		fatal("AST serialization error: %v", err)
	}
}

// buildTree converts the AST into a generic map for JSON output.
func buildTree(n Node) map[string]any {
	if n == nil {
		return nil
	}
	m := map[string]any{"_type": n.NodeType()}

	switch v := n.(type) {
	case *Program:
		imports := make([]any, len(v.Imports))
		for i, imp := range v.Imports {
			imports[i] = buildTree(imp)
		}
		stmts := make([]any, len(v.Statements))
		for i, s := range v.Statements {
			stmts[i] = buildTree(s)
		}
		m["imports"] = imports
		m["statements"] = stmts

	case *ImportDecl:
		m["path"] = v.Path

	case *TypeAliasDecl:
		m["name"] = v.Name
		m["base_type"] = v.BaseType

	case *VarDecl:
		m["name"] = v.Name
		m["type"] = v.TypeAnnot
		m["pointer"] = v.IsPointer
		m["volatile"] = v.IsVolatile
		if v.IsConst {
			m["const"] = true
		}
		if v.ArraySize != "" {
			m["array_size"] = v.ArraySize
		}
		if v.Value != nil {
			m["value"] = buildTree(v.Value)
		}

	case *FuncDecl:
		params := make([]any, len(v.Params))
		for i, p := range v.Params {
			params[i] = map[string]any{
				"name":     p.Name,
				"type":     p.TypeAnnot,
				"pointer":  p.IsPointer,
				"volatile": p.IsVolatile,
			}
		}
		m["return_type"] = v.ReturnType
		m["return_types"] = v.ReturnTypes
		m["return_pointer"] = v.RetPointer
		m["name"] = v.Name
		m["params"] = params
		m["is_method"] = v.IsMethod
		m["throws"] = v.Throws
		m["body"] = buildTree(v.Body)

	case *StructDecl:
		fields := make([]any, len(v.Fields))
		for i, f := range v.Fields {
			fields[i] = map[string]any{
				"name":    f.Name,
				"type":    f.TypeAnnot,
				"pointer": f.IsPointer,
			}
		}
		methods := make([]any, len(v.Methods))
		for i, meth := range v.Methods {
			methods[i] = buildTree(meth)
		}
		m["name"] = v.Name
		m["fields"] = fields
		m["methods"] = methods

	case *TemplateDecl:
		typeParams := make([]string, len(v.TypeParams))
		for i, tp := range v.TypeParams {
			typeParams[i] = tp.Name
		}
		m["type_params"] = typeParams
		m["declaration"] = buildTree(v.Declaration)

	case *TemplateInstantiation:
		m["name"] = v.Name
		m["type_args"] = v.TypeArgs
		if v.Object != nil {
			m["object"] = buildTree(v.Object)
		}

	case *BlockStmt:
		stmts := make([]any, len(v.Statements))
		for i, s := range v.Statements {
			stmts[i] = buildTree(s)
		}
		m["statements"] = stmts

	case *IfStmt:
		m["condition"] = buildTree(v.Condition)
		m["consequence"] = buildTree(v.Consequence)
		if len(v.Alternatives) > 0 {
			alts := make([]any, len(v.Alternatives))
			for i, a := range v.Alternatives {
				alts[i] = map[string]any{
					"condition": buildTree(a.Condition),
					"body":      buildTree(a.Body),
				}
			}
			m["alternatives"] = alts
		}
		if v.Else != nil {
			m["else"] = buildTree(v.Else)
		}

	case *ForRangeStmt:
		m["var"] = v.Var
		m["from"] = buildTree(v.From)
		m["to"] = buildTree(v.To)
		m["body"] = buildTree(v.Body)

	case *ForCondStmt:
		m["condition"] = buildTree(v.Condition)
		m["body"] = buildTree(v.Body)

	case *ForInfiniteStmt:
		m["body"] = buildTree(v.Body)

	case *MatchStmt:
		m["subject"] = buildTree(v.Subject)
		cases := make([]any, len(v.Cases))
		for i, c := range v.Cases {
			cm := map[string]any{
				"catch_all":    c.IsCatchAll,
				"fall_through": c.FallThrough,
				"body":         buildTree(c.Body),
			}
			if !c.IsCatchAll {
				cm["value"] = buildTree(c.Value)
			}
			cases[i] = cm
		}
		m["cases"] = cases

	case *RaiseStmt:
		m["value"] = buildTree(v.Value)

	case *DeferStmt:
		m["call"] = buildTree(v.Call)

	case *TryCatchStmt:
		m["body"] = buildTree(v.Body)
		catches := make([]any, len(v.Catches))
		for i, cc := range v.Catches {
			cm := map[string]any{
				"catch_all": cc.IsCatchAll,
				"body":      buildTree(cc.Body),
			}
			if !cc.IsCatchAll {
				cm["value"] = buildTree(cc.Value)
			}
			catches[i] = cm
		}
		m["catches"] = catches

	case *ReturnStmt:
		if len(v.Values) > 0 {
			vals := make([]any, len(v.Values))
			for i, val := range v.Values {
				vals[i] = buildTree(val)
			}
			m["values"] = vals
		}

	case *AssignStmt:
		m["target"] = buildTree(v.Target)
		m["value"] = buildTree(v.Value)

	case *MultiAssignStmt:
		targets := make([]any, len(v.Targets))
		for i, t := range v.Targets {
			targets[i] = buildTree(t)
		}
		vals := make([]any, len(v.Values))
		for i, val := range v.Values {
			vals[i] = buildTree(val)
		}
		m["targets"] = targets
		m["values"] = vals

	case *MultiShortVarDecl:
		m["names"] = v.Names
		vals := make([]any, len(v.Values))
		for i, val := range v.Values {
			vals[i] = buildTree(val)
		}
		m["values"] = vals

	case *CompoundAssignStmt:
		m["target"] = buildTree(v.Target)
		m["op"] = v.Op
		m["value"] = buildTree(v.Value)

	case *ExprStmt:
		m["expression"] = buildTree(v.Expression)

	case *AsmBlock:
		m["lines"] = v.Lines

	case *Identifier:
		m["value"] = v.Value

	case *IntLiteral:
		m["value"] = v.Value

	case *FloatLiteral:
		m["value"] = v.Value

	case *StringLiteral:
		m["value"] = v.Value

	case *CharLiteral:
		m["value"] = string(v.Value)

	case *BoolLiteral:
		m["value"] = v.Value

	case *ArrayLiteral:
		elems := make([]any, len(v.Elements))
		for i, e := range v.Elements {
			elems[i] = buildTree(e)
		}
		m["elements"] = elems

	case *BreakStmt:
	case *ContinueStmt:
	}

	return m
}

var _ = fmt.Sprintf
