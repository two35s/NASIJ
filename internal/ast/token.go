package ast

import "fmt"

type TokenType int

const (
	ILLEGAL TokenType = iota
	EOF

	// Identifiers & literals
	IDENTIFIER
	NUMBER
	STRING
	TEMPLATE_LITERAL  // full backtick string with no substitutions
	TEMPLATE_HEAD     // `foo${  (part before first substitution)
	TEMPLATE_MIDDLE   // }foo${  (middle parts)
	TEMPLATE_TAIL     // }foo`   (part after last substitution)
	REGEX

	// Keywords
	FUNCTION
	RETURN
	IF
	ELSE
	FOR
	WHILE
	DO
	VAR
	LET
	CONST
	IMPORT
	EXPORT
	FROM
	DEFAULT
	AS
	CLASS
	EXTENDS
	NEW
	THIS
	SUPER
	DELETE
	VOID
	TYPEOF
	INSTANCEOF
	IN
	OF
	ASYNC
	AWAIT
	YIELD
	TRY
	CATCH
	FINALLY
	THROW
	SWITCH
	CASE
	BREAK
	CONTINUE
	STATIC
	NULL
	TRUE
	FALSE
	UNDEFINED

	// Operators
	PLUS
	MINUS
	STAR
	SLASH
	PERCENT
	EQ          // =
	EQ_EQ       // ==
	EQ_EQ_EQ    // ===
	NOT_EQ      // !=
	NOT_EQ_EQ   // !==
	LT          // <
	GT          // >
	LTE         // <=
	GTE         // >=
	AND_AND     // &&
	OR_OR       // ||
	NOT         // !
	TILDE       // ~
	AMPERSAND   // &
	PIPE        // |
	CARET       // ^
	LT_LT       // <<
	GT_GT       // >>
	GT_GT_GT    // >>>
	QUESTION    // ?
	QUESTION_DOT // ?.
	COLON       // :
	ARROW       // =>
	PLUS_PLUS   // ++
	MINUS_MINUS // --
	PLUS_ASSIGN // +=
	MINUS_ASSIGN
	STAR_ASSIGN
	SLASH_ASSIGN
	PERCENT_ASSIGN
	AND_ASSIGN
	OR_ASSIGN
	CARET_ASSIGN
	LT_LT_ASSIGN
	GT_GT_ASSIGN

	// Punctuation
	DOT
	COMMA
	SEMICOLON
	ELLIPSIS
	AT
	HASH

	// Brackets
	LPAREN
	RPAREN
	LBRACKET
	RBRACKET
	LBRACE
	RBRACE
)

var tokenNames = map[TokenType]string{
	ILLEGAL: "ILLEGAL", EOF: "EOF",
	IDENTIFIER: "IDENTIFIER", NUMBER: "NUMBER", STRING: "STRING",
	TEMPLATE_LITERAL: "TEMPLATE_LITERAL", TEMPLATE_HEAD: "TEMPLATE_HEAD",
	TEMPLATE_MIDDLE: "TEMPLATE_MIDDLE", TEMPLATE_TAIL: "TEMPLATE_TAIL",
	REGEX: "REGEX",
	FUNCTION: "FUNCTION", RETURN: "RETURN", IF: "IF", ELSE: "ELSE",
	FOR: "FOR", WHILE: "WHILE", DO: "DO",
	VAR: "VAR", LET: "LET", CONST: "CONST",
	IMPORT: "IMPORT", EXPORT: "EXPORT", FROM: "FROM",
	DEFAULT: "DEFAULT", AS: "AS",
	CLASS: "CLASS", EXTENDS: "EXTENDS",
	NEW: "NEW", THIS: "THIS", SUPER: "SUPER",
	DELETE: "DELETE", VOID: "VOID", TYPEOF: "TYPEOF",
	INSTANCEOF: "INSTANCEOF", IN: "IN", OF: "OF",
	ASYNC: "ASYNC", AWAIT: "AWAIT", YIELD: "YIELD",
	TRY: "TRY", CATCH: "CATCH", FINALLY: "FINALLY",
	THROW: "THROW", SWITCH: "SWITCH", CASE: "CASE",
	BREAK: "BREAK", CONTINUE: "CONTINUE", STATIC: "static",
	NULL: "NULL", TRUE: "TRUE", FALSE: "FALSE", UNDEFINED: "UNDEFINED",
	PLUS: "+", MINUS: "-", STAR: "*", SLASH: "/", PERCENT: "%",
	EQ: "=", EQ_EQ: "==", EQ_EQ_EQ: "===",
	NOT_EQ: "!=", NOT_EQ_EQ: "!==",
	LT: "<", GT: ">", LTE: "<=", GTE: ">=",
	AND_AND: "&&", OR_OR: "||", NOT: "!", TILDE: "~",
	AMPERSAND: "&", PIPE: "|", CARET: "^",
	LT_LT: "<<", GT_GT: ">>", GT_GT_GT: ">>>",
	QUESTION: "?", QUESTION_DOT: "?.", COLON: ":",
	ARROW: "=>",
	PLUS_PLUS: "++", MINUS_MINUS: "--",
	PLUS_ASSIGN: "+=", MINUS_ASSIGN: "-=", STAR_ASSIGN: "*=",
	SLASH_ASSIGN: "/=", PERCENT_ASSIGN: "%=",
	AND_ASSIGN: "&=", OR_ASSIGN: "|=", CARET_ASSIGN: "^=",
	LT_LT_ASSIGN: "<<=", GT_GT_ASSIGN: ">>=",
	DOT: ".", COMMA: ",", SEMICOLON: ";", ELLIPSIS: "...",
	AT: "@", HASH: "#",
	LPAREN: "(", RPAREN: ")", LBRACKET: "[", RBRACKET: "]",
	LBRACE: "{", RBRACE: "}",
}

func (t TokenType) String() string {
	if s, ok := tokenNames[t]; ok {
		return s
	}
	return fmt.Sprintf("Token(%d)", t)
}

type Pos struct {
	Line int
	Col  int
}

type Token struct {
	Type    TokenType
	Literal string
	Pos     Pos
}
