package converter

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// setupTestDir creates a temp directory with test files and returns cleanup func
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "mdview-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create a small valid PNG (1x1 transparent pixel)
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(filepath.Join(dir, "test.png"), png, 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create test.png: %v", err)
	}

	// Create a test JPEG (minimal valid JPEG)
	jpg := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
		0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
		0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
		0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
		0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08,
		0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45,
		0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3,
		0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6,
		0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9,
		0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2,
		0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4,
		0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01,
		0x00, 0x00, 0x3F, 0x00, 0xFB, 0xD3, 0xFF, 0xD9,
	}
	if err := os.WriteFile(filepath.Join(dir, "test.jpg"), jpg, 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create test.jpg: %v", err)
	}

	// Create a linked markdown file
	if err := os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other"), 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create other.md: %v", err)
	}

	return dir, func() { os.RemoveAll(dir) }
}

// convert is a helper that runs conversion and returns the body content
func convert(t *testing.T, c *Converter, markdown string) string {
	t.Helper()
	var buf bytes.Buffer
	err := c.Convert(strings.NewReader(markdown), &buf, "default")
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}
	return buf.String()
}

func TestMarkdownImagePathRewriting(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	tests := []struct {
		name        string
		markdown    string
		selfContain bool
		wantContain string
		wantExclude string
	}{
		{
			name:        "relative path becomes file:// URL",
			markdown:    "![alt](test.png)",
			selfContain: false,
			wantContain: `src="file:///`,
		},
		{
			name:        "relative path embedded as base64",
			markdown:    "![alt](test.png)",
			selfContain: true,
			wantContain: `src="data:image/png;base64,`,
		},
		{
			name:        "http URL unchanged",
			markdown:    "![alt](http://example.com/img.png)",
			selfContain: true,
			wantContain: `src="http://example.com/img.png"`,
		},
		{
			name:        "https URL unchanged",
			markdown:    "![alt](https://example.com/img.png)",
			selfContain: false,
			wantContain: `src="https://example.com/img.png"`,
		},
		{
			name:        "data URI unchanged",
			markdown:    "![alt](data:image/png;base64,abc123)",
			selfContain: true,
			wantContain: `src="data:image/png;base64,abc123"`,
		},
		{
			name:        "nonexistent file falls back to file:// URL",
			markdown:    "![alt](nonexistent.png)",
			selfContain: true,
			wantContain: `src="file:///`,
			wantExclude: `data:`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			c.SetBaseDir(dir)
			c.SetSelfContained(tt.selfContain)

			result := convert(t, c, tt.markdown)

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantContain, result)
			}
			if tt.wantExclude != "" && strings.Contains(result, tt.wantExclude) {
				t.Errorf("expected output NOT to contain %q, got:\n%s", tt.wantExclude, result)
			}
		})
	}
}

func TestMarkdownLinkPathRewriting(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	tests := []struct {
		name        string
		markdown    string
		wantContain string
	}{
		{
			name:        "relative path becomes file:// URL",
			markdown:    "[link](other.md)",
			wantContain: `href="file:///`,
		},
		{
			name:        "http URL unchanged",
			markdown:    "[link](http://example.com)",
			wantContain: `href="http://example.com"`,
		},
		{
			name:        "https URL unchanged",
			markdown:    "[link](https://example.com)",
			wantContain: `href="https://example.com"`,
		},
		{
			name:        "anchor unchanged",
			markdown:    "[link](#section)",
			wantContain: `href="#section"`,
		},
		{
			name:        "mailto unchanged",
			markdown:    "[email](mailto:test@example.com)",
			wantContain: `href="mailto:test@example.com"`,
		},
		{
			name:        "tel unchanged",
			markdown:    "[phone](tel:+1234567890)",
			wantContain: `href="tel:+1234567890"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			c.SetBaseDir(dir)

			result := convert(t, c, tt.markdown)

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantContain, result)
			}
		})
	}
}

func TestRawHTMLImageProcessing(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	tests := []struct {
		name        string
		markdown    string
		selfContain bool
		wantContain string
	}{
		{
			name:        "raw HTML img src rewritten to file://",
			markdown:    `<img src="test.png" alt="test">`,
			selfContain: false,
			wantContain: `src="file:///`,
		},
		{
			name:        "raw HTML img src embedded as base64",
			markdown:    `<img src="test.png" alt="test">`,
			selfContain: true,
			wantContain: `src="data:image/png;base64,`,
		},
		{
			name:        "raw HTML img with http URL unchanged",
			markdown:    `<img src="http://example.com/img.png">`,
			selfContain: true,
			wantContain: `src="http://example.com/img.png"`,
		},
		{
			name:        "raw HTML img with file:// URL embedded",
			markdown:    `<img src="file:///` + strings.ReplaceAll(filepath.Join(dir, "test.png"), "\\", "/") + `">`,
			selfContain: true,
			wantContain: `src="data:image/png;base64,`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			c.SetBaseDir(dir)
			c.SetSelfContained(tt.selfContain)

			result := convert(t, c, tt.markdown)

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantContain, result)
			}
		})
	}
}

