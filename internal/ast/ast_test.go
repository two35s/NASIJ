package ast

import (
	"testing"
)

func TestTokenizeIdentifiers(t *testing.T) {
	tokens := TokenizeAll("foo bar _hello $test")
	if len(tokens) != 5 {
		t.Fatalf("expected 5 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != IDENTIFIER || tokens[0].Literal != "foo" {
		t.Errorf("expected IDENTIFIER 'foo', got %s %q", tokens[0].Type, tokens[0].Literal)
	}
	if tokens[1].Type != IDENTIFIER || tokens[1].Literal != "bar" {
		t.Errorf("expected IDENTIFIER 'bar', got %s %q", tokens[1].Type, tokens[1].Literal)
	}
	if tokens[2].Type != IDENTIFIER || tokens[2].Literal != "_hello" {
		t.Errorf("expected IDENTIFIER '_hello', got %s %q", tokens[2].Type, tokens[2].Literal)
	}
	if tokens[3].Type != IDENTIFIER || tokens[3].Literal != "$test" {
		t.Errorf("expected IDENTIFIER '$test', got %s %q", tokens[3].Type, tokens[3].Literal)
	}
	if tokens[4].Type != EOF {
		t.Errorf("expected EOF, got %s", tokens[4].Type)
	}
}

func TestTokenizeKeywords(t *testing.T) {
	tokens := TokenizeAll("function return if else for while var let const class")
	expected := []TokenType{FUNCTION, RETURN, IF, ELSE, FOR, WHILE, VAR, LET, CONST, CLASS}
	if len(tokens) != len(expected)+1 {
		t.Fatalf("expected %d tokens, got %d", len(expected)+1, len(tokens))
	}
	for i, exp := range expected {
		if tokens[i].Type != exp {
			t.Errorf("token %d: expected %s, got %s", i, exp, tokens[i].Type)
		}
	}
}

func TestTokenizeNumbers(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{"42", "42"},
		{"3.14", "3.14"},
		{"0xFF", "255"},
		{"0b1010", "10"},
		{"0o77", "63"},
	}
	for _, tc := range tests {
		tokens := TokenizeAll(tc.input)
		if len(tokens) < 2 {
			t.Fatalf("%s: expected 2+ tokens, got %d", tc.input, len(tokens))
		}
		if tokens[0].Type != NUMBER || tokens[0].Literal != tc.value {
			t.Errorf("%s: expected NUMBER %q, got %s %q", tc.input, tc.value, tokens[0].Type, tokens[0].Literal)
		}
	}
}

func TestTokenizeStrings(t *testing.T) {
	tokens := TokenizeAll(`"hello" 'world'`)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0].Literal != "hello" {
		t.Errorf("expected 'hello', got %q", tokens[0].Literal)
	}
	if tokens[1].Literal != "world" {
		t.Errorf("expected 'world', got %q", tokens[1].Literal)
	}
}

func TestTokenizeOperators(t *testing.T) {
	tokens := TokenizeAll("+ - * / == === != !== <= >= && || => ...")
	if len(tokens) != 15 {
		t.Fatalf("expected 15 tokens (14 operators + EOF), got %d", len(tokens))
	}
	expected := []TokenType{PLUS, MINUS, STAR, SLASH, EQ_EQ, EQ_EQ_EQ, NOT_EQ, NOT_EQ_EQ, LTE, GTE, AND_AND, OR_OR, ARROW, ELLIPSIS}
	for i, exp := range expected {
		if tokens[i].Type != exp {
			t.Errorf("token %d: expected %s, got %s", i, exp, tokens[i].Type)
		}
	}
}

func TestTokenizeComments(t *testing.T) {
	tokens := TokenizeAll("// comment\n42 /* block */ foo")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens (NUMBER, IDENTIFIER, EOF), got %d", len(tokens))
	}
	if tokens[0].Type != NUMBER || tokens[0].Literal != "42" {
		t.Errorf("expected NUMBER 42, got %s %q", tokens[0].Type, tokens[0].Literal)
	}
	if tokens[1].Type != IDENTIFIER || tokens[1].Literal != "foo" {
		t.Errorf("expected IDENTIFIER 'foo', got %s %q", tokens[1].Type, tokens[1].Literal)
	}
}

func TestTokenizeTemplate(t *testing.T) {
	tokens := TokenizeAll("`hello`")
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TEMPLATE_LITERAL {
		t.Errorf("expected TEMPLATE_LITERAL, got %s", tokens[0].Type)
	}
}

