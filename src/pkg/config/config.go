package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config holds the global application configuration
type Config struct {
	// General settings
	General struct {
		DataDir string `mapstructure:"data_dir"`
		Debug   bool   `mapstructure:"debug"`
	} `mapstructure:"general"`

	// Path settings
	Paths struct {
		VaultPaths []string `mapstructure:"vault_paths"`
		VaultPath  string   `mapstructure:"vault_path"` // For backward compatibility
		ConfigPath string   `mapstructure:"config_path"`
		CachePath  string   `mapstructure:"cache_path"`
	} `mapstructure:"paths"`

	// Daemon settings
	Daemon struct {
		Port       int    `mapstructure:"port"`
		Host       string `mapstructure:"host"`
		PIDFile    string `mapstructure:"pid_file"`
		LogFile    string `mapstructure:"log_file"`
		LogLevel   string `mapstructure:"log_level"`
		MaxRetries int    `mapstructure:"max_retries"`
	} `mapstructure:"daemon"`

	// API settings
	API struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	} `mapstructure:"api"`

	// Embedding model settings
	Embedding struct {
		Provider    string `mapstructure:"provider"`
		ModelName   string `mapstructure:"model_name"`
		ServerURL   string `mapstructure:"server_url"`
		Dimensions  int    `mapstructure:"dimensions"`
		BatchSize   int    `mapstructure:"batch_size"`
		MaxAttempts int    `mapstructure:"max_attempts"`
		Timeout     int    `mapstructure:"timeout_seconds"`
	} `mapstructure:"embedding"`

	// Qdrant vector database settings
	Qdrant struct {
		Host       string `mapstructure:"host"`
		Port       int    `mapstructure:"port"`
		APIKey     string `mapstructure:"api_key"`
		Embedded   bool   `mapstructure:"embedded"`
		DataPath   string `mapstructure:"data_path"`
		Collection string `mapstructure:"collection"`
		Distance   string `mapstructure:"distance"` // cosine, dot, or euclid
	} `mapstructure:"qdrant"`

	// Indexing settings
	Indexing struct {
		ChunkStrategy    string   `mapstructure:"chunk_strategy"`
		MinChunkSize     int      `mapstructure:"min_chunk_size"`
		MaxChunkSize     int      `mapstructure:"max_chunk_size"`
		WindowSize       int      `mapstructure:"window_size"`
		WindowOverlap    int      `mapstructure:"window_overlap"`
		IncludePatterns  []string `mapstructure:"include_patterns"`
		ExcludePatterns  []string `mapstructure:"exclude_patterns"`
		BatchSize        int      `mapstructure:"batch_size"`
		RescoreResults   bool     `mapstructure:"rescore_results"`
		ReindexOnStartup bool     `mapstructure:"reindex_on_startup"`
	} `mapstructure:"indexing"`

	// FileWatcher settings
	FileWatcher struct {
		DebounceTime     int  `mapstructure:"debounce_time_ms"`
		ScanInterval     int  `mapstructure:"scan_interval_seconds"`
		MaxEventQueue    int  `mapstructure:"max_event_queue"`
		IgnoreDotFiles   bool `mapstructure:"ignore_dot_files"`
		IgnoreGitChanges bool `mapstructure:"ignore_git_changes"`
	} `mapstructure:"file_watcher"`
}

