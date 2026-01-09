package archive

import (
	"os"
	"path/filepath"
	"testing"
)

// createTestFile creates a temporary markdown file with the given content
func createTestFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	return path
}

func TestBuildGraph_SingleFile(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create a single markdown file with no links
	rootPath := createTestFile(t, tempDir, "root.md", "# Hello\n\nNo links here.")

	// Build graph
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 1 {
		t.Errorf("graph.Count = %d, want 1", graph.Count)
	}

	if graph.Root != rootPath {
		t.Errorf("graph.Root = %s, want %s", graph.Root, rootPath)
	}

	rootNode := graph.GetNode(rootPath)
	if rootNode == nil {
		t.Fatal("Root node not found in graph")
	}

	if rootNode.Depth != 0 {
		t.Errorf("rootNode.Depth = %d, want 0", rootNode.Depth)
	}
}

func TestBuildGraph_LinearChain(t *testing.T) {
	tempDir := t.TempDir()

	// Create a chain: root.md -> a.md -> b.md
	createTestFile(t, tempDir, "b.md", "# B\n\nEnd of chain.")
	createTestFile(t, tempDir, "a.md", "# A\n\nSee [B](b.md).")
	rootPath := createTestFile(t, tempDir, "root.md", "# Root\n\nSee [A](a.md).")

	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 3 {
		t.Errorf("graph.Count = %d, want 3", graph.Count)
	}

	// Check depths
	rootNode := graph.GetNode(rootPath)
	if rootNode.Depth != 0 {
		t.Errorf("root depth = %d, want 0", rootNode.Depth)
	}

	aPath := filepath.Join(tempDir, "a.md")
	aNode := graph.GetNode(aPath)
	if aNode == nil {
		t.Fatal("a.md node not found")
	}
	if aNode.Depth != 1 {
		t.Errorf("a.md depth = %d, want 1", aNode.Depth)
	}

	bPath := filepath.Join(tempDir, "b.md")
	bNode := graph.GetNode(bPath)
	if bNode == nil {
		t.Fatal("b.md node not found")
	}
	if bNode.Depth != 2 {
		t.Errorf("b.md depth = %d, want 2", bNode.Depth)
	}
}

func TestBuildGraph_CycleDetection(t *testing.T) {
	tempDir := t.TempDir()

	// Create a cycle: root.md -> a.md -> b.md -> root.md
	createTestFile(t, tempDir, "b.md", "# B\n\nBack to [root](root.md).")
	createTestFile(t, tempDir, "a.md", "# A\n\nSee [B](b.md).")
	rootPath := createTestFile(t, tempDir, "root.md", "# Root\n\nSee [A](a.md).")

	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Should have all 3 nodes, but no infinite loop
	if graph.Count != 3 {
		t.Errorf("graph.Count = %d, want 3", graph.Count)
	}

	// Verify the cycle is captured in links
	bPath := filepath.Join(tempDir, "b.md")
	bNode := graph.GetNode(bPath)
	if bNode == nil {
		t.Fatal("b.md node not found")
	}

	// b.md should have a link back to root.md
	foundRoot := false
	for _, link := range bNode.Links {
		if link == rootPath {
			foundRoot = true
			break
		}
	}
	if !foundRoot {
		t.Error("Expected b.md to link back to root.md")
	}
}

func TestBuildGraph_MaxPagesLimit(t *testing.T) {
	tempDir := t.TempDir()

	// Create a chain longer than the limit
	createTestFile(t, tempDir, "d.md", "# D")
	createTestFile(t, tempDir, "c.md", "# C\n\n[D](d.md)")
	createTestFile(t, tempDir, "b.md", "# B\n\n[C](c.md)")
	createTestFile(t, tempDir, "a.md", "# A\n\n[B](b.md)")
	rootPath := createTestFile(t, tempDir, "root.md", "# Root\n\n[A](a.md)")

	// Limit to 3 pages
	graph, err := BuildGraph(rootPath, 3)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 3 {
		t.Errorf("graph.Count = %d, want 3 (limited)", graph.Count)
	}

	// Should have root, a, and b (BFS order)
	if !graph.HasNode(rootPath) {
		t.Error("Missing root node")
	}
	if !graph.HasNode(filepath.Join(tempDir, "a.md")) {
		t.Error("Missing a.md node")
	}
	if !graph.HasNode(filepath.Join(tempDir, "b.md")) {
		t.Error("Missing b.md node")
	}

	// Should NOT have c or d
	if graph.HasNode(filepath.Join(tempDir, "c.md")) {
		t.Error("Should not have c.md (exceeds limit)")
	}
	if graph.HasNode(filepath.Join(tempDir, "d.md")) {
		t.Error("Should not have d.md (exceeds limit)")
	}
}

func TestBuildGraph_MissingFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create root that links to non-existent file
	rootPath := createTestFile(t, tempDir, "root.md", "# Root\n\nSee [missing](missing.md).")

	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Should still build graph with just root
	if graph.Count != 1 {
		t.Errorf("graph.Count = %d, want 1", graph.Count)
	}

	// Root should still have the link recorded
	rootNode := graph.GetNode(rootPath)
	if len(rootNode.Links) != 1 {
		t.Errorf("root has %d links, want 1", len(rootNode.Links))
	}
}

func TestBuildGraph_MultipleLinks(t *testing.T) {
	tempDir := t.TempDir()

	// Create a root with multiple links
	createTestFile(t, tempDir, "a.md", "# A")
	createTestFile(t, tempDir, "b.md", "# B")
	createTestFile(t, tempDir, "c.md", "# C")
	rootPath := createTestFile(t, tempDir, "root.md", "# Root\n\n[A](a.md) [B](b.md) [C](c.md)")

	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 4 {
		t.Errorf("graph.Count = %d, want 4", graph.Count)
	}

	// All linked files should have depth 1
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		path := filepath.Join(tempDir, name)
		node := graph.GetNode(path)
		if node == nil {
			t.Errorf("Node for %s not found", name)
			continue
		}
		if node.Depth != 1 {
			t.Errorf("%s depth = %d, want 1", name, node.Depth)
		}
	}
}

func TestComputeRelativePath(t *testing.T) {
	tests := []struct {
		name   string
		source string
		target string
		want   string
	}{
		{
			name:   "same directory",
			source: "C:\\docs\\a.md",
			target: "C:\\docs\\b.md",
			want:   "b.md",
		},
		{
			name:   "subdirectory",
			source: "C:\\docs\\a.md",
			target: "C:\\docs\\sub\\b.md",
			want:   "sub\\b.md",
		},
		{
			name:   "parent directory",
			source: "C:\\docs\\sub\\a.md",
			target: "C:\\docs\\b.md",
			want:   "..\\b.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeRelativePath(tt.source, tt.target)
			if err != nil {
				t.Fatalf("ComputeRelativePath() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ComputeRelativePath() = %v, want %v", got, tt.want)
			}
		})
	}
}
