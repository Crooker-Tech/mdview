package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressData(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "small text",
			input: "Hello, World!",
		},
		{
			name:  "repeated text",
			input: strings.Repeat("compression test ", 100),
		},
		{
			name:  "html content",
			input: "<html><body><h1>Test</h1><p>This is a test.</p></body></html>",
		},
		{
			name:  "empty string",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := compressData([]byte(tt.input))
			if err != nil {
				t.Fatalf("compressData() error = %v", err)
			}

			if len(compressed) == 0 && len(tt.input) > 0 {
				t.Error("compressData() returned empty result for non-empty input")
			}

			// For repeated text, compression should be effective
			if tt.name == "repeated text" && len(compressed) >= len(tt.input) {
				t.Errorf("compressData() did not compress repeated text effectively: input=%d, compressed=%d",
					len(tt.input), len(compressed))
			}
		})
	}
}

func TestInjectBeforeClosingTag(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		tag        string
		content    string
		wantSubstr string
	}{
		{
			name:       "inject before body",
			html:       "<html><body>content</body></html>",
			tag:        "</body>",
			content:    "<script>test</script>",
			wantSubstr: "<script>test</script></body>",
		},
		{
			name:       "tag not found",
			html:       "<html><body>content</body></html>",
			tag:        "</head>",
			content:    "<script>test</script>",
			wantSubstr: "<script>test</script>",
		},
		{
			name:       "multiple occurrences",
			html:       "<div></div><div></div>",
			tag:        "</div>",
			content:    "X",
			wantSubstr: "X</div>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectBeforeClosingTag(tt.html, tt.tag, tt.content)

			if !strings.Contains(result, tt.wantSubstr) {
				t.Errorf("injectBeforeClosingTag() result does not contain %q\nGot: %s",
					tt.wantSubstr, result)
			}

			// Verify original content is preserved
			if !strings.Contains(result, "content") && strings.Contains(tt.html, "content") {
				t.Error("injectBeforeClosingTag() lost original content")
			}
		})
	}
}

func TestExtractArticleContent(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantHas  string
		wantNot  string
	}{
		{
			name: "full html with article",
			html: `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<article class="markdown-body">
<h1>Hello</h1>
<p>Content</p>
</article>
</body>
</html>`,
			wantHas: "<h1>Hello</h1>",
			wantNot: "<!DOCTYPE html>",
		},
		{
			name:    "no article tag",
			html:    "<html><body><h1>Test</h1></body></html>",
			wantHas: "<h1>Test</h1>",
			wantNot: "",
		},
		{
			name: "article with nested content",
			html: `<article class="markdown-body">
<h1>Title</h1>
<ul><li>Item 1</li><li>Item 2</li></ul>
</article>`,
			wantHas: "<ul><li>Item 1</li>",
			wantNot: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractArticleContent([]byte(tt.html))
			resultStr := string(result)

			if tt.wantHas != "" && !strings.Contains(resultStr, tt.wantHas) {
				t.Errorf("ExtractArticleContent() result does not contain %q", tt.wantHas)
			}

			if tt.wantNot != "" && strings.Contains(resultStr, tt.wantNot) {
				t.Errorf("ExtractArticleContent() result should not contain %q", tt.wantNot)
			}
		})
	}
}

func TestArchiveConverter_GenerateArchiveResources(t *testing.T) {
	// Create a minimal graph for testing
	graph := NewGraph("C:\\test\\root.md")
	graph.AddNode("C:\\test\\root.md", "root.md", 0)

	ac := NewConverter(graph, "default", true, false, "")

	archiveData := map[string]string{
		"root.md": "dGVzdCBkYXRh", // base64 "test data"
	}

	resources := ac.generateArchiveResources(archiveData)

	// Verify all required components are present
	requiredComponents := []string{
		"pako",                     // Decompression library
		"window.mdviewArchive",     // Archive data object
		"mdviewLoadPage",           // Navigation function
	}

	for _, component := range requiredComponents {
		if !strings.Contains(resources, component) {
			t.Errorf("generateArchiveResources() missing required component: %q", component)
		}
	}

	// Verify archive data is properly embedded
	if !strings.Contains(resources, "root.md") {
		t.Error("generateArchiveResources() does not contain archive data")
	}

	// Verify root path is embedded
	if !strings.Contains(resources, "C:/test/root.md") {
		t.Error("generateArchiveResources() does not contain root path")
	}
}

