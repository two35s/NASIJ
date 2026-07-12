package ast

import (
	"strconv"
	"strings"
)

type Tokenizer struct {
	input    string
	pos      int
	line     int
	col      int
	prevLine int
	prevCol  int
	ch       byte
	peekCh   byte
}

type tokState struct {
	pos      int
	line     int
	col      int
	prevLine int
	prevCol  int
	ch       byte
	peekCh   byte
}

func (t *Tokenizer) snap() tokState {
	return tokState{t.pos, t.line, t.col, t.prevLine, t.prevCol, t.ch, t.peekCh}
}

func (t *Tokenizer) restore(s tokState) {
	t.pos, t.line, t.col, t.prevLine, t.prevCol, t.ch, t.peekCh = s.pos, s.line, s.col, s.prevLine, s.prevCol, s.ch, s.peekCh
}

func NewTokenizer(input string) *Tokenizer {
	t := &Tokenizer{input: input, line: 1, col: 1}
	t.advance()
	return t
}

func (t *Tokenizer) advance() {
	t.prevLine = t.line
	t.prevCol = t.col
	if t.pos >= len(t.input) {
		t.ch = 0
		t.peekCh = 0
		t.pos++
		return
	}
	t.ch = t.input[t.pos]
	t.pos++
	t.col++
	if t.ch == '\n' {
		t.line++
		t.col = 1
	}
	if t.pos < len(t.input) {
		t.peekCh = t.input[t.pos]
	} else {
		t.peekCh = 0
	}
}

func (t *Tokenizer) skipWhitespace() {
	for t.ch != 0 && (t.ch == ' ' || t.ch == '\t' || t.ch == '\n' || t.ch == '\r') {
		t.advance()
	}
}

func (t *Tokenizer) skipLineComment() {
	for t.ch != 0 && t.ch != '\n' {
		t.advance()
	}
}

func (t *Tokenizer) skipBlockComment() {
	for t.ch != 0 {
		if t.ch == '*' && t.peekCh == '/' {
			t.advance()
			t.advance()
			return
		}
		t.advance()
	}
}

func (t *Tokenizer) Next() Token {
	t.skipWhitespace()

	line, col := t.prevLine, t.prevCol
	pos := t.pos

	if t.ch == 0 {
		return Token{Type: EOF, Pos: Pos{Line: line, Col: col}}
	}

	// Single-line comments
	if t.ch == '/' && t.peekCh == '/' {
		t.skipLineComment()
		return t.Next()
	}
	// Block comments
	if t.ch == '/' && t.peekCh == '*' {
		t.advance()
		t.advance()
		t.skipBlockComment()
		return t.Next()
	}

	// Strings
	if t.ch == '"' || t.ch == '\'' {
		return t.readString()
	}
	// Template literals
	if t.ch == '`' {
		return t.readTemplate()
	}

	// Numbers
	if isDigit(t.ch) || (t.ch == '.' && isDigit(t.peekCh)) {
		return t.readNumber()
	}

	// Identifiers and keywords
	if isIdentStart(t.ch) {
		return t.readIdent()
	}

	// Punctuation and operators
	tok := t.readPunctuation()
	tok.Pos = Pos{Line: line, Col: col}
	_ = pos
	return tok
}

func (t *Tokenizer) readString() Token {
	quote := t.ch
	line, col := t.prevLine, t.prevCol
	t.advance() // skip opening quote

	var buf strings.Builder
	for t.ch != 0 {
		if t.ch == '\\' {
			t.advance()
			switch t.ch {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case 'r':
				buf.WriteByte('\r')
			case '0':
				buf.WriteByte(0)
			case '"', '\'', '\\', '`':
				buf.WriteByte(t.ch)
			default:
				buf.WriteByte('\\')
				if t.ch != 0 {
					buf.WriteByte(t.ch)
				}
			}
			t.advance()
			continue
		}
		if t.ch == quote {
			t.advance()
			return Token{
				Type:    STRING,
				Literal: buf.String(),
				Pos:     Pos{Line: line, Col: col},
			}
		}
		if t.ch == '\n' {
			// Unterminated string — return what we have
			break
		}
		buf.WriteByte(t.ch)
		t.advance()
	}

	return Token{Type: ILLEGAL, Literal: "unterminated string", Pos: Pos{Line: line, Col: col}}
}

