package consts

import (
	"os"
	"path/filepath"
)

// LogDirPermissions defines the permissions for the log directory
const LogDirPermissions = 0755

// LogFilePermissions defines the permissions for log files
const LogFilePermissions = 0666

// LogDirectoryName is the name of the log directory within the config directory
const LogDirectoryName = "logs"

// DefaultDaemonLogFileName is the default log file name for the daemon
const DefaultDaemonLogFileName = "obsfindd.log"

// DefaultConfigDirPath is the relative path to the config directory from home directory
const DefaultConfigDirPath = ".config/obsfind"

// GetLogDirectory returns the path to the log directory
func GetLogDirectory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, DefaultConfigDirPath, LogDirectoryName), nil
}

// GetDaemonLogFilePath returns the path to the daemon log file
func GetDaemonLogFilePath() (string, error) {
	logDir, err := GetLogDirectory()
	if err != nil {
		return "", err
	}

	return filepath.Join(logDir, DefaultDaemonLogFileName), nil
}

// EnsureLogDirectoryExists creates the log directory if it doesn't exist
func EnsureLogDirectoryExists() (string, error) {
	logDir, err := GetLogDirectory()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(logDir, LogDirPermissions); err != nil {
		return "", err
	}

	return logDir, nil
}