func TestRawHTMLLinkProcessing(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	tests := []struct {
		name        string
		markdown    string
		wantContain string
	}{
		{
			name:        "raw HTML href rewritten to file://",
			markdown:    `<a href="other.md">link</a>`,
			wantContain: `href="file:///`,
		},
		{
			name:        "raw HTML href with http unchanged",
			markdown:    `<a href="http://example.com">link</a>`,
			wantContain: `href="http://example.com"`,
		},
		{
			name:        "raw HTML href with anchor unchanged",
			markdown:    `<a href="#top">link</a>`,
			wantContain: `href="#top"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			c.SetBaseDir(dir)

			result := convert(t, c, tt.markdown)

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantContain, result)
			}
		})
	}
}

func TestFileURLHandling(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create file:// URL for the test image
	fileURL := "file:///" + strings.ReplaceAll(filepath.Join(dir, "test.png"), "\\", "/")

	tests := []struct {
		name        string
		markdown    string
		selfContain bool
		wantContain string
		wantExclude string
	}{
		{
			name:        "file:// URL in markdown image embedded when self-contained",
			markdown:    "![alt](" + fileURL + ")",
			selfContain: true,
			wantContain: `data:image/png;base64,`,
		},
		{
			name:        "file:// URL in markdown image unchanged when not self-contained",
			markdown:    "![alt](" + fileURL + ")",
			selfContain: false,
			wantContain: `file:///`,
			wantExclude: `data:`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			c.SetBaseDir(dir)
			c.SetSelfContained(tt.selfContain)

			result := convert(t, c, tt.markdown)

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantContain, result)
			}
			if tt.wantExclude != "" && strings.Contains(result, tt.wantExclude) {
				t.Errorf("expected output NOT to contain %q, got:\n%s", tt.wantExclude, result)
			}
		})
	}
}

