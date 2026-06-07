package main

import (
	"fmt"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// Lexer
// ---------------------------------------------------------------------------

// Lexer is a stateful scanner over a source string.
// It emits a flat slice of Tokens including virtual INDENT / DEDENT tokens
// that encode Python-style significant whitespace.
type Lexer struct {
	source   string
	filePath string
	pos      int    // current byte position in source
	line     int    // current 1-based line number
	col      int    // current 1-based column number
	tokens   []Token

	// Indentation stack.  Initialised with a single element [0] representing
	// the base (column 0) indentation level.
	indentStack []int

	// CBlock raw-mode fields: when a cblock body is detected, the lexer
	// skips tokenization and captures the raw lines verbatim into a single
	// TOKEN_CBLOCK_RAW token, so ALL C syntax (including #, \, etc.) works.
	cblockPending bool     // set after lexing "cblock:" on a line
	cblockDepth   int      // > 0 when inside a cblock raw body
	cblockIndent  int      // the indent level of cblock content (first content line)
	cblockLines   []string // accumulated raw lines of the cblock body
}

// NewLexer constructs a Lexer ready to tokenize src.
func NewLexer(src string) *Lexer {
	return &Lexer{
		source:      src,
		pos:         0,
		line:        1,
		col:         1,
		indentStack: []int{0},
	}
}

// NewLexerWithFile constructs a Lexer with the source file path (used for error messages).
func NewLexerWithFile(src string, filePath string) *Lexer {
	return &Lexer{
		source:      src,
		filePath:    filePath,
		pos:         0,
		line:        1,
		col:         1,
		indentStack: []int{0},
	}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Tokenize runs the full scan and returns the complete token slice.
func (l *Lexer) Tokenize() []Token {
	for !l.isEOF() {
		l.scanLine()
	}
	// Close any remaining open indent levels.
	l.closeAllIndents()
	l.emit(TOKEN_EOF, "")
	return l.tokens
}

// ---------------------------------------------------------------------------
// Core scanning
// ---------------------------------------------------------------------------

// scanLine handles one logical source line: leading whitespace (indent logic),
// then all tokens on the line, then the trailing NEWLINE.
// When a cblock body is active, the content is captured as raw text instead.
func (l *Lexer) scanLine() {
	// If a cblock body is active, handle it as raw content.
	if l.cblockPending {
		l.handleCBlockStart()
		return
	}
	if l.cblockDepth > 0 {
		l.handleCBlockBody()
		return
	}

	// Skip blank lines and comment-only lines without touching the indent stack.
	if l.isBlankOrCommentLine() {
		l.skipToNextLine()
		return
	}

	// Count leading spaces to determine indentation level.
	spaces, bytes := l.countLeadingSpaces()

	// Emit INDENT or DEDENT tokens as required.
	l.handleIndent(spaces)

	// Skip past the leading whitespace we just measured.
	l.advance(bytes)

	// Scan tokens until end-of-line or EOF.
	for !l.isEOF() && l.current() != '\n' {
		l.skipWhitespace()
		if l.isEOF() || l.current() == '\n' {
			break
		}
		if l.current() == '#' {
			l.skipLineComment()
			break
		}
		if l.tryMultiLineComment() {
			continue
		}
		l.scanToken()
	}

	// After scanning all tokens, check if this line started a cblock.
	if !l.cblockPending && l.hasCBlockStart() {
		l.cblockPending = true
	}

	l.emit(TOKEN_NEWLINE, "\n")

	// Consume the newline character itself.
	if !l.isEOF() && l.current() == '\n' {
		l.advance(1)
		l.line++
		l.col = 1
	}
}

// hasCBlockStart checks whether the tokens emitted for the current line end with
// TOKEN_CBLOCK followed by TOKEN_COLON. If so, it also verifies the next line
// has content (is not blank) and records its indent level.
func (l *Lexer) hasCBlockStart() bool {
	tokens := l.tokens
	n := len(tokens)
	// Look backwards from the end for the pattern ... CBLOCK COLON
	for i := n - 1; i >= 0; i-- {
		tt := tokens[i].Type
		if tt == TOKEN_NEWLINE {
			return false
		}
		if tt == TOKEN_COLON && i > 0 && tokens[i-1].Type == TOKEN_CBLOCK {
			// Found cblock: — skip past the \n and check the next line
			nextPos := l.pos
			for nextPos < len(l.source) && l.source[nextPos] != '\n' {
				nextPos++
			}
			if nextPos < len(l.source) {
				nextPos++ // skip \n
			}
			// Check if next line has non-blank, non-comment content
			for nextPos < len(l.source) && (l.source[nextPos] == ' ' || l.source[nextPos] == '\t') {
				nextPos++
			}
			if nextPos < len(l.source) && l.source[nextPos] != '\n' {
				// Content exists — cblock body follows
				return true
			}
			return false
		}
	}
	return false
}

// handleCBlockStart is called by scanLine when cblockPending is true.
// It processes the first content line of a cblock: counts its indent,
// emits the INDENT token (to satisfy the parser), captures the raw line,
// and sets cblockDepth for subsequent lines.
func (l *Lexer) handleCBlockStart() {
	l.cblockPending = false

	// Count leading spaces of this (first content) line.
	spaces, bytes := l.countLeadingSpaces()

	// Emit INDENT if this line is more indented than the cblock: line.
	l.handleIndent(spaces)

	// Record the indent level of cblock content.
	l.cblockIndent = spaces
	l.cblockDepth = 1

	// Capture the raw content of this first line.
	lineStart := l.pos + bytes
	lineEnd := strings.IndexByte(l.source[lineStart:], '\n')
	if lineEnd < 0 {
		lineEnd = len(l.source)
	} else {
		lineEnd += lineStart
	}
	l.cblockLines = append(l.cblockLines, l.source[lineStart:lineEnd])

	// Advance past the entire line.
	l.pos = lineEnd
	if l.pos < len(l.source) && l.source[l.pos] == '\n' {
		l.advance(1)
		l.line++
		l.col = 1
	}
}

// handleCBlockBody is called by scanLine when cblockDepth > 0.
// It captures raw lines until the indent drops below cblockIndent,
// then emits a single TOKEN_CBLOCK_RAW token and falls through to
// normal line processing for the DEDENT.
func (l *Lexer) handleCBlockBody() {
	spaces, bytes := l.countLeadingSpaces()

	// Check if this line is blank (nothing after leading whitespace before \n or EOF).
	restStart := l.pos + bytes
	restIsBlank := restStart >= len(l.source) || l.source[restStart] == '\n'
	if restIsBlank && len(l.cblockLines) > 0 {
		// Blank line inside cblock — preserve as empty string.
		l.cblockLines = append(l.cblockLines, "")
		l.pos = restStart
		if l.pos < len(l.source) && l.source[l.pos] == '\n' {
			l.advance(1)
			l.line++
			l.col = 1
		}
		return
	}

	if spaces < l.cblockIndent && len(l.cblockLines) > 0 {
		// End of cblock body — emit the raw content as one token.
		body := strings.Join(l.cblockLines, "\n")
		l.emit(TOKEN_CBLOCK_RAW, body)
		l.cblockLines = nil
		l.cblockDepth = 0
		// Fall through to normal line processing (handleIndent will emit DEDENT).
	} else if spaces >= l.cblockIndent || (len(l.cblockLines) == 0 && !restIsBlank) {
		// Still inside cblock OR first non-blank line of empty cblock: capture this line.
		lineStart := l.pos + bytes
		lineEnd := strings.IndexByte(l.source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(l.source)
		} else {
			lineEnd += lineStart
		}
		if spaces >= l.cblockIndent || len(l.cblockLines) == 0 {
			l.cblockLines = append(l.cblockLines, l.source[lineStart:lineEnd])
		}
		// Advance past this line.
		l.pos = lineEnd
		if l.pos < len(l.source) && l.source[l.pos] == '\n' {
			l.advance(1)
			l.line++
			l.col = 1
		}
		if len(l.cblockLines) == 0 {
			l.cblockDepth = 0
		}
		return
	}

	// --- Fall through for DEDENT handling on the closing line ---

	// Emit INDENT/DEDENT for this closing line.
	l.handleIndent(spaces)
	l.advance(bytes)

	// Scan remaining tokens on this line normally.
	for !l.isEOF() && l.current() != '\n' {
		l.skipWhitespace()
		if l.isEOF() || l.current() == '\n' {
			break
		}
		if l.current() == '#' {
			l.skipLineComment()
			break
		}
		if l.tryMultiLineComment() {
			continue
		}
		l.scanToken()
	}

	l.emit(TOKEN_NEWLINE, "\n")
	if !l.isEOF() && l.current() == '\n' {
		l.advance(1)
		l.line++
		l.col = 1
	}
}

// scanToken reads exactly one token from the current position.
func (l *Lexer) scanToken() {
	ch := l.current()

	switch {
	case isDigit(ch) || (ch == '-' && l.peekIsDigit()):
		l.scanNumber()
	case isLetter(ch) || ch == '_':
		l.scanIdentOrKeyword()
	case ch == '"':
		l.scanString()
	case ch == '\'':
		l.scanChar()
	case ch == '&':
		if l.peek() == '&' {
			l.emitDouble(TOKEN_AMPERSAND)
		} else {
			l.emitSingle(TOKEN_AMPERSAND)
		}
	case ch == '|':
		if l.peek() == '|' {
			l.emitDouble(TOKEN_PIPE)
		} else {
			l.emitSingle(TOKEN_PIPE)
		}
	case ch == '!':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_NEQ)
		} else {
			l.emitSingle(TOKEN_EXCLAMATION)
		}
	case ch == '=':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_EQ)
		} else {
			l.emitSingle(TOKEN_ASSIGN)
		}
	case ch == '<':
		switch l.peek() {
		case '<':
			l.emitDouble(TOKEN_LSHIFT)
		case '=':
			l.emitDouble(TOKEN_LTE)
		default:
			l.emitSingle(TOKEN_LT)
		}
	case ch == '>':
		switch l.peek() {
		case '>':
			l.emitDouble(TOKEN_RSHIFT)
		case '=':
			l.emitDouble(TOKEN_GTE)
		default:
			l.emitSingle(TOKEN_GT)
		}
	case ch == '+':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_PLUS_ASSIGN)
		} else {
			l.emitSingle(TOKEN_PLUS)
		}
	case ch == '-':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_MINUS_ASSIGN)
		} else {
			l.emitSingle(TOKEN_MINUS)
		}
	case ch == '*':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_STAR_ASSIGN)
		} else {
			l.emitSingle(TOKEN_STAR)
		}
	case ch == '/':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_SLASH_ASSIGN)
		} else {
			l.emitSingle(TOKEN_SLASH)
		}
	case ch == '%':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_MOD_ASSIGN)
		} else {
			l.emitSingle(TOKEN_PERCENT)
		}
	case ch == '^':
		l.emitSingle(TOKEN_CARET)
	case ch == '(':
		l.emitSingle(TOKEN_LPAREN)
	case ch == ')':
		l.emitSingle(TOKEN_RPAREN)
	case ch == '[':
		l.emitSingle(TOKEN_LBRACKET)
	case ch == ']':
		l.emitSingle(TOKEN_RBRACKET)
	case ch == '{':
		l.emitSingle(TOKEN_LBRACE)
	case ch == '}':
		l.emitSingle(TOKEN_RBRACE)
	case ch == ':':
		if l.peek() == '=' {
			l.emitDouble(TOKEN_DEFINE)
		} else {
			l.emitSingle(TOKEN_COLON)
		}
	case ch == ',':
		l.emitSingle(TOKEN_COMMA)
	case ch == '.':
		l.emitSingle(TOKEN_DOT)
	case ch == ';':
		l.emitSingle(TOKEN_SEMI)
	case ch == '?':
		l.emitSingle(TOKEN_QUESTION)
	case ch == '@':
		l.emitSingle(TOKEN_AT)
	case ch == '~':
		l.emitSingle(TOKEN_TILDE)
	case ch == '`':
		l.emitSingle(TOKEN_BACKTICK)
	default:
		l.lexError(fmt.Sprintf("unexpected character '%c'", ch))
	}
}

