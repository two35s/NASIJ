package ast

import (
	"fmt"
)

type Parser struct {
	tok    *Tokenizer
	cur    Token
	peek   Token
	errors []string
}

func Parse(input string) (*Program, []error) {
	p := &Parser{tok: NewTokenizer(input)}
	p.nextToken()
	p.nextToken()

	prog := p.parseProgram()

	var errs []error
	for _, e := range p.errors {
		errs = append(errs, fmt.Errorf("%s", e))
	}
	return prog, errs
}

func (p *Parser) nextToken() {
	// fmt.Printf("  nextToken: cur=%s(%q) peek=%s(%q)\n", p.cur.Type, p.cur.Literal, p.peek.Type, p.peek.Literal)
	p.cur = p.peek
	p.peek = p.tok.Next()
}

func (p *Parser) expect(tt TokenType) Token {
	if p.cur.Type == tt {
		tok := p.cur
		p.nextToken()
		return tok
	}
	p.error("expected %s, got %s (%q)", tt, p.cur.Type, p.cur.Literal)
	return Token{Type: tt, Literal: "", Pos: p.cur.Pos}
}

func (p *Parser) error(format string, args ...interface{}) {
	msg := fmt.Sprintf("line %d:%d: %s", p.cur.Pos.Line, p.cur.Pos.Col, fmt.Sprintf(format, args...))
	p.errors = append(p.errors, msg)
}

// ---------------------------------------------------------------------------
// Program
// ---------------------------------------------------------------------------

func (p *Parser) parseProgram() *Program {
	prog := &Program{StartPos: Pos{Line: 1, Col: 1}}
	for p.cur.Type != EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			prog.Body = append(prog.Body, stmt)
		}
	}
	return prog
}

// ---------------------------------------------------------------------------
// Statements
// ---------------------------------------------------------------------------

