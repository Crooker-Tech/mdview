package register

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	progID      = "mdview.md"
	fileExt     = ".md"
	appName     = "mdview"
	description = "Markdown Viewer"
)

// Register sets mdview as the default program for .md files.
// Uses HKEY_CURRENT_USER so no admin privileges are required.
func Register() error {
	// Get the absolute path to the current executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create the ProgID key: HKCU\Software\Classes\mdview.md
	progIDKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\`+progID,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("failed to create ProgID key: %w", err)
	}
	defer progIDKey.Close()

	// Set the description
	if err := progIDKey.SetStringValue("", description); err != nil {
		return fmt.Errorf("failed to set ProgID description: %w", err)
	}

	// Create the shell\open\command key
	commandKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\`+progID+`\shell\open\command`,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("failed to create command key: %w", err)
	}
	defer commandKey.Close()

	// Set the command: "path\to\mdview.exe" "%1"
	command := fmt.Sprintf(`"%s" "%%1"`, exePath)
	if err := commandKey.SetStringValue("", command); err != nil {
		return fmt.Errorf("failed to set command: %w", err)
	}

	// Create the DefaultIcon key
	iconKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\`+progID+`\DefaultIcon`,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("failed to create icon key: %w", err)
	}
	defer iconKey.Close()

	// Set icon to the executable
	if err := iconKey.SetStringValue("", exePath+",0"); err != nil {
		return fmt.Errorf("failed to set icon: %w", err)
	}

	// Create the file extension key: HKCU\Software\Classes\.md
	extKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\`+fileExt,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("failed to create extension key: %w", err)
	}
	defer extKey.Close()

	// Set the default value to our ProgID
	if err := extKey.SetStringValue("", progID); err != nil {
		return fmt.Errorf("failed to set extension association: %w", err)
	}

	return nil
}

// Unregister removes mdview as the default program for .md files.
func Unregister() error {
	// Delete the ProgID key and all subkeys
	if err := registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\`+progID+`\shell\open\command`); err != nil {
		// Ignore "not found" errors
		if err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete command key: %w", err)
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\`+progID+`\shell\open`); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete open key: %w", err)
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\`+progID+`\shell`); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete shell key: %w", err)
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\`+progID+`\DefaultIcon`); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete icon key: %w", err)
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\`+progID); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete ProgID key: %w", err)
		}
	}

	// Remove the extension association if it points to us
	extKey, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\`+fileExt, registry.QUERY_VALUE|registry.SET_VALUE)
	if err == nil {
		defer extKey.Close()
		val, _, err := extKey.GetStringValue("")
		if err == nil && val == progID {
			extKey.DeleteValue("")
		}
	}

	return nil
}
