package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
)

type NodeType string

const (
	NodePage          NodeType = "page"
	NodeAPIEndpoint   NodeType = "api_endpoint"
	NodeComponent     NodeType = "component"
	NodeJSModule      NodeType = "js_module"
	NodeCookie        NodeType = "cookie"
	NodeStorage       NodeType = "storage"
	NodeAuthFlow      NodeType = "auth_flow"
	NodeAuthEndpoint  NodeType = "auth_endpoint"
	NodeFramework     NodeType = "framework"
	NodeFinding       NodeType = "finding"
	NodeDependency    NodeType = "dependency"
	NodeVulnerability NodeType = "vulnerability"
	NodeServiceWorker NodeType = "service_worker"
)

type EdgeType string

const (
	EdgeContains      EdgeType = "contains"
	EdgeCalls         EdgeType = "calls"
	EdgeDependsOn     EdgeType = "depends_on"
	EdgeAuthenticates EdgeType = "authenticates"
	EdgeStores        EdgeType = "stores"
	EdgeHasFinding    EdgeType = "has_finding"
	EdgeDetected      EdgeType = "detected"
	EdgeServes        EdgeType = "serves"
	EdgeReferences    EdgeType = "references"
	EdgeConnects      EdgeType = "connects"
)

type Node struct {
	ID         string         `json:"id"`
	Type       NodeType       `json:"type"`
	Label      string         `json:"label"`
	Properties map[string]any `json:"properties"`
}

type Edge struct {
	ID         string         `json:"id"`
	Type       EdgeType       `json:"type"`
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Label      string         `json:"label"`
	Properties map[string]any `json:"properties"`
}

type GraphStats struct {
	TotalNodes  int              `json:"total_nodes"`
	TotalEdges  int              `json:"total_edges"`
	NodesByType map[NodeType]int `json:"nodes_by_type"`
	EdgesByType map[EdgeType]int `json:"edges_by_type"`
}

type SearchResult struct {
	Node  *Node `json:"node"`
	Score int   `json:"score"`
}

type Graph struct {
	nodes    map[string]*Node
	edges    []*Edge
	inEdges  map[string][]*Edge
	outEdges map[string][]*Edge
}

func New() *Graph {
	return &Graph{
		nodes:    make(map[string]*Node),
		edges:    make([]*Edge, 0),
		inEdges:  make(map[string][]*Edge),
		outEdges: make(map[string][]*Edge),
	}
}

func (g *Graph) AddNode(n *Node) *Node {
	if existing, ok := g.nodes[n.ID]; ok {
		return existing
	}
	clone := *n
	if clone.Properties == nil {
		clone.Properties = make(map[string]any)
	}
	g.nodes[clone.ID] = &clone
	return g.nodes[clone.ID]
}

func (g *Graph) AddEdge(e *Edge) *Edge {
	clone := *e
	if clone.Properties == nil {
		clone.Properties = make(map[string]any)
	}
	if clone.ID == "" {
		clone.ID = fmt.Sprintf("%s->%s:%s", e.Source, e.Target, e.Type)
	}
	g.edges = append(g.edges, &clone)
	g.outEdges[e.Source] = append(g.outEdges[e.Source], &clone)
	g.inEdges[e.Target] = append(g.inEdges[e.Target], &clone)
	return &clone
}

func (g *Graph) GetNode(id string) *Node {
	n, ok := g.nodes[id]
	if !ok {
		return nil
	}
	return n
}

func (g *Graph) HasNode(id string) bool {
	_, ok := g.nodes[id]
	return ok
}