func (p *Parser) parseStatement() Statement {
	switch p.cur.Type {
	case FUNCTION:
		decl := p.parseFunctionDecl()
		if decl.Name == nil {
			p.error("function statement requires a name")
		}
		return decl
	case VAR, LET, CONST:
		return p.parseVarDecl()
	case RETURN:
		return p.parseReturn()
	case IF:
		return p.parseIf()
	case FOR:
		return p.parseFor()
	case WHILE:
		return p.parseWhile()
	case DO:
		return p.parseDoWhile()
	case BREAK:
		return p.parseBreak()
	case CONTINUE:
		return p.parseContinue()
	case THROW:
		return p.parseThrow()
	case TRY:
		return p.parseTry()
	case SWITCH:
		return p.parseSwitch()
	case IMPORT:
		if p.peek.Type == LPAREN {
			return p.parseExpressionStatement()
		}
		return p.parseImport()
	case EXPORT:
		return p.parseExport()
	case CLASS:
		return p.parseClassDecl()
	case LBRACE:
		return p.parseBlock()
	case SEMICOLON:
		p.nextToken()
		return nil
	case ASYNC:
		return p.parseAsyncStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseFunctionDecl() *FunctionDeclaration {
	pos := p.cur.Pos
	p.nextToken()
	gen := p.cur.Type == STAR
	if gen {
		p.nextToken()
	}
	var name *Identifier
	if p.cur.Type == IDENTIFIER {
		name = p.parseIdentifier()
	}
	p.expect(LPAREN)
	params := p.parseParams()
	p.expect(RPAREN)
	body := p.parseBlock()
	return &FunctionDeclaration{Name: name, Params: params, Body: body, Generator: gen, StartPos: pos}
}

func (p *Parser) parseAsyncStatement() Statement {
	pos := p.cur.Pos
	p.nextToken()

	if p.cur.Type == FUNCTION {
		p.nextToken()
		gen := p.cur.Type == STAR
		if gen {
			p.nextToken()
		}
	var name *Identifier
	if p.cur.Type == IDENTIFIER {
		name = p.parseIdentifier()
	} else {
		p.error("function statement requires a name")
	}
	p.expect(LPAREN)
		params := p.parseParams()
		p.expect(RPAREN)
		body := p.parseBlock()
		return &FunctionDeclaration{Name: name, Params: params, Body: body, Async: true, Generator: gen, StartPos: pos}
	}

	// async arrow function
	if p.cur.Type == LPAREN || p.cur.Type == IDENTIFIER {
		params := p.parseArrowParams()
		p.expect(ARROW)
		body := p.parseArrowBody()
		return &ExpressionStatement{
			Expr: &ArrowFunctionExpression{Params: params, Body: body, Async: true, StartPos: pos},
		}
	}

	p.error("unexpected token after async: %s", p.cur.Type)
	return nil
}

func (p *Parser) parseVarDecl() *VariableDeclaration {
	pos := p.cur.Pos
	kind := p.cur.Literal
	p.nextToken()
	var decls []VariableDeclarator
	for {
		decl := p.parseVarDeclarator()
		decls = append(decls, decl)
		if p.cur.Type == COMMA {
			p.nextToken()
			continue
		}
		break
	}
	p.optionalSemicolons()
	return &VariableDeclaration{Kind: kind, Decls: decls, StartPos: pos}
}

func (p *Parser) parseVarDeclarator() VariableDeclarator {
	pos := p.cur.Pos
	name := p.parseIdentifier()
	var init Expression
	if p.cur.Type == EQ {
		p.nextToken()
		init = p.parseExpression()
	}
	return VariableDeclarator{Name: name, Init: init, StartPos: pos}
}

func (p *Parser) parseReturn() *ReturnStatement {
	pos := p.cur.Pos
	p.nextToken()
	var arg Expression
	if !p.isStmtEnd() {
		arg = p.parseExpression()
	}
	p.optionalSemicolons()
	return &ReturnStatement{Arg: arg, StartPos: pos}
}

func (p *Parser) parseIf() *IfStatement {
	pos := p.cur.Pos
	p.nextToken()
	p.expect(LPAREN)
	test := p.parseExpression()
	p.expect(RPAREN)
	consequent := p.parseStatement()
	var alternate Statement
	if p.cur.Type == ELSE {
		p.nextToken()
		alternate = p.parseStatement()
	}
	return &IfStatement{Test: test, Consequent: consequent, Alternate: alternate, StartPos: pos}
}

func (p *Parser) parseFor() Statement {
	pos := p.cur.Pos
	p.nextToken()
	await := p.cur.Type == AWAIT
	if await {
		p.nextToken()
	}
	p.expect(LPAREN)

	// for (;;)
	if p.cur.Type == SEMICOLON {
		p.nextToken()
		test := p.parseMaybeExpr()
		p.expect(SEMICOLON)
		update := p.parseMaybeExpr()
		p.expect(RPAREN)
		body := p.parseStatement()
		return &ForStatement{Test: test, Update: update, Body: body, StartPos: pos}
	}

	init := p.parseForInit()

	if p.cur.Type == IN {
		p.nextToken()
		right := p.parseExpression()
		p.expect(RPAREN)
		body := p.parseStatement()
		return &ForInStatement{Left: init, Right: right, Body: body, StartPos: pos}
	}
	if p.cur.Type == OF {
		p.nextToken()
		right := p.parseExpression()
		p.expect(RPAREN)
		body := p.parseStatement()
		return &ForOfStatement{Left: init, Right: right, Body: body, Await: await, StartPos: pos}
	}

	if p.cur.Type == SEMICOLON {
		p.nextToken()
	}
	test := p.parseMaybeExpr()
	p.expect(SEMICOLON)
	update := p.parseMaybeExpr()
	p.expect(RPAREN)
	body := p.parseStatement()
	return &ForStatement{Init: init, Test: test, Update: update, Body: body, StartPos: pos}
}

func (p *Parser) parseForInit() Expression {
	switch p.cur.Type {
	case VAR, LET, CONST:
		return p.parseVarDecl()
	default:
		return p.parseMaybeExpr()
	}
}

func (p *Parser) parseWhile() *WhileStatement {
	pos := p.cur.Pos
	p.nextToken()
	p.expect(LPAREN)
	test := p.parseExpression()
	p.expect(RPAREN)
	body := p.parseStatement()
	return &WhileStatement{Test: test, Body: body, StartPos: pos}
}

func (p *Parser) parseDoWhile() *DoWhileStatement {
	pos := p.cur.Pos
	p.nextToken()
	body := p.parseStatement()
	p.expect(WHILE)
	p.expect(LPAREN)
	test := p.parseExpression()
	p.expect(RPAREN)
	p.optionalSemicolons()
	return &DoWhileStatement{Body: body, Test: test, StartPos: pos}
}

func (p *Parser) parseBreak() *BreakStatement {
	pos := p.cur.Pos
	p.nextToken()
	p.optionalSemicolons()
	return &BreakStatement{StartPos: pos}
}

func (p *Parser) parseContinue() *ContinueStatement {
	pos := p.cur.Pos
	p.nextToken()
	p.optionalSemicolons()
	return &ContinueStatement{StartPos: pos}
}

func (p *Parser) parseThrow() *ThrowStatement {
	pos := p.cur.Pos
	p.nextToken()
	arg := p.parseExpression()
	p.optionalSemicolons()
	return &ThrowStatement{Arg: arg, StartPos: pos}
}

func (p *Parser) parseTry() *TryStatement {
	pos := p.cur.Pos
	p.nextToken()
	block := p.parseBlock()
	var handler *CatchClause
	if p.cur.Type == CATCH {
		p.nextToken()
		if p.cur.Type == LPAREN {
			p.nextToken()
			p.parseIdentifier()
			p.expect(RPAREN)
		}
		body := p.parseBlock()
		handler = &CatchClause{Param: nil, Body: body, StartPos: pos}
	}
	var finalizer *BlockStatement
	if p.cur.Type == FINALLY {
		p.nextToken()
		finalizer = p.parseBlock()
	}
	return &TryStatement{Block: block, Handler: handler, Finalizer: finalizer, StartPos: pos}
}

func (p *Parser) parseSwitch() *SwitchStatement {
	pos := p.cur.Pos
	p.nextToken()
	p.expect(LPAREN)
	disc := p.parseExpression()
	p.expect(RPAREN)
	p.expect(LBRACE)
	var cases []*SwitchCase
	for p.cur.Type != RBRACE && p.cur.Type != EOF {
		sc := p.parseSwitchCase()
		cases = append(cases, sc)
	}
	p.expect(RBRACE)
	return &SwitchStatement{Discriminant: disc, Cases: cases, StartPos: pos}
}

func (p *Parser) parseSwitchCase() *SwitchCase {
	pos := p.cur.Pos
	var test Expression
	if p.cur.Type == CASE {
		p.nextToken()
		test = p.parseExpression()
		p.expect(COLON)
	} else if p.cur.Type == DEFAULT {
		p.nextToken()
		p.expect(COLON)
	} else {
		p.error("expected case or default")
		return &SwitchCase{StartPos: pos}
	}
	var body []Statement
	for p.cur.Type != CASE && p.cur.Type != DEFAULT && p.cur.Type != RBRACE && p.cur.Type != EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}
	return &SwitchCase{Test: test, Body: body, StartPos: pos}
}

func (p *Parser) parseBlock() *BlockStatement {
	pos := p.cur.Pos
	p.expect(LBRACE)
	var stmts []Statement
	for p.cur.Type != RBRACE && p.cur.Type != EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	p.expect(RBRACE)
	return &BlockStatement{Body: stmts, StartPos: pos}
}

// ---------------------------------------------------------------------------
// Imports / Exports
// ---------------------------------------------------------------------------

func (p *Parser) parseImport() *ImportDeclaration {
	pos := p.cur.Pos
	p.nextToken()
	decl := &ImportDeclaration{StartPos: pos}

	if p.cur.Type == STRING {
		decl.Source = p.parseLiteral()
		p.optionalSemicolons()
		return decl
	}

	if p.cur.Type == LBRACE {
		p.nextToken()
		decl.Specifiers = p.parseImportSpecifiers()
		p.expect(RBRACE)
	} else if p.cur.Type == STAR {
		p.nextToken()
		p.expect(AS)
		decl.Specifiers = append(decl.Specifiers, ImportSpecifier{Local: p.parseIdentifier()})
	} else {
		spec := ImportSpecifier{Local: p.parseIdentifier(), Default: true}
		decl.Specifiers = append(decl.Specifiers, spec)
		if p.cur.Type == COMMA {
			p.nextToken()
			if p.cur.Type == LBRACE {
				p.nextToken()
				named := p.parseImportSpecifiers()
				p.expect(RBRACE)
				decl.Specifiers = append(decl.Specifiers, named...)
			} else if p.cur.Type == STAR {
				p.nextToken()
				p.expect(AS)
				decl.Specifiers = append(decl.Specifiers, ImportSpecifier{Local: p.parseIdentifier()})
			}
		}
	}

	p.expect(FROM)
	decl.Source = p.parseLiteral()
	p.optionalSemicolons()
	return decl
}

func (p *Parser) parseImportSpecifiers() []ImportSpecifier {
	var specs []ImportSpecifier
	for p.cur.Type != RBRACE && p.cur.Type != EOF {
		if p.cur.Type == COMMA {
			p.nextToken()
			continue
		}
		spec := ImportSpecifier{}
		if p.cur.Type == IDENTIFIER {
			spec.Local = p.parseIdentifier()
			if p.cur.Type == AS {
				p.nextToken()
				spec.Imported = spec.Local
				spec.Local = p.parseIdentifier()
			}
		}
		specs = append(specs, spec)
	}
	return specs
}

func (p *Parser) parseExport() *ExportDeclaration {
	pos := p.cur.Pos
	p.nextToken()
	exp := &ExportDeclaration{StartPos: pos}

	if p.cur.Type == DEFAULT {
		exp.Default = true
		p.nextToken()
		if p.cur.Type == FUNCTION {
			exp.Declaration = p.parseFunctionDecl()
		} else if p.cur.Type == CLASS {
			exp.Declaration = p.parseClassDecl()
		} else {
			exp.Declaration = p.parseStatement()
		}
		return exp
	}
	if p.cur.Type == STAR {
		exp.All = true
		p.nextToken()
		p.expect(FROM)
		exp.Source = p.parseLiteral()
		p.optionalSemicolons()
		return exp
	}
	if p.cur.Type == LBRACE {
		p.nextToken()
		for p.cur.Type != RBRACE && p.cur.Type != EOF {
			if p.cur.Type == COMMA {
				p.nextToken()
				continue
			}
			spec := ExportSpecifier{Local: p.parseIdentifier()}
			if p.cur.Type == AS {
				p.nextToken()
				spec.Exported = p.parseIdentifier()
			}
			exp.Specifiers = append(exp.Specifiers, spec)
		}
		p.expect(RBRACE)
		if p.cur.Type == FROM {
			p.nextToken()
			exp.Source = p.parseLiteral()
		}
		p.optionalSemicolons()
		return exp
	}

	exp.Declaration = p.parseStatement()
	return exp
}

// ---------------------------------------------------------------------------
// Classes
// ---------------------------------------------------------------------------

func (p *Parser) parseClassDecl() *ClassDeclaration {
	pos := p.cur.Pos
	p.nextToken()
	name := p.parseIdentifier()
	var superClass Expression
	if p.cur.Type == EXTENDS {
		p.nextToken()
		superClass = p.parseLeftHandSideExpr()
	}
	body := p.parseClassBody()
	return &ClassDeclaration{Name: name, SuperClass: superClass, Body: body, StartPos: pos}
}

func (p *Parser) parseClassBody() *ClassBody {
	pos := p.cur.Pos
	p.expect(LBRACE)
	var methods []*ClassMethod
	for p.cur.Type != RBRACE && p.cur.Type != EOF {
		m := p.parseClassMethod()
		methods = append(methods, m)
	}
	p.expect(RBRACE)
	return &ClassBody{Methods: methods, StartPos: pos}
}

func (p *Parser) parseClassMethod() *ClassMethod {
	pos := p.cur.Pos
	static := false
	async := false
	gen := false

	if p.cur.Type == STATIC {
		static = true
		p.nextToken()
	}
	if p.cur.Type == ASYNC {
		async = true
		p.nextToken()
	}
	if p.cur.Type == STAR {
		gen = true
		p.nextToken()
	}

	name := p.parsePropertyName()
	p.expect(LPAREN)
	params := p.parseParams()
	p.expect(RPAREN)
	body := p.parseBlock()
	return &ClassMethod{Name: name, Params: params, Body: body, Static: static, Async: async, Generator: gen, StartPos: pos}
}

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	pos := p.cur.Pos
	expr := p.parseExpression()
	if p.cur.Type == COMMA {
		list := []Expression{expr}
		for p.cur.Type == COMMA {
			p.nextToken()
			list = append(list, p.parseExpression())
		}
		expr = &SequenceExpression{Expressions: list, StartPos: pos}
	}
	p.optionalSemicolons()
	return &ExpressionStatement{Expr: expr, StartPos: pos}
}