// ---------------------------------------------------------------------------
// Indentation handling
// ---------------------------------------------------------------------------

// handleIndent compares spaces to the top of the indent stack and emits the
// appropriate INDENT or DEDENT tokens.
func (l *Lexer) handleIndent(spaces int) {
	top := l.indentStack[len(l.indentStack)-1]

	if spaces > top {
		l.indentStack = append(l.indentStack, spaces)
		l.emitAt(TOKEN_INDENT, "", l.line, 1)
		return
	}

	if spaces < top {
		for len(l.indentStack) > 1 {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			l.emitAt(TOKEN_DEDENT, "", l.line, 1)
			if l.indentStack[len(l.indentStack)-1] == spaces {
				return
			}
		}
		// If we exit the loop without matching, the indentation is invalid.
		if l.indentStack[0] != spaces {
			l.lexError(fmt.Sprintf(
				"mismatched indentation (got %d spaces, expected %d or 0)",
				spaces, top,
			))
		}
	}
	// spaces == top: same level, nothing to emit.
}

// closeAllIndents pops every remaining indent level at end-of-file.
func (l *Lexer) closeAllIndents() {
	for len(l.indentStack) > 1 {
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		l.emitAt(TOKEN_DEDENT, "", l.line, 1)
	}
}

// countLeadingSpaces counts the number of leading spaces or tabs.
// Tabs are treated as 4 spaces for the purpose of the indentation stack.
// Returns (logicalSpaces, bytesConsumed).
func (l *Lexer) countLeadingSpaces() (int, int) {
	spaces := 0
	bytes := 0
	i := l.pos
	for i < len(l.source) {
		ch := l.source[i]
		if ch == ' ' {
			spaces++
			i++
			bytes++
		} else if ch == '\t' {
			spaces += 4
			i++
			bytes++
		} else {
			break
		}
	}
	return spaces, bytes
}

