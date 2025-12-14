package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings holds user configuration that persists across restarts.
type Settings struct {
	DBPath string `json:"db_path,omitempty"`
}

// defaultSettingsPath returns the default path for the settings file.
func defaultSettingsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "jenkins-flow", "settings.json"), nil
}

// Load reads settings from the default location.
// Returns an empty Settings struct if the file doesn't exist.
func Load() (*Settings, error) {
	path, err := defaultSettingsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// File doesn't exist yet, return empty settings
		return &Settings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	return &settings, nil
}

// Save writes settings to the default location.
func (s *Settings) Save() error {
	path, err := defaultSettingsPath()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// GetDefaultDBPath returns the default database path, considering settings.
func GetDefaultDBPath() (string, error) {
	// First check if settings has a custom path
	settings, err := Load()
	if err != nil {
		return "", err
	}

	if settings.DBPath != "" {
		return settings.DBPath, nil
	}

	// Return default path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "jenkins-flow", "jenkins-flow.db"), nil
}
