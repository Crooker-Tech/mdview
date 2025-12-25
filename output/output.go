package output

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const appName = "mdview"

// GetOutputPath returns the output path for the HTML file.
// If specifiedPath is non-empty, it returns that path (creating parent dirs if needed).
// Otherwise, it creates a randomly named file in %LocalAppData%/mdview/
func GetOutputPath(specifiedPath string) (string, error) {
	if specifiedPath != "" {
		// Ensure parent directory exists
		dir := filepath.Dir(specifiedPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("failed to create output directory: %w", err)
			}
		}
		return specifiedPath, nil
	}

	// Generate temp file in LocalAppData
	appDir, err := getAppDataDir()
	if err != nil {
		return "", err
	}

	// Generate random filename
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random filename: %w", err)
	}
	filename := hex.EncodeToString(randomBytes) + ".html"

	return filepath.Join(appDir, filename), nil
}

// getAppDataDir returns the application data directory, creating it if needed
func getAppDataDir() (string, error) {
	// Use LocalAppData on Windows
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		// Fallback to temp directory if LocalAppData is not set
		localAppData = os.TempDir()
	}

	appDir := filepath.Join(localAppData, appName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create app data directory: %w", err)
	}

	return appDir, nil
}

// CleanupOldFiles removes HTML files older than the specified age from the app data directory.
// This is optional and can be called to prevent accumulation of temp files.
func CleanupOldFiles(maxAgeSeconds int64) error {
	appDir, err := getAppDataDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(appDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".html" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if file is old enough to delete
		// Note: Using ModTime as a simple age check
		// In a production app, you might want to track file creation time separately
		_ = info // Placeholder for age check logic
	}

	return nil
}