// ---------------------------------------------------------------------------
// Literal scanners
// ---------------------------------------------------------------------------

// scanNumber reads an integer or float literal (decimal or 0x hex).
func (l *Lexer) scanNumber() {
	start := l.pos
	startCol := l.col
	isFloat := false

	if l.current() == '-' {
		l.advance(1)
	}

	if l.current() == '0' && (l.peek() == 'x' || l.peek() == 'X') {
		// Hexadecimal
		l.advance(2) // skip 0x or 0X
		if l.isEOF() || !isHexDigit(l.current()) {
			l.lexError("invalid hex literal: expected hex digits after '0x'")
			return
		}
		for !l.isEOF() && isHexDigit(l.current()) {
			l.advance(1)
		}
	} else {
		for !l.isEOF() && isDigit(l.current()) {
			l.advance(1)
		}
		if !l.isEOF() && l.current() == '.' && isDigit(l.peekAt(1)) {
			isFloat = true
			l.advance(1) // skip '.'
			for !l.isEOF() && isDigit(l.current()) {
				l.advance(1)
			}
		}
		if !l.isEOF() && (l.current() == 'e' || l.current() == 'E') {
			isFloat = true
			l.advance(1)
			if l.current() == '+' || l.current() == '-' {
				l.advance(1)
			}
			for !l.isEOF() && isDigit(l.current()) {
				l.advance(1)
			}
		}
	}

	lit := l.source[start:l.pos]
	if isFloat {
		l.tokens = append(l.tokens, Token{TOKEN_FLOAT, lit, l.line, startCol})
	} else {
		l.tokens = append(l.tokens, Token{TOKEN_INT, lit, l.line, startCol})
	}
}

