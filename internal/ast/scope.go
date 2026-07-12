package ast

type ScopeInfo struct {
	Scope *Scope
	Node  Node
}

func CollectScopes(prog *Program) []ScopeInfo {
	var result []ScopeInfo
	Walk(prog, func(n Node) VisitorFunc {
		switch n.(type) {
		case *Program, *FunctionDeclaration, *BlockStatement, *ArrowFunctionExpression, *ClassDeclaration:
			result = append(result, ScopeInfo{Node: n})
		}
		return nil
	})
	return result
}

func EnclosingScope(sa *ScopeAnalysis, node Node) *Scope {
	for n := node; n != nil; {
		if s, ok := sa.ScopeMap[n]; ok {
			return s
		}
		if p, ok := n.(*Program); ok {
			return sa.ScopeMap[p]
		}
		break
	}
	return sa.Global
}

func (sa *ScopeAnalysis) Resolve(name string, from Node) *Symbol {
	scope := EnclosingScope(sa, from)
	if scope != nil {
		return scope.Lookup(name)
	}
	return nil
}
