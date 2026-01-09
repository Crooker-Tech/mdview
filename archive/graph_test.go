package archive

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	rootPath := "C:\\test\\root.md"
	g := NewGraph(rootPath)

	if g.Root != rootPath {
		t.Errorf("NewGraph().Root = %v, want %v", g.Root, rootPath)
	}

	if g.Count != 0 {
		t.Errorf("NewGraph().Count = %v, want 0", g.Count)
	}

	if g.Nodes == nil {
		t.Error("NewGraph().Nodes is nil")
	}
}

func TestAddNode(t *testing.T) {
	g := NewGraph("C:\\test\\root.md")

	node1 := g.AddNode("C:\\test\\doc.md", "doc.md", 1)
	if node1 == nil {
		t.Fatal("AddNode returned nil")
	}

	if g.Count != 1 {
		t.Errorf("After AddNode, Count = %v, want 1", g.Count)
	}

	if node1.Path != "C:\\test\\doc.md" {
		t.Errorf("node.Path = %v, want C:\\test\\doc.md", node1.Path)
	}

	if node1.RelativePath != "doc.md" {
		t.Errorf("node.RelativePath = %v, want doc.md", node1.RelativePath)
	}

	if node1.Depth != 1 {
		t.Errorf("node.Depth = %v, want 1", node1.Depth)
	}

	// Adding same node again should return existing node
	node2 := g.AddNode("C:\\test\\doc.md", "doc.md", 1)
	if node2 != node1 {
		t.Error("AddNode with same path should return existing node")
	}

	if g.Count != 1 {
		t.Errorf("After duplicate AddNode, Count = %v, want 1", g.Count)
	}
}

func TestHasNode(t *testing.T) {
	g := NewGraph("C:\\test\\root.md")

	if g.HasNode("C:\\test\\doc.md") {
		t.Error("HasNode returned true for non-existent node")
	}

	g.AddNode("C:\\test\\doc.md", "doc.md", 1)

	if !g.HasNode("C:\\test\\doc.md") {
		t.Error("HasNode returned false for existing node")
	}
}

func TestGetNode(t *testing.T) {
	g := NewGraph("C:\\test\\root.md")

	if g.GetNode("C:\\test\\doc.md") != nil {
		t.Error("GetNode returned non-nil for non-existent node")
	}

	expected := g.AddNode("C:\\test\\doc.md", "doc.md", 1)
	got := g.GetNode("C:\\test\\doc.md")

	if got != expected {
		t.Error("GetNode did not return the correct node")
	}
}

func TestOrderedNodes(t *testing.T) {
	g := NewGraph("C:\\test\\root.md")

	// Add nodes in random order with various depths
	g.AddNode("C:\\test\\depth2.md", "depth2.md", 2)
	g.AddNode("C:\\test\\depth0.md", "depth0.md", 0)
	g.AddNode("C:\\test\\depth1.md", "depth1.md", 1)
	g.AddNode("C:\\test\\depth3.md", "depth3.md", 3)

	ordered := g.OrderedNodes()

	if len(ordered) != 4 {
		t.Fatalf("OrderedNodes returned %d nodes, want 4", len(ordered))
	}

	// Check they're sorted by depth
	for i := 0; i < len(ordered); i++ {
		if ordered[i].Depth != i {
			t.Errorf("OrderedNodes[%d].Depth = %d, want %d", i, ordered[i].Depth, i)
		}
	}
}

func TestGraphString(t *testing.T) {
	g := NewGraph("C:\\test\\root.md")
	g.AddNode("C:\\test\\a.md", "a.md", 1)
	g.AddNode("C:\\test\\b.md", "b.md", 1)

	str := g.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Should contain root path and count
	if !contains(str, "root.md") {
		t.Error("String() does not contain root path")
	}

	if !contains(str, "2") {
		t.Error("String() does not contain count")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
