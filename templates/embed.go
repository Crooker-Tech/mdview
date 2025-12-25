package templates

import (
	"embed"
	"fmt"
	"path"
)

//go:embed default/*
var templateFS embed.FS

// Template holds the content of a template's files
type Template struct {
	HTML string
	CSS  string
	JS   string
}

// Get retrieves a template by name. Returns the template content or an error.
// Missing files within a template are allowed (they'll be empty strings).
func Get(name string) (*Template, error) {
	t := &Template{}

	// Check if template directory exists by trying to read it
	entries, err := templateFS.ReadDir(name)
	if err != nil {
		return nil, fmt.Errorf("template %q not found: %w", name, err)
	}

	// Build a set of available files
	files := make(map[string]bool)
	for _, e := range entries {
		files[e.Name()] = true
	}

	// Read template.html if it exists
	if files["template.html"] {
		data, err := templateFS.ReadFile(path.Join(name, "template.html"))
		if err != nil {
			return nil, fmt.Errorf("failed to read template.html: %w", err)
		}
		t.HTML = string(data)
	}

	// Read template.css if it exists
	if files["template.css"] {
		data, err := templateFS.ReadFile(path.Join(name, "template.css"))
		if err != nil {
			return nil, fmt.Errorf("failed to read template.css: %w", err)
		}
		t.CSS = string(data)
	}

	// Read template.js if it exists
	if files["template.js"] {
		data, err := templateFS.ReadFile(path.Join(name, "template.js"))
		if err != nil {
			return nil, fmt.Errorf("failed to read template.js: %w", err)
		}
		t.JS = string(data)
	}

	return t, nil
}

// List returns the names of all available templates
func List() ([]string, error) {
	entries, err := templateFS.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
