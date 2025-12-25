package browser

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Open opens the specified file path in the default web browser.
// The path should be an absolute file path.
func Open(filePath string) error {
	// Convert to absolute path if not already
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Convert file path to file:// URL
	// On Windows, we need to handle the path format properly
	url := pathToFileURL(absPath)

	// On Windows, use cmd /c start to open the default browser
	// The empty string argument after "start" is the window title
	// This prevents issues with paths containing spaces
	cmd := exec.Command("cmd", "/c", "start", "", url)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

// pathToFileURL converts a file path to a file:// URL
func pathToFileURL(path string) string {
	// Replace backslashes with forward slashes
	path = strings.ReplaceAll(path, "\\", "/")

	// Ensure the path starts with a slash (for the file:// protocol)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return "file://" + path
}
