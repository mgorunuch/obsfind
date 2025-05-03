package consts

import (
	"os"
	"path/filepath"
)

// ConfigFileLocations returns a list of default config file locations in order of priority
func ConfigFileLocations() []string {
	locations := []string{
		"./config.yaml",
		"./config/config.yaml",
		"/etc/obsfind/config.yaml",
	}

	// Try user config directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		locations = append(locations, filepath.Join(homeDir, ".config", "obsfind", "config.yaml"))
	}

	return locations
}