// LoadConfig reads in config file and ENV variables if set
func LoadConfig(configPath string) (*Config, error) {
	var config Config
	var configFile string

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// If config path is specified, use it
	if configPath != "" {
		viper.SetConfigFile(configPath)
		configFile = configPath
	} else {
		// Otherwise try standard locations
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.config/obsfind")
		viper.AddConfigPath("/etc/obsfind")

		// Set default config file path for creation if needed
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		configFile = filepath.Join(homeDir, ".config", "obsfind", "config.yaml")
	}

	// Read in environment variables that match
	viper.AutomaticEnv()
	viper.SetEnvPrefix("OBSFIND")

	// Try to read the config file
	if err := viper.ReadInConfig(); err != nil {
		// If config file is not found, create one with defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Config file not found, creating with defaults...")
			config = DefaultConfig()

			// Ensure config directory exists
			configDir := filepath.Dir(configFile)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create config directory: %w", err)
			}

			// Set the config values in viper
			if err := mapConfigToViper(&config); err != nil {
				return nil, fmt.Errorf("failed to map config to viper: %w", err)
			}

			// Write default config
			viper.SetConfigFile(configFile)
			if err := viper.WriteConfig(); err != nil {
				return nil, fmt.Errorf("failed to write default config: %w", err)
			}

			fmt.Printf("Created default config at: %s\n", configFile)
			return &config, nil
		}

		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Handle backward compatibility for vault paths
	migrateConfiguration(&config)

	return &config, nil
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	dataDir := filepath.Join(homeDir, ".local", "share", "obsfind")

	config := Config{}

	// General defaults
	config.General.DataDir = dataDir
	config.General.Debug = false

	// Path defaults
	defaultVaultPath := filepath.Join(homeDir, "Documents", "Obsidian")
	config.Paths.VaultPath = defaultVaultPath
	config.Paths.VaultPaths = []string{defaultVaultPath}
	config.Paths.ConfigPath = filepath.Join(homeDir, ".config", "obsfind")
	config.Paths.CachePath = filepath.Join(dataDir, "cache")

	// Daemon defaults
	config.Daemon.Port = 8090
	config.Daemon.Host = "localhost"
	config.Daemon.PIDFile = filepath.Join(dataDir, "obsfind.pid")
	config.Daemon.LogFile = filepath.Join(dataDir, "obsfind.log")
	config.Daemon.LogLevel = "info"
	config.Daemon.MaxRetries = 3

	// API defaults
	config.API.Port = 8091
	config.API.Host = "localhost"

	// Embedding defaults
	config.Embedding.Provider = "ollama"
	config.Embedding.ModelName = "nomic-embed-text"
	config.Embedding.ServerURL = "http://localhost:11434"
	config.Embedding.Dimensions = 768
	config.Embedding.BatchSize = 8   // Reduced batch size for more reliable processing
	config.Embedding.MaxAttempts = 5 // Increased retry attempts
	config.Embedding.Timeout = 60    // Increased timeout to 60 seconds

	// Qdrant defaults
	config.Qdrant.Host = "localhost"
	config.Qdrant.Port = 6334
	config.Qdrant.APIKey = ""
	config.Qdrant.Embedded = true
	config.Qdrant.DataPath = filepath.Join(dataDir, "qdrant")
	config.Qdrant.Collection = "obsidian" // Default collection name for Obsidian documents
	config.Qdrant.Distance = "cosine"

	// Indexing defaults
	config.Indexing.ChunkStrategy = "hybrid"
	config.Indexing.MinChunkSize = 100
	config.Indexing.MaxChunkSize = 1000
	config.Indexing.WindowSize = 500
	config.Indexing.WindowOverlap = 100
	config.Indexing.IncludePatterns = []string{"*.md"}
	config.Indexing.ExcludePatterns = []string{".git/*", ".obsidian/*"}
	config.Indexing.BatchSize = 50
	config.Indexing.RescoreResults = true
	config.Indexing.ReindexOnStartup = false

	// FileWatcher defaults
	config.FileWatcher.DebounceTime = 500
	config.FileWatcher.ScanInterval = 600
	config.FileWatcher.MaxEventQueue = 1000
	config.FileWatcher.IgnoreDotFiles = true
	config.FileWatcher.IgnoreGitChanges = true

	return config
}

