# Multi-Page Archive Feature for --self-contained

## Overview

This feature extends `--self-contained` to create recursive multi-page HTML archives. When a markdown file links to other local .md files, those files are automatically converted to HTML, compressed, base64-encoded, and embedded in the output. JavaScript handles navigation between pages via a full-screen overlay system.

## User Experience

```bash
# Basic multi-page archive (max 10 files)
mdview --self-contained document.md archive.html

# Override page limit
mdview --self-contained --max-pages 25 document.md archive.html

# With preloading for faster image embedding
mdview --self-contained --preload document.md archive.html
```

### Behavior

1. **Root page loads normally** - Shows the converted markdown content
2. **Clicking .md link** - Opens embedded page in full-screen overlay (no browser navigation)
3. **Navigation** - Back links to source pages close overlay and return to previous page
4. **Images** - Embedded as base64 in each page (existing --self-contained logic)
5. **Limits** - Default max 10 files, configurable with --max-pages

## Architecture

### 1. Dependency Graph Builder

**Purpose**: Parse markdown files to discover all linked .md files and build a complete dependency graph before conversion.

**Location**: New package `mdview/archive/`

**Key Components**:

```
archive/
├── graph.go           # Dependency graph data structure
├── scanner.go         # Markdown link scanner
└── builder.go         # Graph construction logic
```

#### graph.go - Graph Data Structure

```go
type Node struct {
    Path         string            // Absolute path to .md file
    RelativePath string            // Path relative to root document
    Links        []string          // Paths to linked .md files
    Depth        int               // Distance from root (BFS depth)
}

type Graph struct {
    Root  string                  // Root document path
    Nodes map[string]*Node        // Path -> Node mapping
    Count int                     // Total nodes in graph
}

// Methods:
// - AddNode(path string) *Node
// - HasNode(path string) bool
// - GetNode(path string) *Node
// - Build(rootPath string, maxPages int) error
// - OrderedNodes() []*Node  // Returns nodes in BFS order for conversion
```

#### scanner.go - Markdown Link Scanner

**Purpose**: Extract all local .md file links from markdown content without full conversion.

**Approach**: Use goldmark's AST parser to walk the document tree and collect links.

```go
func ScanMarkdownLinks(content []byte, baseDir string) ([]string, error)
```