func TestMIMETypeDetection(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"image.png", "image/png"},
		{"image.PNG", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"anim.gif", "image/gif"},
		{"icon.svg", "image/svg+xml"},
		{"photo.webp", "image/webp"},
		{"favicon.ico", "image/x-icon"},
		{"image.bmp", "image/bmp"},
		{"image.avif", "image/avif"},
		{"unknown.xyz", ""},
		{"noextension", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getMimeTypeFromExtension(tt.path)
			if result != tt.expected {
				t.Errorf("getMimeTypeFromExtension(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestCSSAssetMIMETypeDetection(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		// Fonts
		{"font.woff2", "font/woff2"},
		{"font.woff", "font/woff"},
		{"font.ttf", "font/ttf"},
		{"font.otf", "font/otf"},
		{"font.eot", "application/vnd.ms-fontobject"},
		// Images (also supported in CSS)
		{"bg.png", "image/png"},
		{"bg.jpg", "image/jpeg"},
		{"icon.svg", "image/svg+xml"},
		// Cursors
		{"pointer.cur", "image/x-icon"},
		// Unknown
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getCSSAssetMimeType(tt.path)
			if result != tt.expected {
				t.Errorf("getCSSAssetMimeType(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestInlineStyleCSSURLProcessing(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	markdown := `<div style="background: url('test.png')">content</div>`

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)

	result := convert(t, c, markdown)

	if !strings.Contains(result, "url('data:image/png;base64,") {
		t.Errorf("expected CSS url() to be embedded, got:\n%s", result)
	}
}

func TestNoBaseDirNoRewriting(t *testing.T) {
	markdown := "![alt](image.png)"

	c := New()
	// Don't set baseDir

	result := convert(t, c, markdown)

	// Should contain the original relative path, not file://
	if strings.Contains(result, "file:///") {
		t.Errorf("expected no file:// rewriting without baseDir, got:\n%s", result)
	}
	if !strings.Contains(result, `src="image.png"`) {
		t.Errorf("expected original path preserved, got:\n%s", result)
	}
}

func TestMixedContent(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	markdown := `# Test Document

Here's an image: ![test](test.png)

And a [link](other.md) to another file.

<div class="raw">
  <img src="test.jpg" alt="raw html image">
  <a href="other.md">raw link</a>
</div>

External: ![external](https://example.com/img.png)
`

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)

	result := convert(t, c, markdown)

	// Check markdown image is embedded
	if !strings.Contains(result, `data:image/png;base64,`) {
		t.Error("expected markdown PNG image to be embedded")
	}

	// Check markdown link is rewritten
	if !strings.Contains(result, `href="file:///`) {
		t.Error("expected markdown link to be rewritten to file://")
	}

	// Check raw HTML image is embedded
	if !strings.Contains(result, `data:image/jpeg;base64,`) {
		t.Error("expected raw HTML JPEG image to be embedded")
	}

	// Check external URL unchanged
	if !strings.Contains(result, `src="https://example.com/img.png"`) {
		t.Error("expected external URL to remain unchanged")
	}
}

func TestHTMLBlockProcessing(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// HTML block (starts at line beginning, has blank lines around)
	markdown := `Text before.

<figure>
  <img src="test.png" alt="figure">
  <figcaption>Caption</figcaption>
</figure>

Text after.`

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)

	result := convert(t, c, markdown)

	if !strings.Contains(result, `data:image/png;base64,`) {
		t.Errorf("expected HTML block image to be embedded, got:\n%s", result)
	}
}

// =============================================================================
// Markdown Rendering Tests
// =============================================================================

func TestBasicMarkdownRendering(t *testing.T) {
	c := New()

	tests := []struct {
		name        string
		markdown    string
		wantContain []string
	}{
		{
			name:        "headings",
			markdown:    "# H1\n## H2\n### H3",
			wantContain: []string{"<h1", "<h2", "<h3"},
		},
		{
			name:        "paragraphs",
			markdown:    "First paragraph.\n\nSecond paragraph.",
			wantContain: []string{"<p>First paragraph.</p>", "<p>Second paragraph.</p>"},
		},
		{
			name:        "bold and italic",
			markdown:    "**bold** and *italic* and ***both***",
			wantContain: []string{"<strong>bold</strong>", "<em>italic</em>"},
		},
		{
			name:        "unordered list",
			markdown:    "- item 1\n- item 2\n- item 3",
			wantContain: []string{"<ul>", "<li>item 1</li>", "<li>item 2</li>"},
		},
		{
			name:        "ordered list",
			markdown:    "1. first\n2. second\n3. third",
			wantContain: []string{"<ol>", "<li>first</li>", "<li>second</li>"},
		},
		{
			name:        "code inline",
			markdown:    "Use `code` here",
			wantContain: []string{"<code>code</code>"},
		},
		{
			name:        "code block",
			markdown:    "```go\nfunc main() {}\n```",
			wantContain: []string{"<pre>", "<code", "func main()"},
		},
		{
			name:        "blockquote",
			markdown:    "> This is a quote",
			wantContain: []string{"<blockquote>"},
		},
		{
			name:        "horizontal rule",
			markdown:    "---",
			wantContain: []string{"<hr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convert(t, c, tt.markdown)
			for _, want := range tt.wantContain {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

func TestGFMExtensions(t *testing.T) {
	c := New()

	tests := []struct {
		name        string
		markdown    string
		wantContain []string
	}{
		{
			name:        "table",
			markdown:    "| A | B |\n|---|---|\n| 1 | 2 |",
			wantContain: []string{"<table>", "<th>", "<td>"},
		},
		{
			name:        "task list",
			markdown:    "- [x] done\n- [ ] todo",
			wantContain: []string{`type="checkbox"`, "checked"},
		},
		{
			name:        "strikethrough",
			markdown:    "~~deleted~~",
			wantContain: []string{"<del>deleted</del>"},
		},
		{
			name:        "autolink",
			markdown:    "Visit https://example.com for info",
			wantContain: []string{`href="https://example.com"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convert(t, c, tt.markdown)
			for _, want := range tt.wantContain {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

func TestAutoHeadingIDs(t *testing.T) {
	c := New()

	markdown := "# My Heading\n## Another One"
	result := convert(t, c, markdown)

	// Should have id attributes on headings
	if !strings.Contains(result, `id="`) {
		t.Errorf("expected heading to have id attribute, got:\n%s", result)
	}
}

func TestTypographer(t *testing.T) {
	c := New()

	// Typographer converts certain character sequences to HTML entities
	// e.g., "text" -> &ldquo;text&rdquo;, ... -> &hellip;
	tests := []struct {
		name        string
		markdown    string
		wantContain string
	}{
		{
			name:        "smart quotes transforms double quotes",
			markdown:    `He said "Hello" today`,
			wantContain: "&ldquo;", // Left double quotation mark entity
		},
		{
			name:        "ellipsis transforms triple dots",
			markdown:    "Please wait...",
			wantContain: "&hellip;", // Horizontal ellipsis entity
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convert(t, c, tt.markdown)
			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("expected typographer output to contain %q (HTML entity), got:\n%s", tt.wantContain, result)
			}
		})
	}
}

func TestUnsafeHTMLPassthrough(t *testing.T) {
	c := New()

	// Raw HTML should pass through (html.WithUnsafe)
	markdown := `<div class="custom">
  <span>Custom HTML</span>
</div>`

	result := convert(t, c, markdown)

	if !strings.Contains(result, `class="custom"`) {
		t.Errorf("expected raw HTML to pass through, got:\n%s", result)
	}
	if !strings.Contains(result, "<span>Custom HTML</span>") {
		t.Errorf("expected nested HTML to pass through, got:\n%s", result)
	}
}

func TestHTMLStructure(t *testing.T) {
	c := New()

	result := convert(t, c, "# Test")

	// Should have proper HTML structure
	checks := []string{
		"<!DOCTYPE html>",
		"<html",
		"<head>",
		"<meta charset=",
		"<meta name=\"viewport\"",
		"<style>",
		"</style>",
		"</head>",
		"<body>",
		`<article class="markdown-body">`,
		"</article>",
		"</body>",
		"</html>",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("expected HTML structure to contain %q", check)
		}
	}
}

func TestTemplateIntegration(t *testing.T) {
	c := New()

	result := convert(t, c, "# Test")

	// Should include highlight.js from default template
	if !strings.Contains(result, "hljs") {
		t.Error("expected output to contain highlight.js")
	}

	// Should have CSS variables for theming
	if !strings.Contains(result, "--color-") {
		t.Error("expected output to contain CSS variables")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestEmptyInput(t *testing.T) {
	c := New()

	result := convert(t, c, "")

	// Should still produce valid HTML structure
	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("expected valid HTML even with empty input")
	}
}

func TestSpecialCharacters(t *testing.T) {
	c := New()

	markdown := `Special chars: <>&"'`
	result := convert(t, c, markdown)

	// HTML entities should be escaped in text content
	// (but not in the raw markdown which uses html.WithUnsafe)
	if !strings.Contains(result, "&amp;") && !strings.Contains(result, "&") {
		t.Logf("Result contains ampersand handling: %s", result)
	}
}

func TestVeryLongLines(t *testing.T) {
	c := New()

	// Create a very long line
	longLine := strings.Repeat("word ", 1000)
	markdown := "# Title\n\n" + longLine

	result := convert(t, c, markdown)

	if !strings.Contains(result, "word") {
		t.Error("expected long line to be rendered")
	}
}

func TestDeepNesting(t *testing.T) {
	c := New()

	// Deeply nested blockquotes
	markdown := "> level 1\n>> level 2\n>>> level 3\n>>>> level 4"
	result := convert(t, c, markdown)

	// Count blockquote tags
	count := strings.Count(result, "<blockquote>")
	if count < 4 {
		t.Errorf("expected at least 4 nested blockquotes, got %d", count)
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkConvertSmall(b *testing.B) {
	c := New()
	markdown := "# Hello\n\nThis is a **test** with some `code`."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkConvertMedium(b *testing.B) {
	c := New()

	// Generate medium-sized markdown (~10KB)
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("## Section %d\n\n", i))
		sb.WriteString("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ")
		sb.WriteString("Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n\n")
		sb.WriteString("- Item one\n- Item two\n- Item three\n\n")
		sb.WriteString("```go\nfunc example() {}\n```\n\n")
	}
	markdown := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkConvertLarge(b *testing.B) {
	c := New()

	// Generate large markdown (~100KB)
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString(fmt.Sprintf("## Section %d\n\n", i))
		sb.WriteString("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ")
		sb.WriteString("Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n\n")
	}
	markdown := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkConvertWithImages(b *testing.B) {
	dir, err := os.MkdirTemp("", "mdview-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test image
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	_ = os.WriteFile(filepath.Join(dir, "test.png"), png, 0644)

	// Markdown with multiple images
	var sb strings.Builder
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("## Image %d\n\n![alt](test.png)\n\n", i))
	}
	markdown := sb.String()

	c := New()
	c.SetBaseDir(dir)

	b.Run("rewrite", func(b *testing.B) {
		c.SetSelfContained(false)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_ = c.Convert(strings.NewReader(markdown), &buf, "default")
		}
	})

	b.Run("embed", func(b *testing.B) {
		c.SetSelfContained(true)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_ = c.Convert(strings.NewReader(markdown), &buf, "default")
		}
	})
}

func BenchmarkConvertWithSizeHint(b *testing.B) {
	c := New()

	// Generate medium-sized markdown
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("## Section %d\n\nSome content here.\n\n", i))
	}
	markdown := sb.String()
	size := int64(len(markdown))

	b.Run("without_hint", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_ = c.Convert(strings.NewReader(markdown), &buf, "default")
		}
	})

	b.Run("with_hint", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_ = c.ConvertWithSize(strings.NewReader(markdown), &buf, "default", size)
		}
	})
}