func TestTokenizeTemplateHead(t *testing.T) {
	tokens := TokenizeAll("`hello${42}`")
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TEMPLATE_HEAD || tokens[1].Type != NUMBER || tokens[2].Type != RBRACE {
		t.Errorf("expected TEMPLATE_HEAD, NUMBER, RBRACE, got %s %s %s", tokens[0].Type, tokens[1].Type, tokens[2].Type)
	}
}

// --- Parser tests ---

func testParse(t *testing.T, input string) *Program {
	prog, errs := Parse(input)
	if len(errs) > 0 {
		t.Helper()
		for _, e := range errs {
			t.Errorf("parse error: %v", e)
		}
	}
	return prog
}

func TestParseEmpty(t *testing.T) {
	prog := testParse(t, "")
	if len(prog.Body) != 0 {
		t.Errorf("expected empty body, got %d statements", len(prog.Body))
	}
}

func TestParseFunctionDecl(t *testing.T) {
	prog := testParse(t, "function foo(x, y) { return x + y; }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	fn, ok := prog.Body[0].(*FunctionDeclaration)
	if !ok {
		t.Fatalf("expected *FunctionDeclaration, got %T", prog.Body[0])
	}
	if fn.Name == nil || fn.Name.Name != "foo" {
		t.Errorf("expected name 'foo', got %v", fn.Name)
	}
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Name != "x" || fn.Params[1].Name != "y" {
		t.Errorf("expected params [x y], got %v", fn.Params)
	}
}

func TestParseAsyncFunction(t *testing.T) {
	prog := testParse(t, "async function fetchData() { return await api.get(); }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	fn, ok := prog.Body[0].(*FunctionDeclaration)
	if !ok {
		t.Fatalf("expected *FunctionDeclaration, got %T", prog.Body[0])
	}
	if !fn.Async {
		t.Errorf("expected async=true")
	}
}

func TestParseVarDecl(t *testing.T) {
	prog := testParse(t, "var x = 1; let y; const z = 3;")
	if len(prog.Body) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(prog.Body))
	}
	v1, ok := prog.Body[0].(*VariableDeclaration)
	if !ok || v1.Kind != "var" || len(v1.Decls) != 1 {
		t.Errorf("expected var decl, got %T kind=%s decls=%d", prog.Body[0], v1.Kind, len(v1.Decls))
	}
	v2, ok := prog.Body[1].(*VariableDeclaration)
	if !ok || v2.Kind != "let" {
		t.Errorf("expected let decl, got %T kind=%s", prog.Body[1], v2.Kind)
	}
	v3, ok := prog.Body[2].(*VariableDeclaration)
	if !ok || v3.Kind != "const" {
		t.Errorf("expected const decl, got %T kind=%s", prog.Body[2], v3.Kind)
	}
}

