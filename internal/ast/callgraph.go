package ast

type CallGraphNode struct {
	Name     string
	Kind     string
	Children []*CallGraphEdge
}

type CallGraphEdge struct {
	Target *CallGraphNode
	Pos    Pos
}

type CallGraph struct {
	Nodes  []*CallGraphNode
	NodeMap map[string]*CallGraphNode
}

func NewCallGraph() *CallGraph {
	return &CallGraph{
		NodeMap: make(map[string]*CallGraphNode),
	}
}

func (cg *CallGraph) EnsureNode(name, kind string) *CallGraphNode {
	if n, ok := cg.NodeMap[name]; ok {
		return n
	}
	n := &CallGraphNode{Name: name, Kind: kind}
	cg.NodeMap[name] = n
	cg.Nodes = append(cg.Nodes, n)
	return n
}

func BuildCallGraph(prog *Program, sa *ScopeAnalysis) *CallGraph {
	cg := NewCallGraph()

	// First pass: collect function declarations
	Walk(prog, func(n Node) VisitorFunc {
		switch n := n.(type) {
		case *FunctionDeclaration:
			name := "<anonymous>"
			if n.Name != nil {
				name = n.Name.Name
			}
			cg.EnsureNode(name, "function")
		case *ArrowFunctionExpression:
			cg.EnsureNode("<anonymous>", "arrow")
		case *ClassDeclaration:
			cg.EnsureNode(n.Name.Name, "class")
		}
		return nil
	})

	// Second pass: find calls and build edges using the walker stack
	w := &Walker{}
	w.walk(prog, func(n Node) VisitorFunc {
		call, ok := n.(*CallExpression)
		if !ok {
			return nil
		}
		fnName := cg.enclosingFuncFromStack(w.stack)
		if fnName == "" {
			fnName = "(global)"
		}
		targetName := cg.callTarget(call)
		if targetName == "" {
			return nil
		}
		tgt := cg.NodeMap[targetName]
		if tgt == nil {
			return nil
		}
		src := cg.EnsureNode(fnName, "function")
		src.Children = append(src.Children, &CallGraphEdge{
			Target: tgt,
			Pos:    call.StartPos,
		})
		return nil
	})

	return cg
}

func (cg *CallGraph) enclosingFuncFromStack(stack []Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *FunctionDeclaration:
			if n.Name != nil {
				return n.Name.Name
			}
			return "<anonymous>"
		case *Program:
			return "(global)"
		}
	}
	return "(global)"
}

func (cg *CallGraph) callTarget(call *CallExpression) string {
	switch callee := call.Callee.(type) {
	case *Identifier:
		return callee.Name
	case *MemberExpression:
		if id, ok := callee.Property.(*Identifier); ok {
			return id.Name
		}
	}
	return ""
}

func (cg *CallGraph) Callees(name string) []*CallGraphNode {
	if n, ok := cg.NodeMap[name]; ok {
		var result []*CallGraphNode
		for _, e := range n.Children {
			result = append(result, e.Target)
		}
		return result
	}
	return nil
}

func (cg *CallGraph) Callers(name string) []string {
	var result []string
	for _, n := range cg.Nodes {
		for _, e := range n.Children {
			if e.Target.Name == name {
				result = append(result, n.Name)
			}
		}
	}
	return result
}