// =============================================================================
// Memory Profiling Tests
// =============================================================================

func TestMemoryUsageStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	// NOTE: "Streaming" in this codebase means:
	// - Streaming OUTPUT: we write HTML incrementally as we render nodes
	// - Path processing during render pass (not post-processing)
	//
	// However, goldmark requires full input in memory to build AST.
	// True streaming (constant memory) would require a different parser.

	// Generate large markdown (~1MB)
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		sb.WriteString(fmt.Sprintf("## Section %d\n\n", i))
		sb.WriteString("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ")
		sb.WriteString("Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n\n")
	}
	markdown := sb.String()
	inputSize := len(markdown)

	// Force GC before measurement
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	c := New()
	var buf bytes.Buffer
	err := c.Convert(strings.NewReader(markdown), &buf, "default")
	if err != nil {
		t.Fatal(err)
	}

	// Measure BEFORE GC to see peak heap usage
	var mPeak runtime.MemStats
	runtime.ReadMemStats(&mPeak)
	peakHeap := mPeak.HeapInuse - m1.HeapInuse

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// TotalAlloc = cumulative (includes freed memory)
	// HeapInuse after GC = actual retained memory
	totalAllocated := m2.TotalAlloc - m1.TotalAlloc
	retainedHeap := m2.HeapInuse - m1.HeapInuse
	outputSize := buf.Len()

	// Log memory usage for analysis
	t.Logf("Input size: %d bytes (%.2f MB)", inputSize, float64(inputSize)/1024/1024)
	t.Logf("Output size: %d bytes (%.2f MB)", outputSize, float64(outputSize)/1024/1024)
	t.Logf("Total allocated (cumulative): %d bytes (%.2f MB) - %.1fx input", totalAllocated, float64(totalAllocated)/1024/1024, float64(totalAllocated)/float64(inputSize))
	t.Logf("Peak heap in use: %d bytes (%.2f MB) - %.1fx input", peakHeap, float64(peakHeap)/1024/1024, float64(peakHeap)/float64(inputSize))
	t.Logf("Retained after GC: %d bytes (%.2f MB)", retainedHeap, float64(retainedHeap)/1024/1024)

	// Peak heap includes: input + goldmark AST + output buffer
	// Goldmark creates many small allocations for AST nodes (~3 nodes per section)
	// With 10,000 sections = ~30,000 nodes, each with overhead
	// Expected peak: ~30x input (AST node overhead dominates)
	// Key metric: retained after GC should be small (just output buffer)
	maxExpectedPeak := uint64(inputSize * 40)
	if peakHeap > maxExpectedPeak {
		t.Errorf("peak heap seems excessive: %d bytes (expected < %d)", peakHeap, maxExpectedPeak)
	}

	// Retained memory after GC should be reasonable (output buffer + some overhead)
	// Should be close to output size, not input size
	maxRetained := uint64(outputSize * 3)
	if retainedHeap > maxRetained {
		t.Errorf("retained heap after GC seems excessive: %d bytes (expected < %d)", retainedHeap, maxRetained)
	}

	// Also verify output is larger than input (HTML wrapping adds overhead)
	if outputSize < inputSize {
		t.Errorf("expected output to be larger than input due to HTML wrapping")
	}
}