func (t *Tokenizer) readTemplate() Token {
	line, col := t.prevLine, t.prevCol
	t.advance() // skip opening backtick

	var buf strings.Builder
	for t.ch != 0 {
		if t.ch == '\\' {
			t.advance()
			if t.ch != 0 {
				buf.WriteByte(t.ch)
				t.advance()
			}
			continue
		}
		if t.ch == '$' && t.peekCh == '{' {
			t.advance()
			t.advance()
			if buf.Len() > 0 {
				return Token{
					Type:    TEMPLATE_HEAD,
					Literal: buf.String(),
					Pos:     Pos{Line: line, Col: col},
				}
			}
			return Token{Type: TEMPLATE_HEAD, Literal: "", Pos: Pos{Line: line, Col: col}}
		}
		if t.ch == '`' {
			t.advance()
			return Token{
				Type:    TEMPLATE_LITERAL,
				Literal: buf.String(),
				Pos:     Pos{Line: line, Col: col},
			}
		}
		buf.WriteByte(t.ch)
		t.advance()
	}

	return Token{Type: ILLEGAL, Literal: "unterminated template", Pos: Pos{Line: line, Col: col}}
}

// ReadTemplateMiddle returns a TEMPLATE_MIDDLE or TEMPLATE_TAIL token
// after parsing a ${...} expression.
func (t *Tokenizer) ReadTemplateMiddle() Token {
	line, col := t.prevLine, t.prevCol
	// We've just consumed }, now read until ` or ${ again
	var buf strings.Builder
	for t.ch != 0 {
		if t.ch == '\\' {
			t.advance()
			if t.ch != 0 {
				buf.WriteByte(t.ch)
				t.advance()
			}
			continue
		}
		if t.ch == '$' && t.peekCh == '{' {
			t.advance()
			t.advance()
			return Token{
				Type:    TEMPLATE_MIDDLE,
				Literal: buf.String(),
				Pos:     Pos{Line: line, Col: col},
			}
		}
		if t.ch == '`' {
			t.advance()
			return Token{
				Type:    TEMPLATE_TAIL,
				Literal: buf.String(),
				Pos:     Pos{Line: line, Col: col},
			}
		}
		buf.WriteByte(t.ch)
		t.advance()
	}

	return Token{Type: ILLEGAL, Literal: "unterminated template", Pos: Pos{Line: line, Col: col}}
}

func (t *Tokenizer) readNumber() Token {
	line, col := t.prevLine, t.prevCol
	start := t.pos

	// Count leading zeros for hex/octal/binary detection
	if t.ch == '0' {
		t.advance()
		if t.ch == 'x' || t.ch == 'X' {
			t.advance()
			for isHexDigit(t.ch) {
				t.advance()
			}
			raw := t.input[start-1 : t.pos-1]
			val, _ := strconv.ParseInt(raw[2:], 16, 64)
			return Token{
				Type:    NUMBER,
				Literal: strconv.FormatInt(val, 10),
				Pos:     Pos{Line: line, Col: col},
			}
		}
		if t.ch == 'b' || t.ch == 'B' {
			t.advance()
			for isBinDigit(t.ch) {
				t.advance()
			}
			raw := t.input[start-1 : t.pos-1]
			val, _ := strconv.ParseInt(raw[2:], 2, 64)
			return Token{
				Type:    NUMBER,
				Literal: strconv.FormatInt(val, 10),
				Pos:     Pos{Line: line, Col: col},
			}
		}
		if t.ch == 'o' || t.ch == 'O' {
			t.advance()
			for isOctDigit(t.ch) {
				t.advance()
			}
			raw := t.input[start-1 : t.pos-1]
			val, _ := strconv.ParseInt(raw[2:], 8, 64)
			return Token{
				Type:    NUMBER,
				Literal: strconv.FormatInt(val, 10),
				Pos:     Pos{Line: line, Col: col},
			}
		}
		// Leading zero followed by digits is a decimal
		for isDigit(t.ch) {
			t.advance()
		}
		raw := t.input[start-1 : t.pos-1]
		return Token{
			Type:    NUMBER,
			Literal: raw,
			Pos:     Pos{Line: line, Col: col},
		}
	}

	for isDigit(t.ch) {
		t.advance()
	}
	if t.ch == '.' && isDigit(t.peekCh) {
		t.advance()
		for isDigit(t.ch) {
			t.advance()
		}
	}
	if t.ch == 'e' || t.ch == 'E' {
		t.advance()
		if t.ch == '+' || t.ch == '-' {
			t.advance()
		}
		for isDigit(t.ch) {
			t.advance()
		}
	}

	raw := t.input[start-1 : t.pos-1]
	return Token{
		Type:    NUMBER,
		Literal: raw,
		Pos:     Pos{Line: line, Col: col},
	}
}

