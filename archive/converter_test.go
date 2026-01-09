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

	ac := NewConverter(graph, "default", true, false)

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
	ac := NewConverter(graph, "default", true, false)

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

	ac := NewConverter(graph, "default", true, false)

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

	ac := NewConverter(graph, "default", true, false)

	archiveData := map[string]string{
		"path\\with\\backslash.md": "data1",
		"path\"with\"quotes.md":    "data2",
	}

	resources := ac.generateArchiveResources(archiveData)

	// Verify backslashes are escaped
	if !strings.Contains(resources, "\\\\") {
		t.Error("generateArchiveResources() did not escape backslashes")
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
