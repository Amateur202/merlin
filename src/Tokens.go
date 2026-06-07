package main

// TokenType is a string tag identifying what a token represents.
type TokenType string

// Token holds a single lexical unit extracted from source.
type Token struct {
	Type    TokenType
	Literal string // Exact slice of source text
	Line    int    // 1-based line number
	Col     int    // 1-based column offset
}

// ---------------------------------------------------------------------------
// Token type constants — grouped by category for readability.
// ---------------------------------------------------------------------------

const (
	// --- Structural / Virtual ---
	TOKEN_INDENT  TokenType = "INDENT"  // Emitted when indentation level increases
	TOKEN_DEDENT  TokenType = "DEDENT"  // Emitted when indentation level decreases
	TOKEN_NEWLINE TokenType = "NEWLINE" // End of logical line
	TOKEN_EOF     TokenType = "EOF"     // End of input

	// --- Identifiers & Literals ---
	TOKEN_IDENT  TokenType = "IDENT"  // e.g.  myVar, configure
	TOKEN_INT    TokenType = "INT"    // e.g.  42, 0xFF, -7
	TOKEN_FLOAT  TokenType = "FLOAT"  // e.g.  3.14, 1.0
	TOKEN_STRING TokenType = "STRING" // e.g.  "hello"
	TOKEN_CHAR   TokenType = "CHAR"   // e.g.  'A', '\n', '\x00'

	// --- Primitive Type Keywords ---
	TOKEN_VOID    TokenType = "void"
	TOKEN_BOOL    TokenType = "bool"
	TOKEN_CHAR_KW TokenType = "char"    // keyword 'char' vs TOKEN_CHAR literal
	TOKEN_STRING_KW TokenType = "string"
	TOKEN_INT_KW  TokenType = "int"
	TOKEN_INT8    TokenType = "int8"
	TOKEN_INT16   TokenType = "int16"
	TOKEN_INT32   TokenType = "int32"
	TOKEN_INT64   TokenType = "int64"
	TOKEN_UINT8   TokenType = "uint8"
	TOKEN_UINT16  TokenType = "uint16"
	TOKEN_UINT32  TokenType = "uint32"
	TOKEN_UINT64  TokenType = "uint64"
	TOKEN_FLOAT_KW  TokenType = "float"
	TOKEN_FLOAT32 TokenType = "float32"
	TOKEN_FLOAT64 TokenType = "float64"

	// --- Language Keywords ---
	TOKEN_IMPORT   TokenType = "import"
	TOKEN_TYPE     TokenType = "type"
	TOKEN_STRUCT   TokenType = "struct"
	TOKEN_ENUM     TokenType = "enum"
	TOKEN_IF       TokenType = "if"
	TOKEN_ELIF     TokenType = "elif"
	TOKEN_ELSE     TokenType = "else"
	TOKEN_FOR      TokenType = "for"
	TOKEN_IN       TokenType = "in"
	TOKEN_RANGE    TokenType = "range"
	TOKEN_MATCH    TokenType = "match"
	TOKEN_CASE     TokenType = "case"
	TOKEN_RETURN   TokenType = "return"
	TOKEN_BREAK    TokenType = "break"
	TOKEN_CONTINUE TokenType = "continue"
	TOKEN_PASS     TokenType = "pass"
	TOKEN_ASM      TokenType = "asm"
	TOKEN_CBLOCK   TokenType = "cblock"
	TOKEN_VOLATILE TokenType = "volatile"
	TOKEN_EXTERNAL  TokenType = "external"
	TOKEN_LINK      TokenType = "link"
	TOKEN_SELF      TokenType = "self"
	TOKEN_TEMPLATE  TokenType = "template"
	TOKEN_TRUE      TokenType = "true"
	TOKEN_FALSE     TokenType = "false"
	TOKEN_THROWS    TokenType = "throws"
	TOKEN_RAISE     TokenType = "raise"
	TOKEN_TRY       TokenType = "try"
	TOKEN_CATCH     TokenType = "catch"
	TOKEN_CONST     TokenType = "const"
	TOKEN_DEFER     TokenType = "defer"
	TOKEN_PRIVATE   TokenType = "private"

	// --- Arithmetic Operators ---
	TOKEN_PLUS    TokenType = "+"
	TOKEN_MINUS   TokenType = "-"
	TOKEN_STAR    TokenType = "*"
	TOKEN_SLASH   TokenType = "/"
	TOKEN_PERCENT TokenType = "%"

	// --- Compound Assignment Operators ---
	TOKEN_PLUS_ASSIGN  TokenType = "+="
	TOKEN_MINUS_ASSIGN TokenType = "-="
	TOKEN_STAR_ASSIGN  TokenType = "*="
	TOKEN_SLASH_ASSIGN TokenType = "/="
	TOKEN_MOD_ASSIGN   TokenType = "%="

	// --- Comparison Operators ---
	TOKEN_EQ  TokenType = "=="
	TOKEN_NEQ TokenType = "!="
	TOKEN_LT  TokenType = "<"
	TOKEN_GT  TokenType = ">"
	TOKEN_LTE TokenType = "<="
	TOKEN_GTE TokenType = ">="

	// --- Logical Operators ---
	TOKEN_AND TokenType = "and"
	TOKEN_OR  TokenType = "or"
	TOKEN_NOT TokenType = "not"

	// --- Bitwise Operators ---
	TOKEN_AMPERSAND TokenType = "&"   // Also used as address-of / pointer prefix
	TOKEN_PIPE      TokenType = "|"
	TOKEN_CARET     TokenType = "^"
	TOKEN_LSHIFT    TokenType = "<<"
	TOKEN_RSHIFT    TokenType = ">>"

	// --- Assignment ---
	TOKEN_ASSIGN TokenType = "="
	TOKEN_DEFINE TokenType = ":="

	// --- Raw block content (cblock body emitted by lexer) ---
	TOKEN_CBLOCK_RAW TokenType = "CBLOCK_RAW"

	// --- Delimiters ---
	TOKEN_LPAREN   TokenType = "("
	TOKEN_RPAREN   TokenType = ")"
	TOKEN_LBRACKET TokenType = "["
	TOKEN_RBRACKET TokenType = "]"
	TOKEN_LBRACE   TokenType = "{"
	TOKEN_RBRACE   TokenType = "}"
	TOKEN_COLON    TokenType = ":"
	TOKEN_COMMA    TokenType = ","
	TOKEN_SEMI     TokenType = ";"
	TOKEN_DOT      TokenType = "."
	TOKEN_AT       TokenType = "@"
	TOKEN_QUESTION    TokenType = "?"
	TOKEN_EXCLAMATION TokenType = "!"
	TOKEN_TILDE       TokenType = "~"
	TOKEN_BACKTICK    TokenType = "`"
)