var keywords = map[string]TokenType{
	"function": FUNCTION, "return": RETURN,
	"if": IF, "else": ELSE, "for": FOR, "while": WHILE, "do": DO,
	"var": VAR, "let": LET, "const": CONST,
	"import": IMPORT, "export": EXPORT, "from": FROM,
	"default": DEFAULT, "as": AS,
	"class": CLASS, "extends": EXTENDS,
	"new": NEW, "this": THIS, "super": SUPER,
	"delete": DELETE, "void": VOID, "typeof": TYPEOF,
	"instanceof": INSTANCEOF, "in": IN, "of": OF,
	"async": ASYNC, "await": AWAIT, "yield": YIELD,
	"try": TRY, "catch": CATCH, "finally": FINALLY,
	"throw": THROW, "switch": SWITCH, "case": CASE,
	"break": BREAK, "continue": CONTINUE, "static": STATIC,
	"null": NULL, "true": TRUE, "false": FALSE, "undefined": UNDEFINED,
}

func (t *Tokenizer) readIdent() Token {
	line, col := t.prevLine, t.prevCol
	start := t.pos - 1
	for isIdentPart(t.ch) {
		t.advance()
	}
	word := t.input[start : t.pos-1]

	if tt, ok := keywords[word]; ok {
		return Token{Type: tt, Literal: word, Pos: Pos{Line: line, Col: col}}
	}
	return Token{Type: IDENTIFIER, Literal: word, Pos: Pos{Line: line, Col: col}}
}

