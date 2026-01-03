# mdview

A Go command-line tool that converts Markdown to styled HTML and opens it in the default browser (Windows).

## Quick Start

```bash
# Build
go build -o mdview.exe .

# Basic usage (opens in browser)
mdview document.md

# Self-contained output (embeds images as base64)
mdview --self-contained document.md

# Self-contained with parallel image preloading (faster for many images)
mdview --self-contained --preload document.md

# Output to specific file without opening browser
mdview --no-browser input.md output.html
```

## Implementation Gotchas

This section documents issues discovered during implementation and testing that may help future contributors.

### 1. Goldmark's `html` Package Conflicts with Standard Library

**Problem:** Goldmark has its own `html` package (`github.com/yuin/goldmark/renderer/html`), which conflicts with Go's standard library `html` package when you need both.

**Symptom:** `html.EscapeString undefined` error when trying to use the standard library function.

**Solution:** Import the standard library with an alias:
```go
import (
    htmlpkg "html"
    "github.com/yuin/goldmark/renderer/html"
)

// Use htmlpkg.EscapeString() instead of html.EscapeString()
```

### 2. Typographer Outputs HTML Entities, Not Unicode

**Problem:** Goldmark's typographer extension converts sequences like `"text"` and `...` but outputs HTML entities, not raw Unicode characters.

**Symptom:** Tests checking for Unicode characters like `\u201c` (") fail even though typographer is working.

**Actual output:**
```
Input:  He said "Hello"...
Output: He said &ldquo;Hello&rdquo;&hellip;
```

**Solution:** Test for HTML entities instead:
```go
// Wrong
wantContain: "\u201c"  // Unicode left double quote

// Correct
wantContain: "&ldquo;" // HTML entity
```

### 3. file:// URLs Need Special Handling

**Problem:** When markdown is processed, relative paths get converted to `file:///` URLs. Code that skips paths containing `://` (to avoid processing http/https URLs) will also skip local file references.

**Symptom:** Images show as `file:///C:/path/to/image.png` instead of being embedded when using `--self-contained`.

**Solution:** Explicitly check for and handle `file://` protocol:
```go
if strings.HasPrefix(path, "file:///") {
    // Extract local path: file:///C:/path -> C:/path
    localPath := strings.TrimPrefix(path, "file:///")
    // Process localPath...
}
```

### 4. Goldmark Is NOT a Streaming Parser

**Problem:** Despite using "streaming" terminology, goldmark builds a complete AST in memory before rendering. This is fundamental to how goldmark works and cannot be changed without switching parsers.

**What "streaming" means in this codebase:**
- Path rewriting during render pass (not a second HTML pass)
- Buffer pool reuses allocations across conversions
- Output written incrementally as nodes render

**What it does NOT mean:**
- Constant memory usage regardless of input size
- Ability to process arbitrarily large files in fixed memory

**Memory profile for 1.35 MB input:**
```
Peak heap:        38.55 MB (28x input) - AST nodes dominate
Retained after GC: 3.73 MB            - just output buffer
```

**Why goldmark anyway?** Feature set: GFM tables, task lists, strikethrough, typographer, auto heading IDs, raw HTML passthrough.

### 5. Memory Test Thresholds

**Problem:** Memory tests need realistic thresholds accounting for:
- Goldmark AST overhead (~30 nodes per KB of structured markdown)
- Template size (~500KB for highlight.js)
- Output buffer

**Solution:**
```go
// Peak heap: allow 40x input (AST node overhead)
// Retained after GC: should be ~output size (verify no leaks)
```

### 6. Raw HTML Requires Separate Processing Path

**Problem:** Goldmark passes raw HTML through unchanged (with `html.WithUnsafe()`). Custom renderers for `ast.KindImage` and `ast.KindLink` don't affect `<img>` and `<a>` tags in raw HTML blocks.

**Symptom:** Markdown images get processed but HTML `<img>` tags don't.

**Solution:** Register additional renderers for raw HTML nodes:
```go
reg.Register(ast.KindHTMLBlock, r.renderRawHTML)
reg.Register(ast.KindRawHTML, r.renderRawHTML)

func (r *pathRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) {
    // Use regex to find and process src/href attributes
    content := processRawHTMLContent(rawHTML, r.baseDir, r.selfContained)
    w.WriteString(content)
}
```

### 7. Goldmark Renderer Priority Matters

**Problem:** When registering multiple node renderers, the priority determines which one handles the node.

**Gotcha:** Lower priority number = higher priority (runs first). If your custom renderer should override the default, give it a lower number:
```go
renderer.WithNodeRenderers(
    util.Prioritized(html.NewRenderer(...), 1000),  // Default renderer
    util.Prioritized(&customRenderer{}, 100),       // Custom (higher priority)
)
```

### 8. CSS url() References Have Multiple Quote Styles

**Problem:** CSS `url()` can be written multiple ways: `url("path")`, `url('path')`, or `url(path)`.

**Solution:** Use a regex that handles all variants:
```go
cssURLPattern = regexp.MustCompile(`(url\(["']?)([^"')]+)(["']?\))`)
```

### 9. Windows Path Handling in Tests

**Problem:** Tests that create temp files need to handle Windows path separators and drive letters.

**Gotcha:** `filepath.Join` handles this correctly, but string concatenation doesn't:
```go
// Wrong - may produce invalid paths on Windows
path := dir + "/" + "file.html"

// Correct
path := filepath.Join(dir, "file.html")
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run benchmarks
go test -bench=. ./converter

# Run benchmarks with memory allocation stats
go test -bench=. -benchmem ./converter
```

## Project Structure

```
mdview/
├── main.go              # CLI entry point, flag parsing
├── converter/           # Markdown-to-HTML conversion with custom renderers
├── templates/           # Embedded CSS, JS, HTML via //go:embed
├── browser/             # Windows browser opener
├── output/              # Output path handling
└── register/            # Windows registry integration
```
