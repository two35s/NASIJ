package ast

import "fmt"

// Node is the base interface for all AST nodes.
type Node interface {
	String() string
	Pos() Pos
}

// --- Top-level ---

type Program struct {
	Body     []Statement
	StartPos Pos
}

func (p *Program) Pos() Pos { return p.StartPos }
func (p *Program) String() string {
	s := "(Program"
	for _, stmt := range p.Body {
		s += " " + stmt.String()
	}
	return s + ")"
}

// --- Statements ---

type Statement interface {
	Node
	stmtNode()
}

type ExpressionStatement struct {
	Expr     Expression
	Semicolon bool
	StartPos  Pos
}

func (s *ExpressionStatement) Pos() Pos { return s.StartPos }
func (s *ExpressionStatement) stmtNode() {}
func (s *ExpressionStatement) String() string {
	return "(ExprStmt " + s.Expr.String() + ")"
}

type BlockStatement struct {
	Body     []Statement
	StartPos Pos
}

func (s *BlockStatement) Pos() Pos { return s.StartPos }
func (s *BlockStatement) stmtNode() {}
func (s *BlockStatement) String() string {
	r := "(Block"
	for _, stmt := range s.Body {
		r += " " + stmt.String()
	}
	return r + ")"
}

type VariableDeclaration struct {
	Kind   string // "var", "let", "const"
	Decls  []VariableDeclarator
	StartPos Pos
}

func (s *VariableDeclaration) Pos() Pos { return s.StartPos }
func (s *VariableDeclaration) stmtNode() {}
func (s *VariableDeclaration) exprNode() {} // used in for-init position
func (s *VariableDeclaration) String() string {
	r := "(Var " + s.Kind
	for _, d := range s.Decls {
		r += " " + d.String()
	}
	return r + ")"
}

type VariableDeclarator struct {
	Name  *Identifier
	Init  Expression // nil if no init
	StartPos Pos
}

func (d *VariableDeclarator) Pos() Pos { return d.StartPos }
func (d *VariableDeclarator) String() string {
	if d.Init != nil {
		return d.Name.String() + "=" + d.Init.String()
	}
	return d.Name.String()
}

type FunctionDeclaration struct {
	Name        *Identifier // nil for anonymous
	Params      []*Identifier
	Body        *BlockStatement
	Async       bool
	Generator   bool
	StartPos    Pos
}

func (s *FunctionDeclaration) Pos() Pos { return s.StartPos }
func (s *FunctionDeclaration) stmtNode() {}
func (s *FunctionDeclaration) String() string {
	r := "(Func"
	if s.Name != nil {
		r += " " + s.Name.String()
	}
	for _, p := range s.Params {
		r += " " + p.String()
	}
	r += " " + s.Body.String()
	return r + ")"
}

type ReturnStatement struct {
	Arg      Expression // nil if bare return
	StartPos Pos
}

func (s *ReturnStatement) Pos() Pos { return s.StartPos }
func (s *ReturnStatement) stmtNode() {}
func (s *ReturnStatement) String() string {
	if s.Arg != nil {
		return "(Return " + s.Arg.String() + ")"
	}
	return "(Return)"
}

type IfStatement struct {
	Test       Expression
	Consequent Statement
	Alternate  Statement // nil if no else
	StartPos   Pos
}

func (s *IfStatement) Pos() Pos { return s.StartPos }
func (s *IfStatement) stmtNode() {}
func (s *IfStatement) String() string {
	r := "(If " + s.Test.String() + " " + s.Consequent.String()
	if s.Alternate != nil {
		r += " " + s.Alternate.String()
	}
	return r + ")"
}

type ForStatement struct {
	Init      Expression // or *VariableDeclaration
	Test      Expression
	Update    Expression
	Body      Statement
	StartPos  Pos
}

func (s *ForStatement) Pos() Pos { return s.StartPos }
func (s *ForStatement) stmtNode() {}
func (s *ForStatement) String() string {
	return "(For ... " + s.Body.String() + ")"
}

type ForInStatement struct {
	Left     Expression // or *VariableDeclaration
	Right    Expression
	Body     Statement
	StartPos Pos
}

