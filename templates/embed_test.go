package templates

import (
	"strings"
	"testing"
)

func TestGetDefaultTemplate(t *testing.T) {
	tmpl, err := Get("default")
	if err != nil {
		t.Fatalf("failed to get default template: %v", err)
	}

	// Default template should have CSS
	if tmpl.CSS == "" {
		t.Error("expected default template to have CSS")
	}

	// Default template should have JS (highlight.js)
	if tmpl.JS == "" {
		t.Error("expected default template to have JS")
	}

	// CSS should contain expected styling
	if !strings.Contains(tmpl.CSS, "markdown-body") {
		t.Error("expected CSS to contain markdown-body class")
	}

	// JS should contain highlight.js
	if !strings.Contains(tmpl.JS, "hljs") {
		t.Error("expected JS to contain highlight.js (hljs)")
	}
}

func TestGetNonexistentTemplate(t *testing.T) {
	_, err := Get("nonexistent-template-xyz")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got: %v", err)
	}
}

func TestListTemplates(t *testing.T) {
	names, err := List()
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}

	if len(names) == 0 {
		t.Error("expected at least one template")
	}

	// Default template should be in the list
	found := false
	for _, name := range names {
		if name == "default" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'default' in template list, got: %v", names)
	}
}

func TestTemplateCSSDarkModeDefault(t *testing.T) {
	tmpl, err := Get("default")
	if err != nil {
		t.Fatalf("failed to get default template: %v", err)
	}

	// Should have dark mode as default with light mode override
	if !strings.Contains(tmpl.CSS, "prefers-color-scheme") {
		t.Error("expected CSS to contain prefers-color-scheme media query")
	}
}

func BenchmarkGetTemplate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Get("default")
		if err != nil {
			b.Fatal(err)
		}
	}
}