func TestArchiveConverter_ConvertToArchive(t *testing.T) {
	tempDir := t.TempDir()

	// Create test markdown files
	rootPath := filepath.Join(tempDir, "root.md")
	aPath := filepath.Join(tempDir, "a.md")

	rootContent := "# Root\n\n[Link to A](a.md)\n"
	aContent := "# Page A\n\n[Back to root](root.md)\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(aPath, []byte(aContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Build graph
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 2 {
		t.Fatalf("Expected graph with 2 nodes, got %d", graph.Count)
	}

	// Create archive converter
	ac := NewConverter(graph, "default", true, false, "")

	// Convert to archive
	outputPath := filepath.Join(tempDir, "archive.html")
	err = ac.ConvertToArchive(outputPath)
	if err != nil {
		t.Fatalf("ConvertToArchive() error = %v", err)
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("ConvertToArchive() did not create output file")
	}

	// Read and verify output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Verify structure
	requiredElements := []string{
		"<!DOCTYPE html>",
		"<html",
		"<body>",
		"</body>",
		"</html>",
		"window.mdviewArchive",
		"pako",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(outputStr, elem) {
			t.Errorf("Archive output missing required element: %q", elem)
		}
	}

	// Verify both pages are in archive data
	if !strings.Contains(outputStr, "root.md") {
		t.Error("Archive data missing root.md")
	}
	if !strings.Contains(outputStr, "a.md") {
		t.Error("Archive data missing a.md")
	}

	// Verify root content is visible (not compressed in overlay)
	if !strings.Contains(outputStr, "Root") {
		t.Error("Archive missing root page content")
	}
}

func TestWriteArchive(t *testing.T) {
	tempDir := t.TempDir()

	// Create test markdown files
	rootPath := filepath.Join(tempDir, "root.md")
	rootContent := "# Root Document\n\nThis is the root.\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputPath := filepath.Join(tempDir, "output.html")

	// Test WriteArchive convenience function
	err := WriteArchive(rootPath, outputPath, "default", 10, true, false)
	if err != nil {
		t.Fatalf("WriteArchive() error = %v", err)
	}

	// Verify output exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("WriteArchive() did not create output file")
	}

	// Verify it's valid HTML
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if !strings.Contains(string(output), "<!DOCTYPE html>") {
		t.Error("WriteArchive() output is not valid HTML")
	}
}

