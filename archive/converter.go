package archive

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mdview/converter"
	"mdview/templates"
)

//go:embed pako.min.js
var pakoJS string

//go:embed navigation.js
var navigationJS string

// ArchiveConverter handles conversion of a graph of markdown files to a single HTML archive
type ArchiveConverter struct {
	graph         *Graph
	templateName  string
	selfContained bool
	preload       bool
	title         string
}

// NewConverter creates a new ArchiveConverter
func NewConverter(graph *Graph, templateName string, selfContained bool, preload bool, title string) *ArchiveConverter {
	return &ArchiveConverter{
		graph:         graph,
		templateName:  templateName,
		selfContained: selfContained,
		preload:       preload,
		title:         title,
	}
}

// ConvertToArchive converts all pages in the graph and generates a single self-contained HTML archive
func (ac *ArchiveConverter) ConvertToArchive(outputPath string) error {
	// Convert each page to HTML and compress
	archiveData := make(map[string]string)

	for _, node := range ac.graph.OrderedNodes() {
		// Convert to HTML (no title for embedded pages)
		htmlContent, err := ac.convertPage(node.Path, "")
		if err != nil {
			return fmt.Errorf("failed to convert %s: %w", node.Path, err)
		}

		// Compress with gzip
		compressed, err := compressData(htmlContent)
		if err != nil {
			return fmt.Errorf("failed to compress %s: %w", node.Path, err)
		}

		// Base64 encode
		encoded := base64.StdEncoding.EncodeToString(compressed)

		// Store with relative path as key
		archiveData[node.RelativePath] = encoded
	}

	// Get root HTML content (full document structure)
	rootHTML, err := ac.convertRootPage(ac.graph.Root)
	if err != nil {
		return fmt.Errorf("failed to convert root page: %w", err)
	}

	// Generate archive resources (overlay HTML, CSS, JS, archive data)
	archiveResources := ac.generateArchiveResources(archiveData)

	// Inject archive resources before closing </body> tag
	finalHTML := injectBeforeClosingTag(rootHTML, "</body>", archiveResources)

	// Write to output file
	return os.WriteFile(outputPath, []byte(finalHTML), 0644)
}

// convertPage converts a single markdown file to HTML content (just the <article> content)
func (ac *ArchiveConverter) convertPage(mdPath string, title string) ([]byte, error) {
	// Open markdown file
	mdFile, err := os.Open(mdPath)
	if err != nil {
		return nil, err
	}
	defer mdFile.Close()

	// Get file size for buffer pre-allocation
	var fileSize int64
	if stat, err := mdFile.Stat(); err == nil {
		fileSize = stat.Size()
	}

	// Create converter
	conv := converter.New()
	conv.SetBaseDir(filepath.Dir(mdPath))
	conv.SetSelfContained(ac.selfContained)
	conv.SetPreload(ac.preload)
	conv.SetArchiveMode(true) // Convert .md links to javascript:mdviewLoadPage() calls
	conv.SetArchiveRootDir(filepath.Dir(ac.graph.Root)) // Root directory for computing archive-relative paths
	if title != "" {
		conv.SetTitle(title)
	}

	// Convert to HTML
	var htmlBuf bytes.Buffer
	if err := conv.ConvertWithSize(mdFile, &htmlBuf, ac.templateName, fileSize); err != nil {
		return nil, err
	}

	return htmlBuf.Bytes(), nil
}

// convertRootPage converts the root markdown file to a complete HTML document
func (ac *ArchiveConverter) convertRootPage(mdPath string) (string, error) {
	htmlBytes, err := ac.convertPage(mdPath, ac.title)
	if err != nil {
		return "", err
	}
	return string(htmlBytes), nil
}

