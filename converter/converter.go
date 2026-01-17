package converter

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	htmlpkg "html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"mdview/templates"
)

// ImageCache holds preloaded image data for faster embedding
type ImageCache struct {
	data            sync.Map // map[string][]byte - path -> file contents
	preloadedDirs   sync.Map // map[string]bool - directories already preloaded
	preloadingDirs  sync.Map // map[string]*sync.WaitGroup - directories currently preloading
}

// NewImageCache creates a new image cache
func NewImageCache() *ImageCache {
	return &ImageCache{}
}

// Get retrieves cached image data, returns nil if not cached
func (c *ImageCache) Get(path string) []byte {
	if val, ok := c.data.Load(path); ok {
		return val.([]byte)
	}
	return nil
}

// Set stores image data in the cache
func (c *ImageCache) Set(path string, data []byte) {
	c.data.Store(path, data)
}

// PreloadDirectory loads all images from a directory into the cache asynchronously.
// Returns a WaitGroup that completes when preloading is done.
func (c *ImageCache) PreloadDirectory(dir string) *sync.WaitGroup {
	// Check if already preloaded
	if _, loaded := c.preloadedDirs.Load(dir); loaded {
		return nil
	}

	// Check if currently preloading, return existing WaitGroup
	if wg, loading := c.preloadingDirs.Load(dir); loading {
		return wg.(*sync.WaitGroup)
	}

	// Start new preload
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Try to claim this directory for preloading
	if _, loaded := c.preloadingDirs.LoadOrStore(dir, wg); loaded {
		// Another goroutine beat us to it
		if existing, ok := c.preloadingDirs.Load(dir); ok {
			return existing.(*sync.WaitGroup)
		}
		return nil
	}

	go func() {
		defer wg.Done()
		defer c.preloadedDirs.Store(dir, true)
		defer c.preloadingDirs.Delete(dir)

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		// Load all image files in parallel
		var loadWg sync.WaitGroup
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			mimeType := getMimeTypeFromExtension(path)
			if mimeType == "" {
				continue // Not an image
			}

			loadWg.Add(1)
			go func(p string) {
				defer loadWg.Done()
				data, err := os.ReadFile(p)
				if err == nil {
					c.Set(p, data)
				}
			}(path)
		}
		loadWg.Wait()
	}()

	return wg
}

const (
	// Default read chunk size for streaming reads
	chunkSize = 32 * 1024 // 32KB chunks
	// Default initial buffer size when file size is unknown
	defaultBufSize = 64 * 1024 // 64KB
)

// Buffer pool to reuse byte buffers across conversions
var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, defaultBufSize))
	},
}

// Converter handles markdown to HTML conversion with streaming output
type Converter struct {
	baseDir           string      // Base directory for resolving relative paths
	selfContained     bool        // Embed images as base64 data URIs instead of file:// URLs
	preload           bool        // Preload all images in a directory when first image is referenced
	archiveMode       bool        // Keep .md links as relative paths for archive navigation
	archiveRootDir    string      // Root directory of the archive (for computing relative paths)
	imageCache        *ImageCache // Cache for preloaded images (only used when preload is enabled)
	title             string      // Custom page title (replaces template default)
}

// Regex patterns for finding src and href attributes in raw HTML
var (
	srcPattern  = regexp.MustCompile(`(src=["'])([^"']+)(["'])`)
	hrefPattern = regexp.MustCompile(`(href=["'])([^"']+)(["'])`)
	// Pattern for CSS url() references: url("path"), url('path'), or url(path)
	cssURLPattern = regexp.MustCompile(`(url\(["']?)([^"')]+)(["']?\))`)
	// Pattern for anchor tags without target attribute (to add target="_blank")
	anchorNoTargetPattern = regexp.MustCompile(`(<a\s+[^>]*href=["'])([^"']+)(["'][^>]*)(>)`)
	// Pattern for HTML title tag
	titlePattern = regexp.MustCompile(`<title>[^<]*</title>`)
)

// New creates a new Converter instance
func New() *Converter {
	return &Converter{}
}

// SetBaseDir sets the base directory for resolving relative paths in the output.
// Relative paths in src and href attributes will be converted to absolute file:// URLs.
func (c *Converter) SetBaseDir(dir string) {
	c.baseDir = dir
}

// SetSelfContained enables embedding images as base64 data URIs instead of file:// URLs.
// When enabled, local images referenced in the markdown will be read and embedded directly
// in the HTML output, making the file fully self-contained and portable.
func (c *Converter) SetSelfContained(enabled bool) {
	c.selfContained = enabled
}

