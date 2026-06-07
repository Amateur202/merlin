package main

import "fmt"

// Node is the base interface for every AST element.
type Node interface {
	TokenLiteral() string
	NodeType() string
}

// ===========================================================================
// Top-Level Program
// ===========================================================================

type Program struct {
	Imports    []*ImportDecl
	Statements []Node
}

func (p *Program) TokenLiteral() string { return "program" }
func (p *Program) NodeType() string     { return "Program" }

// ===========================================================================
// Declaration Nodes
// ===========================================================================

type ImportDecl struct {
	Tok    Token
	Path   string
	Global bool
}

func (n *ImportDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *ImportDecl) NodeType() string     { return "ImportDecl" }

type TypeAliasDecl struct {
	Tok       Token
	Name      string
	BaseType  string
	IsPrivate bool
}

func (n *TypeAliasDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *TypeAliasDecl) NodeType() string     { return "TypeAliasDecl" }

type FieldDecl struct {
	Tok        Token
	Name       string
	TypeAnnot  string
	IsPointer  bool
	IsVolatile bool
	IsPrivate  bool
}

func (n *FieldDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *FieldDecl) NodeType() string     { return "FieldDecl" }

type Param struct {
	Tok        Token
	Name       string
	TypeAnnot  string
	IsPointer  bool
	IsVolatile bool
	Default    Node
}

func (n *Param) TokenLiteral() string { return n.Tok.Literal }
func (n *Param) NodeType() string     { return "Param" }

type FuncDecl struct {
	Tok         Token
	ReturnType  string
	ReturnTypes []string
	RetPointer  bool
	Name        string
	Params      []*Param
	Body        *BlockStmt
	IsMethod    bool
	IsPrivate   bool
	TypeParams  []*TypeParam
	Throws      bool
}

func (n *FuncDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *FuncDecl) NodeType() string     { return "FuncDecl" }

type ExternalFuncDecl struct {
	Tok        Token
	ReturnType string
	RetPointer bool
	Name       string
	Params     []*Param
	LinkLib    string
	IsPrivate  bool
}

func (n *ExternalFuncDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *ExternalFuncDecl) NodeType() string     { return "ExternalFuncDecl" }

type StructDecl struct {
	Tok       Token
	Name      string
	Fields    []*FieldDecl
	Methods   []*FuncDecl
	IsPrivate bool
}

func (n *StructDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *StructDecl) NodeType() string     { return "StructDecl" }

type EnumValueDecl struct {
	Tok       Token
	Name      string
	Value     Node
	IsPrivate bool
}

type EnumDecl struct {
	Tok       Token
	Name      string
	Values    []*EnumValueDecl
	IsPrivate bool
}

func (n *EnumDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *EnumDecl) NodeType() string     { return "EnumDecl" }

type VarDecl struct {
	Tok        Token
	Name       string
	TypeAnnot  string
	IsPointer  bool
	IsVolatile bool
	IsConst    bool
	IsPrivate  bool
	ArraySize  string
	Value      Node
}

func (n *VarDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *VarDecl) NodeType() string     { return "VarDecl" }

type ShortVarDecl struct {
	Tok       Token
	Name      string
	Value     Node
	TypeAnnot string
	IsConst   bool
}

func (n *ShortVarDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *ShortVarDecl) NodeType() string     { return "ShortVarDecl" }

// ===========================================================================
// Statement Nodes
// ===========================================================================

type BlockStmt struct {
	Tok        Token
	Statements []Node
}

func (n *BlockStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *BlockStmt) NodeType() string     { return "BlockStmt" }

type AssignStmt struct {
	Tok    Token
	Target Node
	Value  Node
}

func (n *AssignStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *AssignStmt) NodeType() string     { return "AssignStmt" }

type CompoundAssignStmt struct {
	Tok    Token
	Target Node
	Op     string
	Value  Node
}

func (n *CompoundAssignStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *CompoundAssignStmt) NodeType() string     { return "CompoundAssignStmt" }

type ReturnStmt struct {
	Tok    Token
	Values []Node
}

func (n *ReturnStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *ReturnStmt) NodeType() string     { return "ReturnStmt" }

type MultiAssignStmt struct {
	Tok     Token
	Targets []Node
	Values  []Node
}

func (n *MultiAssignStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *MultiAssignStmt) NodeType() string     { return "MultiAssignStmt" }