// scanIdentOrKeyword reads an identifier or keyword.
func (l *Lexer) scanIdentOrKeyword() {
	start := l.pos
	startCol := l.col
	for !l.isEOF() && (isLetter(l.current()) || isDigit(l.current()) || l.current() == '_') {
		l.advance(1)
	}
	lit := l.source[start:l.pos]
	tt := LookupIdent(lit)
	l.tokens = append(l.tokens, Token{tt, lit, l.line, startCol})
}

// scanString reads a double-quoted string literal.
func (l *Lexer) scanString() {
	startCol := l.col
	l.advance(1) // opening "
	var sb strings.Builder
	for !l.isEOF() && l.current() != '"' {
		if l.current() == '\\' {
			l.advance(1)
			sb.WriteByte(l.scanEscape())
		} else {
			sb.WriteByte(l.current())
			l.advance(1)
		}
	}
	if l.isEOF() {
		l.lexError("unterminated string literal")
	}
	l.advance(1) // closing "
	l.tokens = append(l.tokens, Token{TOKEN_STRING, sb.String(), l.line, startCol})
}

// scanMultiLineString reads a triple-quoted string literal """...""".
func (l *Lexer) scanMultiLineString() {
	startLine := l.line
	startCol := l.col
	l.advance(3) // opening """
	var sb strings.Builder
	for !l.isEOF() {
		if l.current() == '"' && l.peek() == '"' && l.peekAt(2) == '"' {
			l.advance(3) // closing """
			l.tokens = append(l.tokens, Token{TOKEN_STRING, sb.String(), startLine, startCol})
			return
		}
		if l.current() == '\\' {
			l.advance(1)
			sb.WriteByte(l.scanEscape())
		} else if l.current() == '\n' {
			sb.WriteByte(l.current())
			l.advance(1)
			l.line++
			l.col = 1
		} else {
			sb.WriteByte(l.current())
			l.advance(1)
		}
	}
	l.lexError("unterminated multi-line string literal")
}