// SetPreload enables preloading all images in a directory when the first image from
// that directory is referenced. This can significantly speed up self-contained output
// for documents with many images in the same directory by loading them in parallel.
func (c *Converter) SetPreload(enabled bool) {
	c.preload = enabled
	if enabled && c.imageCache == nil {
		c.imageCache = NewImageCache()
	}
}

// SetArchiveMode enables archive mode where .md links are converted to
// javascript:mdviewLoadPage('...') calls with archive-relative paths.
func (c *Converter) SetArchiveMode(enabled bool) {
	c.archiveMode = enabled
}

// SetArchiveRootDir sets the root directory of the archive for computing
// relative paths that match the archive keys.
func (c *Converter) SetArchiveRootDir(dir string) {
	c.archiveRootDir = dir
}

// SetTitle sets a custom page title for the HTML output.
// If not set, the template's default title will be used.
func (c *Converter) SetTitle(title string) {
	c.title = title
}

// createMarkdown builds a goldmark instance with appropriate settings
func (c *Converter) createMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown
			extension.Typographer,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
		goldmark.WithRenderer(
			renderer.NewRenderer(
				renderer.WithNodeRenderers(
					util.Prioritized(html.NewRenderer(
						html.WithHardWraps(),
						html.WithXHTML(),
						html.WithUnsafe(),
					), 1000),
					util.Prioritized(&pathRenderer{
						baseDir:        c.baseDir,
						selfContained:  c.selfContained,
						preload:        c.preload,
						archiveMode:    c.archiveMode,
						archiveRootDir: c.archiveRootDir,
						imageCache:     c.imageCache,
					}, 100), // Higher priority (lower number) for our custom renderer
				),
			),
		),
	)
}

// Convert reads markdown from the reader and writes HTML to the writer.
// It streams the output as it generates HTML, minimizing memory usage.
// Use ConvertWithSize if you know the input size for better memory efficiency.
func (c *Converter) Convert(reader io.Reader, writer io.Writer, templateName string) error {
	return c.ConvertWithSize(reader, writer, templateName, 0)
}

// ConvertWithSize reads markdown and writes HTML, with a size hint for buffer pre-allocation.
// If sizeHint is 0 or negative, a default buffer size is used.
// The size hint allows pre-allocating the exact buffer size needed, avoiding reallocations.
func (c *Converter) ConvertWithSize(reader io.Reader, writer io.Writer, templateName string, sizeHint int64) error {
	// Get template
	tmpl, err := templates.Get(templateName)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	// Use buffered writer for efficient streaming output
	bufWriter := bufio.NewWriter(writer)

	// Write HTML header
	if err := c.writeHeader(bufWriter, tmpl); err != nil {
		return err
	}

	// Read markdown content using pooled buffer
	source, err := c.readSource(reader, sizeHint)
	if err != nil {
		return fmt.Errorf("failed to read markdown: %w", err)
	}

	// Create markdown converter with current settings (handles images during rendering)
	md := c.createMarkdown()

	// Convert markdown to HTML - buffer for href rewriting (images handled inline)
	var htmlBuf bytes.Buffer
	convertErr := md.Convert(source, &htmlBuf)

	// Release source buffer back to pool immediately after conversion
	c.releaseBuffer(source)

	if convertErr != nil {
		return fmt.Errorf("failed to convert markdown: %w", convertErr)
	}

	// Write the HTML content (all paths handled during rendering)
	if _, err := io.WriteString(bufWriter, htmlBuf.String()); err != nil {
		return err
	}

	// Write HTML footer
	if err := c.writeFooter(bufWriter, tmpl); err != nil {
		return err
	}

	return bufWriter.Flush()
}

// pathRenderer is a custom goldmark renderer for images and links that handles
// path resolution and base64 embedding during the rendering pass
type pathRenderer struct {
	baseDir        string
	selfContained  bool
	preload        bool
	archiveMode    bool
	archiveRootDir string
	imageCache     *ImageCache
}

// RegisterFuncs implements renderer.NodeRenderer
func (r *pathRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindHTMLBlock, r.renderRawHTML)
	reg.Register(ast.KindRawHTML, r.renderRawHTML)
}