func TestArchiveConverter_WithInvalidTemplate(t *testing.T) {
	tempDir := t.TempDir()
	rootPath := filepath.Join(tempDir, "root.md")
	outputPath := filepath.Join(tempDir, "output.html")

	if err := os.WriteFile(rootPath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to use non-existent template
	err := WriteArchive(rootPath, outputPath, "nonexistent", 10, true, false)
	if err == nil {
		t.Error("WriteArchive() should fail with invalid template")
	}

	if !strings.Contains(err.Error(), "template") {
		t.Errorf("Expected template error, got: %v", err)
	}
}

func TestArchiveConverter_EmptyGraph(t *testing.T) {
	tempDir := t.TempDir()

	// Create empty graph
	graph := NewGraph("C:\\test\\empty.md")

	ac := NewConverter(graph, "default", true, false, "")

	outputPath := filepath.Join(tempDir, "empty.html")

	// Should handle empty graph gracefully
	err := ac.ConvertToArchive(outputPath)

	// This might fail or succeed depending on implementation
	// At minimum, it should not panic
	if err == nil {
		// If it succeeds, verify output exists
		if _, statErr := os.Stat(outputPath); statErr != nil {
			t.Error("ConvertToArchive() succeeded but did not create output file")
		}
	}
}

func TestArchiveConverter_PathEscaping(t *testing.T) {
	// Test that paths with special characters are properly escaped
	graph := NewGraph("C:\\test\\root.md")
	graph.AddNode("C:\\test\\root.md", "root.md", 0)

	ac := NewConverter(graph, "default", true, false, "")

	archiveData := map[string]string{
		"path\\with\\backslash.md": "data1",
		"path\"with\"quotes.md":    "data2",
	}

	resources := ac.generateArchiveResources(archiveData)

	// Verify backslashes are normalized to forward slashes (to match link generation)
	if !strings.Contains(resources, "path/with/backslash.md") {
		t.Error("generateArchiveResources() did not normalize backslashes to forward slashes")
	}

	// Verify quotes are escaped
	if strings.Count(resources, "\\\"") < 2 {
		t.Error("generateArchiveResources() did not escape quotes")
	}

	// Verify it's valid JavaScript (no syntax errors in data structure)
	if !strings.Contains(resources, "window.mdviewArchive") {
		t.Error("generateArchiveResources() generated invalid JavaScript structure")
	}
}

// =============================================================================
// Custom Title Tests for Archive
// =============================================================================

func TestArchiveConverter_WithCustomTitle(t *testing.T) {
	tempDir := t.TempDir()

	// Create test markdown file
	rootPath := filepath.Join(tempDir, "root.md")
	rootContent := "# Root Document\n\nThis is the root.\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Build graph
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Create archive converter with custom title
	ac := NewConverter(graph, "default", true, false, "My Custom Archive")

	outputPath := filepath.Join(tempDir, "archive.html")
	err = ac.ConvertToArchive(outputPath)
	if err != nil {
		t.Fatalf("ConvertToArchive() error = %v", err)
	}

	// Read output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	outputStr := string(output)

	// Verify custom title is present
	if !strings.Contains(outputStr, "<title>My Custom Archive</title>") {
		t.Error("Archive output missing custom title")
	}

	// Should NOT contain default title
	if strings.Contains(outputStr, "<title>Markdown Preview</title>") {
		t.Error("Archive output should not contain default title when custom title is set")
	}
}

func TestWriteArchive_SetsTitle(t *testing.T) {
	tempDir := t.TempDir()

	// Create test markdown file
	rootPath := filepath.Join(tempDir, "root.md")
	rootContent := "# Root Document\n\nThis is the root.\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Output filename will be used as title
	outputPath := filepath.Join(tempDir, "MyDocumentation.html")

	err := WriteArchive(rootPath, outputPath, "default", 10, true, false)
	if err != nil {
		t.Fatalf("WriteArchive() error = %v", err)
	}

	// Read output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	outputStr := string(output)

	// Verify title is set to output filename (without extension)
	if !strings.Contains(outputStr, "<title>MyDocumentation</title>") {
		t.Errorf("WriteArchive() should set title to output filename, got:\n%s", outputStr)
	}
}

func TestArchiveConverter_NestedDirectoryPaths(t *testing.T) {
	// Test that nested directory paths are normalized consistently
	// This is the bug that caused "Page not found in archive" errors
	tempDir := t.TempDir()

	// Create nested directory structure
	subDir := filepath.Join(tempDir, "roles", "engineering")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create test markdown files
	rootPath := filepath.Join(tempDir, "root.md")
	nestedPath := filepath.Join(subDir, "README.md")

	rootContent := "# Root\n\n[Engineering](roles/engineering/README.md)\n"
	nestedContent := "# Engineering\n\n[Back to root](../../root.md)\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}
	if err := os.WriteFile(nestedPath, []byte(nestedContent), 0644); err != nil {
		t.Fatalf("Failed to create nested file: %v", err)
	}

	// Build graph
	graph, err := BuildGraph(rootPath, 10)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph.Count != 2 {
		t.Fatalf("Expected graph with 2 nodes, got %d", graph.Count)
	}

	// Convert to archive
	ac := NewConverter(graph, "default", true, false, "")
	outputPath := filepath.Join(tempDir, "archive.html")
	if err := ac.ConvertToArchive(outputPath); err != nil {
		t.Fatalf("ConvertToArchive() error = %v", err)
	}

	// Read output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	outputStr := string(output)

	// KEY TEST: The archive key must use forward slashes
	// This matches how the javascript:mdviewLoadPage() calls are generated
	if !strings.Contains(outputStr, `"roles/engineering/README.md"`) {
		t.Error("Archive key for nested path should use forward slashes")
	}

	// The link should also use forward slashes (quotes may be HTML-encoded as &#39;)
	hasForwardSlashLink := strings.Contains(outputStr, "javascript:mdviewLoadPage('roles/engineering/README.md')") ||
		strings.Contains(outputStr, "javascript:mdviewLoadPage(&#39;roles/engineering/README.md&#39;)")
	if !hasForwardSlashLink {
		t.Error("Archive link for nested path should use forward slashes")
	}

	// Verify there are no Windows backslashes in the archive data section
	// Find the archive data section and check for backslashes
	archiveStart := strings.Index(outputStr, "window.mdviewArchive")
	if archiveStart == -1 {
		t.Fatal("Could not find archive data in output")
	}
	archiveSection := outputStr[archiveStart : archiveStart+2000] // Check first 2000 chars of archive
	if strings.Contains(archiveSection, `roles\\engineering`) {
		t.Error("Archive data contains Windows-style backslash paths, should be normalized to forward slashes")
	}
}

func TestWriteArchive_TitleFromFilenameWithHyphens(t *testing.T) {
	tempDir := t.TempDir()

	// Create test markdown file
	rootPath := filepath.Join(tempDir, "root.md")
	rootContent := "# Root\n"

	if err := os.WriteFile(rootPath, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Output filename with hyphens
	outputPath := filepath.Join(tempDir, "my-project-docs.html")

	err := WriteArchive(rootPath, outputPath, "default", 10, true, false)
	if err != nil {
		t.Fatalf("WriteArchive() error = %v", err)
	}

	// Read output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	outputStr := string(output)

	// Verify title preserves hyphens
	if !strings.Contains(outputStr, "<title>my-project-docs</title>") {
		t.Errorf("WriteArchive() should preserve hyphens in title, got:\n%s", outputStr)
	}
}