// scanChar reads a single-quoted character literal.
func (l *Lexer) scanChar() {
	startCol := l.col
	l.advance(1) // opening '
	var ch byte
	if l.current() == '\\' {
		l.advance(1)
		ch = l.scanEscape()
	} else {
		ch = l.current()
		l.advance(1)
	}
	if l.isEOF() || l.current() != '\'' {
		l.lexError("unterminated or multi-character char literal")
	}
	l.advance(1) // closing '
	l.tokens = append(l.tokens, Token{TOKEN_CHAR, string(ch), l.line, startCol})
}

// scanEscape handles the character following a backslash in a string or char.
// Returns the resolved byte value.
func (l *Lexer) scanEscape() byte {
	if l.isEOF() {
		l.lexError("unexpected EOF in escape sequence")
	}
	ch := l.current()
	l.advance(1)
	switch ch {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case '\\':
		return '\\'
	case '\'':
		return '\''
	case '"':
		return '"'
	case '0':
		return 0
	case 'x':
		// Hex byte: \xNN
		if l.pos+2 > len(l.source) {
			l.lexError("incomplete hex escape sequence")
		}
		if !isHexDigit(l.source[l.pos]) || !isHexDigit(l.source[l.pos+1]) {
			l.lexError("invalid hex digit in \\x escape sequence")
		}
		hi := hexVal(l.source[l.pos])
		lo := hexVal(l.source[l.pos+1])
		l.advance(2)
		return byte(hi<<4 | lo)
	default:
		l.lexError(fmt.Sprintf("unknown escape sequence '\\%c'", ch))
		return 0
	}
}

