package converter

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"mdview/templates"
)

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
	md goldmark.Markdown
}

// New creates a new Converter instance
func New() *Converter {
	md := goldmark.New(
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
	)

	return &Converter{md: md}
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

	// Convert and stream directly to writer
	// goldmark.Convert writes to the io.Writer as it generates HTML
	convertErr := c.md.Convert(source, bufWriter)

	// Release source buffer back to pool immediately after conversion
	// This allows GC to reclaim memory before we finish writing
	c.releaseBuffer(source)

	if convertErr != nil {
		return fmt.Errorf("failed to convert markdown: %w", convertErr)
	}

	// Write HTML footer
	if err := c.writeFooter(bufWriter, tmpl); err != nil {
		return err
	}

	return bufWriter.Flush()
}

// readSource reads all content from reader into a pooled buffer.
// Uses chunked reading to avoid large allocations during the read loop.
func (c *Converter) readSource(reader io.Reader, sizeHint int64) ([]byte, error) {
	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	// Pre-grow buffer if we have a size hint (avoids reallocations)
	if sizeHint > 0 {
		buf.Grow(int(sizeHint))
	}

	// Read in chunks - this is memory efficient as we reuse the chunk buffer
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
			// Return buffer to pool on error
			bufferPool.Put(buf)
			return nil, err
		}
	}

	// Return the bytes (we'll return the buffer to pool after conversion)
	return buf.Bytes(), nil
}

// releaseBuffer returns the buffer backing the byte slice to the pool.
// The slice must have come from readSource.
func (c *Converter) releaseBuffer(source []byte) {
	// Create a buffer pointing to the same backing array
	// This works because readSource returns buf.Bytes() which shares the backing array
	if cap(source) > 0 {
		buf := bytes.NewBuffer(source[:0])
		bufferPool.Put(buf)
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
		if _, err := io.WriteString(w, tmpl.HTML); err != nil {
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
		if _, err := io.WriteString(w, tmpl.CSS); err != nil {
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