// Precedence table
var precMap = map[TokenType]int{
	COMMA:              1,
	EQ: 2, PLUS_ASSIGN: 2, MINUS_ASSIGN: 2, STAR_ASSIGN: 2,
	SLASH_ASSIGN: 2, PERCENT_ASSIGN: 2, AND_ASSIGN: 2, OR_ASSIGN: 2,
	CARET_ASSIGN: 2, LT_LT_ASSIGN: 2, GT_GT_ASSIGN: 2,
	QUESTION: 3,
	OR_OR:   4,
	AND_AND: 5,
	PIPE:    6,
	CARET:   7,
	AMPERSAND: 8,
	EQ_EQ: 9, NOT_EQ: 9, EQ_EQ_EQ: 10, NOT_EQ_EQ: 10,
	LT: 11, GT: 11, LTE: 11, GTE: 11, IN: 11, INSTANCEOF: 11,
	LT_LT: 12, GT_GT: 12, GT_GT_GT: 12,
	PLUS: 13, MINUS: 13,
	STAR: 14, SLASH: 14, PERCENT: 14,
}

func precOf(tt TokenType) int {
	if p, ok := precMap[tt]; ok {
		return p
	}
	return 0
}

func (p *Parser) parseExpression() Expression {
	return p.parseMaybeAssignment(1)
}

