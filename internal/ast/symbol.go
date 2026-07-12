package ast

type SymbolKind int

const (
	SymVar SymbolKind = iota
	SymFunc
	SymClass
	SymParam
	SymImport
)

type Symbol struct {
	Name     string
	Kind     SymbolKind
	Scope    *Scope
	DeclNode Node
}

type Scope struct {
	Parent   *Scope
	Symbols  map[string]*Symbol
	Children []*Scope
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		Parent:  parent,
		Symbols: make(map[string]*Symbol),
	}
}

func (s *Scope) Lookup(name string) *Symbol {
	if sym, ok := s.Symbols[name]; ok {
		return sym
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}
	return nil
}

func (s *Scope) Declare(name string, kind SymbolKind, node Node) *Symbol {
	sym := &Symbol{Name: name, Kind: kind, Scope: s, DeclNode: node}
	s.Symbols[name] = sym
	return sym
}

type ScopeAnalysis struct {
	Program    *Program
	Global     *Scope
	ScopeMap   map[Node]*Scope
	SymbolMap  map[Node]*Symbol
}

func AnalyzeScopes(prog *Program) *ScopeAnalysis {
	sa := &ScopeAnalysis{
		Program:   prog,
		Global:    NewScope(nil),
		ScopeMap:  make(map[Node]*Scope),
		SymbolMap: make(map[Node]*Symbol),
	}
	sa.analyze(prog, sa.Global)
	return sa
}

func (sa *ScopeAnalysis) analyze(node Node, scope *Scope) {
	if node == nil {
		return
	}
	sa.ScopeMap[node] = scope

	switch n := node.(type) {
	case *Program:
		for _, stmt := range n.Body {
			sa.analyze(stmt, scope)
		}

	case *FunctionDeclaration:
		if n.Name != nil {
			sym := scope.Declare(n.Name.Name, SymFunc, n)
			sa.SymbolMap[n.Name] = sym
		}
		fnScope := NewScope(scope)
		scope.Children = append(scope.Children, fnScope)
		for _, p := range n.Params {
			sym := fnScope.Declare(p.Name, SymParam, p)
			sa.SymbolMap[p] = sym
		}
		sa.analyze(n.Body, fnScope)

	case *ArrowFunctionExpression:
		fnScope := NewScope(scope)
		scope.Children = append(scope.Children, fnScope)
		for _, p := range n.Params {
			sym := fnScope.Declare(p.Name, SymParam, p)
			sa.SymbolMap[p] = sym
		}
		sa.analyze(n.Body, fnScope)

	case *BlockStatement:
		blockScope := NewScope(scope)
		scope.Children = append(scope.Children, blockScope)
		for _, stmt := range n.Body {
			sa.analyze(stmt, blockScope)
		}

	case *VariableDeclaration:
		for _, d := range n.Decls {
			sym := scope.Declare(d.Name.Name, SymVar, &d)
			sa.SymbolMap[d.Name] = sym
			if d.Init != nil {
				sa.analyze(d.Init, scope)
			}
		}

	case *ClassDeclaration:
		sym := scope.Declare(n.Name.Name, SymClass, n)
		sa.SymbolMap[n.Name] = sym
		if n.SuperClass != nil {
			sa.analyze(n.SuperClass, scope)
		}
		classScope := NewScope(scope)
		scope.Children = append(scope.Children, classScope)
		for _, m := range n.Body.Methods {
			methodScope := NewScope(classScope)
			classScope.Children = append(classScope.Children, methodScope)
			for _, p := range m.Params {
				sym := methodScope.Declare(p.Name, SymParam, p)
				sa.SymbolMap[p] = sym
			}
			sa.analyze(m.Body, methodScope)
		}

	case *ImportDeclaration:
		for _, s := range n.Specifiers {
			if s.Local != nil {
				sym := scope.Declare(s.Local.Name, SymImport, n)
				sa.SymbolMap[s.Local] = sym
			}
		}

	case *ExportDeclaration:
		if n.Declaration != nil {
			sa.analyze(n.Declaration, scope)
		}

	case *ExpressionStatement:
		sa.analyze(n.Expr, scope)

	case *Identifier:
		if sym := scope.Lookup(n.Name); sym != nil {
			sa.SymbolMap[n] = sym
		}

	default:
		Walk(node, func(n Node) VisitorFunc {
			switch nn := n.(type) {
			case *Identifier:
				if sym := scope.Lookup(nn.Name); sym != nil {
					sa.SymbolMap[nn] = sym
				}
			}
			return nil
		})
	}
}