// ValidateConfig checks if the configuration is valid
func ValidateConfig(config *Config) error {
	// Validate data directory
	if config.General.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.General.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Validate vault paths
	vaultPaths := config.GetVaultPaths()
	if len(vaultPaths) == 0 {
		return fmt.Errorf("at least one vault path must be specified")
	}

	// Validate embedding
	if config.Embedding.Provider == "" {
		return fmt.Errorf("embedding provider cannot be empty")
	}

	if config.Embedding.ModelName == "" {
		return fmt.Errorf("embedding model name cannot be empty")
	}

	if config.Embedding.Dimensions <= 0 {
		return fmt.Errorf("embedding dimensions must be positive")
	}

	// Validate Qdrant
	if config.Qdrant.Collection == "" {
		return fmt.Errorf("qdrant collection name cannot be empty")
	}

	if config.Qdrant.Embedded {
		// Make sure the Qdrant data path exists
		if err := os.MkdirAll(config.Qdrant.DataPath, 0755); err != nil {
			return fmt.Errorf("failed to create Qdrant data directory: %w", err)
		}
	} else {
		// For external Qdrant, validate connection params
		if config.Qdrant.Host == "" {
			return fmt.Errorf("qdrant host cannot be empty for external Qdrant")
		}

		if config.Qdrant.Port <= 0 {
			return fmt.Errorf("qdrant port must be positive for external Qdrant")
		}
	}

	return nil
}

// GetQdrantURL returns the complete URL for Qdrant
func (c *Config) GetQdrantURL() string {
	return fmt.Sprintf("http://%s:%d", c.Qdrant.Host, c.Qdrant.Port)
}

// GetOllamaURL returns the Ollama server URL
func (c *Config) GetOllamaURL() string {
	return c.Embedding.ServerURL
}

// GetDaemonURL returns the URL for the daemon API
func (c *Config) GetDaemonURL() string {
	return fmt.Sprintf("http://%s:%d", c.Daemon.Host, c.Daemon.Port)
}

// GetDaemonSocketPath returns the path to the Unix socket (if using Unix socket)
func (c *Config) GetDaemonSocketPath() string {
	return filepath.Join(c.General.DataDir, "obsfind.sock")
}

// GetIndexingDebounceTime returns the debounce time for file indexing events
func (c *Config) GetIndexingDebounceTime() time.Duration {
	return time.Duration(c.FileWatcher.DebounceTime) * time.Millisecond
}

// GetEmbeddingTimeout returns the timeout for embedding operations
func (c *Config) GetEmbeddingTimeout() time.Duration {
	return time.Duration(c.Embedding.Timeout) * time.Second
}

// GetVaultPaths returns all vault paths from the configuration
func (c *Config) GetVaultPaths() []string {
	// If we have explicit vault paths, use those
	if len(c.Paths.VaultPaths) > 0 {
		return c.Paths.VaultPaths
	}

	// Otherwise, fall back to the single vault path if it's set
	if c.Paths.VaultPath != "" {
		return []string{c.Paths.VaultPath}
	}

	// Should never reach here as we always set at least one path in DefaultConfig
	return []string{}
}

// AddVaultPath adds a path to the list of vault paths if not already present
func (c *Config) AddVaultPath(path string) {
	// First, normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		// If we can't resolve the path, use the original
		absPath = path
	}

	// Check if the path is already in the list
	for _, existingPath := range c.Paths.VaultPaths {
		if existingPath == absPath {
			return // Path already exists, no need to add
		}
	}

	// Add the path to our list
	c.Paths.VaultPaths = append(c.Paths.VaultPaths, absPath)

	// Also update the single path field for backward compatibility
	// Only update if it's the first path
	if len(c.Paths.VaultPaths) == 1 {
		c.Paths.VaultPath = absPath
	}
}

// migrateConfiguration handles backward compatibility for configurations
func migrateConfiguration(config *Config) {
	// If we have a VaultPath but no VaultPaths, initialize the array
	if config.Paths.VaultPath != "" && len(config.Paths.VaultPaths) == 0 {
		config.Paths.VaultPaths = []string{config.Paths.VaultPath}
	}

	// If we have VaultPaths but no VaultPath, set the first one as VaultPath for backward compatibility
	if len(config.Paths.VaultPaths) > 0 && config.Paths.VaultPath == "" {
		config.Paths.VaultPath = config.Paths.VaultPaths[0]
	}
}