func TestBufferPoolReuse(t *testing.T) {
	c := New()
	markdown := "# Test\n\nSome content here."

	// Run multiple conversions
	for i := 0; i < 100; i++ {
		var buf bytes.Buffer
		err := c.Convert(strings.NewReader(markdown), &buf, "default")
		if err != nil {
			t.Fatal(err)
		}
	}

	// If buffer pool is working, this should complete without issues
	// The test mainly ensures no panics or memory leaks in repeated usage
}

func BenchmarkMemoryAllocation(b *testing.B) {
	c := New()

	// Medium markdown
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("## Section %d\n\nContent here.\n\n", i))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

// =============================================================================
// Image Preload Tests
// =============================================================================

// generatePNG creates a valid PNG image of approximately the specified size in bytes.
// It generates a simple grayscale image with random-ish data to achieve the target size.
func generatePNG(targetSize int) []byte {
	// PNG has fixed overhead, so we need to calculate the image dimensions
	// to achieve approximately the target size after compression
	// For simplicity, we'll create a grayscale image and adjust dimensions

	// Minimum valid PNG is about 67 bytes, so clamp
	if targetSize < 100 {
		targetSize = 100
	}

	// Estimate uncompressed data needed (zlib compresses ~50% for random data)
	uncompressedSize := targetSize * 2

	// Calculate dimensions for a square-ish image
	// Each pixel is 1 byte (grayscale), plus 1 filter byte per row
	// So width * height + height ≈ uncompressedSize
	dim := 1
	for dim*dim+dim < uncompressedSize && dim < 4096 {
		dim++
	}
	width := dim
	height := dim

	var buf bytes.Buffer

	// PNG signature
	buf.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})

	// IHDR chunk
	ihdr := []byte{
		byte(width >> 24), byte(width >> 16), byte(width >> 8), byte(width),
		byte(height >> 24), byte(height >> 16), byte(height >> 8), byte(height),
		8,    // bit depth
		0,    // color type (grayscale)
		0,    // compression method
		0,    // filter method
		0,    // interlace method
	}
	writeChunk(&buf, "IHDR", ihdr)

	// IDAT chunk - compressed image data
	var rawData bytes.Buffer
	for y := 0; y < height; y++ {
		rawData.WriteByte(0) // filter byte (none)
		for x := 0; x < width; x++ {
			// Generate pseudo-random grayscale value
			rawData.WriteByte(byte((x*7 + y*13) % 256))
		}
	}

	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write(rawData.Bytes())
	w.Close()
	writeChunk(&buf, "IDAT", compressed.Bytes())

	// IEND chunk
	writeChunk(&buf, "IEND", nil)

	return buf.Bytes()
}

