package archive

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// linkCollector walks the AST and collects links
type linkCollector struct {
	links   []string
	baseDir string
}

// ScanMarkdownLinks extracts all local .md file links from markdown content
func ScanMarkdownLinks(content []byte, baseDir string) ([]string, error) {
	// Create a goldmark parser with GFM support
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)

	// Parse markdown to AST
	reader := text.NewReader(content)
	doc := md.Parser().Parse(reader)

	// Collect links from AST
	collector := &linkCollector{
		links:   []string{},
		baseDir: baseDir,
	}
	ast.Walk(doc, collector.visit)

	// Also scan raw HTML blocks with regex (fallback for HTML links)
	htmlLinks := scanHTMLLinks(content, baseDir)
	collector.links = append(collector.links, htmlLinks...)

	// Deduplicate
	return deduplicateLinks(collector.links), nil
}

// visit is called for each AST node
func (lc *linkCollector) visit(node ast.Node, entering bool) (ast.WalkStatus, error) {
	// Only process when entering nodes
	if !entering {
		return ast.WalkContinue, nil
	}

	// Check if it's a link node
	if link, ok := node.(*ast.Link); ok {
		dest := string(link.Destination)
		if absPath := lc.processLink(dest); absPath != "" {
			lc.links = append(lc.links, absPath)
		}
	}

	return ast.WalkContinue, nil
}

// processLink checks if a link is a local .md file and returns its absolute path
func (lc *linkCollector) processLink(href string) string {
	// Skip empty links
	if href == "" {
		return ""
	}

	// Skip anchor-only links
	if strings.HasPrefix(href, "#") {
		return ""
	}

	// Handle file:// URLs first (before general protocol check)
	if strings.HasPrefix(href, "file:///") {
		href = strings.TrimPrefix(href, "file:///")
		href = filepath.FromSlash(href)

		// Strip fragment and query string
		href = strings.Split(href, "#")[0]
		href = strings.Split(href, "?")[0]

		// Check if it's a .md file
		if !strings.HasSuffix(strings.ToLower(href), ".md") {
			return ""
		}

		return filepath.Clean(href)
	}

	// Skip external protocols
	if strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") ||
		strings.HasPrefix(href, "ftp://") ||
		strings.Contains(href, "://") {
		return ""
	}

	// Strip fragment and query string
	href = strings.Split(href, "#")[0]
	href = strings.Split(href, "?")[0]

	// Check if it's a .md file
	if !strings.HasSuffix(strings.ToLower(href), ".md") {
		return ""
	}

	// Resolve relative path to absolute
	absPath := filepath.Join(lc.baseDir, href)
	absPath = filepath.Clean(absPath)

	return absPath
}

// scanHTMLLinks uses regex to find links in raw HTML blocks
var hrefPattern = regexp.MustCompile(`href=["']([^"']+)["']`)

func scanHTMLLinks(content []byte, baseDir string) []string {
	links := []string{}
	matches := hrefPattern.FindAllSubmatch(content, -1)

	lc := &linkCollector{baseDir: baseDir}

	for _, match := range matches {
		if len(match) >= 2 {
			href := string(match[1])
			if absPath := lc.processLink(href); absPath != "" {
				links = append(links, absPath)
			}
		}
	}

	return links
}

// deduplicateLinks removes duplicate paths
func deduplicateLinks(links []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, link := range links {
		if !seen[link] {
			seen[link] = true
			result = append(result, link)
		}
	}

	return result
}

// HasMarkdownLinks checks if a markdown file contains any links to local .md files
func HasMarkdownLinks(mdPath string) (bool, error) {
	content, err := os.ReadFile(mdPath)
	if err != nil {
		return false, err
	}

	baseDir := filepath.Dir(mdPath)
	links, err := ScanMarkdownLinks(content, baseDir)
	if err != nil {
		return false, err
	}

	return len(links) > 0, nil
}
