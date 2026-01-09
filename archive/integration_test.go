package archive

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_SinglePageWithNoLinks tests that a single page with no links
// works correctly (should use single-file conversion, not archive)
func TestIntegration_SinglePageWithNoLinks(t *testing.T) {
	tempDir := t.TempDir()

	// Create a single markdown file with no links
	rootPath := filepath.Join(tempDir, "single.md")
	content := "# Single Page\n\nThis page has no links to other markdown files.\n"

	if err := os.WriteFile(rootPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Check HasMarkdownLinks
	hasLinks, err := HasMarkdownLinks(rootPath)
	if err != nil {
		t.Fatalf("HasMarkdownLinks() error = %v", err)
	}

	if hasLinks {
		t.Error("HasMarkdownLinks() returned true for page with no markdown links")
	}
}

// TestIntegration_TwoPageArchive tests a simple two-page archive
func TestIntegration_TwoPageArchive(t *testing.T) {
	tempDir := t.TempDir()

	// Create two linked markdown files
	rootPath := filepath.Join(tempDir, "root.md")
	docPath := filepath.Join(tempDir, "doc.md")

	rootContent := "# Root\n\nGo to [documentation](doc.md).\n"
	docContent := "# Documentation\n\nReturn to [root](root.md).\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("Failed to create doc file: %v", err)
	}

	// Build graph
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 2 {
		t.Errorf("Expected 2 pages in graph, got %d", graph.Count)
	}

	// Verify both nodes are present
	if !graph.HasNode(rootPath) {
		t.Error("Graph missing root node")
	}
	if !graph.HasNode(docPath) {
		t.Error("Graph missing doc node")
	}

	// Convert to archive
	outputPath := filepath.Join(tempDir, "archive.html")
	err = WriteArchive(rootPath, outputPath, "default", 10, true, false)
	if err != nil {
		t.Fatalf("WriteArchive() error = %v", err)
	}

	// Verify output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	outputStr := string(output)

	// Verify it's a complete HTML document
	if !strings.Contains(outputStr, "<!DOCTYPE html>") {
		t.Error("Output is not a complete HTML document")
	}

	// Verify archive components
	if !strings.Contains(outputStr, "mdviewArchive") {
		t.Error("Output missing archive data")
	}
	if !strings.Contains(outputStr, "pako") {
		t.Error("Output missing pako decompression library")
	}

	// Verify both pages are in archive
	if !strings.Contains(outputStr, "root.md") {
		t.Error("Archive missing root.md reference")
	}
	if !strings.Contains(outputStr, "doc.md") {
		t.Error("Archive missing doc.md reference")
	}

	// Verify root content is visible (not in overlay)
	if !strings.Contains(outputStr, "Root") {
		t.Error("Archive missing root page title")
	}
}