func (p *Parser) parseMaybeExpr() Expression {
	if p.cur.Type == SEMICOLON || p.cur.Type == RPAREN || p.cur.Type == RBRACE || p.cur.Type == EOF {
		return nil
	}
	if p.cur.Type == COMMA {
		return nil
	}
	return p.parseExpression()
}

func (p *Parser) parseMaybeAssignment(prec int) Expression {
	left := p.parseArrowFunction()

	switch p.cur.Type {
	case EQ, PLUS_ASSIGN, MINUS_ASSIGN, STAR_ASSIGN, SLASH_ASSIGN,
		PERCENT_ASSIGN, AND_ASSIGN, OR_ASSIGN, CARET_ASSIGN,
		LT_LT_ASSIGN, GT_GT_ASSIGN:
		op := p.cur.Literal
		p.nextToken()
		right := p.parseMaybeAssignment(precOf(EQ))
		left = &AssignmentExpression{Left: left, Op: op, Right: right, StartPos: left.Pos()}
	}

	return left
}

func (p *Parser) parseArrowFunction() Expression {
	if p.cur.Type == LPAREN {
		startPos := p.cur.Pos
		savedTokens := p.drainTokens()
		if p.tryArrowAfterParen() {
			// It's an arrow function
			p.nextToken() // eat LPAREN
			params := p.parseParams()
			if p.cur.Type != RPAREN {
				return p.parseTernaryFrom(savedTokens)
			}
			p.expect(RPAREN)
			p.expect(ARROW)
			body := p.parseArrowBody()
			return &ArrowFunctionExpression{
				Params:   params,
				Body:     body,
				StartPos: startPos,
			}
		}
		return p.parseTernaryFrom(savedTokens)
	}

	if p.cur.Type == IDENTIFIER && p.peek.Type == ARROW {
		pos := p.cur.Pos
		param := p.parseIdentifier()
		p.nextToken() // ARROW
		body := p.parseArrowBody()
		return &ArrowFunctionExpression{
			Params:   []*Identifier{param},
			Body:     body,
			StartPos: pos,
		}
	}

	return p.parseTernary()
}