// renderImage handles image rendering with path resolution or base64 embedding
func (r *pathRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*ast.Image)
	dest := string(n.Destination)

	// Process the image path
	finalDest := r.processImagePath(dest)

	// Write the img tag
	_, _ = w.WriteString("<img src=\"")
	_, _ = w.WriteString(htmlpkg.EscapeString(finalDest))
	_, _ = w.WriteString("\"")

	// Write alt text
	_, _ = w.WriteString(" alt=\"")
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if text, ok := child.(*ast.Text); ok {
			_, _ = w.Write(text.Segment.Value(source))
		}
	}
	_, _ = w.WriteString("\"")

	// Write title if present
	if n.Title != nil {
		_, _ = w.WriteString(" title=\"")
		_, _ = w.WriteString(htmlpkg.EscapeString(string(n.Title)))
		_, _ = w.WriteString("\"")
	}

	_, _ = w.WriteString(" />")

	return ast.WalkSkipChildren, nil
}

// renderLink handles link rendering with path resolution
func (r *pathRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)

	if entering {
		dest := string(n.Destination)
		finalDest := r.processLinkPath(dest)

		_, _ = w.WriteString("<a href=\"")
		_, _ = w.WriteString(htmlpkg.EscapeString(finalDest))
		_, _ = w.WriteString("\"")

		// Open external links in new tab (not javascript: or anchors)
		if !strings.HasPrefix(finalDest, "javascript:") && !strings.HasPrefix(finalDest, "#") {
			_, _ = w.WriteString(" target=\"_blank\"")
		}

		if n.Title != nil {
			_, _ = w.WriteString(" title=\"")
			_, _ = w.WriteString(htmlpkg.EscapeString(string(n.Title)))
			_, _ = w.WriteString("\"")
		}
		_, _ = w.WriteString(">")
	} else {
		_, _ = w.WriteString("</a>")
	}

	return ast.WalkContinue, nil
}

// renderRawHTML processes raw HTML blocks/inline elements to rewrite paths
func (r *pathRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	var content string
	switch n := node.(type) {
	case *ast.HTMLBlock:
		var buf bytes.Buffer
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			buf.Write(line.Value(source))
		}
		content = buf.String()
	case *ast.RawHTML:
		var buf bytes.Buffer
		for i := 0; i < n.Segments.Len(); i++ {
			segment := n.Segments.At(i)
			buf.Write(segment.Value(source))
		}
		content = buf.String()
	default:
		return ast.WalkContinue, nil
	}

	// Process paths in the raw HTML content
	content = r.processRawHTMLContent(content)
	_, _ = w.WriteString(content)

	return ast.WalkSkipChildren, nil
}

// processRawHTMLContent handles src, href, and CSS url() in raw HTML
func (r *pathRenderer) processRawHTMLContent(content string) string {
	// Process src attributes
	content = srcPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatches := srcPattern.FindStringSubmatch(match)
		if len(submatches) != 4 {
			return match
		}
		prefix, path, suffix := submatches[1], submatches[2], submatches[3]
		return prefix + r.processImagePath(path) + suffix
	})

	// Process href attributes and add target="_blank" for external links
	content = anchorNoTargetPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatches := anchorNoTargetPattern.FindStringSubmatch(match)
		if len(submatches) != 5 {
			return match
		}
		prefix, path, middle, suffix := submatches[1], submatches[2], submatches[3], submatches[4]
		processedPath := r.processLinkPath(path)

		// Add target="_blank" for non-javascript, non-anchor links (if not already has target)
		if !strings.HasPrefix(processedPath, "javascript:") &&
			!strings.HasPrefix(processedPath, "#") &&
			!strings.Contains(middle, "target=") {
			return prefix + processedPath + middle + " target=\"_blank\"" + suffix
		}
		return prefix + processedPath + middle + suffix
	})

	// Process CSS url() references if self-contained
	if r.selfContained {
		content = cssURLPattern.ReplaceAllStringFunc(content, func(match string) string {
			submatches := cssURLPattern.FindStringSubmatch(match)
			if len(submatches) != 4 {
				return match
			}
			prefix, path, suffix := submatches[1], submatches[2], submatches[3]
			processed := r.processCSSAssetPath(path)
			return prefix + processed + suffix
		})
	}

	return content
}