// TestIntegration_CircularReferences tests handling of circular references
func TestIntegration_CircularReferences(t *testing.T) {
	tempDir := t.TempDir()

	// Create circular links: a -> b -> c -> a
	aPath := filepath.Join(tempDir, "a.md")
	bPath := filepath.Join(tempDir, "b.md")
	cPath := filepath.Join(tempDir, "c.md")

	aContent := "# A\n\n[Go to B](b.md)\n"
	bContent := "# B\n\n[Go to C](c.md)\n"
	cContent := "# C\n\n[Go to A](a.md)\n"

	if err := os.WriteFile(aPath, []byte(aContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	if err := os.WriteFile(bPath, []byte(bContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	if err := os.WriteFile(cPath, []byte(cContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Build graph - should handle cycle without infinite loop
	graph, err := BuildGraph(aPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Should have all 3 pages despite circular reference
	if graph.Count != 3 {
		t.Errorf("Expected 3 pages in graph (with cycle), got %d", graph.Count)
	}

	// Verify all nodes are present
	for _, path := range []string{aPath, bPath, cPath} {
		if !graph.HasNode(path) {
			t.Errorf("Graph missing node: %s", path)
		}
	}

	// Convert to archive - should succeed
	outputPath := filepath.Join(tempDir, "archive.html")
	err = WriteArchive(aPath, outputPath, "default", 10, true, false)
	if err != nil {
		t.Fatalf("WriteArchive() with circular refs error = %v", err)
	}

	// Verify output exists and is valid
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if len(output) == 0 {
		t.Error("Output file is empty")
	}
}

// TestIntegration_MaxPagesLimit tests that max pages limit is enforced
func TestIntegration_MaxPagesLimit(t *testing.T) {
	tempDir := t.TempDir()

	// Create a chain of 5 linked pages
	paths := make([]string, 5)
	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("page%d.md", i)
		paths[i] = filepath.Join(tempDir, filename)

		var content string
		if i < 4 {
			nextFilename := fmt.Sprintf("page%d.md", i+1)
			content = fmt.Sprintf("# Page %d\n\n[Next](%s)\n", i, nextFilename)
		} else {
			content = fmt.Sprintf("# Page %d\n\nLast page.\n", i)
		}

		if err := os.WriteFile(paths[i], []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Build graph with limit of 3
	graph, err := BuildGraph(paths[0], 3)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Should have exactly 3 pages due to limit
	if graph.Count != 3 {
		t.Errorf("Expected 3 pages (limited), got %d", graph.Count)
	}

	// Should have first 3 pages only
	for i := 0; i < 3; i++ {
		if !graph.HasNode(paths[i]) {
			t.Errorf("Graph missing page %d (should be included)", i)
		}
	}

	// Should NOT have pages 3 and 4
	for i := 3; i < 5; i++ {
		if graph.HasNode(paths[i]) {
			t.Errorf("Graph has page %d (should be excluded by limit)", i)
		}
	}
}

// TestIntegration_SubdirectoryLinks tests links to files in subdirectories
func TestIntegration_SubdirectoryLinks(t *testing.T) {
	tempDir := t.TempDir()

	// Create directory structure
	docsDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create docs directory: %v", err)
	}

	// Create root file linking to subdirectory
	rootPath := filepath.Join(tempDir, "root.md")
	rootContent := "# Root\n\n[Documentation](docs/doc.md)\n"

	docPath := filepath.Join(docsDir, "doc.md")
	docContent := "# Documentation\n\n[Back](../root.md)\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create root: %v", err)
	}
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("Failed to create doc: %v", err)
	}

	// Build graph
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 2 {
		t.Errorf("Expected 2 pages, got %d", graph.Count)
	}

	// Verify relative paths are correct
	rootNode := graph.GetNode(rootPath)
	if rootNode == nil {
		t.Fatal("Root node not found")
	}

	docNode := graph.GetNode(docPath)
	if docNode == nil {
		t.Fatal("Doc node not found")
	}

	// Verify relative path for doc is correct
	if !strings.Contains(docNode.RelativePath, "docs") {
		t.Errorf("Doc relative path incorrect: %s", docNode.RelativePath)
	}
}

// TestIntegration_MixedLinks tests a page with both markdown and non-markdown links
func TestIntegration_MixedLinks(t *testing.T) {
	tempDir := t.TempDir()

	rootPath := filepath.Join(tempDir, "root.md")
	docPath := filepath.Join(tempDir, "doc.md")

	// Root has markdown link, image link, and external link
	rootContent := `# Root

[Documentation](doc.md)
[External](https://example.com)
![Image](image.png)
[PDF](document.pdf)
`

	docContent := "# Documentation\n\n[Back](root.md)\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create root: %v", err)
	}
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("Failed to create doc: %v", err)
	}

	// Should detect markdown links
	hasLinks, err := HasMarkdownLinks(rootPath)
	if err != nil {
		t.Fatalf("HasMarkdownLinks() error = %v", err)
	}

	if !hasLinks {
		t.Error("HasMarkdownLinks() should detect doc.md link")
	}

	// Build graph - should only include markdown files
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 2 {
		t.Errorf("Expected 2 markdown pages, got %d", graph.Count)
	}
}

// TestIntegration_WithImages tests that archives with images work correctly
func TestIntegration_WithImages(t *testing.T) {
	tempDir := t.TempDir()

	// Create markdown file with image reference
	rootPath := filepath.Join(tempDir, "root.md")
	rootContent := "# Root\n\n![Test Image](test.png)\n"

	// Create dummy image file
	imagePath := filepath.Join(tempDir, "test.png")
	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create root: %v", err)
	}
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		t.Fatalf("Failed to create image: %v", err)
	}

	// Convert with self-contained
	outputPath := filepath.Join(tempDir, "output.html")
	err := WriteArchive(rootPath, outputPath, "default", 10, true, false)
	if err != nil {
		t.Fatalf("WriteArchive() error = %v", err)
	}

	// Verify output contains embedded image
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	outputStr := string(output)

	// Should contain base64 data URI for image
	if !strings.Contains(outputStr, "data:image/png;base64,") {
		t.Error("Output missing embedded image data")
	}
}

// TestIntegration_CLIBuild tests that the CLI tool builds successfully
func TestIntegration_CLIBuild(t *testing.T) {
	// Skip if not in CI or if go command not available
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	// Try to build the main package
	cmd := exec.Command("go", "build", "-o", "mdview_test.exe", ".")
	output, err := cmd.CombinedOutput()

	// Clean up test binary
	defer os.Remove("mdview_test.exe")

	if err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, output)
	}

	// Verify binary exists
	if _, err := os.Stat("mdview_test.exe"); os.IsNotExist(err) {
		t.Error("Build succeeded but binary not found")
	}
}