// generateArchiveResources creates archive resources (JS and data for navigation)
func (ac *ArchiveConverter) generateArchiveResources(archiveData map[string]string) string {
	var sb strings.Builder

	// 1. Add pako.js for decompression
	sb.WriteString("\n<!-- mdview archive -->\n")
	sb.WriteString("<script>\n")
	sb.WriteString(pakoJS)
	sb.WriteString("\n</script>\n\n")

	// 2. Add archive data
	sb.WriteString("<script>\n")
	sb.WriteString("// mdview archive data - compressed pages\n")
	sb.WriteString("window.mdviewArchive = {\n")
	sb.WriteString("  pages: {\n")

	// Add each page
	first := true
	for relPath, encodedData := range archiveData {
		if !first {
			sb.WriteString(",\n")
		}
		first = false

		// Normalize path to forward slashes (must match how links are generated in converter)
		normalizedPath := strings.ReplaceAll(relPath, "\\", "/")
		// Escape for JavaScript string literal
		escapedPath := strings.ReplaceAll(normalizedPath, "\"", "\\\"")

		sb.WriteString(fmt.Sprintf("    \"%s\": \"%s\"", escapedPath, encodedData))
	}

	sb.WriteString("\n  },\n")

	// Add root path (normalized with forward slashes for consistency)
	rootPath := strings.ReplaceAll(ac.graph.Root, "\\", "/")
	escapedRoot := strings.ReplaceAll(rootPath, "\"", "\\\"")
	sb.WriteString(fmt.Sprintf("  root: \"%s\"\n", escapedRoot))

	sb.WriteString("};\n")
	sb.WriteString("</script>\n\n")

	// 3. Add navigation.js
	sb.WriteString("<script>\n")
	sb.WriteString(navigationJS)
	sb.WriteString("\n</script>\n")

	return sb.String()
}

// compressData compresses data using gzip
func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// injectBeforeClosingTag finds the last occurrence of a closing tag and injects content before it
func injectBeforeClosingTag(html, closingTag, content string) string {
	index := strings.LastIndex(html, closingTag)
	if index == -1 {
		// If tag not found, just append
		return html + content
	}

	return html[:index] + content + html[index:]
}

// ConvertToArchiveWithTemplate is a convenience function that loads the template and converts
func ConvertToArchiveWithTemplate(graph *Graph, outputPath, templateName string, selfContained, preload bool, title string) error {
	// Validate template exists
	if _, err := templates.Get(templateName); err != nil {
		return fmt.Errorf("template error: %w", err)
	}

	// Create converter
	ac := NewConverter(graph, templateName, selfContained, preload, title)

	// Convert
	return ac.ConvertToArchive(outputPath)
}

// ExtractArticleContent extracts just the <article> content from a full HTML document
// This is used when embedding pages to strip the header/footer/scripts
func ExtractArticleContent(fullHTML []byte) []byte {
	html := string(fullHTML)

	// Find <article class="markdown-body">
	startTag := "<article class=\"markdown-body\">"
	endTag := "</article>"

	startIdx := strings.Index(html, startTag)
	if startIdx == -1 {
		// Fallback: return everything between <body> and </body>
		bodyStart := strings.Index(html, "<body>")
		bodyEnd := strings.Index(html, "</body>")
		if bodyStart != -1 && bodyEnd != -1 {
			return []byte(html[bodyStart+6 : bodyEnd])
		}
		return fullHTML
	}

	endIdx := strings.Index(html[startIdx:], endTag)
	if endIdx == -1 {
		return fullHTML
	}

	// Include the opening and closing tags
	content := html[startIdx : startIdx+endIdx+len(endTag)]
	return []byte(content)
}

// WriteArchive is a high-level function that builds a graph and converts it to an archive
func WriteArchive(rootPath, outputPath, templateName string, maxPages int, selfContained, preload bool) error {
	// Build graph
	graph, err := BuildGraph(rootPath, maxPages)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	fmt.Printf("Building archive with %d pages...\n", graph.Count)

	// Extract title from output filename (without extension)
	outputBase := filepath.Base(outputPath)
	title := strings.TrimSuffix(outputBase, filepath.Ext(outputBase))

	// Convert to archive
	return ConvertToArchiveWithTemplate(graph, outputPath, templateName, selfContained, preload, title)
}