func (p *Parser) drainTokens() []Token {
	return nil // simplified — no backtracking needed
}

func (p *Parser) tryArrowAfterParen() bool {
	// Check if content between parens looks like arrow parameters.
	// We only support simple cases: () => or (ident) => or (ident, ident) =>.
	// For anything complex, assume grouped expression.
	// We must properly restore tokenizer state after scanning.
	state := p.tok.snap()
	savedCur := p.cur
	savedPeek := p.peek

	p.nextToken()
	if p.cur.Type == RPAREN && p.peek.Type == ARROW {
		p.cur = savedCur
		p.peek = savedPeek
		p.tok.restore(state)
		return true
	}

	if p.cur.Type == IDENTIFIER {
		state2 := p.tok.snap()
		p.nextToken()
		if p.cur.Type == RPAREN && p.peek.Type == ARROW {
			p.cur = savedCur
			p.peek = savedPeek
			p.tok.restore(state)
			return true
		}
		if p.cur.Type == COMMA {
			// Could be multi-param: (a, b) => ... 
			// Simplified: scan remaining
			for {
				p.nextToken()
				if p.cur.Type == EOF || p.cur.Type == RBRACE {
					break
				}
				if p.cur.Type == RPAREN && p.peek.Type == ARROW {
					p.cur = savedCur
					p.peek = savedPeek
					p.tok.restore(state)
					return true
				}
			}
		}
		p.tok.restore(state2)
	}

	p.cur = savedCur
	p.peek = savedPeek
	p.tok.restore(state)
	return false
}

func (p *Parser) parseArrowParams() []*Identifier {
	if p.cur.Type == LPAREN {
		p.nextToken()
		params := p.parseParams()
		p.expect(RPAREN)
		return params
	}
	if p.cur.Type == IDENTIFIER {
		return []*Identifier{p.parseIdentifier()}
	}
	return nil
}

func (p *Parser) parseArrowBody() Statement {
	if p.cur.Type == LBRACE {
		return p.parseBlock()
	}
	expr := p.parseExpression()
	return &ExpressionStatement{Expr: expr}
}

func (p *Parser) parseTernaryFrom(saved []Token) Expression {
	// Continue parsing expression starting from saved tokens
	return p.parseTernary()
}

func (p *Parser) parseTernary() Expression {
	left := p.parseBinary(1)

	if p.cur.Type == QUESTION {
		p.nextToken()
		consequent := p.parseExpression()
		p.expect(COLON)
		alternate := p.parseExpression()
		return &ConditionalExpression{
			Test:       left,
			Consequent: consequent,
			Alternate:  alternate,
			StartPos:   left.Pos(),
		}
	}

	return left
}

func (p *Parser) parseBinary(prec int) Expression {
	left := p.parseUpdateExpr()

	for {
		tt := p.cur.Type
		tp := precOf(tt)
		if tp == 0 || tp <= prec {
			break
		}
		if tt == QUESTION {
			break
		}

		op := p.cur.Literal
		p.nextToken()
		right := p.parseBinary(tp)
		left = &BinaryExpression{Left: left, Op: op, Right: right, StartPos: left.Pos()}
	}

	return left
}

func (p *Parser) parseUnary() Expression {
	switch p.cur.Type {
	case NOT, TILDE, MINUS, PLUS, DELETE, VOID, TYPEOF, AWAIT:
		op := p.cur.Literal
		p.nextToken()
		arg := p.parseUnary()
		return &UnaryExpression{Op: op, Arg: arg, Prefix: true, StartPos: arg.Pos()}
	}

	return p.parseLeftHandSideExpr()
}