func (s *ForInStatement) Pos() Pos { return s.StartPos }
func (s *ForInStatement) stmtNode() {}
func (s *ForInStatement) String() string {
	return "(ForIn ... " + s.Body.String() + ")"
}

type ForOfStatement struct {
	Left     Expression // or *VariableDeclaration
	Right    Expression
	Body     Statement
	Await    bool
	StartPos Pos
}

func (s *ForOfStatement) Pos() Pos { return s.StartPos }
func (s *ForOfStatement) stmtNode() {}
func (s *ForOfStatement) String() string {
	return "(ForOf ... " + s.Body.String() + ")"
}

type WhileStatement struct {
	Test     Expression
	Body     Statement
	StartPos Pos
}

func (s *WhileStatement) Pos() Pos { return s.StartPos }
func (s *WhileStatement) stmtNode() {}
func (s *WhileStatement) String() string {
	return "(While " + s.Test.String() + " " + s.Body.String() + ")"
}

type DoWhileStatement struct {
	Body     Statement
	Test     Expression
	StartPos Pos
}

func (s *DoWhileStatement) Pos() Pos { return s.StartPos }
func (s *DoWhileStatement) stmtNode() {}
func (s *DoWhileStatement) String() string {
	return "(DoWhile " + s.Body.String() + " " + s.Test.String() + ")"
}

type BreakStatement struct {
	Label    *Identifier
	StartPos Pos
}

func (s *BreakStatement) Pos() Pos { return s.StartPos }
func (s *BreakStatement) stmtNode() {}
func (s *BreakStatement) String() string {
	return "(Break)"
}

type ContinueStatement struct {
	Label    *Identifier
	StartPos Pos
}

func (s *ContinueStatement) Pos() Pos { return s.StartPos }
func (s *ContinueStatement) stmtNode() {}
func (s *ContinueStatement) String() string {
	return "(Continue)"
}

type ThrowStatement struct {
	Arg      Expression
	StartPos Pos
}

func (s *ThrowStatement) Pos() Pos { return s.StartPos }
func (s *ThrowStatement) stmtNode() {}
func (s *ThrowStatement) String() string {
	return "(Throw " + s.Arg.String() + ")"
}

type TryStatement struct {
	Block      *BlockStatement
	Handler    *CatchClause // nil if no catch
	Finalizer  *BlockStatement // nil if no finally
	StartPos   Pos
}

func (s *TryStatement) Pos() Pos { return s.StartPos }
func (s *TryStatement) stmtNode() {}
func (s *TryStatement) String() string {
	return "(Try ...)"
}

type CatchClause struct {
	Param    *Identifier // nil if no parameter
	Body     *BlockStatement
	StartPos Pos
}

type SwitchStatement struct {
	Discriminant Expression
	Cases        []*SwitchCase
	StartPos     Pos
}

func (s *SwitchStatement) Pos() Pos { return s.StartPos }
func (s *SwitchStatement) stmtNode() {}
func (s *SwitchStatement) String() string {
	return "(Switch ...)"
}

type SwitchCase struct {
	Test     Expression // nil for default
	Body     []Statement
	StartPos Pos
}

type ClassDeclaration struct {
	Name      *Identifier
	SuperClass Expression // nil if no extends
	Body      *ClassBody
	StartPos  Pos
}

func (s *ClassDeclaration) Pos() Pos { return s.StartPos }
func (s *ClassDeclaration) stmtNode() {}
func (s *ClassDeclaration) exprNode() {}
func (s *ClassDeclaration) String() string {
	r := "(Class " + s.Name.String()
	if s.SuperClass != nil {
		r += " : " + s.SuperClass.String()
	}
	return r + " ...)"
}

type ClassBody struct {
	Methods  []*ClassMethod
	StartPos Pos
}

type ClassMethod struct {
	Name      Expression // Identifier or String
	Params    []*Identifier
	Body      *BlockStatement
	Static    bool
	Async     bool
	Generator bool
	StartPos  Pos
}

// --- Imports / Exports ---

type ImportDeclaration struct {
	Specifiers []ImportSpecifier
	Source     *Literal // string literal
	StartPos   Pos
}

func (s *ImportDeclaration) Pos() Pos { return s.StartPos }
func (s *ImportDeclaration) stmtNode() {}
func (s *ImportDeclaration) String() string {
	return "(Import " + s.Source.String() + " ...)"
}