type MultiShortVarDecl struct {
	Tok    Token
	Names  []string
	Values []Node
}

func (n *MultiShortVarDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *MultiShortVarDecl) NodeType() string     { return "MultiShortVarDecl" }

type BreakStmt struct{ Tok Token }

func (n *BreakStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *BreakStmt) NodeType() string     { return "BreakStmt" }

type ContinueStmt struct{ Tok Token }

func (n *ContinueStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *ContinueStmt) NodeType() string     { return "ContinueStmt" }

type DeferStmt struct {
	Tok  Token
	Call Node
}

func (n *DeferStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *DeferStmt) NodeType() string     { return "DeferStmt" }

type PassStmt struct{ Tok Token }

func (n *PassStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *PassStmt) NodeType() string     { return "PassStmt" }

type ElifClause struct {
	Tok       Token
	Condition Node
	Body      *BlockStmt
}

type IfStmt struct {
	Tok          Token
	Condition    Node
	Consequence  *BlockStmt
	Alternatives []*ElifClause
	Else         *BlockStmt
}

func (n *IfStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *IfStmt) NodeType() string     { return "IfStmt" }

type ForRangeStmt struct {
	Tok  Token
	Var  string
	From Node
	To   Node
	Body *BlockStmt
}

func (n *ForRangeStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *ForRangeStmt) NodeType() string     { return "ForRangeStmt" }

type ForCondStmt struct {
	Tok       Token
	Condition Node
	Body      *BlockStmt
}

func (n *ForCondStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *ForCondStmt) NodeType() string     { return "ForCondStmt" }

type ForInfiniteStmt struct {
	Tok  Token
	Body *BlockStmt
}

func (n *ForInfiniteStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *ForInfiniteStmt) NodeType() string     { return "ForInfiniteStmt" }

type MatchCase struct {
	Tok         Token
	Value       Node
	IsCatchAll  bool
	FallThrough bool
	Body        *BlockStmt
}

type MatchStmt struct {
	Tok     Token
	Subject Node
	Cases   []*MatchCase
}

func (n *MatchStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *MatchStmt) NodeType() string     { return "MatchStmt" }

type AsmBlock struct {
	Tok   Token
	Lines []string
}

func (n *AsmBlock) TokenLiteral() string { return n.Tok.Literal }
func (n *AsmBlock) NodeType() string     { return "AsmBlock" }

type CBlock struct {
	Tok   Token
	Lines []string
}

func (n *CBlock) TokenLiteral() string { return n.Tok.Literal }
func (n *CBlock) NodeType() string     { return "CBlock" }

type ExprStmt struct {
	Tok        Token
	Expression Node
}

func (n *ExprStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *ExprStmt) NodeType() string     { return "ExprStmt" }

type RaiseStmt struct {
	Tok   Token
	Value Node
}

func (n *RaiseStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *RaiseStmt) NodeType() string     { return "RaiseStmt" }

type CatchClause struct {
	Tok        Token
	Value      Node
	IsCatchAll bool
	Body       *BlockStmt
}

type TryCatchStmt struct {
	Tok     Token
	Body    *BlockStmt
	Catches []*CatchClause
}

func (n *TryCatchStmt) TokenLiteral() string { return n.Tok.Literal }
func (n *TryCatchStmt) NodeType() string     { return "TryCatchStmt" }

// ===========================================================================
// Expression Nodes
// ===========================================================================

type Identifier struct {
	Tok   Token
	Value string
}

func (n *Identifier) TokenLiteral() string { return n.Tok.Literal }
func (n *Identifier) NodeType() string     { return "Identifier" }

type IntLiteral struct {
	Tok   Token
	Value int64
}

func (n *IntLiteral) TokenLiteral() string { return n.Tok.Literal }
func (n *IntLiteral) NodeType() string     { return "IntLiteral" }

type FloatLiteral struct {
	Tok   Token
	Value float64
}

func (n *FloatLiteral) TokenLiteral() string { return n.Tok.Literal }
func (n *FloatLiteral) NodeType() string     { return "FloatLiteral" }

type StringLiteral struct {
	Tok   Token
	Value string
}

func (n *StringLiteral) TokenLiteral() string { return n.Tok.Literal }
func (n *StringLiteral) NodeType() string     { return "StringLiteral" }

type CharLiteral struct {
	Tok   Token
	Value rune
}