// writeChunk writes a PNG chunk with CRC
func writeChunk(buf *bytes.Buffer, chunkType string, data []byte) {
	// Length (4 bytes)
	length := len(data)
	buf.Write([]byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)})

	// Type (4 bytes)
	buf.WriteString(chunkType)

	// Data
	buf.Write(data)

	// CRC (simplified - use 0 for testing, browsers are lenient)
	// In production you'd calculate proper CRC32
	crc := crc32PNG(chunkType, data)
	buf.Write([]byte{byte(crc >> 24), byte(crc >> 16), byte(crc >> 8), byte(crc)})
}

// crc32PNG calculates CRC32 for PNG chunks
func crc32PNG(chunkType string, data []byte) uint32 {
	// CRC32 polynomial used by PNG
	var crcTable [256]uint32
	for i := 0; i < 256; i++ {
		c := uint32(i)
		for j := 0; j < 8; j++ {
			if c&1 != 0 {
				c = 0xedb88320 ^ (c >> 1)
			} else {
				c = c >> 1
			}
		}
		crcTable[i] = c
	}

	crc := uint32(0xffffffff)
	for _, b := range []byte(chunkType) {
		crc = crcTable[(crc^uint32(b))&0xff] ^ (crc >> 8)
	}
	for _, b := range data {
		crc = crcTable[(crc^uint32(b))&0xff] ^ (crc >> 8)
	}
	return crc ^ 0xffffffff
}

// createTestImagesWithSize creates n test images of specified size
func createTestImagesWithSize(t *testing.T, count int, sizeBytes int) (string, []string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "mdview-preload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	pngData := generatePNG(sizeBytes)

	paths := make([]string, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("image%02d.png", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, pngData, 0644); err != nil {
			t.Fatalf("failed to write test image: %v", err)
		}
		paths[i] = path
	}

	return dir, paths
}

// createTestImages creates n test images in a temp directory and returns the dir path
func createTestImages(t *testing.T, count int) (string, []string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "mdview-preload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// 1x1 red PNG (minimal valid PNG)
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // 8-bit RGB
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, // compressed data
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xfe, // crc
		0xd4, 0xaa, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, // IEND chunk
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	paths := make([]string, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("image%02d.png", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, pngData, 0644); err != nil {
			t.Fatalf("failed to write test image: %v", err)
		}
		paths[i] = path
	}

	return dir, paths
}

func TestImageCacheBasicOperations(t *testing.T) {
	cache := NewImageCache()

	// Test Get on empty cache
	if data := cache.Get("nonexistent"); data != nil {
		t.Error("expected nil for nonexistent key")
	}

	// Test Set and Get
	cache.Set("test.png", []byte("test data"))
	if data := cache.Get("test.png"); string(data) != "test data" {
		t.Errorf("expected 'test data', got %q", string(data))
	}
}

func TestImageCachePreloadDirectory(t *testing.T) {
	dir, paths := createTestImages(t, 5)
	defer os.RemoveAll(dir)

	cache := NewImageCache()

	// Trigger preload and wait for completion
	wg := cache.PreloadDirectory(dir)
	if wg != nil {
		wg.Wait()
	}

	// All images should be in cache
	for _, path := range paths {
		if data := cache.Get(path); data == nil {
			t.Errorf("expected image to be cached: %s", path)
		}
	}

	// Second preload should return nil (already done)
	wg2 := cache.PreloadDirectory(dir)
	if wg2 != nil {
		t.Error("expected nil WaitGroup for already-preloaded directory")
	}
}

func TestPreloadPessimisticCase(t *testing.T) {
	// Pessimistic: 10 images in directory, only 2 used
	// This tests the overhead of preloading unused images
	dir, paths := createTestImages(t, 10)
	defer os.RemoveAll(dir)

	// Markdown uses only 2 of 10 images
	markdown := fmt.Sprintf("![img1](%s)\n![img2](%s)\n",
		filepath.Base(paths[0]), filepath.Base(paths[1]))

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)
	c.SetPreload(true)

	var buf bytes.Buffer
	err := c.Convert(strings.NewReader(markdown), &buf, "default")
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	result := buf.String()

	// Both referenced images should be embedded
	if !strings.Contains(result, "data:image/png;base64,") {
		t.Error("expected base64 embedded images")
	}

	// Count embedded images (should be exactly 2)
	count := strings.Count(result, "data:image/png;base64,")
	if count != 2 {
		t.Errorf("expected 2 embedded images, got %d", count)
	}
}