func (p *Parser) parseUpdateExpr() Expression {
	if p.cur.Type == PLUS_PLUS || p.cur.Type == MINUS_MINUS {
		op := p.cur.Literal
		p.nextToken()
		arg := p.parseUpdateExpr()
		return &UpdateExpression{Op: op, Arg: arg, Prefix: true, StartPos: arg.Pos()}
	}

	left := p.parseUnary()

	if p.cur.Type == PLUS_PLUS || p.cur.Type == MINUS_MINUS {
		op := p.cur.Literal
		p.nextToken()
		return &UpdateExpression{Op: op, Arg: left, Prefix: false, StartPos: left.Pos()}
	}

	return left
}

func (p *Parser) parseLeftHandSideExpr() Expression {
	left := p.parsePrimary()

	for {
		switch p.cur.Type {
		case DOT:
			p.nextToken()
			prop := p.parseIdentifier()
			left = &MemberExpression{
				Object: left, Property: prop, Computed: false, StartPos: left.Pos(),
			}
		case QUESTION_DOT:
			p.nextToken()
			if p.cur.Type == LBRACKET {
				p.nextToken()
				prop := p.parseExpression()
				p.expect(RBRACKET)
				left = &MemberExpression{
					Object: left, Property: prop, Computed: true, Optional: true, StartPos: left.Pos(),
				}
			} else if p.cur.Type == LPAREN {
				args := p.parseCallArgs()
				left = &CallExpression{Callee: left, Arguments: args, Optional: true, StartPos: left.Pos()}
			} else {
				prop := p.parseIdentifier()
				left = &MemberExpression{
					Object: left, Property: prop, Computed: false, Optional: true, StartPos: left.Pos(),
				}
			}
		case LBRACKET:
			p.nextToken()
			prop := p.parseExpression()
			p.expect(RBRACKET)
			left = &MemberExpression{
				Object: left, Property: prop, Computed: true, StartPos: left.Pos(),
			}
		case LPAREN:
			args := p.parseCallArgs()
			left = &CallExpression{Callee: left, Arguments: args, StartPos: left.Pos()}
		case TEMPLATE_HEAD, TEMPLATE_LITERAL:
			tl := p.parseTemplateLiteral()
			left = &TaggedTemplateExpression{Tag: left, Quasi: tl, StartPos: left.Pos()}
		default:
			return left
		}
	}
}

func (p *Parser) parseCallArgs() []Expression {
	p.nextToken()
	var args []Expression
	for p.cur.Type != RPAREN && p.cur.Type != EOF {
		if p.cur.Type == ELLIPSIS {
			pos := p.cur.Pos
			p.nextToken()
			args = append(args, &SpreadElement{Arg: p.parseExpression(), StartPos: pos})
		} else if p.cur.Type == COMMA {
			p.nextToken()
		} else {
			args = append(args, p.parseExpression())
			if p.cur.Type == COMMA {
				p.nextToken()
			}
		}
	}
	p.expect(RPAREN)
	return args
}

func (p *Parser) parsePrimary() Expression {
	switch p.cur.Type {
	case IDENTIFIER:
		return p.parseIdentifier()
	case NUMBER, STRING:
		return p.parseLiteral()
	case NULL:
		return p.parseLiteral()
	case TRUE, FALSE:
		return p.parseLiteral()
	case UNDEFINED:
		return p.parseIdentifier()
	case THIS:
		return p.parseThis()
	case NEW:
		return p.parseNew()
	case LBRACKET:
		return p.parseArray()
	case LBRACE:
		return p.parseObject()
	case LPAREN:
		return p.parseGrouped()
	case FUNCTION:
		return p.parseFuncExpr()
	case CLASS:
		return p.parseClassExpr()
	case TEMPLATE_LITERAL, TEMPLATE_HEAD:
		return p.parseTemplateLiteral()
	case IMPORT:
		return p.parseDynamicImport()
	case ASYNC:
		return p.parseAsyncExpr()

	default:
		p.error("unexpected token %s (%q)", p.cur.Type, p.cur.Literal)
		p.nextToken()
		return nil
	}
}

func (p *Parser) parseIdentifier() *Identifier {
	pos := p.cur.Pos
	lit := p.cur.Literal
	if p.cur.Type == IDENTIFIER || p.cur.Type == UNDEFINED || p.cur.Type == ASYNC || p.cur.Type == OF || p.cur.Type == STATIC {
		p.nextToken()
		return &Identifier{Name: lit, StartPos: pos}
	}
	p.error("expected identifier, got %s (%q)", p.cur.Type, p.cur.Literal)
	return &Identifier{Name: "<error>", StartPos: pos}
}