func TestParseIfStatement(t *testing.T) {
	prog := testParse(t, "if (a) { b; } else { c; }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	ifstmt, ok := prog.Body[0].(*IfStatement)
	if !ok {
		t.Fatalf("expected *IfStatement, got %T", prog.Body[0])
	}
	if ifstmt.Alternate == nil {
		t.Errorf("expected alternate (else) to be non-nil")
	}
}

func TestParseForStatement(t *testing.T) {
	prog := testParse(t, "for (let i = 0; i < 10; i++) { body(); }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	_, ok := prog.Body[0].(*ForStatement)
	if !ok {
		t.Fatalf("expected *ForStatement, got %T", prog.Body[0])
	}
}

func TestParseForIn(t *testing.T) {
	prog := testParse(t, "for (let x in obj) { process(x); }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	_, ok := prog.Body[0].(*ForInStatement)
	if !ok {
		t.Fatalf("expected *ForInStatement, got %T", prog.Body[0])
	}
}

func TestParseForOf(t *testing.T) {
	prog := testParse(t, "for (let x of items) { process(x); }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	_, ok := prog.Body[0].(*ForOfStatement)
	if !ok {
		t.Fatalf("expected *ForOfStatement, got %T", prog.Body[0])
	}
}

func TestParseWhile(t *testing.T) {
	prog := testParse(t, "while (true) { break; }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	_, ok := prog.Body[0].(*WhileStatement)
	if !ok {
		t.Fatalf("expected *WhileStatement, got %T", prog.Body[0])
	}
}

func TestParseDoWhile(t *testing.T) {
	prog := testParse(t, "do { x++; } while (x < 10);")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	_, ok := prog.Body[0].(*DoWhileStatement)
	if !ok {
		t.Fatalf("expected *DoWhileStatement, got %T", prog.Body[0])
	}
}

func TestParseTryCatch(t *testing.T) {
	prog := testParse(t, "try { risky(); } catch (e) { handle(e); } finally { cleanup(); }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	ts, ok := prog.Body[0].(*TryStatement)
	if !ok {
		t.Fatalf("expected *TryStatement, got %T", prog.Body[0])
	}
	if ts.Handler == nil {
		t.Errorf("expected handler (catch) to be non-nil")
	}
	if ts.Finalizer == nil {
		t.Errorf("expected finalizer (finally) to be non-nil")
	}
}

func TestParseSwitch(t *testing.T) {
	prog := testParse(t, "switch (x) { case 1: break; default: break; }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	ss, ok := prog.Body[0].(*SwitchStatement)
	if !ok {
		t.Fatalf("expected *SwitchStatement, got %T", prog.Body[0])
	}
	if len(ss.Cases) != 2 {
		t.Errorf("expected 2 cases, got %d", len(ss.Cases))
	}
}

func TestParseThrow(t *testing.T) {
	prog := testParse(t, "throw new Error('fail');")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	_, ok := prog.Body[0].(*ThrowStatement)
	if !ok {
		t.Fatalf("expected *ThrowStatement, got %T", prog.Body[0])
	}
}

func TestParseImport(t *testing.T) {
	prog := testParse(t, `import fs from "fs"; import { readFile, writeFile } from "fs"; import * as all from "mod";`)
	if len(prog.Body) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(prog.Body))
	}
	imp1, ok := prog.Body[0].(*ImportDeclaration)
	if !ok || imp1.Source.Value != "fs" || len(imp1.Specifiers) != 1 {
		t.Errorf("expected import from fs with 1 specifier, got source=%q spec=%d", imp1.Source.Value, len(imp1.Specifiers))
	}
	if !imp1.Specifiers[0].Default {
		t.Errorf("expected default import")
	}
	imp2, ok := prog.Body[1].(*ImportDeclaration)
	if !ok || len(imp2.Specifiers) != 2 {
		t.Errorf("expected import with 2 specifiers, got %d", len(imp2.Specifiers))
	}
}

func TestParseExport(t *testing.T) {
	prog := testParse(t, "export default function() {}; export { foo, bar };")
	if len(prog.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Body))
	}
	exp1, ok := prog.Body[0].(*ExportDeclaration)
	if !ok || !exp1.Default {
		t.Errorf("expected default export")
	}
	exp2, ok := prog.Body[1].(*ExportDeclaration)
	if !ok || len(exp2.Specifiers) != 2 {
		t.Errorf("expected export with 2 specifiers, got %d", len(exp2.Specifiers))
	}
}

func TestParseClass(t *testing.T) {
	prog := testParse(t, "class Animal { constructor(name) { this.name = name; } speak() { return this.name; } }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	cls, ok := prog.Body[0].(*ClassDeclaration)
	if !ok {
		t.Fatalf("expected *ClassDeclaration, got %T", prog.Body[0])
	}
	if cls.Name.Name != "Animal" {
		t.Errorf("expected name Animal, got %s", cls.Name.Name)
	}
	if len(cls.Body.Methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(cls.Body.Methods))
	}
}

func TestParseClassExtends(t *testing.T) {
	prog := testParse(t, "class Dog extends Animal {}")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	cls, ok := prog.Body[0].(*ClassDeclaration)
	if !ok {
		t.Fatalf("expected *ClassDeclaration, got %T", prog.Body[0])
	}
	if cls.SuperClass == nil {
		t.Errorf("expected superclass (extends)")
	}
}

func TestParseArrowFunction(t *testing.T) {
	prog := testParse(t, "const add = (a, b) => a + b;")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseArrowSingleParam(t *testing.T) {
	prog := testParse(t, "const double = x => x * 2;")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseAsyncArrow(t *testing.T) {
	prog := testParse(t, "async () => { return 42; }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseBinaryExpr(t *testing.T) {
	prog := testParse(t, "x + y * z / 2;")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseCallExpr(t *testing.T) {
	prog := testParse(t, "foo(1, 2, 3);")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	es, ok := prog.Body[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("expected *ExpressionStatement, got %T", prog.Body[0])
	}
	call, ok := es.Expr.(*CallExpression)
	if !ok {
		t.Fatalf("expected *CallExpression, got %T", es.Expr)
	}
	if len(call.Arguments) != 3 {
		t.Errorf("expected 3 args, got %d", len(call.Arguments))
	}
}

func TestParseMemberExpr(t *testing.T) {
	prog := testParse(t, "obj.prop; obj[expr];")
	if len(prog.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Body))
	}
}

func TestParseNewExpr(t *testing.T) {
	prog := testParse(t, "new Foo(1, 2);")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseArray(t *testing.T) {
	prog := testParse(t, "[1, 2, 3];")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseObject(t *testing.T) {
	prog := testParse(t, "({a: 1, b: 2});")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseTemplateLiteral(t *testing.T) {
	prog := testParse(t, "`hello ${name}!`;")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	es, ok := prog.Body[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("expected *ExpressionStatement, got %T", prog.Body[0])
	}
	tl, ok := es.Expr.(*TemplateLiteral)
	if !ok {
		t.Fatalf("expected *TemplateLiteral, got %T", es.Expr)
	}
	if len(tl.Parts) != 3 {
		t.Errorf("expected 3 template parts, got %d", len(tl.Parts))
	}
}

func TestParseTaggedTemplate(t *testing.T) {
	prog := testParse(t, "html`<div>${content}</div>`;")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	es, ok := prog.Body[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("expected *ExpressionStatement, got %T", prog.Body[0])
	}
	tte, ok := es.Expr.(*TaggedTemplateExpression)
	if !ok {
		t.Fatalf("expected *TaggedTemplateExpression, got %T", es.Expr)
	}
	if id, ok := tte.Tag.(*Identifier); !ok || id.Name != "html" {
		t.Errorf("expected tag 'html', got %s", tte.Tag.String())
	}
}

func TestParseThis(t *testing.T) {
	prog := testParse(t, "this.foo();")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseSpread(t *testing.T) {
	prog := testParse(t, "foo(...args);")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseOptionalChaining(t *testing.T) {
	prog := testParse(t, "obj?.prop; obj?.[expr];")
	if len(prog.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Body))
	}
}

func TestParseUpdateExpr(t *testing.T) {
	prog := testParse(t, "i++; --j;")
	if len(prog.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Body))
	}
}

func TestParseTernary(t *testing.T) {
	prog := testParse(t, "a ? b : c;")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseSequence(t *testing.T) {
	prog := testParse(t, "a, b, c;")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseNestedBlocks(t *testing.T) {
	prog := testParse(t, "{ { { } } }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseMultipleStatements(t *testing.T) {
	prog := testParse(t, "a; b; c;")
	if len(prog.Body) != 3 {
		t.Errorf("expected 3 statements, got %d", len(prog.Body))
	}
}

// --- Scope Analysis tests ---

func TestScopeGlobal(t *testing.T) {
	prog := testParse(t, "var x = 1;")
	sa := AnalyzeScopes(prog)
	sym := sa.Global.Lookup("x")
	if sym == nil {
		t.Fatal("expected symbol 'x' in global scope")
	}
	if sym.Kind != SymVar {
		t.Errorf("expected SymVar, got %v", sym.Kind)
	}
}

func TestScopeFunction(t *testing.T) {
	prog := testParse(t, "function foo(a, b) { var c = 3; }")
	sa := AnalyzeScopes(prog)
	sym := sa.Global.Lookup("foo")
	if sym == nil {
		t.Fatal("expected symbol 'foo' in global scope")
	}
	declSym := sa.SymbolMap[prog.Body[0].(*FunctionDeclaration).Name]
	if declSym == nil || declSym.Name != "foo" {
		t.Errorf("expected function symbol for foo")
	}
}

func TestScopeNested(t *testing.T) {
	prog := testParse(t, "function outer() { var x = 1; function inner() { var y = 2; } }")
	sa := AnalyzeScopes(prog)
	sym := sa.Global.Lookup("outer")
	if sym == nil {
		t.Fatal("expected symbol 'outer' in global scope")
	}
}

func TestScopeResolution(t *testing.T) {
	prog := testParse(t, "var x = 1; function f() { return x; }")
	sa := AnalyzeScopes(prog)
	if sa.SymbolMap == nil {
		t.Fatal("SymbolMap is nil")
	}
}

func TestScopeImports(t *testing.T) {
	prog := testParse(t, `import fs from "fs";`)
	sa := AnalyzeScopes(prog)
	sym := sa.Global.Lookup("fs")
	if sym == nil {
		t.Fatal("expected symbol 'fs' in global scope")
	}
	if sym.Kind != SymImport {
		t.Errorf("expected SymImport, got %v", sym.Kind)
	}
}

func TestScopeClass(t *testing.T) {
	prog := testParse(t, "class MyClass {}")
	sa := AnalyzeScopes(prog)
	sym := sa.Global.Lookup("MyClass")
	if sym == nil {
		t.Fatal("expected symbol 'MyClass' in global scope")
	}
	if sym.Kind != SymClass {
		t.Errorf("expected SymClass, got %v", sym.Kind)
	}
}

// --- Walker tests ---

func TestWalkProgram(t *testing.T) {
	prog := testParse(t, "a; b; c;")
	var count int
	Walk(prog, func(n Node) VisitorFunc {
		count++
		return nil
	})
	if count == 0 {
		t.Errorf("walker visited no nodes")
	}
}

func TestWalkFunctionBody(t *testing.T) {
	prog := testParse(t, "function f() { var x = 1; return x; }")
	var fnCount, varCount, retCount int
	Walk(prog, func(n Node) VisitorFunc {
		switch n.(type) {
		case *FunctionDeclaration:
			fnCount++
		case *VariableDeclaration:
			varCount++
		case *ReturnStatement:
			retCount++
		}
		return nil
	})
	if fnCount != 1 {
		t.Errorf("expected 1 function, got %d", fnCount)
	}
	if varCount != 1 {
		t.Errorf("expected 1 var decl, got %d", varCount)
	}
	if retCount != 1 {
		t.Errorf("expected 1 return, got %d", retCount)
	}
}

// --- Import Resolver tests ---

func TestImportResolver(t *testing.T) {
	prog := testParse(t, `import { readFile } from "fs"; import def from "mod";`)
	sa := AnalyzeScopes(prog)
	r := NewImportResolver(prog, sa)
	errs := r.Resolve()
	if len(errs) > 0 {
		t.Fatalf("resolve errors: %v", errs)
	}
	if len(r.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(r.Imports))
	}
	if r.Imports[0].Source != "fs" || r.Imports[0].Local != "readFile" {
		t.Errorf("expected import from fs with local readFile, got %+v", r.Imports[0])
	}
	if r.Imports[1].Source != "mod" || !r.Imports[1].IsDefault {
		t.Errorf("expected default import from mod, got %+v", r.Imports[1])
	}
}

func TestExportResolver(t *testing.T) {
	prog := testParse(t, "export default function() {}")
	sa := AnalyzeScopes(prog)
	r := NewImportResolver(prog, sa)
	errs := r.Resolve()
	if len(errs) > 0 {
		t.Fatalf("resolve errors: %v", errs)
	}
	if len(r.Exports) != 1 {
		t.Fatalf("expected 1 export, got %d", len(r.Exports))
	}
	if r.Exports[0].ExportName != "default" {
		t.Errorf("expected export name 'default', got %q", r.Exports[0].ExportName)
	}
}

func TestExportNamed(t *testing.T) {
	prog := testParse(t, "export const foo = 1;")
	sa := AnalyzeScopes(prog)
	r := NewImportResolver(prog, sa)
	r.Resolve()
	if len(r.Exports) != 1 {
		t.Fatalf("expected 1 export, got %d", len(r.Exports))
	}
	if r.Exports[0].LocalName != "foo" {
		t.Errorf("expected local name 'foo', got %q", r.Exports[0].LocalName)
	}
}

// --- DepGraph tests ---

func TestDepGraph(t *testing.T) {
	prog := testParse(t, `import a from "a"; import b from "b";`)
	sa := AnalyzeScopes(prog)
	r := NewImportResolver(prog, sa)
	r.Resolve()
	dg := BuildDepGraph(prog, r)
	if len(dg.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %v", dg.Nodes)
	}
	if len(dg.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(dg.Edges))
	}
}

func TestDepGraphTransitive(t *testing.T) {
	prog := testParse(t, `import a from "a"; import b from "b";`)
	sa := AnalyzeScopes(prog)
	r := NewImportResolver(prog, sa)
	r.Resolve()
	dg := BuildDepGraph(prog, r)
	deps := dg.TransitiveDeps("(entry)")
	if len(deps) != 2 {
		t.Errorf("expected 2 transitive deps, got %v", deps)
	}
}

// --- Template Resolver tests ---

func TestTemplateResolver(t *testing.T) {
	prog := testParse(t, "`hello ${name}!`")
	tr := NewTemplateResolver(prog)
	calls := tr.ResolveAll()
	if len(calls) != 1 {
		t.Fatalf("expected 1 template call, got %d", len(calls))
	}
	if len(calls[0].Exprs) != 1 {
		t.Errorf("expected 1 expression, got %d", len(calls[0].Exprs))
	}
}

func TestTemplateResolverStatic(t *testing.T) {
	prog := testParse(t, "`hello world`")
	tr := NewTemplateResolver(prog)
	calls := tr.ResolveAll()
	if len(calls) != 1 {
		t.Fatalf("expected 1 template call, got %d", len(calls))
	}
	if !tr.IsStatic(&TemplateLiteral{Parts: []TemplatePart{{Value: "hello world"}}}) {
		t.Errorf("expected static template")
	}
}

func TestTemplateTagged(t *testing.T) {
	prog := testParse(t, "html`<div>${content}</div>`")
	tr := NewTemplateResolver(prog)
	calls := tr.ResolveAll()
	if len(calls) != 1 {
		t.Fatalf("expected 1 template call, got %d", len(calls))
	}
	if !calls[0].IsTagged {
		t.Errorf("expected tagged template")
	}
	if calls[0].Tag != "html" {
		t.Errorf("expected tag 'html', got %q", calls[0].Tag)
	}
}

// --- Call Graph tests ---

func TestCallGraph(t *testing.T) {
	prog := testParse(t, "function a() { b(); } function b() { c(); }")
	sa := AnalyzeScopes(prog)
	cg := BuildCallGraph(prog, sa)
	if len(cg.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(cg.Nodes))
	}
	a := cg.NodeMap["a"]
	if a == nil {
		t.Fatal("expected node 'a'")
	}
	if len(a.Children) != 1 {
		t.Errorf("expected a to call 1 function, got %d", len(a.Children))
	}
	if a.Children[0].Target.Name != "b" {
		t.Errorf("expected a to call b, got %s", a.Children[0].Target.Name)
	}
}

func TestCallGraphCallees(t *testing.T) {
	prog := testParse(t, "function a() { b(); } function b() {}")
	sa := AnalyzeScopes(prog)
	cg := BuildCallGraph(prog, sa)
	callees := cg.Callees("a")
	if len(callees) != 1 || callees[0].Name != "b" {
		t.Errorf("expected a to callee b, got %v", callees)
	}
}

func TestCallGraphCallers(t *testing.T) {
	prog := testParse(t, "function a() { b(); } function b() {}")
	sa := AnalyzeScopes(prog)
	cg := BuildCallGraph(prog, sa)
	callers := cg.Callers("b")
	if len(callers) != 1 || callers[0] != "a" {
		t.Errorf("expected b to have caller a, got %v", callers)
	}
}

// --- Position tests ---

func TestParsePositions(t *testing.T) {
	prog := testParse(t, "function foo() {}")
	fn := prog.Body[0].(*FunctionDeclaration)
	if fn.StartPos.Line != 1 || fn.StartPos.Col != 1 {
		t.Errorf("expected position (1,1), got (%d,%d)", fn.StartPos.Line, fn.StartPos.Col)
	}
}

func TestParseMultiLinePositions(t *testing.T) {
	prog := testParse(t, "\n\nvar x = 1;")
	v := prog.Body[0].(*VariableDeclaration)
	if v.StartPos.Line != 3 {
		t.Errorf("expected line 3, got %d", v.StartPos.Line)
	}
}

// --- Error handling tests ---

func TestParseError(t *testing.T) {
	_, errs := Parse("function () {}")
	if len(errs) == 0 {
		t.Errorf("expected parse error for unnamed function in statement position")
	}
}

func TestParseErrorUnterminatedString(t *testing.T) {
	tokens := TokenizeAll(`"unterminated`)
	for _, tok := range tokens {
		if tok.Type == ILLEGAL {
			return
		}
	}
	t.Errorf("expected ILLEGAL token for unterminated string")
}

func TestParseStaticClassMethod(t *testing.T) {
	prog := testParse(t, "class Foo { static bar() {} }")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
	cls := prog.Body[0].(*ClassDeclaration)
	if len(cls.Body.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(cls.Body.Methods))
	}
	if !cls.Body.Methods[0].Static {
		t.Errorf("expected static method")
	}
}

func TestParseDynamicImport(t *testing.T) {
	prog := testParse(t, "import('mod').then(fn);")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseSpreadArray(t *testing.T) {
	prog := testParse(t, "[...items, 1];")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}

func TestParseSpreadObject(t *testing.T) {
	prog := testParse(t, "({...obj, a: 1});")
	if len(prog.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Body))
	}
}