// ---------------------------------------------------------------------------
// Comment handling
// ---------------------------------------------------------------------------

// skipLineComment advances past all characters until end-of-line.
func (l *Lexer) skipLineComment() {
	for !l.isEOF() && l.current() != '\n' {
		l.advance(1)
	}
}

// tryMultiLineComment checks for a triple-quote """ and, if found, scans
// until the closing """ and emits a TOKEN_STRING.
func (l *Lexer) tryMultiLineComment() bool {
	if l.pos+2 >= len(l.source) {
		return false
	}
	if l.source[l.pos:l.pos+3] != `"""` {
		return false
	}
	l.scanMultiLineString()
	return true
}

// ---------------------------------------------------------------------------
// Helper predicates & emitters
// ---------------------------------------------------------------------------

func (l *Lexer) isEOF() bool { return l.pos >= len(l.source) }

func (l *Lexer) current() byte {
	if l.isEOF() {
		return 0
	}
	return l.source[l.pos]
}

func (l *Lexer) peek() byte { return l.peekAt(1) }

func (l *Lexer) peekAt(offset int) byte {
	p := l.pos + offset
	if p >= len(l.source) {
		return 0
	}
	return l.source[p]
}

func (l *Lexer) peekIsDigit() bool { return isDigit(l.peek()) }

func (l *Lexer) advance(n int) {
	for i := 0; i < n; i++ {
		if l.isEOF() {
			return
		}
		l.pos++
		l.col++
	}
}

func (l *Lexer) skipWhitespace() {
	for !l.isEOF() && (l.current() == ' ' || l.current() == '\t' || l.current() == '\r') {
		l.advance(1)
	}
}

func (l *Lexer) skipToNextLine() {
	for !l.isEOF() && l.current() != '\n' {
		l.advance(1)
	}
	if !l.isEOF() {
		l.advance(1)
		l.line++
		l.col = 1
	}
}

// 	isBlankOrCommentLine returns true if the rest of the current line is empty,
// whitespace-only, or a comment, so the indentation stack should not be disturbed.
func (l *Lexer) isBlankOrCommentLine() bool {
	i := l.pos
	for i < len(l.source) && (l.source[i] == ' ' || l.source[i] == '\t') {
		i++
	}
	if i >= len(l.source) {
		return true
	}
	ch := l.source[i]
	return ch == '\n' || ch == '#'
}

func (l *Lexer) emit(tt TokenType, lit string) {
	l.tokens = append(l.tokens, Token{tt, lit, l.line, l.col})
}

func (l *Lexer) emitAt(tt TokenType, lit string, line, col int) {
	l.tokens = append(l.tokens, Token{tt, lit, line, col})
}

func (l *Lexer) emitSingle(tt TokenType) {
	lit := string(l.current())
	l.tokens = append(l.tokens, Token{tt, lit, l.line, l.col})
	l.advance(1)
}

func (l *Lexer) emitDouble(tt TokenType) {
	lit := string(l.source[l.pos : l.pos+2])
	l.tokens = append(l.tokens, Token{tt, lit, l.line, l.col})
	l.advance(2)
}

func (l *Lexer) lexError(msg string) {
	fmt.Fprintf(os.Stderr, "[merlin] ERROR (%s, line %d, col %d): %s\n", l.filePath, l.line, l.col, msg)
	os.Exit(1)
}

// ---------------------------------------------------------------------------
// Character classification helpers
// ---------------------------------------------------------------------------

func isDigit(ch byte) bool    { return ch >= '0' && ch <= '9' }
func isHexDigit(ch byte) bool { return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') }
func isLetter(ch byte) bool   { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') }

func hexVal(ch byte) byte {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0'
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10
	}
	return 0
}

