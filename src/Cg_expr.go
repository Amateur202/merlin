package main

import (
	"fmt"
	"strings"
)

func (cg *CodeGen) genPrint(call *CallExpr, prog *Program) {
	cg.needsStdio = true
	for i, arg := range call.Args {
		if i > 0 {
			cg.output.WriteString("printf(\" \"); ")
		}
		switch arg.(type) {
		case *StringLiteral:
			cg.output.WriteString("({ MerlinString _s = ")
			cg.genNode(arg, prog)
			cg.output.WriteString("; printf(\"%.*s\", _s.length, _s.data); })")
		case *BoolLiteral:
			cg.output.WriteString("printf(")
			cg.genNode(arg, prog)
			cg.output.WriteString(" ? \"true\" : \"false\")")
		case *CharLiteral:
			cg.output.WriteString("putchar(")
			cg.genNode(arg, prog)
			cg.output.WriteString(")")
		case *FloatLiteral:
			cg.output.WriteString("printf(\"%f\", ")
			cg.genNode(arg, prog)
			cg.output.WriteString(")")
		default:
			argType := cg.getExprType(arg)
			if strings.HasPrefix(argType, "string") {
				cg.output.WriteString("({ MerlinString _s = ")
				cg.genNode(arg, prog)
				cg.output.WriteString("; printf(\"%.*s\", _s.length, _s.data); })")
			} else if argType == "char" {
				cg.output.WriteString("putchar(")
				cg.genNode(arg, prog)
				cg.output.WriteString(")")
			} else if argType == "bool" {
				cg.output.WriteString("printf(")
				cg.genNode(arg, prog)
				cg.output.WriteString(" ? \"true\" : \"false\")")
			} else if argType == "float" || argType == "float32" || argType == "float64" {
				cg.output.WriteString("printf(\"%f\", ")
				cg.genNode(arg, prog)
				cg.output.WriteString(")")
			} else if strings.HasPrefix(argType, "u") {
				cg.output.WriteString("printf(\"%llu\", (unsigned long long)(")
				cg.genNode(arg, prog)
				cg.output.WriteString("))")
			} else if strings.Contains(argType, "[") {
				start := strings.Index(argType, "[")
				size := argType[start+1 : len(argType)-1]
				fmt.Fprintf(cg.output, "printf(\"[\");\n")
				fmt.Fprintf(cg.output, "for (int _i = 0; _i < %s; _i++) {\n", size)
				cg.output.WriteString("if (_i > 0) printf(\", \");\n")
				cg.output.WriteString("printf(\"%lld\", (long long)(")
				cg.genNode(arg, prog)
				cg.output.WriteString("[_i]))")
				cg.output.WriteString(";\n}\n")
				cg.output.WriteString("printf(\"]\")")
			} else {
				cg.output.WriteString("printf(\"%lld\", (long long)(")
				cg.genNode(arg, prog)
				cg.output.WriteString("))")
			}
		}
		cg.output.WriteString(";\n")
	}
}

func (cg *CodeGen) emitPrintString(expr Node, prog *Program) {
	cg.output.WriteString("({ MerlinString _s = ")
	cg.genNode(expr, prog)
	cg.output.WriteString("; printf(\"%.*s\", _s.length, _s.data); })")
}

func (cg *CodeGen) genInput(call *CallExpr, prog *Program) {
	cg.needsStdio = true
	bufNum := cg.inputCounter
	cg.inputCounter++
	bufName := fmt.Sprintf("_input_buf_%d", bufNum)
	lenName := fmt.Sprintf("_input_len_%d", bufNum)
	cg.output.WriteString(fmt.Sprintf("char %s[4096];\n", bufName))
	if len(call.Args) == 1 {
		cg.emitPrintString(call.Args[0], prog)
		cg.output.WriteString(";\n")
	}
	cg.output.WriteString(fmt.Sprintf("fgets(%s, 4096, stdin);\n", bufName))
	cg.output.WriteString(fmt.Sprintf("int %s = strlen(%s);\n", lenName, bufName))
	cg.output.WriteString(fmt.Sprintf("if (%s > 0 && %s[%s-1] == '\\n') { %s[--%s] = '\\0'; }\n",
		lenName, bufName, lenName, bufName, lenName))
	cg.output.WriteString(fmt.Sprintf("char* _input_data_%d = malloc(%s + 1);\n", bufNum, lenName))
	cg.output.WriteString(fmt.Sprintf("memcpy(_input_data_%d, %s, %s + 1);\n", bufNum, bufName, lenName))
}