func (t *Tokenizer) readPunctuation() Token {
	line, col := t.prevLine, t.prevCol

	switch t.ch {
	case '.':
		if t.peekCh == '.' && len(t.input) > t.pos+1 && t.input[t.pos+1] == '.' {
			t.advance()
			t.advance()
			t.advance()
			return Token{Type: ELLIPSIS, Literal: "...", Pos: Pos{Line: line, Col: col}}
		}
		// Check for number starting with .
		if isDigit(t.peekCh) {
			return t.readNumber()
		}
		t.advance()
		return Token{Type: DOT, Literal: ".", Pos: Pos{Line: line, Col: col}}
	case ',':
		t.advance()
		return Token{Type: COMMA, Literal: ",", Pos: Pos{Line: line, Col: col}}
	case ';':
		t.advance()
		return Token{Type: SEMICOLON, Literal: ";", Pos: Pos{Line: line, Col: col}}
	case '(':
		t.advance()
		return Token{Type: LPAREN, Literal: "(", Pos: Pos{Line: line, Col: col}}
	case ')':
		t.advance()
		return Token{Type: RPAREN, Literal: ")", Pos: Pos{Line: line, Col: col}}
	case '[':
		t.advance()
		return Token{Type: LBRACKET, Literal: "[", Pos: Pos{Line: line, Col: col}}
	case ']':
		t.advance()
		return Token{Type: RBRACKET, Literal: "]", Pos: Pos{Line: line, Col: col}}
	case '{':
		t.advance()
		return Token{Type: LBRACE, Literal: "{", Pos: Pos{Line: line, Col: col}}
	case '}':
		t.advance()
		return Token{Type: RBRACE, Literal: "}", Pos: Pos{Line: line, Col: col}}
	case '@':
		t.advance()
		return Token{Type: AT, Literal: "@", Pos: Pos{Line: line, Col: col}}
	case '#':
		t.advance()
		return Token{Type: HASH, Literal: "#", Pos: Pos{Line: line, Col: col}}
	case '~':
		t.advance()
		return Token{Type: TILDE, Literal: "~", Pos: Pos{Line: line, Col: col}}
	case '?':
		if t.peekCh == '.' {
			t.advance()
			t.advance()
			return Token{Type: QUESTION_DOT, Literal: "?.", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: QUESTION, Literal: "?", Pos: Pos{Line: line, Col: col}}
	case ':':
		t.advance()
		return Token{Type: COLON, Literal: ":", Pos: Pos{Line: line, Col: col}}
	case '=':
		if t.peekCh == '=' {
			t.advance()
			if len(t.input) > t.pos+1 && t.input[t.pos] == '=' {
				t.advance()
				t.advance()
				return Token{Type: EQ_EQ_EQ, Literal: "===", Pos: Pos{Line: line, Col: col}}
			}
			t.advance()
			return Token{Type: EQ_EQ, Literal: "==", Pos: Pos{Line: line, Col: col}}
		}
		if t.peekCh == '>' {
			t.advance()
			t.advance()
			return Token{Type: ARROW, Literal: "=>", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: EQ, Literal: "=", Pos: Pos{Line: line, Col: col}}
	case '!':
		if t.peekCh == '=' {
			t.advance()
			if len(t.input) > t.pos+1 && t.input[t.pos] == '=' {
				t.advance()
				t.advance()
				return Token{Type: NOT_EQ_EQ, Literal: "!==", Pos: Pos{Line: line, Col: col}}
			}
			t.advance()
			return Token{Type: NOT_EQ, Literal: "!=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: NOT, Literal: "!", Pos: Pos{Line: line, Col: col}}
	case '<':
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: LTE, Literal: "<=", Pos: Pos{Line: line, Col: col}}
		}
		if t.peekCh == '<' {
			t.advance()
			if len(t.input) > t.pos+1 && t.input[t.pos] == '=' {
				t.advance()
				t.advance()
				return Token{Type: LT_LT_ASSIGN, Literal: "<<=", Pos: Pos{Line: line, Col: col}}
			}
			t.advance()
			return Token{Type: LT_LT, Literal: "<<", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: LT, Literal: "<", Pos: Pos{Line: line, Col: col}}
	case '>':
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: GTE, Literal: ">=", Pos: Pos{Line: line, Col: col}}
		}
		if t.peekCh == '>' {
			t.advance()
			if len(t.input) > t.pos+1 && t.input[t.pos] == '>' {
				t.advance()
				if len(t.input) > t.pos+1 && t.input[t.pos] == '=' {
					t.advance()
					t.advance()
					return Token{Type: GT_GT_ASSIGN, Literal: ">>>=", Pos: Pos{Line: line, Col: col}}
				}
				t.advance()
				return Token{Type: GT_GT_GT, Literal: ">>>", Pos: Pos{Line: line, Col: col}}
			}
			if len(t.input) > t.pos+1 && t.input[t.pos] == '=' {
				t.advance()
				t.advance()
				return Token{Type: GT_GT_ASSIGN, Literal: ">>=", Pos: Pos{Line: line, Col: col}}
			}
			t.advance()
			return Token{Type: GT_GT, Literal: ">>", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: GT, Literal: ">", Pos: Pos{Line: line, Col: col}}
	case '+':
		if t.peekCh == '+' {
			t.advance()
			t.advance()
			return Token{Type: PLUS_PLUS, Literal: "++", Pos: Pos{Line: line, Col: col}}
		}
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: PLUS_ASSIGN, Literal: "+=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: PLUS, Literal: "+", Pos: Pos{Line: line, Col: col}}
	case '-':
		if t.peekCh == '-' {
			t.advance()
			t.advance()
			return Token{Type: MINUS_MINUS, Literal: "--", Pos: Pos{Line: line, Col: col}}
		}
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: MINUS_ASSIGN, Literal: "-=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: MINUS, Literal: "-", Pos: Pos{Line: line, Col: col}}
	case '*':
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: STAR_ASSIGN, Literal: "*=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: STAR, Literal: "*", Pos: Pos{Line: line, Col: col}}
	case '/':
		t.advance()
		return Token{Type: SLASH, Literal: "/", Pos: Pos{Line: line, Col: col}}
	case '%':
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: PERCENT_ASSIGN, Literal: "%=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: PERCENT, Literal: "%", Pos: Pos{Line: line, Col: col}}
	case '&':
		if t.peekCh == '&' {
			t.advance()
			t.advance()
			return Token{Type: AND_AND, Literal: "&&", Pos: Pos{Line: line, Col: col}}
		}
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: AND_ASSIGN, Literal: "&=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: AMPERSAND, Literal: "&", Pos: Pos{Line: line, Col: col}}
	case '|':
		if t.peekCh == '|' {
			t.advance()
			t.advance()
			return Token{Type: OR_OR, Literal: "||", Pos: Pos{Line: line, Col: col}}
		}
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: OR_ASSIGN, Literal: "|=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: PIPE, Literal: "|", Pos: Pos{Line: line, Col: col}}
	case '^':
		if t.peekCh == '=' {
			t.advance()
			t.advance()
			return Token{Type: CARET_ASSIGN, Literal: "^=", Pos: Pos{Line: line, Col: col}}
		}
		t.advance()
		return Token{Type: CARET, Literal: "^", Pos: Pos{Line: line, Col: col}}
	default:
		t.advance()
		return Token{Type: ILLEGAL, Literal: string(t.ch), Pos: Pos{Line: line, Col: col}}
	}
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
func isBinDigit(ch byte) bool   { return ch == '0' || ch == '1' }
func isOctDigit(ch byte) bool   { return ch >= '0' && ch <= '7' }
func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$'
}
func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}

// TokenizeAll is a convenience for testing: tokenizes the entire input.
func TokenizeAll(input string) []Token {
	t := NewTokenizer(input)
	var tokens []Token
	for {
		tok := t.Next()
		tokens = append(tokens, tok)
		if tok.Type == EOF || tok.Type == ILLEGAL {
			break
		}
	}
	return tokens
}

// TokenizeWithPositions tokenizes and returns tokens with position info.
func TokenizeWithPositions(input string) []Token {
	return TokenizeAll(input)
}
