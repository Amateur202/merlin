package main

import (
	"fmt"
	"strings"
)

func mangleTemplateName(name string) string {
	r := strings.NewReplacer("<", "_", ">", "", ", ", "_", ",", "_", ".", "_")
	if strings.Contains(name, "<") {
		return r.Replace(name)
	}
	return name
}

func charToCLiteral(r rune) string {
	switch r {
	case '\n':
		return "'\\n'"
	case '\t':
		return "'\\t'"
	case '\r':
		return "'\\r'"
	case '\\':
		return "'\\\\'"
	case '\x27':
		return "'\\''"
	case 0:
		return "'\\0'"
	default:
		if r >= 32 && r <= 126 {
			return fmt.Sprintf("'%c'", r)
		}
		return fmt.Sprintf("'\\x%02x'", r)
	}
}

func cStringLiteral(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, b := range []byte(s) {
		switch b {
		case '\n':
			sb.WriteString("\\n")
		case '\t':
			sb.WriteString("\\t")
		case '\r':
			sb.WriteString("\\r")
		case '\\':
			sb.WriteString("\\\\")
		case '"':
			sb.WriteString("\\\"")
		case 0:
			sb.WriteString("\\0")
		default:
			if b >= 32 && b <= 126 {
				sb.WriteByte(b)
			} else {
				sb.WriteString(fmt.Sprintf("\\x%02x", b))
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func isConstantInit(n Node) bool {
	if n == nil {
		return true
	}
	switch n.(type) {
	case *IntLiteral, *FloatLiteral, *BoolLiteral, *CharLiteral:
		return true
	case *StringLiteral:
		return false
	case *UnaryExpr:
		return isConstantInit(n.(*UnaryExpr).Operand)
	case *BinaryExpr:
		return isConstantInit(n.(*BinaryExpr).Left) && isConstantInit(n.(*BinaryExpr).Right)
	case *ArrayLiteral:
		for _, elem := range n.(*ArrayLiteral).Elements {
			if !isConstantInit(elem) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
