# mdview

A Go command-line tool that converts Markdown to styled HTML and opens it in the default browser (Windows).

## Project Structure

```
mdview/
├── main.go              # CLI entry point, flag parsing, orchestration
├── converter/           # Streaming markdown-to-HTML conversion (goldmark + sync.Pool)
├── templates/           # Embedded template system (CSS, JS, HTML via //go:embed)
│   └── default/         # Default template with highlight.js and dark mode
├── browser/             # Windows browser opener via cmd /c start
├── output/              # Output path handling (temp files in %LocalAppData%\mdview)
└── register/            # Windows registry integration for .md file association
```

## Usage

```bash
# Build
go build -o mdview.exe .

# Basic usage (opens in browser)
mdview document.md

# Output to specific file without opening browser
mdview --no-browser input.md output.html

# Use a different template
mdview --template minimal document.md

# List available templates
mdview --list-templates

# Create self-contained HTML with embedded images/fonts
mdview --self-contained document.md output.html

# Self-contained with parallel image preloading (11-22% faster for multiple images)
mdview --self-contained --preload document.md

# Register/unregister as default .md handler
mdview --register
mdview --unregister

# Show version
mdview --version
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--template <name>` | Template name to use for styling (default: "default") |
| `--no-browser` | Don't open browser after conversion |
| `--self-contained` | Embed images/fonts as base64 data URIs (portable HTML) |
| `--preload` | Preload all images in a directory asynchronously (use with --self-contained) |
| `--list-templates` | List available templates |
| `--register` | Register mdview as default program for .md files |
| `--unregister` | Unregister mdview as default program for .md files |
| `--version` | Show version information |

## Adding Templates

1. Create folder: `templates/<name>/`
2. Add any of: `template.css`, `template.html`, `template.js`
3. Update embed directive in `templates/embed.go`:
   ```go
   //go:embed default/* newtemplate/*
   ```

## Key Design Decisions

- **Memory efficiency**: Uses `sync.Pool` for buffer reuse, pre-allocates based on file size hints
- **Template embedding**: Templates compiled into binary via `//go:embed`
- **Syntax highlighting**: highlight.js v11.9.0 inlined in default template (no CDN dependency)
- **Dark mode default**: Respects `prefers-color-scheme: light` for light mode override
- **Markdown features**: goldmark with GFM, Typographer, auto heading IDs, raw HTML passthrough
- **Streaming conversion**: Reads input and writes output in chunks to minimize memory usage
- **Self-contained mode**: Embeds images, fonts, and CSS assets as base64 data URIs for portable HTML

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/yuin/goldmark` | v1.7.8 | Markdown parsing with GFM support |
| `golang.org/x/sys` | v0.39.0 | Windows registry access for file association |

## SKYGOD Principles Applied

- **S (SOLID)**: Each package has single responsibility (converter, browser, output, register)
- **K (KISS)**: Simple streaming conversion, no framework overhead
- **Y (YAGNI)**: Only implements what's needed for the use case
- **G (GRASP)**: High cohesion within packages, low coupling between them
- **O (O&O)**: Observable naming (`templateName` not `tmpl`, `sizeHint` not `sz`)
- **D (DRY)**: Template loading logic centralized in `templates/embed.go`

## CLAUDE.md Rules

1. **Path Separators (Windows)**:
   - **Bash Tool**: Use forward slashes (`/`)
   - **All Other File Tools** (`Read`, `Write`, `Edit`, `Glob`, `Grep`): Use backslashes (`\`)

2. **Observable Naming**: No aggressive abbreviations.

3. **No Generic Names**: Avoid "manager", "utilities", "helper", "service".

## Key Assumptions

1. **Windows Only**: Browser opening uses `cmd /c start`, registry uses Windows APIs
2. **Go 1.24+**: Requires Go 1.24 or higher
3. **Temp File Location**: Generated HTML stored in `%LocalAppData%\mdview\` when no output path specified
4. **Default Template**: Dark mode by default, highlight.js for syntax highlighting
5. **File Hash Naming**: Temp files use content-based hash names (e.g., `e2b8587203f5d2a9.html`)