func (p *Parser) parseLiteral() *Literal {
	pos := p.cur.Pos
	switch p.cur.Type {
	case STRING:
		lit := &Literal{Kind: "string", Value: p.cur.Literal, Raw: "'" + p.cur.Literal + "'", StartPos: pos}
		p.nextToken()
		return lit
	case NUMBER:
		lit := &Literal{Kind: "number", Value: p.cur.Literal, Raw: p.cur.Literal, StartPos: pos}
		p.nextToken()
		return lit
	case NULL:
		p.nextToken()
		return &Literal{Kind: "null", Value: "null", Raw: "null", StartPos: pos}
	case TRUE:
		p.nextToken()
		return &Literal{Kind: "boolean", Value: "true", Raw: "true", StartPos: pos}
	case FALSE:
		p.nextToken()
		return &Literal{Kind: "boolean", Value: "false", Raw: "false", StartPos: pos}
	default:
		p.error("expected literal, got %s", p.cur.Type)
		return &Literal{Kind: "string", Value: "", Raw: "", StartPos: pos}
	}
}

func (p *Parser) parseThis() *ThisExpression {
	pos := p.cur.Pos
	p.nextToken()
	return &ThisExpression{StartPos: pos}
}

func (p *Parser) parseNew() *NewExpression {
	pos := p.cur.Pos
	p.nextToken()
	callee := p.parseLeftHandSideExpr()
	var args []Expression
	if p.cur.Type == LPAREN {
		args = p.parseCallArgs()
	}
	return &NewExpression{Callee: callee, Arguments: args, StartPos: pos}
}

func (p *Parser) parseGrouped() Expression {
	p.nextToken()
	expr := p.parseExpression()
	if p.cur.Type == COMMA {
		list := []Expression{expr}
		for p.cur.Type == COMMA {
			p.nextToken()
			list = append(list, p.parseExpression())
		}
		expr = &SequenceExpression{Expressions: list}
	}
	p.expect(RPAREN)
	return expr
}

func (p *Parser) parseFuncExpr() Expression {
	pos := p.cur.Pos
	p.nextToken()
	gen := p.cur.Type == STAR
	if gen {
		p.nextToken()
	}
	var name *Identifier
	if p.cur.Type == IDENTIFIER {
		name = p.parseIdentifier()
	}
	p.expect(LPAREN)
	params := p.parseParams()
	p.expect(RPAREN)
	body := p.parseBlock()
	_ = name
	_ = gen
	return &ArrowFunctionExpression{Params: params, Body: body, StartPos: pos}
}

func (p *Parser) parseClassExpr() Expression {
	pos := p.cur.Pos
	p.nextToken()
	name := p.parseIdentifier()
	var superClass Expression
	if p.cur.Type == EXTENDS {
		p.nextToken()
		superClass = p.parseLeftHandSideExpr()
	}
	body := p.parseClassBody()
	return &ClassDeclaration{Name: name, SuperClass: superClass, Body: body, StartPos: pos}
}

func (p *Parser) parseArray() *ArrayExpression {
	pos := p.cur.Pos
	p.nextToken()
	var elements []Expression
	for p.cur.Type != RBRACKET && p.cur.Type != EOF {
		if p.cur.Type == COMMA {
			p.nextToken()
			continue
		}
		if p.cur.Type == ELLIPSIS {
			pos2 := p.cur.Pos
			p.nextToken()
			elements = append(elements, &SpreadElement{Arg: p.parseExpression(), StartPos: pos2})
		} else {
			elements = append(elements, p.parseExpression())
		}
		if p.cur.Type == COMMA {
			p.nextToken()
		}
	}
	p.expect(RBRACKET)
	return &ArrayExpression{Elements: elements, StartPos: pos}
}

func (p *Parser) parseObject() *ObjectExpression {
	pos := p.cur.Pos
	p.nextToken()
	var props []*ObjectProperty
	for p.cur.Type != RBRACE && p.cur.Type != EOF {
		if p.cur.Type == COMMA {
			p.nextToken()
			continue
		}
		prop := p.parseObjectProperty()
		if prop != nil {
			props = append(props, prop)
		}
		if p.cur.Type == COMMA {
			p.nextToken()
		}
	}
	p.expect(RBRACE)
	return &ObjectExpression{Properties: props, StartPos: pos}
}

func (p *Parser) parseObjectProperty() *ObjectProperty {
	pos := p.cur.Pos

	if p.cur.Type == ELLIPSIS {
		p.nextToken()
		return &ObjectProperty{Value: p.parseExpression(), Spread: true, StartPos: pos}
	}

	if p.cur.Type == ASYNC {
		saved := p.cur
		p.nextToken()
		if p.cur.Type == IDENTIFIER {
			name := p.parseIdentifier()
			if p.cur.Type == LPAREN {
				p.nextToken()
				params := p.parseParams()
				p.expect(RPAREN)
				body := p.parseBlock()
				return &ObjectProperty{
					Key: name,
					Value: &ArrowFunctionExpression{Params: params, Body: body, Async: true, StartPos: pos},
					StartPos: pos,
				}
			}
		}
		_ = saved
	}

	if p.cur.Type == STAR {
		p.nextToken()
	}

	key := p.parsePropertyName()

	if p.cur.Type == COMMA || p.cur.Type == RBRACE {
		return &ObjectProperty{Key: key, Value: key, Shorthand: true, StartPos: pos}
	}

	if p.cur.Type == LPAREN {
		p.nextToken()
		params := p.parseParams()
		p.expect(RPAREN)
		body := p.parseBlock()
		return &ObjectProperty{
			Key: key,
			Value: &ArrowFunctionExpression{Params: params, Body: body, StartPos: pos},
			StartPos: pos,
		}
	}

	if p.cur.Type == COLON {
		p.nextToken()
		val := p.parseExpression()
		return &ObjectProperty{Key: key, Value: val, StartPos: pos}
	}

	return nil
}