// ---------------------------------------------------------------------------
// keywords maps raw identifier strings to their keyword token type.
// The lexer consults this after scanning an identifier.
// ---------------------------------------------------------------------------

var keywords = map[string]TokenType{
	"void":     TOKEN_VOID,
	"bool":     TOKEN_BOOL,
	"char":     TOKEN_CHAR_KW,
	"string":   TOKEN_STRING_KW,
	"int":      TOKEN_INT_KW,
	"int8":     TOKEN_INT8,
	"int16":    TOKEN_INT16,
	"int32":    TOKEN_INT32,
	"int64":    TOKEN_INT64,
	"uint8":    TOKEN_UINT8,
	"uint16":   TOKEN_UINT16,
	"uint32":   TOKEN_UINT32,
	"uint64":   TOKEN_UINT64,
	"float":    TOKEN_FLOAT_KW,
	"float32":  TOKEN_FLOAT32,
	"float64":  TOKEN_FLOAT64,
	"import":   TOKEN_IMPORT,
	"type":     TOKEN_TYPE,
	"struct":   TOKEN_STRUCT,
	"enum":     TOKEN_ENUM,
	"if":       TOKEN_IF,
	"elif":     TOKEN_ELIF,
	"else":     TOKEN_ELSE,
	"for":      TOKEN_FOR,
	"in":       TOKEN_IN,
	"range":    TOKEN_RANGE,
	"match":    TOKEN_MATCH,
	"case":     TOKEN_CASE,
	"return":   TOKEN_RETURN,
	"break":    TOKEN_BREAK,
	"continue": TOKEN_CONTINUE,
	"pass":     TOKEN_PASS,
	"and":      TOKEN_AND,
	"or":       TOKEN_OR,
	"not":      TOKEN_NOT,
	"asm":      TOKEN_ASM,
	"cblock":   TOKEN_CBLOCK,
	"volatile": TOKEN_VOLATILE,
	"external": TOKEN_EXTERNAL,
	"link":      TOKEN_LINK,
	"self":     TOKEN_SELF,
	"template": TOKEN_TEMPLATE,
	"true":     TOKEN_TRUE,
	"false":    TOKEN_FALSE,
	"throws":   TOKEN_THROWS,
	"raise":    TOKEN_RAISE,
	"try":      TOKEN_TRY,
	"catch":    TOKEN_CATCH,
	"const":    TOKEN_CONST,
	"defer":    TOKEN_DEFER,
	"private":  TOKEN_PRIVATE,
}

// LookupIdent returns the keyword TokenType for s, or TOKEN_IDENT if s is
// not a reserved word.
func LookupIdent(s string) TokenType {
	if tok, ok := keywords[s]; ok {
		return tok
	}
	return TOKEN_IDENT
}

// ---------------------------------------------------------------------------
// typeKeywords is the set of tokens that represent a data type annotation.
// The parser uses this to distinguish type names from general identifiers.
// ---------------------------------------------------------------------------

var typeKeywords = map[TokenType]bool{
	TOKEN_VOID:      true,
	TOKEN_BOOL:      true,
	TOKEN_CHAR_KW:   true,
	TOKEN_STRING_KW: true,
	TOKEN_INT_KW:    true,
	TOKEN_INT8:      true,
	TOKEN_INT16:     true,
	TOKEN_INT32:     true,
	TOKEN_INT64:     true,
	TOKEN_UINT8:     true,
	TOKEN_UINT16:    true,
	TOKEN_UINT32:    true,
	TOKEN_UINT64:    true,
	TOKEN_FLOAT_KW:  true,
	TOKEN_FLOAT32:   true,
	TOKEN_FLOAT64:   true,
	TOKEN_IDENT:     true, // Custom type names are plain identifiers
}

// IsTypeToken returns true if t can begin a type annotation.
func IsTypeToken(t TokenType) bool {
	return typeKeywords[t]
}
