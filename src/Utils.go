package main

// parseInt converts an integer literal string (decimal or 0x hex) to int64.
func parseInt(s string) int64 {
	var val int64
	neg := false
	i := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		i = 1
	}
	if len(s) > i+1 && s[i] == '0' && s[i+1] == 'x' {
		i += 2
		for ; i < len(s); i++ {
			val = val*16 + int64(hexVal(s[i]))
		}
	} else {
		for ; i < len(s); i++ {
			val = val*10 + int64(s[i]-'0')
		}
	}
	if neg {
		val = -val
	}
	return val
}

// parseFloat converts a float literal string to float64.
func parseFloat(s string) float64 {
	var intPart, fracPart float64
	neg := false
	i := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		i = 1
	}
	for i < len(s) && s[i] != '.' {
		intPart = intPart*10 + float64(s[i]-'0')
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		factor := 0.1
		for i < len(s) {
			fracPart += float64(s[i]-'0') * factor
			factor *= 0.1
			i++
		}
	}
	val := intPart + fracPart
	if neg {
		val = -val
	}
	return val
}

func cloneNode(n Node) Node {
	if n == nil {
		return nil
	}
	switch node := n.(type) {
	case *IntLiteral:
		c := *node
		return &c
	case *FloatLiteral:
		c := *node
		return &c
	case *StringLiteral:
		c := *node
		return &c
	case *CharLiteral:
		c := *node
		return &c
	case *BoolLiteral:
		c := *node
		return &c
	case *Identifier:
		c := *node
		return &c
	case *UnaryExpr:
		return &UnaryExpr{Tok: node.Tok, Op: node.Op, Operand: cloneNode(node.Operand)}
	case *BinaryExpr:
		return &BinaryExpr{Tok: node.Tok, Left: cloneNode(node.Left), Op: node.Op, Right: cloneNode(node.Right)}
	case *CastExpr:
		return &CastExpr{Tok: node.Tok, TargetType: node.TargetType, IsPointer: node.IsPointer, IsVolatile: node.IsVolatile, Operand: cloneNode(node.Operand)}
	case *ConvExpr:
		return &ConvExpr{Tok: node.Tok, TargetType: node.TargetType, Operand: cloneNode(node.Operand)}
	default:
		return n
	}
}