// TestIntegration_EmptyMarkdownFile tests handling of empty markdown files
func TestIntegration_EmptyMarkdownFile(t *testing.T) {
	tempDir := t.TempDir()

	rootPath := filepath.Join(tempDir, "empty.md")
	if err := os.WriteFile(rootPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Should handle empty file gracefully
	hasLinks, err := HasMarkdownLinks(rootPath)
	if err != nil {
		t.Fatalf("HasMarkdownLinks() error on empty file = %v", err)
	}

	if hasLinks {
		t.Error("Empty file should not have markdown links")
	}

	// Build graph with empty file
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error on empty file = %v", err)
	}

	if graph.Count != 1 {
		t.Errorf("Expected 1 node for empty file, got %d", graph.Count)
	}
}

// TestIntegration_BidirectionalLinks tests pages that link to each other
func TestIntegration_BidirectionalLinks(t *testing.T) {
	tempDir := t.TempDir()

	aPath := filepath.Join(tempDir, "a.md")
	bPath := filepath.Join(tempDir, "b.md")

	aContent := "# Page A\n\n[Go to B](b.md)\n"
	bContent := "# Page B\n\n[Go to A](a.md)\n"

	if err := os.WriteFile(aPath, []byte(aContent), 0644); err != nil {
		t.Fatalf("Failed to create a.md: %v", err)
	}
	if err := os.WriteFile(bPath, []byte(bContent), 0644); err != nil {
		t.Fatalf("Failed to create b.md: %v", err)
	}

	// Build graph starting from A
	graph, err := BuildGraph(aPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 2 {
		t.Errorf("Expected 2 nodes, got %d", graph.Count)
	}

	// Verify both have links to each other
	aNode := graph.GetNode(aPath)
	bNode := graph.GetNode(bPath)

	if aNode == nil || bNode == nil {
		t.Fatal("Missing nodes in graph")
	}

	// A should link to B
	aLinksTob := false
	for _, link := range aNode.Links {
		if link == bPath {
			aLinksTob = true
			break
		}
	}
	if !aLinksTob {
		t.Error("A does not link to B")
	}

	// B should link to A
	bLinksToA := false
	for _, link := range bNode.Links {
		if link == aPath {
			bLinksToA = true
			break
		}
	}
	if !bLinksToA {
		t.Error("B does not link to A")
	}
}

// TestIntegration_ArchiveWithPreload tests archive creation with --preload flag
func TestIntegration_ArchiveWithPreload(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files with images
	rootPath := filepath.Join(tempDir, "root.md")
	docPath := filepath.Join(tempDir, "doc.md")
	imgPath := filepath.Join(tempDir, "test.png")

	rootContent := "# Root\n\n[Doc](doc.md)\n\n![Image](test.png)\n"
	docContent := "# Doc\n\n[Back](root.md)\n\n![Same Image](test.png)\n"

	// Create a minimal PNG (1x1 transparent)
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create root: %v", err)
	}
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("Failed to create doc: %v", err)
	}
	if err := os.WriteFile(imgPath, pngData, 0644); err != nil {
		t.Fatalf("Failed to create image: %v", err)
	}

	// Test without preload
	outputNoPreload := filepath.Join(tempDir, "no_preload.html")
	err := WriteArchive(rootPath, outputNoPreload, "default", 10, true, false)
	if err != nil {
		t.Fatalf("WriteArchive() without preload error = %v", err)
	}

	// Test with preload
	outputWithPreload := filepath.Join(tempDir, "with_preload.html")
	err = WriteArchive(rootPath, outputWithPreload, "default", 10, true, true)
	if err != nil {
		t.Fatalf("WriteArchive() with preload error = %v", err)
	}

	// Both outputs should exist and be valid
	noPreloadData, err := os.ReadFile(outputNoPreload)
	if err != nil {
		t.Fatalf("Failed to read no_preload output: %v", err)
	}
	withPreloadData, err := os.ReadFile(outputWithPreload)
	if err != nil {
		t.Fatalf("Failed to read with_preload output: %v", err)
	}

	// Both should have the same essential content
	noPreloadStr := string(noPreloadData)
	withPreloadStr := string(withPreloadData)

	// Check for archive components
	for _, s := range []string{"mdviewArchive", "mdviewLoadPage", "root.md", "doc.md"} {
		if !strings.Contains(noPreloadStr, s) {
			t.Errorf("no_preload output missing: %s", s)
		}
		if !strings.Contains(withPreloadStr, s) {
			t.Errorf("with_preload output missing: %s", s)
		}
	}

	// Both should have embedded image (data:image/png)
	if !strings.Contains(noPreloadStr, "data:image/png") {
		t.Error("no_preload output missing embedded image")
	}
	if !strings.Contains(withPreloadStr, "data:image/png") {
		t.Error("with_preload output missing embedded image")
	}

	// Both should have javascript: links for .md files
	if !strings.Contains(noPreloadStr, "javascript:mdviewLoadPage") {
		t.Error("no_preload output missing javascript: links")
	}
	if !strings.Contains(withPreloadStr, "javascript:mdviewLoadPage") {
		t.Error("with_preload output missing javascript: links")
	}
}
