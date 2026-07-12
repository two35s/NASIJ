package ast

import "sort"

type DepEdge struct {
	Source string
	Target string
}

type DepGraph struct {
	Nodes []string
	Edges []DepEdge
}

func BuildDepGraph(prog *Program, resolver *ImportResolver) *DepGraph {
	dg := &DepGraph{}
	seen := make(map[string]bool)

	addNode := func(s string) {
		if !seen[s] {
			seen[s] = true
			dg.Nodes = append(dg.Nodes, s)
		}
	}

	Walk(prog, func(n Node) VisitorFunc {
		imp, ok := n.(*ImportDeclaration)
		if !ok {
			return nil
		}
		src := imp.Source.Value
		addNode(src)

		dg.Edges = append(dg.Edges, DepEdge{
			Source: "(entry)",
			Target: src,
		})
		return nil
	})

	sort.Strings(dg.Nodes)
	return dg
}

func (dg *DepGraph) TransitiveDeps(node string) []string {
	visited := make(map[string]bool)
	var collect func(n string)
	collect = func(n string) {
		if visited[n] {
			return
		}
		visited[n] = true
		for _, e := range dg.Edges {
			if e.Source == n {
				collect(e.Target)
			}
		}
	}
	collect(node)
	var result []string
	for n := range visited {
		if n != node {
			result = append(result, n)
		}
	}
	sort.Strings(result)
	return result
}

func (dg *DepGraph) ReverseDeps(node string) []string {
	var result []string
	for _, e := range dg.Edges {
		if e.Target == node {
			result = append(result, e.Source)
		}
	}
	return result
}