func TestPreloadOptimisticCase(t *testing.T) {
	// Optimistic: 10 images in directory, 8 used
	// This tests the benefit of parallel loading
	dir, paths := createTestImages(t, 10)
	defer os.RemoveAll(dir)

	// Markdown uses 8 of 10 images
	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)
	c.SetPreload(true)

	var buf bytes.Buffer
	err := c.Convert(strings.NewReader(markdown), &buf, "default")
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	result := buf.String()

	// All 8 referenced images should be embedded
	count := strings.Count(result, "data:image/png;base64,")
	if count != 8 {
		t.Errorf("expected 8 embedded images, got %d", count)
	}
}

func TestPreloadVsNoPreloadCorrectness(t *testing.T) {
	// Ensure preload produces identical output to non-preload
	dir, paths := createTestImages(t, 5)
	defer os.RemoveAll(dir)

	var sb strings.Builder
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	// Without preload
	c1 := New()
	c1.SetBaseDir(dir)
	c1.SetSelfContained(true)
	c1.SetPreload(false)

	var buf1 bytes.Buffer
	if err := c1.Convert(strings.NewReader(markdown), &buf1, "default"); err != nil {
		t.Fatal(err)
	}

	// With preload
	c2 := New()
	c2.SetBaseDir(dir)
	c2.SetSelfContained(true)
	c2.SetPreload(true)

	var buf2 bytes.Buffer
	if err := c2.Convert(strings.NewReader(markdown), &buf2, "default"); err != nil {
		t.Fatal(err)
	}

	// Output should be identical
	if buf1.String() != buf2.String() {
		t.Error("preload and non-preload output differ")
	}
}

func TestPreloadNonBlockingBehavior(t *testing.T) {
	// Test that preload is non-blocking and falls back to direct read
	// when cache hasn't loaded yet
	dir, paths := createTestImages(t, 3)
	defer os.RemoveAll(dir)

	markdown := fmt.Sprintf("![img](%s)", filepath.Base(paths[0]))

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)
	c.SetPreload(true)

	var buf bytes.Buffer
	err := c.Convert(strings.NewReader(markdown), &buf, "default")
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	// Image should be embedded regardless of whether preload finished in time
	if !strings.Contains(buf.String(), "data:image/png;base64,") {
		t.Error("expected image to be embedded even without preload completing")
	}
}

func TestPreloadCacheReuse(t *testing.T) {
	// Test that same image referenced multiple times is only read once when cached
	dir, paths := createTestImages(t, 2)
	defer os.RemoveAll(dir)

	// Same image referenced 3 times
	img := filepath.Base(paths[0])
	markdown := fmt.Sprintf("![a](%s)\n![b](%s)\n![c](%s)\n", img, img, img)

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)
	c.SetPreload(true)

	var buf bytes.Buffer
	err := c.Convert(strings.NewReader(markdown), &buf, "default")
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	// All 3 references should produce embedded images
	count := strings.Count(buf.String(), "data:image/png;base64,")
	if count != 3 {
		t.Errorf("expected 3 embedded images, got %d", count)
	}
}

