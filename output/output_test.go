package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetOutputPathWithSpecifiedPath(t *testing.T) {
	// Create temp dir for test
	dir, err := os.MkdirTemp("", "mdview-output-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	specifiedPath := filepath.Join(dir, "output.html")
	result, err := GetOutputPath(specifiedPath)
	if err != nil {
		t.Fatalf("GetOutputPath failed: %v", err)
	}

	if result != specifiedPath {
		t.Errorf("expected %q, got %q", specifiedPath, result)
	}
}

func TestGetOutputPathCreatesParentDirs(t *testing.T) {
	// Create temp dir for test
	dir, err := os.MkdirTemp("", "mdview-output-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Specify a path with nested directories that don't exist
	specifiedPath := filepath.Join(dir, "nested", "dirs", "output.html")
	result, err := GetOutputPath(specifiedPath)
	if err != nil {
		t.Fatalf("GetOutputPath failed: %v", err)
	}

	if result != specifiedPath {
		t.Errorf("expected %q, got %q", specifiedPath, result)
	}

	// Check that parent directory was created
	parentDir := filepath.Dir(specifiedPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Error("expected parent directory to be created")
	}
}

func TestGetOutputPathGeneratesRandomFile(t *testing.T) {
	// When no path is specified, should generate random filename
	result1, err := GetOutputPath("")
	if err != nil {
		t.Fatalf("GetOutputPath failed: %v", err)
	}

	result2, err := GetOutputPath("")
	if err != nil {
		t.Fatalf("GetOutputPath failed: %v", err)
	}

	// Should have .html extension
	if !strings.HasSuffix(result1, ".html") {
		t.Errorf("expected .html extension, got %q", result1)
	}

	// Two calls should generate different filenames
	if result1 == result2 {
		t.Error("expected different random filenames for each call")
	}

	// Should be in a directory containing "mdview"
	if !strings.Contains(result1, "mdview") {
		t.Errorf("expected path to contain 'mdview', got %q", result1)
	}
}

func TestGetOutputPathRandomFilenameLength(t *testing.T) {
	result, err := GetOutputPath("")
	if err != nil {
		t.Fatalf("GetOutputPath failed: %v", err)
	}

	// Filename should be 16 hex chars + .html = 21 chars total
	filename := filepath.Base(result)
	expectedLen := 16 + 5 // 16 hex chars + ".html"
	if len(filename) != expectedLen {
		t.Errorf("expected filename length %d, got %d (%q)", expectedLen, len(filename), filename)
	}
}

func BenchmarkGetOutputPathSpecified(b *testing.B) {
	dir, _ := os.MkdirTemp("", "mdview-bench-*")
	defer os.RemoveAll(dir)
	specifiedPath := filepath.Join(dir, "output.html")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetOutputPath(specifiedPath)
	}
}

func BenchmarkGetOutputPathRandom(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GetOutputPath("")
	}
}
