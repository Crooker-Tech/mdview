package archive

import (
	"fmt"
)

// Node represents a markdown file in the dependency graph
type Node struct {
	Path         string   // Absolute path to .md file
	RelativePath string   // Path relative to root document's directory
	Links        []string // Absolute paths to linked .md files
	Depth        int      // Distance from root (BFS depth)
}

// Graph represents the dependency graph of linked markdown files
type Graph struct {
	Root  string           // Absolute path to root document
	Nodes map[string]*Node // Path -> Node mapping
	Count int              // Total nodes in graph
}

// NewGraph creates a new empty graph with the given root path
func NewGraph(rootPath string) *Graph {
	return &Graph{
		Root:  rootPath,
		Nodes: make(map[string]*Node),
		Count: 0,
	}
}

// AddNode adds or updates a node in the graph
func (g *Graph) AddNode(path string, relativePath string, depth int) *Node {
	if node, exists := g.Nodes[path]; exists {
		return node
	}

	node := &Node{
		Path:         path,
		RelativePath: relativePath,
		Links:        []string{},
		Depth:        depth,
	}
	g.Nodes[path] = node
	g.Count++
	return node
}

// HasNode checks if a node exists in the graph
func (g *Graph) HasNode(path string) bool {
	_, exists := g.Nodes[path]
	return exists
}

// GetNode retrieves a node from the graph
func (g *Graph) GetNode(path string) *Node {
	return g.Nodes[path]
}

// OrderedNodes returns all nodes sorted by BFS depth (closer to root first)
// This ensures parent pages are converted before their linked pages
func (g *Graph) OrderedNodes() []*Node {
	// Create a slice of nodes
	nodes := make([]*Node, 0, g.Count)
	for _, node := range g.Nodes {
		nodes = append(nodes, node)
	}

	// Sort by depth (simple bubble sort, fine for small graphs)
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Depth < nodes[i].Depth {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}

	return nodes
}

// String returns a string representation of the graph for debugging
func (g *Graph) String() string {
	return fmt.Sprintf("Graph{Root: %s, Count: %d}", g.Root, g.Count)
}