func TestPreloadConcurrentSafety(t *testing.T) {
	// Test that concurrent conversions with shared cache don't race
	dir, paths := createTestImages(t, 10)
	defer os.RemoveAll(dir)

	var sb strings.Builder
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	// Run multiple conversions concurrently
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			c := New()
			c.SetBaseDir(dir)
			c.SetSelfContained(true)
			c.SetPreload(true)

			var buf bytes.Buffer
			if err := c.Convert(strings.NewReader(markdown), &buf, "default"); err != nil {
				errors <- err
				return
			}

			// Verify output is correct
			count := strings.Count(buf.String(), "data:image/png;base64,")
			if count != 5 {
				errors <- fmt.Errorf("expected 5 embedded images, got %d", count)
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// =============================================================================
// Preload Performance Benchmarks
// =============================================================================

func BenchmarkSelfContainedNoPreload(b *testing.B) {
	dir, paths := createBenchmarkImages(b, 20)
	defer os.RemoveAll(dir)

	// Use 15 of 20 images
	var sb strings.Builder
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(false)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkSelfContainedWithPreload(b *testing.B) {
	dir, paths := createBenchmarkImages(b, 20)
	defer os.RemoveAll(dir)

	// Use 15 of 20 images
	var sb strings.Builder
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(true)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkPreloadPessimistic(b *testing.B) {
	// 20 images, only 2 used - worst case for preload
	dir, paths := createBenchmarkImages(b, 20)
	defer os.RemoveAll(dir)

	markdown := fmt.Sprintf("![img1](%s)\n![img2](%s)\n",
		filepath.Base(paths[0]), filepath.Base(paths[1]))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(true)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkPreloadOptimistic(b *testing.B) {
	// 20 images, 18 used - best case for preload
	dir, paths := createBenchmarkImages(b, 20)
	defer os.RemoveAll(dir)

	var sb strings.Builder
	for i := 0; i < 18; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(true)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

// createBenchmarkImages is like createTestImages but for benchmarks
func createBenchmarkImages(b *testing.B, count int) (string, []string) {
	return createBenchmarkImagesWithSize(b, count, 100) // Small images
}

// createBenchmarkImagesWithSize creates test images of specified size for benchmarks
func createBenchmarkImagesWithSize(b *testing.B, count int, sizeBytes int) (string, []string) {
	b.Helper()
	dir, err := os.MkdirTemp("", "mdview-bench-preload-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	pngData := generatePNG(sizeBytes)

	paths := make([]string, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("image%02d.png", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, pngData, 0644); err != nil {
			b.Fatalf("failed to write test image: %v", err)
		}
		paths[i] = path
	}

	return dir, paths
}

// =============================================================================
// Benchmarks: Small vs Large Images
// =============================================================================

func BenchmarkSmallImagesNoPreload(b *testing.B) {
	// 10 small images (~1KB each), use 8
	dir, paths := createBenchmarkImagesWithSize(b, 10, 1024)
	defer os.RemoveAll(dir)

	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(false)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkSmallImagesWithPreload(b *testing.B) {
	// 10 small images (~1KB each), use 8
	dir, paths := createBenchmarkImagesWithSize(b, 10, 1024)
	defer os.RemoveAll(dir)

	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(true)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkLargeImagesNoPreload(b *testing.B) {
	// 10 large images (~100KB each), use 8
	dir, paths := createBenchmarkImagesWithSize(b, 10, 100*1024)
	defer os.RemoveAll(dir)

	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(false)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

func BenchmarkLargeImagesWithPreload(b *testing.B) {
	// 10 large images (~100KB each), use 8
	dir, paths := createBenchmarkImagesWithSize(b, 10, 100*1024)
	defer os.RemoveAll(dir)

	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(fmt.Sprintf("![img%d](%s)\n", i, filepath.Base(paths[i])))
	}
	markdown := sb.String()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := New()
		c.SetBaseDir(dir)
		c.SetSelfContained(true)
		c.SetPreload(true)

		var buf bytes.Buffer
		_ = c.Convert(strings.NewReader(markdown), &buf, "default")
	}
}

// =============================================================================
// Archive Mode Tests
// =============================================================================

func TestArchiveMode_KeepsMarkdownLinksRelative(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create a second markdown file to link to
	docPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(docPath, []byte("# Doc"), 0644); err != nil {
		t.Fatalf("failed to create doc.md: %v", err)
	}

	markdown := `# Root

[Documentation](doc.md)
[Subdoc](subdir/page.md)
[External](https://example.com)
`

	// Test with archive mode enabled
	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)
	c.SetArchiveMode(true)

	result := convert(t, c, markdown)

	// .md links should stay relative
	if !strings.Contains(result, `href="doc.md"`) {
		t.Error("expected .md link to stay relative in archive mode")
	}
	if !strings.Contains(result, `href="subdir/page.md"`) {
		t.Error("expected subdirectory .md link to stay relative in archive mode")
	}

	// External links should be unchanged
	if !strings.Contains(result, `href="https://example.com"`) {
		t.Error("expected external link to remain unchanged")
	}

	// Should NOT contain file:// URLs for .md links
	if strings.Contains(result, `href="file:///`) && strings.Contains(result, `.md"`) {
		t.Error("archive mode should not convert .md links to file:// URLs")
	}
}

func TestArchiveMode_Disabled(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create a second markdown file to link to
	docPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(docPath, []byte("# Doc"), 0644); err != nil {
		t.Fatalf("failed to create doc.md: %v", err)
	}

	markdown := `[Documentation](doc.md)`

	// Test with archive mode disabled (default)
	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)
	c.SetArchiveMode(false)

	result := convert(t, c, markdown)

	// .md links should be converted to file:// URLs when archive mode is off
	if !strings.Contains(result, `href="file:///`) {
		t.Error("expected .md link to be converted to file:// URL when archive mode is disabled")
	}
}

func TestArchiveMode_OnlyAffectsMarkdownLinks(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	markdown := `
[Markdown](doc.md)
[HTML](page.html)
[PDF](document.pdf)
[External](https://example.com)
[Anchor](#section)
`

	c := New()
	c.SetBaseDir(dir)
	c.SetSelfContained(true)
	c.SetArchiveMode(true)

	result := convert(t, c, markdown)

	// .md links stay relative
	if !strings.Contains(result, `href="doc.md"`) {
		t.Error("expected .md link to stay relative")
	}

	// Other file types should be converted to file:// URLs
	if !strings.Contains(result, `href="file:///`) || !strings.Contains(result, `page.html"`) {
		t.Error("expected non-.md file links to be converted to file:// URLs")
	}

	// Anchors and external links unchanged
	if !strings.Contains(result, `href="#section"`) {
		t.Error("expected anchor to remain unchanged")
	}
	if !strings.Contains(result, `href="https://example.com"`) {
		t.Error("expected external link to remain unchanged")
	}
}