func (n *CharLiteral) TokenLiteral() string { return n.Tok.Literal }
func (n *CharLiteral) NodeType() string     { return "CharLiteral" }

type BoolLiteral struct {
	Tok   Token
	Value bool
}

func (n *BoolLiteral) TokenLiteral() string { return n.Tok.Literal }
func (n *BoolLiteral) NodeType() string     { return "BoolLiteral" }

type BinaryExpr struct {
	Tok   Token
	Left  Node
	Op    string
	Right Node
}

func (n *BinaryExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *BinaryExpr) NodeType() string     { return "BinaryExpr" }

type UnaryExpr struct {
	Tok     Token
	Op      string
	Operand Node
}

func (n *UnaryExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *UnaryExpr) NodeType() string     { return "UnaryExpr" }

type IndexExpr struct {
	Tok        Token
	Collection Node
	Index      Node
}

func (n *IndexExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *IndexExpr) NodeType() string     { return "IndexExpr" }

type SliceExpr struct {
	Tok        Token
	Collection Node
	Low        Node
	High       Node
	SliceSize  int
}

func (n *SliceExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *SliceExpr) NodeType() string     { return "SliceExpr" }

type CallExpr struct {
	Tok        Token
	Function   Node
	Args       []Node
	TypeResult string
}

func (n *CallExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *CallExpr) NodeType() string     { return "CallExpr" }

type CastExpr struct {
	Tok        Token
	TargetType string
	IsPointer  bool
	IsVolatile bool
	Operand    Node
}

func (n *CastExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *CastExpr) NodeType() string     { return "CastExpr" }

type ConvExpr struct {
	Tok        Token
	TargetType string
	Operand    Node
}

func (n *ConvExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *ConvExpr) NodeType() string     { return "ConvExpr" }

type SelectorExpr struct {
	Tok         Token
	Object      Node
	Field       string
	IsEnumValue bool
}

func (n *SelectorExpr) TokenLiteral() string { return n.Tok.Literal }
func (n *SelectorExpr) NodeType() string     { return "SelectorExpr" }

type AddressOf struct {
	Tok     Token
	Operand Node
}

func (n *AddressOf) TokenLiteral() string { return n.Tok.Literal }
func (n *AddressOf) NodeType() string     { return "AddressOf" }

type Deref struct {
	Tok     Token
	Operand Node
}

func (n *Deref) TokenLiteral() string { return n.Tok.Literal }
func (n *Deref) NodeType() string     { return "Deref" }

type StructLiteral struct {
	Tok      Token
	TypeName string
	Fields   []*FieldInit
}

func (n *StructLiteral) TokenLiteral() string { return n.Tok.Literal }
func (n *StructLiteral) NodeType() string     { return "StructLiteral" }

type FieldInit struct {
	Tok   Token
	Name  string
	Value Node
}

func (n *FieldInit) TokenLiteral() string { return n.Tok.Literal }
func (n *FieldInit) NodeType() string     { return "FieldInit" }

type ArrayLiteral struct {
	Tok      Token
	Elements []Node
}

func (n *ArrayLiteral) TokenLiteral() string { return n.Tok.Literal }
func (n *ArrayLiteral) NodeType() string     { return "ArrayLiteral" }

// ===========================================================================
// Template Nodes
// ===========================================================================

type TypeParam struct {
	Tok  Token
	Name string
}

func (n *TypeParam) TokenLiteral() string { return n.Tok.Literal }
func (n *TypeParam) NodeType() string     { return "TypeParam" }

type TemplateDecl struct {
	Tok         Token
	TypeParams  []*TypeParam
	Declaration Node
}

func (n *TemplateDecl) TokenLiteral() string { return n.Tok.Literal }
func (n *TemplateDecl) NodeType() string     { return "TemplateDecl" }

func (n *TemplateDecl) TemplateName() string {
	switch d := n.Declaration.(type) {
	case *FuncDecl:
		return d.Name
	case *StructDecl:
		return d.Name
	}
	return ""
}

type TemplateInstantiation struct {
	Tok          Token
	Name         string
	TypeArgs     []string
	Object       Node
	ResolvedName string
}

func (n *TemplateInstantiation) TokenLiteral() string { return n.Tok.Literal }
func (n *TemplateInstantiation) NodeType() string     { return "TemplateInstantiation" }

// Alias so buildTree can match TypeAliasDecl (for printer).
var _ = fmt.Sprintf
