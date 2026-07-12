package ast

type VisitorFunc func(Node) VisitorFunc

type Walker struct {
	stack []Node
}

func Walk(root Node, fn VisitorFunc) {
	w := &Walker{}
	w.walk(root, fn)
}

func (w *Walker) walk(node Node, fn VisitorFunc) {
	if node == nil {
		return
	}
	w.stack = append(w.stack, node)
	fn(node)
	w.walkChildren(node, fn)
	w.stack = w.stack[:len(w.stack)-1]
}

func (w *Walker) walkChildren(node Node, fn VisitorFunc) {
	switch n := node.(type) {
	case *Program:
		for _, s := range n.Body {
			w.walk(s, fn)
		}
	case *ExpressionStatement:
		w.walk(n.Expr, fn)
	case *BlockStatement:
		for _, s := range n.Body {
			w.walk(s, fn)
		}
	case *FunctionDeclaration:
		for _, p := range n.Params {
			w.walk(p, fn)
		}
		w.walk(n.Body, fn)
	case *VariableDeclaration:
		for _, d := range n.Decls {
			w.walk(&d, fn)
		}
	case *VariableDeclarator:
		w.walk(n.Name, fn)
		if n.Init != nil {
			w.walk(n.Init, fn)
		}
	case *ReturnStatement:
		if n.Arg != nil {
			w.walk(n.Arg, fn)
		}
	case *IfStatement:
		w.walk(n.Test, fn)
		w.walk(n.Consequent, fn)
		if n.Alternate != nil {
			w.walk(n.Alternate, fn)
		}
	case *ForStatement:
		if n.Init != nil {
			w.walk(n.Init, fn)
		}
		if n.Test != nil {
			w.walk(n.Test, fn)
		}
		if n.Update != nil {
			w.walk(n.Update, fn)
		}
		w.walk(n.Body, fn)
	case *ForInStatement:
		w.walk(n.Left, fn)
		w.walk(n.Right, fn)
		w.walk(n.Body, fn)
	case *ForOfStatement:
		w.walk(n.Left, fn)
		w.walk(n.Right, fn)
		w.walk(n.Body, fn)
	case *WhileStatement:
		w.walk(n.Test, fn)
		w.walk(n.Body, fn)
	case *DoWhileStatement:
		w.walk(n.Body, fn)
		w.walk(n.Test, fn)
	case *BreakStatement:
	case *ContinueStatement:
	case *ThrowStatement:
		w.walk(n.Arg, fn)
	case *TryStatement:
		w.walk(n.Block, fn)
		if n.Handler != nil {
			if n.Handler.Param != nil {
				w.walk(n.Handler.Param, fn)
			}
			w.walk(n.Handler.Body, fn)
		}
		if n.Finalizer != nil {
			w.walk(n.Finalizer, fn)
		}
	case *SwitchStatement:
		w.walk(n.Discriminant, fn)
		for _, c := range n.Cases {
			if c.Test != nil {
				w.walk(c.Test, fn)
			}
			for _, s := range c.Body {
				w.walk(s, fn)
			}
		}
	case *ImportDeclaration:
		for _, s := range n.Specifiers {
			if s.Local != nil {
				w.walk(s.Local, fn)
			}
			if s.Imported != nil {
				w.walk(s.Imported, fn)
			}
		}
		w.walk(n.Source, fn)
	case *ExportDeclaration:
		for _, s := range n.Specifiers {
			w.walk(s.Local, fn)
			if s.Exported != nil {
				w.walk(s.Exported, fn)
			}
		}
		if n.Declaration != nil {
			w.walk(n.Declaration, fn)
		}
		if n.Source != nil {
			w.walk(n.Source, fn)
		}
	case *ClassDeclaration:
		w.walk(n.Name, fn)
		if n.SuperClass != nil {
			w.walk(n.SuperClass, fn)
		}
		for _, m := range n.Body.Methods {
			w.walk(m.Name, fn)
			for _, p := range m.Params {
				w.walk(p, fn)
			}
			w.walk(m.Body, fn)
		}
	case *ArrowFunctionExpression:
		for _, p := range n.Params {
			w.walk(p, fn)
		}
		w.walk(n.Body, fn)
	case *Identifier:
	case *Literal:
	case *BinaryExpression:
		w.walk(n.Left, fn)
		w.walk(n.Right, fn)
	case *UnaryExpression:
		w.walk(n.Arg, fn)
	case *AssignmentExpression:
		w.walk(n.Left, fn)
		w.walk(n.Right, fn)
	case *CallExpression:
		w.walk(n.Callee, fn)
		for _, a := range n.Arguments {
			w.walk(a, fn)
		}
	case *NewExpression:
		w.walk(n.Callee, fn)
		for _, a := range n.Arguments {
			w.walk(a, fn)
		}
	case *MemberExpression:
		w.walk(n.Object, fn)
		w.walk(n.Property, fn)
	case *ConditionalExpression:
		w.walk(n.Test, fn)
		w.walk(n.Consequent, fn)
		w.walk(n.Alternate, fn)
	case *SequenceExpression:
		for _, e := range n.Expressions {
			w.walk(e, fn)
		}
	case *TemplateLiteral:
		for _, p := range n.Parts {
			if p.Expr != nil {
				w.walk(p.Expr, fn)
			}
		}
	case *TaggedTemplateExpression:
		w.walk(n.Tag, fn)
		w.walk(n.Quasi, fn)
	case *ObjectExpression:
		for _, p := range n.Properties {
			w.walk(p.Key, fn)
			if p.Value != nil {
				w.walk(p.Value, fn)
			}
		}
	case *ArrayExpression:
		for _, e := range n.Elements {
			w.walk(e, fn)
		}
	case *ThisExpression:
	case *SpreadElement:
		w.walk(n.Arg, fn)
	case *UpdateExpression:
		w.walk(n.Arg, fn)
	}
}