func (g *Graph) AllNodes() []*Node {
	nodes := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

func (g *Graph) AllEdges() []*Edge {
	return g.edges
}

func (g *Graph) NodesByType(typ NodeType) []*Node {
	var result []*Node
	for _, n := range g.nodes {
		if n.Type == typ {
			result = append(result, n)
		}
	}
	return result
}

func (g *Graph) Neighbors(nodeID string, edgeTypes ...EdgeType) []*Node {
	seen := make(map[string]bool)
	var result []*Node

	check := func(edges []*Edge) {
		for _, e := range edges {
			if len(edgeTypes) > 0 {
				matched := false
				for _, et := range edgeTypes {
					if e.Type == et {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			var neighborID string
			if e.Source == nodeID {
				neighborID = e.Target
			} else if e.Target == nodeID {
				neighborID = e.Source
			} else {
				continue
			}
			if seen[neighborID] {
				continue
			}
			seen[neighborID] = true
			if n := g.GetNode(neighborID); n != nil {
				result = append(result, n)
			}
		}
	}

	check(g.outEdges[nodeID])
	check(g.inEdges[nodeID])
	return result
}

func (g *Graph) EdgesBetween(source, target string) []*Edge {
	var result []*Edge
	for _, e := range g.edges {
		if e.Source == source && e.Target == target {
			result = append(result, e)
		}
	}
	return result
}

func (g *Graph) Search(q string) []*Node {
	q = strings.ToLower(q)
	terms := strings.Fields(q)
	if len(terms) == 0 {
		return nil
	}

	var results []*Node
	for _, n := range g.nodes {
		score := 0
		matchLabel := strings.ToLower(n.Label)
		matchID := strings.ToLower(n.ID)
		matchType := strings.ToLower(string(n.Type))

		for _, term := range terms {
			if strings.Contains(matchLabel, term) {
				score += 3
			}
			if strings.Contains(matchID, term) {
				score += 2
			}
			if strings.Contains(matchType, term) {
				score++
			}
			for _, v := range n.Properties {
				if str, ok := v.(string); ok {
					if strings.Contains(strings.ToLower(str), term) {
						score++
					}
				}
			}
		}

		if score > 0 {
			results = append(results, n)
		}
	}
	return results
}

func (g *Graph) SearchRanked(q string) []SearchResult {
	nodes := g.Search(q)
	results := make([]SearchResult, 0, len(nodes))
	q = strings.ToLower(q)
	terms := strings.Fields(q)

	for _, n := range nodes {
		score := 0
		matchLabel := strings.ToLower(n.Label)
		matchID := strings.ToLower(n.ID)

		for _, term := range terms {
			if strings.Contains(matchLabel, term) {
				score += 3
			}
			if strings.Contains(matchID, term) {
				score += 2
			}
			for _, v := range n.Properties {
				if str, ok := v.(string); ok {
					if strings.Contains(strings.ToLower(str), term) {
						score++
					}
				}
			}
		}
		results = append(results, SearchResult{Node: n, Score: score})
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

func (g *Graph) FindPath(source, target string) [][]string {
	if source == target {
		return [][]string{{source}}
	}

	type pathEntry struct {
		node string
		path []string
		seen map[string]bool
	}

	var paths [][]string
	queue := []pathEntry{{node: source, path: []string{source}, seen: map[string]bool{source: true}}}
	shortest := -1

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if shortest > 0 && len(current.path) >= shortest {
			continue
		}

		for _, e := range g.outEdges[current.node] {
			if current.seen[e.Target] {
				continue
			}
			newPath := append(append([]string{}, current.path...), e.Target)
			if e.Target == target {
				if shortest < 0 || len(newPath) < shortest {
					shortest = len(newPath)
				}
				paths = append(paths, newPath)
				continue
			}
			if shortest > 0 && len(newPath) >= shortest {
				continue
			}
			newSeen := make(map[string]bool)
			for k, v := range current.seen {
				newSeen[k] = v
			}
			newSeen[e.Target] = true
			queue = append(queue, pathEntry{node: e.Target, path: newPath, seen: newSeen})
		}
	}

	return paths
}

func (g *Graph) Stats() GraphStats {
	stats := GraphStats{
		TotalNodes:  len(g.nodes),
		TotalEdges:  len(g.edges),
		NodesByType: make(map[NodeType]int),
		EdgesByType: make(map[EdgeType]int),
	}
	for _, n := range g.nodes {
		stats.NodesByType[n.Type]++
	}
	for _, e := range g.edges {
		stats.EdgesByType[e.Type]++
	}
	return stats
}

func (g *Graph) ToJSON() ([]byte, error) {
	type graphJSON struct {
		Nodes []*Node `json:"nodes"`
		Edges []*Edge `json:"edges"`
	}
	return json.MarshalIndent(graphJSON{Nodes: g.AllNodes(), Edges: g.edges}, "", "  ")
}

func (g *Graph) ToDOT() string {
	var b strings.Builder
	b.WriteString("digraph NASIJ {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box, style=rounded];\n\n")

	for _, n := range g.nodes {
		color := nodeColor(n.Type)
		label := fmt.Sprintf("%s\\n%s", n.Type, n.Label)
		b.WriteString(fmt.Sprintf("  %q [label=%q, color=%q, fontsize=10];\n", n.ID, label, color))
	}
	b.WriteString("\n")

	for _, e := range g.edges {
		b.WriteString(fmt.Sprintf("  %q -> %q [label=%q, fontsize=8];\n", e.Source, e.Target, string(e.Type)))
	}

	b.WriteString("}\n")
	return b.String()
}

func nodeColor(typ NodeType) string {
	switch typ {
	case NodePage:
		return "#4A90D9"
	case NodeAPIEndpoint:
		return "#50C878"
	case NodeComponent:
		return "#9B59B6"
	case NodeJSModule:
		return "#F39C12"
	case NodeCookie:
		return "#E74C3C"
	case NodeStorage:
		return "#1ABC9C"
	case NodeAuthFlow:
		return "#E67E22"
	case NodeAuthEndpoint:
		return "#D35400"
	case NodeFramework:
		return "#3498DB"
	case NodeFinding:
		return "#E74C3C"
	case NodeDependency:
		return "#95A5A6"
	case NodeVulnerability:
		return "#C0392B"
	case NodeServiceWorker:
		return "#8E44AD"
	default:
		return "#7F8C8D"
	}
}
