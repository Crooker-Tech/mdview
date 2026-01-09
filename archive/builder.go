package archive

import (
	"fmt"
	"os"
	"path/filepath"
)

// BuildGraph constructs a dependency graph starting from rootPath using BFS
// Returns error if rootPath doesn't exist or can't be read
// Stops when maxPages is reached (respects the limit during traversal)
func BuildGraph(rootPath string, maxPages int) (*Graph, error) {
	// Validate root file exists
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("root file does not exist: %s", rootPath)
	}

	// Create graph
	graph := NewGraph(rootPath)
	rootDir := filepath.Dir(rootPath)

	// Initialize BFS queue with root
	type queueItem struct {
		path  string
		depth int
	}
	queue := []queueItem{{path: rootPath, depth: 0}}

	// Track visited nodes to prevent cycles
	visited := make(map[string]bool)
	visited[rootPath] = true

	// BFS traversal
	for len(queue) > 0 && graph.Count < maxPages {
		// Dequeue
		item := queue[0]
		queue = queue[1:]

		currentPath := item.path
		currentDepth := item.depth

		// Read file content
		content, err := os.ReadFile(currentPath)
		if err != nil {
			// Warn but continue - don't fail entire build for one bad file
			fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", currentPath, err)
			continue
		}

		// Calculate relative path from root directory
		relPath, err := filepath.Rel(rootDir, currentPath)
		if err != nil {
			// If can't get relative path, use absolute (shouldn't happen normally)
			relPath = currentPath
		}

		// Add node to graph
		node := graph.AddNode(currentPath, relPath, currentDepth)

		// Scan for links
		baseDir := filepath.Dir(currentPath)
		links, err := ScanMarkdownLinks(content, baseDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan links in %s: %v\n", currentPath, err)
			continue
		}

		node.Links = links

		// Add unvisited links to queue
		for _, link := range links {
			if !visited[link] && graph.Count < maxPages {
				// Check if file exists before adding to queue
				if _, err := os.Stat(link); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Warning: linked file does not exist: %s\n", link)
					continue
				}

				visited[link] = true
				queue = append(queue, queueItem{path: link, depth: currentDepth + 1})
			}
		}
	}

	// Warn if we hit the limit
	if len(queue) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: maximum page limit (%d) reached\n", maxPages)
		fmt.Fprintf(os.Stderr, "Archive truncated, %d pages excluded\n", len(queue))
		fmt.Fprintf(os.Stderr, "Use --max-pages to increase limit\n")
	}

	return graph, nil
}

// ComputeRelativePath computes the relative path from source to target
// This is used for resolving links in the navigation system
func ComputeRelativePath(source, target string) (string, error) {
	sourceDir := filepath.Dir(source)
	relPath, err := filepath.Rel(sourceDir, target)
	if err != nil {
		return "", err
	}
	return relPath, nil
}