// mapConfigToViper maps the config struct to viper settings
func mapConfigToViper(config *Config) error {
	// General settings
	viper.Set("general.data_dir", config.General.DataDir)
	viper.Set("general.debug", config.General.Debug)

	// Path settings
	viper.Set("paths.vault_path", config.Paths.VaultPath)
	viper.Set("paths.vault_paths", config.Paths.VaultPaths)
	viper.Set("paths.config_path", config.Paths.ConfigPath)
	viper.Set("paths.cache_path", config.Paths.CachePath)

	// Daemon settings
	viper.Set("daemon.port", config.Daemon.Port)
	viper.Set("daemon.host", config.Daemon.Host)
	viper.Set("daemon.pid_file", config.Daemon.PIDFile)
	viper.Set("daemon.log_file", config.Daemon.LogFile)
	viper.Set("daemon.log_level", config.Daemon.LogLevel)
	viper.Set("daemon.max_retries", config.Daemon.MaxRetries)

	// API settings
	viper.Set("api.port", config.API.Port)
	viper.Set("api.host", config.API.Host)

	// Embedding settings
	viper.Set("embedding.provider", config.Embedding.Provider)
	viper.Set("embedding.model_name", config.Embedding.ModelName)
	viper.Set("embedding.server_url", config.Embedding.ServerURL)
	viper.Set("embedding.dimensions", config.Embedding.Dimensions)
	viper.Set("embedding.batch_size", config.Embedding.BatchSize)
	viper.Set("embedding.max_attempts", config.Embedding.MaxAttempts)
	viper.Set("embedding.timeout_seconds", config.Embedding.Timeout)

	// Qdrant settings
	viper.Set("qdrant.host", config.Qdrant.Host)
	viper.Set("qdrant.port", config.Qdrant.Port)
	viper.Set("qdrant.api_key", config.Qdrant.APIKey)
	viper.Set("qdrant.embedded", config.Qdrant.Embedded)
	viper.Set("qdrant.data_path", config.Qdrant.DataPath)
	viper.Set("qdrant.collection", config.Qdrant.Collection)
	viper.Set("qdrant.distance", config.Qdrant.Distance)

	// Indexing settings
	viper.Set("indexing.chunk_strategy", config.Indexing.ChunkStrategy)
	viper.Set("indexing.min_chunk_size", config.Indexing.MinChunkSize)
	viper.Set("indexing.max_chunk_size", config.Indexing.MaxChunkSize)
	viper.Set("indexing.window_size", config.Indexing.WindowSize)
	viper.Set("indexing.window_overlap", config.Indexing.WindowOverlap)
	viper.Set("indexing.include_patterns", config.Indexing.IncludePatterns)
	viper.Set("indexing.exclude_patterns", config.Indexing.ExcludePatterns)
	viper.Set("indexing.batch_size", config.Indexing.BatchSize)
	viper.Set("indexing.rescore_results", config.Indexing.RescoreResults)
	viper.Set("indexing.reindex_on_startup", config.Indexing.ReindexOnStartup)

	// FileWatcher settings
	viper.Set("file_watcher.debounce_time_ms", config.FileWatcher.DebounceTime)
	viper.Set("file_watcher.scan_interval_seconds", config.FileWatcher.ScanInterval)
	viper.Set("file_watcher.max_event_queue", config.FileWatcher.MaxEventQueue)
	viper.Set("file_watcher.ignore_dot_files", config.FileWatcher.IgnoreDotFiles)
	viper.Set("file_watcher.ignore_git_changes", config.FileWatcher.IgnoreGitChanges)

	return nil
}

// WriteConfig writes the configuration to the specified file path
func WriteConfig(config *Config, configPath string) error {
	// Ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set the config values in viper
	if err := mapConfigToViper(config); err != nil {
		return fmt.Errorf("failed to map config to viper: %w", err)
	}

	// Write the config file
	viper.SetConfigFile(configPath)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// CreateDefaultConfig creates a default configuration file at the specified path
// Returns the path to the created config file and any error encountered
func CreateDefaultConfig(configPath string) (string, error) {
	// If configPath is empty, use the default location
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir := filepath.Join(homeDir, ".config", "obsfind")
		configPath = filepath.Join(configDir, "config.yaml")
	}

	// Create default config
	config := DefaultConfig()

	// Write the config
	if err := WriteConfig(&config, configPath); err != nil {
		return "", err
	}

	return configPath, nil
}