**Logic**:
1. Parse markdown to AST using goldmark
2. Walk AST looking for `ast.Link` nodes
3. Filter for local .md files (extension check, not http://, etc.)
4. Resolve relative paths to absolute paths
5. Return deduplicated list of absolute paths

**Edge Cases**:
- Links in raw HTML blocks: Use regex as fallback
- Links to anchors: Strip fragment, track base file only
- Query parameters: Strip query string
- Mixed separators: Normalize paths

#### builder.go - Graph Construction

```go
func BuildGraph(rootPath string, maxPages int) (*Graph, error)
```

**Algorithm** (Breadth-First Search with cycle detection):

```
1. Initialize:
   - graph = new Graph with Root = rootPath
   - queue = [rootPath]
   - visited = {rootPath: true}

2. While queue not empty AND graph.Count < maxPages:
   a. path = queue.pop_front()
   b. IF not exists, skip and warn
   c. content = read file at path
   d. links = ScanMarkdownLinks(content, dir(path))
   e. node = graph.AddNode(path)
   f. node.Links = links

   g. For each link in links:
      - IF link not in visited AND graph.Count < maxPages:
        * visited[link] = true
        * queue.push_back(link)
        * graph.Count++

3. Return graph
```

**Cycle Detection**: `visited` set prevents infinite loops.

**Depth Tracking**: Track BFS level for each node (useful for debugging/visualization).

**Max Pages Enforcement**: Stop adding nodes when limit reached.

### 2. Archive Converter

**Purpose**: Convert all nodes in the graph to HTML and embed them in the root document.

**Location**: New file `mdview/archive/converter.go`

#### Archive Structure

Each embedded page is stored as a compressed, base64-encoded string in a JavaScript data structure:

```javascript
window.mdviewArchive = {
  pages: {
    "path/to/doc1.md": "H4sIAAAAAAAA...",  // Compressed base64 HTML
    "path/to/doc2.md": "H4sIAAAAAAAA...",
    // ... more pages
  },
  root: "C:/Users/me/docs/root.md"
};
```

**Key**: Relative path from root document's directory (for portability).

**Value**: Base64-encoded, gzip-compressed HTML string.

#### Compression Strategy

**Option 1 - Go-side Compression (RECOMMENDED)**:
- Compress HTML during conversion in Go
- Use `compress/gzip` package
- Decompress in browser with `pako.js` (11KB minified)
- **Pros**: Better compression, smaller final file
- **Cons**: Adds ~11KB for pako library

**Option 2 - JavaScript-side Compression**:
- Store raw HTML as base64
- No compression
- **Pros**: No external dependencies
- **Cons**: Larger file sizes (HTML is verbose)

**Decision**: Use Option 1 (gzip) for optimal file size.

#### Conversion Process

```go
type ArchiveConverter struct {
    baseConverter *converter.Converter
    graph         *Graph
    templateName  string
    selfContained bool
    preload       bool
}

func (ac *ArchiveConverter) ConvertToArchive(
    outputPath string,
) error
```

**Steps**:

1. **Convert all pages**:
   ```go
   embedData := make(map[string]string)

   for _, node := range graph.OrderedNodes() {
       // Convert to HTML
       htmlBuf := new(bytes.Buffer)
       mdFile := open(node.Path)
       converter := createConverter(node.baseDir, selfContained, preload)
       converter.Convert(mdFile, htmlBuf, templateName)

       // Compress
       gzipBuf := new(bytes.Buffer)
       gzipWriter := gzip.NewWriter(gzipBuf)
       gzipWriter.Write(htmlBuf.Bytes())
       gzipWriter.Close()

       // Base64 encode
       encoded := base64.StdEncoding.EncodeToString(gzipBuf.Bytes())

       // Store with relative path key
       relPath := relativePath(node.Path, graph.Root)
       embedData[relPath] = encoded
   }
   ```

2. **Convert root document**:
   - Convert root .md file to HTML normally
   - This HTML forms the visible page when archive opens

3. **Generate archive data script**:
   ```javascript
   window.mdviewArchive = {
     pages: {
       "doc1.md": "H4sIA...",
       "doc2.md": "H4sIA...",
       ...
     },
     root: "C:/path/to/root.md"
   };
   ```

4. **Inject navigation script**:
   - Link interceptor
   - Overlay system
   - Decompression logic

5. **Write final HTML**:
   ```html
   <!DOCTYPE html>
   <html>
   <head>
     <style>/* template CSS */</style>
     <style>/* overlay CSS */</style>
   </head>
   <body>
     <article class="markdown-body">
       <!-- Root page content -->
     </article>

     <!-- Overlay container -->
     <div id="mdview-overlay" class="mdview-overlay">
       <div class="mdview-overlay-content">
         <article class="markdown-body" id="mdview-overlay-body"></article>
       </div>
     </div>

     <script>/* highlight.js */</script>
     <script src="pako.min.js"></script>  <!-- Gzip decompression -->
     <script>/* Archive data */</script>
     <script>/* Navigation logic */</script>
   </body>
   </html>
   ```

### 3. Navigation System (JavaScript)

**Purpose**: Intercept clicks on .md links, load embedded pages into overlay, handle back navigation.

**Location**: New file `mdview/archive/navigation.js` (embedded in template)

#### Components

##### A. Link Interceptor

```javascript
document.addEventListener('click', function(e) {
  const link = e.target.closest('a');
  if (!link) return;

  const href = link.getAttribute('href');
  if (!href) return;

  // Check if it's a local .md file link
  if (isLocalMarkdownLink(href)) {
    e.preventDefault();
    loadEmbeddedPage(href, link);
  }
});

function isLocalMarkdownLink(href) {
  // Skip external, anchors, protocols
  if (href.startsWith('#') ||
      href.startsWith('http://') ||
      href.startsWith('https://') ||
      href.startsWith('mailto:')) {
    return false;
  }

  // Extract path without fragment/query
  const path = href.split('#')[0].split('?')[0];
  return path.endsWith('.md');
}
```

##### B. Page Loader

```javascript
function loadEmbeddedPage(href, sourceLink) {
  // Resolve relative path
  const currentPath = window.mdviewCurrentPage || window.mdviewArchive.root;
  const targetPath = resolvePath(currentPath, href);
  const relPath = relativePath(targetPath, window.mdviewArchive.root);

  // Look up in archive
  const compressed = window.mdviewArchive.pages[relPath];
  if (!compressed) {
    console.warn('Page not found in archive:', relPath);
    return;
  }

  // Decompress
  const decoded = atob(compressed);
  const uint8Array = new Uint8Array(decoded.length);
  for (let i = 0; i < decoded.length; i++) {
    uint8Array[i] = decoded.charCodeAt(i);
  }
  const html = pako.inflate(uint8Array, { to: 'string' });

  // Show in overlay
  showOverlay(html, targetPath);

  // Update state
  window.mdviewCurrentPage = targetPath;
  window.mdviewPreviousPage = currentPath;
}
```

##### C. Overlay System

```javascript
function showOverlay(html, pagePath) {
  const overlay = document.getElementById('mdview-overlay');
  const body = document.getElementById('mdview-overlay-body');

  body.innerHTML = html;
  overlay.classList.add('visible');

  // Re-initialize syntax highlighting for new content
  if (window.hljs) {
    body.querySelectorAll('pre code').forEach((block) => {
      hljs.highlightBlock(block);
    });
  }
}

function hideOverlay() {
  const overlay = document.getElementById('mdview-overlay');
  overlay.classList.remove('visible');

  // Restore state
  window.mdviewCurrentPage = window.mdviewPreviousPage;
}
```

##### D. Back Link Detection

**Challenge**: Determine if a link points back to the page that loaded the current page.

**Approach 1 - Track Navigation History**:
```javascript
window.mdviewHistory = [];  // Stack of page paths

function loadEmbeddedPage(href, sourceLink) {
  // ... load logic ...
  window.mdviewHistory.push(window.mdviewCurrentPage);
}

function isBackLink(href) {
  const targetPath = resolveAndRelativize(href);
  const previousPath = window.mdviewHistory[window.mdviewHistory.length - 1];
  return targetPath === previousPath;
}

// In click handler:
if (isBackLink(href)) {
  e.preventDefault();
  hideOverlay();
  window.mdviewHistory.pop();
  return;
}
```

**Approach 2 - Heuristic Detection**:
- Check if link text contains "back", "return", "←"
- Check if link points to parent directory
- **Cons**: Unreliable, may miss valid back links

**Decision**: Use Approach 1 (history tracking) for accuracy.

#### Overlay CSS

```css
.mdview-overlay {
  display: none;
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: var(--bg-color, #1e1e1e);
  z-index: 9999;
  overflow: auto;
}

.mdview-overlay.visible {
  display: block;
}

.mdview-overlay-content {
  max-width: 900px;
  margin: 0 auto;
  padding: 2rem;
}

/* Add close button for redundancy */
.mdview-overlay::before {
  content: '× Close';
  position: fixed;
  top: 1rem;
  right: 1rem;
  padding: 0.5rem 1rem;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  cursor: pointer;
  z-index: 10000;
}
```

### 4. Link Rewriting

**Challenge**: When embedding a page, its relative links must be rewritten to work in the archive context.

**Scenarios**:

1. **Link to another .md file**: Keep relative path, navigation.js will resolve
2. **Link to image**: Keep as-is (already embedded as base64 by existing logic)
3. **Link to external file**: Convert to file:// URL (existing logic handles this)

**Implementation**: No changes needed. The existing path resolution in `converter/converter.go` already handles this correctly. Each page is converted with its own base directory, so relative paths work naturally.

### 5. Flag Implementation

**Location**: `main.go`

#### Add --max-pages Flag

```go
maxPages := flag.Int("max-pages", 10, "Maximum number of pages to embed in archive (use with --self-contained)")
```

#### Modify Conversion Logic

```go
func run(inputPath, outputPath, templateName string,
         openBrowser, selfContained, preload bool,
         maxPages int) error {

    // ... existing path resolution ...

    if selfContained {
        // Check if document has links to other .md files
        hasMarkdownLinks, err := archive.HasMarkdownLinks(absInputPath)
        if err != nil {
            return err
        }

        if hasMarkdownLinks {
            // Build dependency graph
            graph, err := archive.BuildGraph(absInputPath, maxPages)
            if err != nil {
                return err
            }

            fmt.Printf("Building archive with %d pages...\n", graph.Count)

            // Use archive converter
            archiveConv := archive.NewConverter(graph, templateName, selfContained, preload)
            return archiveConv.ConvertToArchive(finalOutputPath)
        }
    }

    // Fall back to single-file conversion
    // ... existing conversion logic ...
}
```

### 6. Testing Strategy

#### Unit Tests

**archive/scanner_test.go**:
- Test link extraction from various markdown formats
- Test filtering (local vs external, .md vs other extensions)
- Test path resolution (relative, absolute, with fragments)

**archive/builder_test.go**:
- Test graph building with linear chains
- Test cycle detection (A → B → A)
- Test max page limiting
- Test missing file handling

**archive/converter_test.go**:
- Test single-page archive (baseline)
- Test multi-page archive generation
- Test compression/decompression round-trip
- Test relative path calculation

#### Integration Tests

**converter/archive_test.go**:
- Create test markdown files with cross-links
- Convert to archive
- Parse output HTML
- Verify archive data structure
- Verify all pages embedded
- Verify navigation script present

#### Manual Testing Scenarios

1. **Simple two-page archive**:
   - root.md → doc.md
   - Verify clicking link shows doc.md in overlay
   - Verify back link returns to root.md

2. **Deep hierarchy**:
   - root.md → a.md → b.md → c.md
   - Verify navigation through all pages
   - Verify back navigation works at each level

3. **Circular references**:
   - a.md → b.md → a.md
   - Verify both pages embedded
   - Verify bidirectional navigation

4. **Max pages limit**:
   - Create 15 linked pages
   - Convert with --max-pages 5
   - Verify only 5 pages embedded
   - Verify warning about truncation

5. **With images**:
   - Pages with embedded images
   - Verify each page's images are embedded
   - Verify overlay shows images correctly

6. **Edge cases**:
   - Link with fragment: doc.md#section
   - Link with query: doc.md?version=2
   - Mixed separators: doc.md, doc/sub.md, ../other.md
   - Non-existent link target

### 7. Implementation Order

1. **Phase 1 - Graph Building** (archive/scanner.go, archive/graph.go, archive/builder.go)
   - Implement link scanner
   - Implement graph data structure
   - Implement BFS graph builder
   - Add unit tests

2. **Phase 2 - Archive Data Generation** (archive/converter.go)
   - Implement multi-page HTML conversion
   - Implement compression and base64 encoding
   - Generate archive data structure
   - Add unit tests

3. **Phase 3 - Navigation System** (archive/navigation.js, archive/overlay.css)
   - Implement link interceptor
   - Implement overlay system
   - Implement decompression logic
   - Implement history tracking
   - Add pako.js dependency

4. **Phase 4 - Integration** (main.go, converter/converter.go)
   - Add --max-pages flag
   - Wire up archive converter in main.go
   - Ensure backward compatibility (single-file mode)
   - Integration tests

5. **Phase 5 - Polish**
   - Error handling (missing files, compression failures)
   - Warning messages (truncated archives, missing links)
   - Performance optimization (parallel conversion?)
   - Documentation updates

### 8. File Structure After Implementation

```
mdview/
├── archive/
│   ├── builder.go           # Graph construction (BFS)
│   ├── builder_test.go
│   ├── converter.go         # Archive HTML generation
│   ├── converter_test.go
│   ├── graph.go             # Graph data structure
│   ├── graph_test.go
│   ├── navigation.js        # Embedded JavaScript for navigation
│   ├── overlay.css          # Embedded CSS for overlay
│   ├── scanner.go           # Markdown link extraction
│   └── scanner_test.go
├── converter/
│   ├── converter.go         # Existing single-file converter
│   ├── converter_test.go
│   └── archive_test.go      # NEW: Integration tests for archives
├── templates/
│   └── default/
│       ├── template.css
│       ├── template.html
│       └── template.js
├── main.go                  # Add --max-pages flag, wire up archive
├── MULTI_PAGE_ARCHIVE.md    # This document
└── ...
```

### 9. Performance Considerations

#### Conversion Time
- **Sequential conversion**: Process each page one at a time
- **Parallel conversion** (future optimization): Use goroutines to convert multiple pages concurrently
  - Trade-off: Higher memory usage
  - Benefit: Faster for archives with many pages

#### File Size
- **Gzip compression**: Reduces HTML size by ~70-80%
- **Base64 overhead**: Adds ~33% size increase
- **Net result**: Archive is ~40-50% of original uncompressed HTML size
- **Pako.js**: Adds 11KB (minified) for decompression

#### Memory Usage
- **Graph building**: Reads each file once to scan links (~few MB for typical docs)
- **Conversion**: Holds one page in memory at a time
- **Final assembly**: Holds all compressed pages in memory temporarily
- **Optimization**: Stream archive data directly to output instead of buffering

### 10. Error Handling

#### Missing Link Targets
```
Warning: Page not found: /path/to/missing.md
Skipping link from root.md
```

**Behavior**: Continue processing, exclude missing page from archive.

#### Compression Failures
```
Error: Failed to compress page: /path/to/doc.md: [error details]
```

**Behavior**: Abort conversion, show error.

#### Max Pages Exceeded
```
Warning: Maximum page limit (10) reached
Archive truncated, 5 pages excluded
Use --max-pages to increase limit
```

**Behavior**: Embed first N pages (BFS order), warn about truncation.

#### Circular References
**Behavior**: No warning needed, cycles are expected and handled naturally by BFS visited set.

### 11. Alternative Designs Considered

#### A. Tar-like Binary Format
- **Idea**: Store archive as base64-encoded tar/zip file, extract with JavaScript
- **Pros**: Standard format, better compression
- **Cons**: Complex extraction, requires JSZip library (~100KB), slower decompression
- **Decision**: Rejected due to complexity and library size

#### B. Single-Page Application (iframe-based)
- **Idea**: Use iframes to load embedded pages
- **Pros**: True isolation, native browser features
- **Cons**: Can't embed HTML in iframe from JavaScript (security), complex styling
- **Decision**: Rejected due to technical limitations

#### C. Hash-based Routing
- **Idea**: Use URL fragments (#page=doc.md) for navigation
- **Pros**: Browser back button works naturally
- **Cons**: Pollutes browser history, URL bar changes
- **Decision**: Rejected to maintain clean UX (overlay approach cleaner)

#### D. Progressive Loading
- **Idea**: Embed only root page, lazy-load others on demand
- **Pros**: Smaller initial file size
- **Cons**: Breaks "self-contained" promise, requires network access
- **Decision**: Rejected, violates core feature goal

### 12. Future Enhancements

1. **Archive index/TOC**: Auto-generate table of contents showing all embedded pages
2. **Search across archive**: JavaScript search within all embedded pages
3. **Archive metadata**: Embed creation date, mdview version, page count
4. **Visual graph**: Show dependency graph as interactive diagram
5. **Brotli compression**: Better compression than gzip (but requires different decompressor)
6. **Lazy decompression**: Decompress pages on-demand instead of at load time
7. **Archive validation**: Pre-flight check to estimate final file size
8. **Smart max-pages**: Automatically adjust limit based on file sizes
9. **Archive extraction**: CLI tool to extract individual pages from archive

### 13. Documentation Updates

#### README.md
```markdown
### Multi-Page Archives

When using `--self-contained`, mdview automatically detects links to other
local .md files and embeds them as a navigable archive:

mdview --self-contained document.md archive.html

Click any link to a .md file to view it in an overlay. Click back links to
return to the previous page. All navigation happens in JavaScript without
browser navigation.

By default, up to 10 pages are embedded. Increase with `--max-pages`:

mdview --self-contained --max-pages 25 document.md archive.html

The archive is fully portable - it's a single HTML file with everything embedded.
```

#### CLAUDE.md
```markdown
## Multi-Page Archive Feature

See MULTI_PAGE_ARCHIVE.md for detailed design and implementation notes.

### Key Packages:
- `archive/builder.go`: Graph construction using BFS
- `archive/scanner.go`: Markdown link extraction using goldmark AST
- `archive/converter.go`: Archive HTML generation with gzip compression
- `archive/navigation.js`: Client-side navigation and overlay system
```

### 14. Dependencies

#### Go Packages
- `compress/gzip`: Gzip compression (standard library)
- `encoding/base64`: Base64 encoding (standard library)
- `github.com/yuin/goldmark`: Markdown parsing (already a dependency)

#### JavaScript Libraries
- **pako.js** (v2.1.0): Gzip decompression in browser
  - Size: 11KB minified
  - License: MIT
  - CDN: https://cdn.jsdelivr.net/npm/pako@2.1.0/dist/pako.min.js
  - **Embedding**: Download and embed directly in template (no CDN dependency for true self-contained)

### 15. Open Questions

1. **Should we support .html targets?**
   - If root.md links to external.html, embed it too?
   - Decision: No, only .md files (scope creep)

2. **How to handle very large archives?**
   - 100+ pages could create multi-MB HTML file
   - Decision: Rely on --max-pages flag, add size warning in future

3. **Should back button work?**
   - Browser back button could close overlay
   - Decision: No, keep overlay self-contained, would require history API manipulation

4. **What about links to non-md files (PDFs, etc.)?**
   - Decision: Ignore, only embed .md files and images

5. **Should we compress images too?**
   - Images are already binary, gzip won't help much
   - Decision: No, images stay as-is (base64 only)

## Summary

This feature transforms mdview into a powerful tool for creating portable, self-contained documentation archives. By embedding all linked markdown files as compressed data and using JavaScript for navigation, users get a single HTML file that contains an entire documentation site with full navigation capabilities.

**Key Benefits**:
- **Portability**: Single file contains everything
- **Offline**: No network required after generation
- **Navigation**: Click between pages naturally
- **Compatibility**: Works in any modern browser
- **Simplicity**: No server or special viewer needed

**Implementation Complexity**: Medium
- Core graph building: ~300 LOC
- Archive converter: ~400 LOC
- Navigation JavaScript: ~200 LOC
- Tests: ~500 LOC
- **Total**: ~1,400 LOC

**Estimated Development Time**:
(Note: Per CLAUDE.md guidelines, providing actionable breakdown without timelines)

- Graph building and testing
- Archive conversion and compression
- Navigation system and overlay
- Integration and end-to-end testing
- Documentation and polish