// processCSSAssetPath handles path resolution or base64 embedding for CSS assets
func (r *pathRenderer) processCSSAssetPath(path string) string {
	if strings.HasPrefix(path, "data:") ||
		strings.HasPrefix(path, "#") ||
		strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") {
		return path
	}

	var absPath string
	if strings.HasPrefix(path, "file:///") {
		absPath = strings.TrimPrefix(path, "file:///")
		absPath = filepath.FromSlash(absPath)
	} else if strings.Contains(path, "://") {
		return path
	} else if r.baseDir != "" {
		absPath = filepath.Join(r.baseDir, path)
	} else {
		return path
	}

	absPath = filepath.Clean(absPath)

	mimeType := getCSSAssetMimeType(absPath)
	if mimeType == "" {
		return path
	}

	assetData, err := os.ReadFile(absPath)
	if err != nil {
		return path
	}

	encoded := base64.StdEncoding.EncodeToString(assetData)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
}

// processLinkPath handles path resolution for links (no embedding, just file:// conversion)
func (r *pathRenderer) processLinkPath(path string) string {
	// In archive mode, convert ALL .md links to javascript:mdviewLoadPage() calls
	// This must happen FIRST, before any other checks, to catch file:// URLs too
	if r.archiveMode && r.archiveRootDir != "" && strings.HasSuffix(strings.ToLower(path), ".md") {
		var absPath string

		// Handle file:// URLs
		if strings.HasPrefix(path, "file:///") {
			absPath = strings.TrimPrefix(path, "file:///")
			absPath = filepath.FromSlash(absPath)
		} else if strings.Contains(path, "://") {
			// Other protocols with .md - skip (e.g., https://example.com/doc.md)
			return path
		} else if r.baseDir != "" {
			// Relative path - resolve against base directory
			absPath = filepath.Join(r.baseDir, path)
		} else {
			// No base dir, can't resolve
			return path
		}

		absPath = filepath.Clean(absPath)

		// Compute relative path from archive root directory
		relPath, err := filepath.Rel(r.archiveRootDir, absPath)
		if err != nil {
			// Fallback to filename only if rel fails
			relPath = filepath.Base(absPath)
		}

		// Normalize to forward slashes for consistency
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// Return javascript: href with the archive key
		return "javascript:mdviewLoadPage('" + relPath + "')"
	}

	// Skip if already absolute or special protocol
	if strings.Contains(path, "://") ||
		strings.HasPrefix(path, "#") ||
		strings.HasPrefix(path, "data:") ||
		strings.HasPrefix(path, "mailto:") ||
		strings.HasPrefix(path, "tel:") ||
		strings.HasPrefix(path, "javascript:") {
		return path
	}

	if r.baseDir == "" {
		return path
	}

	// Resolve relative path and convert to file:// URL
	absPath := filepath.Join(r.baseDir, path)
	absPath = filepath.Clean(absPath)
	return "file:///" + strings.ReplaceAll(absPath, "\\", "/")
}

// processImagePath handles path resolution or base64 embedding for an image
func (r *pathRenderer) processImagePath(path string) string {
	// Skip non-embeddable references
	if strings.HasPrefix(path, "#") ||
		strings.HasPrefix(path, "data:") ||
		strings.HasPrefix(path, "mailto:") ||
		strings.HasPrefix(path, "tel:") ||
		strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") {
		return path
	}

	var absPath string

	// Handle file:// URLs by extracting the local path
	if strings.HasPrefix(path, "file:///") {
		absPath = strings.TrimPrefix(path, "file:///")
		absPath = filepath.FromSlash(absPath)
	} else if strings.Contains(path, "://") {
		// Skip other protocols we don't recognize
		return path
	} else if r.baseDir != "" {
		// Relative path - resolve against base directory
		absPath = filepath.Join(r.baseDir, path)
	} else {
		return path
	}

	absPath = filepath.Clean(absPath)

	if r.selfContained {
		mimeType := getMimeTypeFromExtension(absPath)
		if mimeType != "" {
			var imageData []byte

			// Try cache first if preload is enabled
			if r.preload && r.imageCache != nil {
				dir := filepath.Dir(absPath)

				// Trigger async preload for this directory (non-blocking)
				// This benefits subsequent images from the same directory
				r.imageCache.PreloadDirectory(dir)

				// Check cache immediately - may hit if preload already ran
				imageData = r.imageCache.Get(absPath)
			}

			// Fall back to direct read if not in cache
			// This happens for first image (preload still running) or cache miss
			if imageData == nil {
				var err error
				imageData, err = os.ReadFile(absPath)
				if err != nil {
					// Fall through to file:// URL if read fails
					goto fileURL
				}
				// Store in cache for potential reuse (e.g., same image referenced twice)
				if r.preload && r.imageCache != nil {
					r.imageCache.Set(absPath, imageData)
				}
			}

			encoded := base64.StdEncoding.EncodeToString(imageData)
			return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
		}
		// Fall through to file:// URL if no mime type
	}