type ImportSpecifier struct {
	Local  *Identifier
	Imported *Identifier // nil for default import
	Default bool
}

type ExportDeclaration struct {
	Declaration Statement     // nil for bare export { ... }
	Specifiers []ExportSpecifier // nil if Declaration is set
	Source     *Literal       // for export ... from "..."
	Default    bool
	All        bool // export * from "..."
	StartPos   Pos
}

func (s *ExportDeclaration) Pos() Pos { return s.StartPos }
func (s *ExportDeclaration) stmtNode() {}
func (s *ExportDeclaration) String() string {
	return "(Export ...)"
}

type ExportSpecifier struct {
	Local  *Identifier
	Exported *Identifier
}

// --- Expressions ---

type Expression interface {
	Node
	exprNode()
}

type Identifier struct {
	Name     string
	StartPos Pos
}

func (e *Identifier) Pos() Pos { return e.StartPos }
func (e *Identifier) exprNode() {}
func (e *Identifier) String() string { return e.Name }

type Literal struct {
	Kind     string // "string", "number", "boolean", "null", "regex"
	Value    string
	Raw      string
	StartPos Pos
}

func (e *Literal) Pos() Pos { return e.StartPos }
func (e *Literal) exprNode() {}
func (e *Literal) String() string { return e.Raw }

type BinaryExpression struct {
	Left     Expression
	Op       string
	Right    Expression
	StartPos Pos
}

func (e *BinaryExpression) Pos() Pos { return e.StartPos }
func (e *BinaryExpression) exprNode() {}
func (e *BinaryExpression) String() string {
	return "(" + e.Left.String() + " " + e.Op + " " + e.Right.String() + ")"
}

type UnaryExpression struct {
	Op       string
	Arg      Expression
	Prefix   bool
	StartPos Pos
}

func (e *UnaryExpression) Pos() Pos { return e.StartPos }
func (e *UnaryExpression) exprNode() {}
func (e *UnaryExpression) String() string {
	return "(" + e.Op + e.Arg.String() + ")"
}

type AssignmentExpression struct {
	Left     Expression
	Op       string // =, +=, -=, etc.
	Right    Expression
	StartPos Pos
}

func (e *AssignmentExpression) Pos() Pos { return e.StartPos }
func (e *AssignmentExpression) exprNode() {}
func (e *AssignmentExpression) String() string {
	return "(= " + e.Left.String() + " " + e.Right.String() + ")"
}

type CallExpression struct {
	Callee    Expression
	Arguments []Expression
	Optional  bool // ?.()
	StartPos  Pos
}

func (e *CallExpression) Pos() Pos { return e.StartPos }
func (e *CallExpression) exprNode() {}
func (e *CallExpression) String() string {
	r := "(Call " + e.Callee.String()
	for _, a := range e.Arguments {
		r += " " + a.String()
	}
	return r + ")"
}

type NewExpression struct {
	Callee    Expression
	Arguments []Expression
	StartPos  Pos
}

func (e *NewExpression) Pos() Pos { return e.StartPos }
func (e *NewExpression) exprNode() {}
func (e *NewExpression) String() string {
	r := "(New " + e.Callee.String()
	for _, a := range e.Arguments {
		r += " " + a.String()
	}
	return r + ")"
}

type MemberExpression struct {
	Object    Expression
	Property  Expression
	Computed  bool // true: obj[prop], false: obj.prop
	Optional  bool // true: obj?.prop
	StartPos  Pos
}

func (e *MemberExpression) Pos() Pos { return e.StartPos }
func (e *MemberExpression) exprNode() {}
func (e *MemberExpression) String() string {
	if e.Computed {
		return "(" + e.Object.String() + "[" + e.Property.String() + "])"
	}
	return "(" + e.Object.String() + "." + e.Property.String() + ")"
}

type ConditionalExpression struct {
	Test       Expression
	Consequent Expression
	Alternate  Expression
	StartPos   Pos
}

func (e *ConditionalExpression) Pos() Pos { return e.StartPos }
func (e *ConditionalExpression) exprNode() {}
func (e *ConditionalExpression) String() string {
	return "(? " + e.Test.String() + " " + e.Consequent.String() + " " + e.Alternate.String() + ")"
}

