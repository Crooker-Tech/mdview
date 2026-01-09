package archive

import (
	"path/filepath"
	"testing"
)

func TestScanMarkdownLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		baseDir  string
		expected []string
	}{
		{
			name:     "no links",
			content:  "# Hello\n\nThis is plain text.",
			baseDir:  "C:\\test",
			expected: []string{},
		},
		{
			name:     "single markdown link",
			content:  "# Hello\n\nSee [other doc](other.md) for details.",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\other.md"},
		},
		{
			name:     "multiple markdown links",
			content:  "[a](a.md) and [b](b.md) and [c](c.md)",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\a.md", "C:\\test\\b.md", "C:\\test\\c.md"},
		},
		{
			name:     "relative paths",
			content:  "[sub](docs/sub.md) and [parent](../parent.md)",
			baseDir:  "C:\\test\\current",
			expected: []string{"C:\\test\\current\\docs\\sub.md", "C:\\test\\parent.md"},
		},
		{
			name:     "skip external links",
			content:  "[external](https://example.com) and [local](doc.md)",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\doc.md"},
		},
		{
			name:     "skip anchors",
			content:  "[anchor](#section) and [doc](doc.md)",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\doc.md"},
		},
		{
			name:     "strip fragments",
			content:  "[doc with anchor](doc.md#section)",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\doc.md"},
		},
		{
			name:     "strip query strings",
			content:  "[doc with query](doc.md?version=2)",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\doc.md"},
		},
		{
			name:     "skip non-markdown files",
			content:  "[image](image.png) and [doc](doc.md)",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\doc.md"},
		},
		{
			name:     "deduplicate links",
			content:  "[a](doc.md) and [b](doc.md) and [c](doc.md)",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\doc.md"},
		},
		{
			name:     "html link in raw html",
			content:  "<a href=\"doc.md\">link</a>",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\doc.md"},
		},
		{
			name:     "mixed markdown and html",
			content:  "[markdown](a.md) and <a href=\"b.md\">html</a>",
			baseDir:  "C:\\test",
			expected: []string{"C:\\test\\a.md", "C:\\test\\b.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links, err := ScanMarkdownLinks([]byte(tt.content), tt.baseDir)
			if err != nil {
				t.Fatalf("ScanMarkdownLinks() error = %v", err)
			}

			// Normalize paths for comparison
			normalized := make([]string, len(links))
			for i, link := range links {
				normalized[i] = filepath.Clean(link)
			}

			if len(normalized) != len(tt.expected) {
				t.Errorf("ScanMarkdownLinks() got %d links, want %d", len(normalized), len(tt.expected))
				t.Errorf("Got: %v", normalized)
				t.Errorf("Want: %v", tt.expected)
				return
			}

			// Check each expected link is present
			for _, expected := range tt.expected {
				found := false
				expectedClean := filepath.Clean(expected)
				for _, link := range normalized {
					if link == expectedClean {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected link %s not found in %v", expectedClean, normalized)
				}
			}
		})
	}
}

func TestProcessLink(t *testing.T) {
	tests := []struct {
		name    string
		href    string
		baseDir string
		want    string
	}{
		{
			name:    "simple relative",
			href:    "doc.md",
			baseDir: "C:\\test",
			want:    "C:\\test\\doc.md",
		},
		{
			name:    "subdirectory",
			href:    "sub/doc.md",
			baseDir: "C:\\test",
			want:    "C:\\test\\sub\\doc.md",
		},
		{
			name:    "parent directory",
			href:    "../doc.md",
			baseDir: "C:\\test\\sub",
			want:    "C:\\test\\doc.md",
		},
		{
			name:    "skip http",
			href:    "http://example.com/doc.md",
			baseDir: "C:\\test",
			want:    "",
		},
		{
			name:    "skip https",
			href:    "https://example.com/doc.md",
			baseDir: "C:\\test",
			want:    "",
		},
		{
			name:    "skip anchor",
			href:    "#section",
			baseDir: "C:\\test",
			want:    "",
		},
		{
			name:    "strip fragment",
			href:    "doc.md#section",
			baseDir: "C:\\test",
			want:    "C:\\test\\doc.md",
		},
		{
			name:    "strip query",
			href:    "doc.md?v=1",
			baseDir: "C:\\test",
			want:    "C:\\test\\doc.md",
		},
		{
			name:    "skip non-md",
			href:    "image.png",
			baseDir: "C:\\test",
			want:    "",
		},
		{
			name:    "file url",
			href:    "file:///C:/test/doc.md",
			baseDir: "C:\\other",
			want:    "C:\\test\\doc.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := &linkCollector{baseDir: tt.baseDir}
			got := lc.processLink(tt.href)

			// Only clean non-empty paths
			if got != "" {
				got = filepath.Clean(got)
			}
			want := tt.want
			if want != "" {
				want = filepath.Clean(want)
			}

			if got != want {
				t.Errorf("processLink() = %v, want %v", got, want)
			}
		})
	}
}