fileURL:
	// Convert to file:// URL
	fileURL := "file:///" + strings.ReplaceAll(absPath, "\\", "/")
	return fileURL
}

// readSource reads all content from reader into a pooled buffer.
func (c *Converter) readSource(reader io.Reader, sizeHint int64) ([]byte, error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	if sizeHint > 0 {
		buf.Grow(int(sizeHint))
	}

	chunk := make([]byte, chunkSize)
	for {
		n, err := reader.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			bufferPool.Put(buf)
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// releaseBuffer returns the buffer backing the byte slice to the pool.
func (c *Converter) releaseBuffer(source []byte) {
	if cap(source) > 0 {
		buf := bytes.NewBuffer(source[:0])
		bufferPool.Put(buf)
	}
}

// embedCSSAssets replaces url() references in CSS with base64 data URIs
func (c *Converter) embedCSSAssets(css string) string {
	return cssURLPattern.ReplaceAllStringFunc(css, func(match string) string {
		submatches := cssURLPattern.FindStringSubmatch(match)
		if len(submatches) != 4 {
			return match
		}

		prefix := submatches[1]
		path := submatches[2]
		suffix := submatches[3]

		// Skip non-embeddable references
		if strings.HasPrefix(path, "data:") ||
			strings.HasPrefix(path, "#") ||
			strings.HasPrefix(path, "http://") ||
			strings.HasPrefix(path, "https://") {
			return match
		}

		var absPath string

		if strings.HasPrefix(path, "file:///") {
			absPath = strings.TrimPrefix(path, "file:///")
			absPath = filepath.FromSlash(absPath)
		} else if strings.Contains(path, "://") {
			return match
		} else {
			absPath = filepath.Join(c.baseDir, path)
		}

		absPath = filepath.Clean(absPath)

		mimeType := getCSSAssetMimeType(absPath)
		if mimeType == "" {
			return match
		}

		assetData, err := os.ReadFile(absPath)
		if err != nil {
			return match
		}

		encoded := base64.StdEncoding.EncodeToString(assetData)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

		return prefix + dataURI + suffix
	})
}

// getMimeTypeFromExtension returns the MIME type for common image extensions
func getMimeTypeFromExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".avif":
		return "image/avif"
	default:
		return ""
	}
}

// getCSSAssetMimeType returns the MIME type for CSS-embeddable assets
func getCSSAssetMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".avif":
		return "image/avif"
	case ".cur":
		return "image/x-icon"
	default:
		return ""
	}
}

// writeHeader writes the HTML document header with embedded template content
func (c *Converter) writeHeader(w io.Writer, tmpl *templates.Template) error {
	if _, err := io.WriteString(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
`); err != nil {
		return err
	}

	if tmpl.HTML != "" {
		templateHTML := tmpl.HTML
		// Replace title if custom title is set
		if c.title != "" {
			newTitle := "<title>" + htmlpkg.EscapeString(c.title) + "</title>"
			templateHTML = titlePattern.ReplaceAllString(templateHTML, newTitle)
		}
		if _, err := io.WriteString(w, templateHTML); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}

	if tmpl.CSS != "" {
		if _, err := io.WriteString(w, "<style>\n"); err != nil {
			return err
		}
		css := tmpl.CSS
		if c.selfContained && c.baseDir != "" {
			css = c.embedCSSAssets(css)
		}
		if _, err := io.WriteString(w, css); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n</style>\n"); err != nil {
			return err
		}
	}

	if _, err := io.WriteString(w, `</head>
<body>
<article class="markdown-body">
`); err != nil {
		return err
	}

	return nil
}

// writeFooter writes the HTML document footer with embedded template JS
func (c *Converter) writeFooter(w io.Writer, tmpl *templates.Template) error {
	if _, err := io.WriteString(w, "\n</article>\n"); err != nil {
		return err
	}

	if tmpl.JS != "" {
		if _, err := io.WriteString(w, "<script>\n"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, tmpl.JS); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n</script>\n"); err != nil {
			return err
		}
	}

	if _, err := io.WriteString(w, `</body>
</html>
`); err != nil {
		return err
	}

	return nil
}