type ArrowFunctionExpression struct {
	Params   []*Identifier
	Body     Statement // BlockStatement or Expression
	Async    bool
	StartPos Pos
}

func (e *ArrowFunctionExpression) Pos() Pos { return e.StartPos }
func (e *ArrowFunctionExpression) exprNode() {}
func (e *ArrowFunctionExpression) stmtNode() {}
func (e *ArrowFunctionExpression) String() string {
	r := "(Arrow"
	for _, p := range e.Params {
		r += " " + p.String()
	}
	r += " " + e.Body.String()
	return r + ")"
}

type SequenceExpression struct {
	Expressions []Expression
	StartPos    Pos
}

func (e *SequenceExpression) Pos() Pos { return e.StartPos }
func (e *SequenceExpression) exprNode() {}
func (e *SequenceExpression) String() string {
	r := "(Seq"
	for _, e := range e.Expressions {
		r += " " + e.String()
	}
	return r + ")"
}

type TemplateLiteral struct {
	Parts  []TemplatePart
	StartPos Pos
}

func (e *TemplateLiteral) Pos() Pos { return e.StartPos }
func (e *TemplateLiteral) exprNode() {}
func (e *TemplateLiteral) String() string {
	r := "(Template"
	for _, p := range e.Parts {
		r += " " + p.String()
	}
	return r + ")"
}

type TemplatePart struct {
	Value string     // literal text
	Expr  Expression // nil for pure text parts
}

func (p *TemplatePart) String() string {
	if p.Expr != nil {
		return "${" + p.Expr.String() + "}"
	}
	return fmt.Sprintf("%q", p.Value)
}

type TaggedTemplateExpression struct {
	Tag   Expression
	Quasi *TemplateLiteral
	StartPos Pos
}

func (e *TaggedTemplateExpression) Pos() Pos { return e.StartPos }
func (e *TaggedTemplateExpression) exprNode() {}
func (e *TaggedTemplateExpression) String() string {
	return "(Tagged " + e.Tag.String() + " " + e.Quasi.String() + ")"
}

type ObjectExpression struct {
	Properties []*ObjectProperty
	StartPos   Pos
}

func (e *ObjectExpression) Pos() Pos { return e.StartPos }
func (e *ObjectExpression) exprNode() {}
func (e *ObjectExpression) String() string {
	r := "(Obj"
	for _, p := range e.Properties {
		r += " " + p.String()
	}
	return r + ")"
}

type ObjectProperty struct {
	Key      Expression
	Value    Expression
	Computed bool
	Shorthand bool
	Spread   bool
	StartPos Pos
}

func (p *ObjectProperty) Pos() Pos { return p.StartPos }
func (p *ObjectProperty) String() string {
	if p.Spread {
		return "..." + p.Value.String()
	}
	return p.Key.String() + ":" + p.Value.String()
}

type ArrayExpression struct {
	Elements []Expression // may contain nil for holes
	StartPos Pos
}

func (e *ArrayExpression) Pos() Pos { return e.StartPos }
func (e *ArrayExpression) exprNode() {}
func (e *ArrayExpression) String() string {
	r := "(Arr"
	for _, e := range e.Elements {
		r += " " + e.String()
	}
	return r + ")"
}

type ThisExpression struct {
	StartPos Pos
}

func (e *ThisExpression) Pos() Pos { return e.StartPos }
func (e *ThisExpression) exprNode() {}
func (e *ThisExpression) String() string { return "this" }

type SpreadElement struct {
	Arg      Expression
	StartPos Pos
}

func (e *SpreadElement) Pos() Pos { return e.StartPos }
func (e *SpreadElement) exprNode() {}
func (e *SpreadElement) String() string {
	return "..." + e.Arg.String()
}

type UpdateExpression struct {
	Arg      Expression
	Op       string // ++ or --
	Prefix   bool   // true: ++x, false: x++
	StartPos Pos
}

func (e *UpdateExpression) Pos() Pos { return e.StartPos }
func (e *UpdateExpression) exprNode() {}
func (e *UpdateExpression) String() string {
	if e.Prefix {
		return "(" + e.Op + e.Arg.String() + ")"
	}
	return "(" + e.Arg.String() + e.Op + ")"
}