func (p *Parser) parsePropertyName() Expression {
	switch p.cur.Type {
	case STRING:
		return p.parseLiteral()
	case NUMBER:
		return p.parseLiteral()
	case LBRACKET:
		p.nextToken()
		expr := p.parseExpression()
		p.expect(RBRACKET)
		return expr
	default:
		return p.parseIdentifier()
	}
}

func (p *Parser) parseTemplateLiteral() *TemplateLiteral {
	pos := p.cur.Pos
	tl := &TemplateLiteral{StartPos: pos}

	if p.cur.Type == TEMPLATE_LITERAL {
		tl.Parts = append(tl.Parts, TemplatePart{Value: p.cur.Literal})
		p.nextToken()
		return tl
	}

	for p.cur.Type == TEMPLATE_HEAD || p.cur.Type == TEMPLATE_MIDDLE {
		tl.Parts = append(tl.Parts, TemplatePart{Value: p.cur.Literal})
		p.nextToken()

		expr := p.parseExpression()
		tl.Parts = append(tl.Parts, TemplatePart{Expr: expr})

		if p.cur.Type != RBRACE {
			p.error("expected } after template expression, got %s", p.cur.Type)
		}
		mid := p.tok.ReadTemplateMiddle()
		p.cur = mid
		p.peek = p.tok.Next()
	}

	if p.cur.Type == TEMPLATE_TAIL {
		tl.Parts = append(tl.Parts, TemplatePart{Value: p.cur.Literal})
		p.nextToken()
	}

	return tl
}

func (p *Parser) parseDynamicImport() Expression {
	pos := p.cur.Pos
	p.nextToken() // eat import
	p.expect(LPAREN)
	arg := p.parseExpression()
	p.expect(RPAREN)
	return &CallExpression{
		Callee:    &Identifier{Name: "import", StartPos: pos},
		Arguments: []Expression{arg},
		StartPos:  pos,
	}
}

func (p *Parser) parseAsyncExpr() Expression {
	pos := p.cur.Pos
	p.nextToken()
	if p.cur.Type == FUNCTION {
		return p.parseFuncExpr()
	}
	if p.cur.Type == LPAREN || p.cur.Type == IDENTIFIER {
		params := p.parseArrowParams()
		p.expect(ARROW)
		body := p.parseArrowBody()
		return &ArrowFunctionExpression{Params: params, Body: body, Async: true, StartPos: pos}
	}
	p.error("unexpected token after async: %s", p.cur.Type)
	return nil
}

// ---------------------------------------------------------------------------
// Parameters
// ---------------------------------------------------------------------------

func (p *Parser) parseParams() []*Identifier {
	var params []*Identifier
	for p.cur.Type != RPAREN && p.cur.Type != EOF {
		if p.cur.Type == COMMA {
			p.nextToken()
			continue
		}
		if p.cur.Type == ELLIPSIS {
			p.nextToken()
		}
		if p.cur.Type == LBRACE || p.cur.Type == LBRACKET {
			p.skipBalanced()
			if p.cur.Type == EQ {
				p.nextToken()
				p.parseExpression()
			}
		} else if p.cur.Type == IDENTIFIER {
			param := p.parseIdentifier()
			params = append(params, param)
			if p.cur.Type == EQ {
				p.nextToken()
				p.parseExpression()
			}
		} else {
			break
		}
		if p.cur.Type == COMMA {
			p.nextToken()
		}
	}
	return params
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *Parser) optionalSemicolons() {
	for p.cur.Type == SEMICOLON {
		p.nextToken()
	}
}

func (p *Parser) isStmtEnd() bool {
	return p.cur.Type == SEMICOLON || p.cur.Type == RBRACE || p.cur.Type == EOF
}

func (p *Parser) skipBalanced() {
	depth := 1
	open := p.cur.Type
	var close TokenType
	switch open {
	case LBRACE:
		close = RBRACE
	case LBRACKET:
		close = RBRACKET
	case LPAREN:
		close = RPAREN
	default:
		return
	}
	p.nextToken()
	for depth > 0 && p.cur.Type != EOF {
		if p.cur.Type == open {
			depth++
		} else if p.cur.Type == close {
			depth--
		}
		p.nextToken()
	}
}
