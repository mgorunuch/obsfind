package main

import (
	"fmt"
	api2 "obsfind/src/pkg/api"
	"obsfind/src/pkg/config"
	consoleutil2 "obsfind/src/pkg/consoleutil"
	"obsfind/src/pkg/consts"
	"obsfind/src/pkg/indexer"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configPath string
	debug      bool
	version    = "0.1.0" // Will be set during build
)

func main() {
	// Create the root command
	rootCmd := &cobra.Command{
		Use:     "obsfind",
		Short:   "ObsFind - Semantic search for Obsidian vaults",
		Long:    `ObsFind provides semantic search capabilities for Obsidian markdown vaults using vector embeddings.`,
		Version: version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode")

	// Add commands
	rootCmd.AddCommand(
		newSearchCommand(),
		newSimilarCommand(),
		newStatusCommand(),
		newReindexCommand(),
		newStartCommand(),
		newStopCommand(),
		newConfigCommand(),
		newVaultCommand(),
		newLogsCommand(),
	)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newSearchCommand creates the search command
func newSearchCommand() *cobra.Command {
	var limit int
	var minScore float32
	var tags string
	var pathPrefix string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for content in your vault",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			// Create API client
			client, err := getClient()
			if err != nil {
				return err
			}

			// Split tags if provided
			var tagSlice []string
			if tags != "" {
				tagSlice = splitTags(tags)
			}

			// Create search request
			req := &api2.SearchRequest{
				Query:      query,
				Limit:      limit,
				MinScore:   minScore,
				Tags:       tagSlice,
				PathPrefix: pathPrefix,
			}

			// Execute search
			results, err := client.Search(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			// Display results
			if len(results) == 0 {
				fmt.Println("No results found.")
				return nil
			}

			// Check if results seem to be mock data
			isMockData := false
			for _, r := range results {
				if strings.HasPrefix(r.Path, "/folder") {
					isMockData = true
					break
				}
			}

			if isMockData {
				// Try to get service status to see component details
				var status *api2.StatusResponse
				status, _ = client.Status(cmd.Context())
				// If we can't get status, don't worry about it

				fmt.Println("WARNING: Results appear to be simulated data.")
				fmt.Println("The vault may not have been indexed yet. Try running 'obsfind reindex' first.")

				// Print component status if available
				if status != nil && status.Config != nil {
					fmt.Println("\nDiagnostic Information:")
					fmt.Printf("  Embedder: %s\n", status.Config["embedding_model"])
					fmt.Printf("  Total documents: %d\n", status.IndexStats.TotalDocuments)
					fmt.Printf("  Indexed documents: %d\n", status.IndexStats.IndexedDocuments)
				}
				fmt.Println("")
			}

			fmt.Printf("Found %d results for query: %s\n\n", len(results), query)

			// Format and print results
			displaySearchResults(convertToAPIResults(results))

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")
	cmd.Flags().Float32Var(&minScore, "score", 0.6, "Minimum similarity score (0-1)")
	cmd.Flags().StringVar(&tags, "tags", "", "Filter by tags (comma-separated)")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "Filter by path prefix")

	return cmd
}

// newSimilarCommand creates the similar command
func newSimilarCommand() *cobra.Command {
	var limit int
	var minScore float32
	var tags string
	var pathPrefix string

	cmd := &cobra.Command{
		Use:   "similar [file_path]",
		Short: "Find notes similar to a reference file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			// Check if file exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", filePath)
			}

			// Create API client
			client, err := getClient()
			if err != nil {
				return err
			}

			// Split tags if provided
			var tagSlice []string
			if tags != "" {
				tagSlice = splitTags(tags)
			}

			// Create similar request
			req := &api2.SimilarRequest{
				Path:       filePath,
				Limit:      limit,
				MinScore:   minScore,
				Tags:       tagSlice,
				PathPrefix: pathPrefix,
			}

			// Execute similar search
			results, err := client.Similar(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("similar search failed: %w", err)
			}

			// Display results
			if len(results) == 0 {
				fmt.Println("No similar documents found.")
				return nil
			}

			// Check if results seem to be mock data
			isMockData := false
			for _, r := range results {
				if strings.HasPrefix(r.Path, "/path/to/similar") {
					isMockData = true
					break
				}
			}

			if isMockData {
				// Try to get service status to see component details
				var status *api2.StatusResponse
				status, _ = client.Status(cmd.Context())
				// If we can't get status, don't worry about it

				fmt.Println("WARNING: Results appear to be simulated data.")
				fmt.Println("The vault may not have been indexed yet. Try running 'obsfind reindex' first.")

				// Print component status if available
				if status != nil && status.Config != nil {
					fmt.Println("\nDiagnostic Information:")
					fmt.Printf("  Embedder: %s\n", status.Config["embedding_model"])
					fmt.Printf("  Total documents: %d\n", status.IndexStats.TotalDocuments)
					fmt.Printf("  Indexed documents: %d\n", status.IndexStats.IndexedDocuments)
				}
				fmt.Println("")
			}

			fmt.Printf("Found %d documents similar to: %s\n\n", len(results), filePath)

			// Format and print results
			displaySearchResults(convertToAPIResults(results))

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")
	cmd.Flags().Float32Var(&minScore, "score", 0.6, "Minimum similarity score (0-1)")
	cmd.Flags().StringVar(&tags, "tags", "", "Filter by tags (comma-separated)")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "Filter by path prefix")

	return cmd
}

// newStatusCommand creates the status command with colorful display
func newStatusCommand() *cobra.Command {
	var watch bool
	var interval int

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check daemon and indexing status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create API client
			client, err := getClient()
			if err != nil {
				return err
			}

			// Check daemon health
			healthy, err := client.Health(cmd.Context())
			if err != nil || !healthy {
				return fmt.Errorf("daemon is not running or not responding. Start the daemon with 'obsfind start' before using this command")
			}

			// Get daemon status
			status, err := client.Status(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get daemon status: %w", err)
			}

			// Get vault paths from config for display
			cfg, err := config.LoadConfig(configPath)

			// Display colored status information
			fmt.Println(consoleutil2.Format("ObsFind Status", consoleutil2.Bold, consoleutil2.FgCyan))
			fmt.Println(consoleutil2.Format("==============", consoleutil2.Bold, consoleutil2.FgCyan))
			fmt.Println("")

			// Daemon status table
			daemonTable := consoleutil2.NewStatusTable("System Status")

			// Determine daemon status type based on status string
			daemonStatus := consoleutil2.StatusActive
			if status.Status != "running" {
				daemonStatus = consoleutil2.StatusPending
			}

			daemonTable.AddRow("Daemon", fmt.Sprintf("%s (Uptime: %s)", status.Status, status.Uptime), daemonStatus)
			daemonTable.AddRow("Version", status.Version, consoleutil2.StatusActive)

			// Index status based on whether indexing is active
			indexStatus := consoleutil2.StatusActive
			if status.IndexStats.Status == "indexing" {
				indexStatus = consoleutil2.StatusPending
			}
			daemonTable.AddRow("Indexer", status.IndexStats.Status, indexStatus)

			fmt.Println(daemonTable.Render())

			// Index stats as a status block
			indexItems := map[string]consoleutil2.StatusRow{
				"total": {
					Label:  "Total Documents",
					Value:  strconv.Itoa(status.IndexStats.TotalDocuments),
					Status: consoleutil2.StatusActive,
				},
				"indexed": {
					Label:  "Indexed Documents",
					Value:  strconv.Itoa(status.IndexStats.IndexedDocuments),
					Status: consoleutil2.StatusActive,
				},
				"failed": {
					Label:  "Failed Documents",
					Value:  strconv.Itoa(status.IndexStats.FailedDocuments),
					Status: getStatusForFailedDocs(status.IndexStats.FailedDocuments),
				},
			}

			// Add indexing progress bar if currently indexing
			if status.IndexStats.Status == "indexing" {
				// Calculate percentage
				var percentComplete float64
				if status.IndexStats.TotalDocuments > 0 {
					percentComplete = float64(status.IndexStats.IndexedDocuments) / float64(status.IndexStats.TotalDocuments) * 100
				}

				// Add indexing progress
				indexItems["progress"] = consoleutil2.StatusRow{
					Label:  "Indexing Progress",
					Value:  fmt.Sprintf("%.1f%%", percentComplete),
					Status: consoleutil2.StatusPending,
				}

				// Add visual progress bar
				fmt.Println(consoleutil2.FormatStatusBlock("Index Statistics", indexItems))
				fmt.Printf("\n%s\n\n", consoleutil2.ProgressBar(int(percentComplete), 50))
			} else {
				fmt.Println(consoleutil2.FormatStatusBlock("Index Statistics", indexItems))
			}

			// Configuration display
			configItems := map[string]consoleutil2.StatusRow{
				"model": {
					Label:  "Embedding Model",
					Value:  status.Config["embedding_model"],
					Status: consoleutil2.StatusActive,
				},
				"chunking": {
					Label:  "Chunking Strategy",
					Value:  status.Config["chunking_strategy"],
					Status: consoleutil2.StatusActive,
				},
			}

			fmt.Println(consoleutil2.FormatStatusBlock("Configuration", configItems))

			// Display vault paths if available
			if err == nil && len(cfg.GetVaultPaths()) > 0 {
				fmt.Println(consoleutil2.Format("\nVault Paths:", consoleutil2.Bold))
				paths := cfg.GetVaultPaths()
				for i, path := range paths {
					pathStatus := consoleutil2.StatusActive
					fmt.Printf("  %s\n", consoleutil2.FormatServiceStatus(
						fmt.Sprintf("Path %d", i+1),
						path,
						pathStatus,
					))
				}
			}

			// Display help text for common operations
			fmt.Println("\n" + consoleutil2.Format("Common Operations:", consoleutil2.Bold, consoleutil2.FgCyan))
			fmt.Println("  " + consoleutil2.Format("obsfind search", consoleutil2.Bold) + " \"query\"    Search your vault")
			fmt.Println("  " + consoleutil2.Format("obsfind reindex", consoleutil2.Bold) + "           Force reindex of vault")
			fmt.Println("  " + consoleutil2.Format("obsfind logs", consoleutil2.Bold) + " --follow    View daemon logs")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch status in real-time")
	cmd.Flags().IntVarP(&interval, "interval", "i", 5, "Update interval in seconds for watch mode")

	return cmd
}

// getStatusForFailedDocs returns the appropriate status for failed documents count
func getStatusForFailedDocs(failedCount int) consoleutil2.Status {
	if failedCount > 0 {
		return consoleutil2.StatusInactive
	}
	return consoleutil2.StatusActive
}

// newReindexCommand creates the reindex command
func newReindexCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Reindex vault contents",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create API client
			client, err := getClient()
			if err != nil {
				return err
			}

			// Check daemon health
			healthy, err := client.Health(cmd.Context())
			if err != nil || !healthy {
				return fmt.Errorf("daemon is not running or not responding. Start the daemon with 'obsfind start' before using this command")
			}

			fmt.Println("Starting reindexing of vault content...")

			// Execute reindexing
			if err := client.Reindex(cmd.Context(), false); err != nil {
				return fmt.Errorf("reindexing failed: %w", err)
			}

			fmt.Println("Reindexing started successfully.")
			fmt.Println("Use 'obsfind status' to check progress.")

			return nil
		},
	}

	return cmd
}

// newStartCommand creates the start command for the daemon
func newStartCommand() *cobra.Command {
	var foreground bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if daemon is already running
			client, _ := getClient()
			healthy, _ := client.Health(cmd.Context())
			if healthy {
				fmt.Println("ObsFind daemon is already running.")
				return nil
			}

			// Find obsfindd executable
			daemonBin, err := exec.LookPath("obsfindd")
			if err != nil {
				return fmt.Errorf("obsfindd executable not found in PATH: %w", err)
			}

			fmt.Println("Starting ObsFind daemon...")

			daemonArgs := []string{}
			if configPath != "" {
				daemonArgs = append(daemonArgs, "--config", configPath)
			}
			if debug {
				daemonArgs = append(daemonArgs, "--debug")
			}

			if foreground {
				// Start daemon in foreground
				daemonCmd := exec.Command(daemonBin, daemonArgs...)
				daemonCmd.Stdout = os.Stdout
				daemonCmd.Stderr = os.Stderr

				return daemonCmd.Run()
			} else {
				// Start daemon in background
				daemonArgs = append(daemonArgs, "--daemon")
				daemonCmd := exec.Command(daemonBin, daemonArgs...)

				err = daemonCmd.Start()
				if err != nil {
					return fmt.Errorf("failed to start daemon: %w", err)
				}

				fmt.Println("ObsFind daemon started in background.")
				return nil
			}
		},
	}

	cmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground instead of as daemon")

	return cmd
}

// newStopCommand creates the stop command for the daemon
func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find daemon process
			daemonProcess, err := findDaemonProcess()
			if err != nil {
				return fmt.Errorf("failed to find daemon process: %w", err)
			}

			if daemonProcess == 0 {
				return fmt.Errorf("daemon is not running. Start the daemon with 'obsfind start' before using this command")
			}

			// Send SIGTERM to the daemon
			process, err := os.FindProcess(daemonProcess)
			if err != nil {
				return fmt.Errorf("failed to find process: %w", err)
			}

			fmt.Println("Stopping ObsFind daemon...")
			if err := process.Signal(os.Interrupt); err != nil {
				return fmt.Errorf("failed to send signal: %w", err)
			}

			fmt.Println("Daemon stopping...")
			return nil
		},
	}

	return cmd
}

// getClient returns an API client configured from settings
func getClient() (*api2.Client, error) {
	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create API client
	baseURL := fmt.Sprintf("http://%s:%d", cfg.API.Host, cfg.API.Port)
	return api2.NewClient(baseURL), nil
}

// findDaemonProcess attempts to find the daemon process ID
func findDaemonProcess() (int, error) {
	// This is a simplified implementation that would need to be
	// replaced with a more robust process-finding mechanism
	cmd := exec.Command("pgrep", "obsfindd")
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns 1 when no processes match
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return 0, nil
		}
		return 0, err
	}

	var pid int
	_, err = fmt.Sscanf(string(output), "%d", &pid)
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// splitTags splits a comma-separated string into a slice
func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}

	var result []string
	for _, tag := range filepath.SplitList(tags) {
		if tag != "" {
			result = append(result, tag)
		}
	}

	return result
}

// convertToAPIResults converts indexer search results to API search results
func convertToAPIResults(results []indexer.SearchResult) []api2.SearchResult {
	apiResults := make([]api2.SearchResult, len(results))
	for i, r := range results {
		apiResults[i] = api2.SearchResult{
			ID:       fmt.Sprintf("result-%d", i+1),
			Path:     r.Path,
			Title:    r.Title,
			Score:    float32(r.Score),
			Tags:     r.Tags,
			Section:  r.Section,
			Metadata: r.Metadata,
			Excerpt:  r.Content,
		}
	}
	return apiResults
}

// displaySearchResults formats and displays search results
func displaySearchResults(results []api2.SearchResult) {
	for i, result := range results {
		fmt.Printf("%d. [%.2f] %s\n", i+1, result.Score, result.Title)
		fmt.Printf("   Path: %s\n", result.Path)
		if result.Section != "" {
			fmt.Printf("   Section: %s\n", result.Section)
		}
		if len(result.Tags) > 0 {
			fmt.Printf("   Tags: %v\n", result.Tags)
		}
		fmt.Printf("   Content: %s\n", truncateString(result.Content, 80))
		fmt.Println()
	}
}

// truncateString truncates a string to maxLength and adds "..." if needed
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}

// newConfigCommand creates the config command for managing configuration
func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ObsFind configuration",
		Long:  `Create, view, and manage ObsFind configuration files.`,
	}

	// Add subcommands
	cmd.AddCommand(
		newConfigInitCommand(),
		newConfigViewCommand(),
		newConfigShowPathCommand(),
		newConfigSetCommand(),
		newConfigTemplateCommand(),
	)

	return cmd
}

// newConfigInitCommand creates a command to initialize a new config file
func newConfigInitCommand() *cobra.Command {
	var outputPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new default configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If output path not specified, use default
			finalPath := outputPath
			if finalPath == "" && configPath != "" {
				finalPath = configPath
			}

			// Check if file already exists
			if finalPath != "" {
				if _, err := os.Stat(finalPath); err == nil && !force {
					return fmt.Errorf("config file already exists at %s. Use --force to overwrite", finalPath)
				}
			}

			// Create default config
			path, err := config.CreateDefaultConfig(finalPath)
			if err != nil {
				return fmt.Errorf("failed to create default config: %w", err)
			}

			fmt.Printf("Created default configuration file at: %s\n", path)
			fmt.Println("You may want to edit this file to match your setup.")
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Path where the config file should be created")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config file if it exists")

	return cmd
}

// newConfigViewCommand creates a command to view the current config
func newConfigViewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View the current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Display configuration settings
			fmt.Println("ObsFind Configuration")
			fmt.Println("====================")

			fmt.Println("\nGeneral Settings:")
			fmt.Printf("Data Directory: %s\n", cfg.General.DataDir)
			fmt.Printf("Debug Mode: %v\n", cfg.General.Debug)

			fmt.Println("\nPaths:")
			fmt.Printf("Vault Path: %s\n", cfg.Paths.VaultPath)

			// Display all vault paths
			vaultPaths := cfg.GetVaultPaths()
			if len(vaultPaths) > 0 {
				fmt.Println("Vault Paths:")
				for i, path := range vaultPaths {
					fmt.Printf("  %d. %s\n", i+1, path)
				}
			}

			fmt.Printf("Config Path: %s\n", cfg.Paths.ConfigPath)
			fmt.Printf("Cache Path: %s\n", cfg.Paths.CachePath)

			fmt.Println("\nEmbedding Model:")
			fmt.Printf("Provider: %s\n", cfg.Embedding.Provider)
			fmt.Printf("Model: %s\n", cfg.Embedding.ModelName)
			fmt.Printf("Dimensions: %d\n", cfg.Embedding.Dimensions)
			fmt.Printf("Server URL: %s\n", cfg.Embedding.ServerURL)

			fmt.Println("\nQdrant Vector Database:")
			fmt.Printf("Embedded: %v\n", cfg.Qdrant.Embedded)
			if cfg.Qdrant.Embedded {
				fmt.Printf("Data Path: %s\n", cfg.Qdrant.DataPath)
			} else {
				fmt.Printf("Host: %s\n", cfg.Qdrant.Host)
				fmt.Printf("Port: %d\n", cfg.Qdrant.Port)
			}
			fmt.Printf("Collection: %s\n", cfg.Qdrant.Collection)

			fmt.Println("\nIndexing Settings:")
			fmt.Printf("Chunk Strategy: %s\n", cfg.Indexing.ChunkStrategy)
			fmt.Printf("Chunk Size Range: %d-%d\n", cfg.Indexing.MinChunkSize, cfg.Indexing.MaxChunkSize)
			fmt.Printf("Include Patterns: %v\n", cfg.Indexing.IncludePatterns)
			fmt.Printf("Exclude Patterns: %v\n", cfg.Indexing.ExcludePatterns)

			return nil
		},
	}

	return cmd
}

// newConfigShowPathCommand creates a command to show the config file path
func newConfigShowPathCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show the current config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			var configFilePath string

			// If config path specified, use it
			if configPath != "" {
				configFilePath = configPath
			} else {
				// Otherwise try to find the currently used config
				v := cmd.Context().Value("viper")
				if v != nil {
					configFilePath = v.(string)
				} else {
					// Fall back to default location
					homeDir, err := os.UserHomeDir()
					if err != nil {
						return fmt.Errorf("failed to get user home directory: %w", err)
					}
					configFilePath = filepath.Join(homeDir, ".config", "obsfind", "config.yaml")
				}
			}

			// Check if the file exists
			_, err := os.Stat(configFilePath)
			if os.IsNotExist(err) {
				fmt.Printf("Config file does not exist at: %s\n", configFilePath)
				fmt.Println("Run 'obsfind config init' to create a default config file.")
				return nil
			}

			fmt.Printf("Config file path: %s\n", configFilePath)
			return nil
		},
	}

	return cmd
}

// newConfigSetCommand creates a command to set config values
func newConfigSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Long: `Set a specific configuration value using dot notation.
Examples:
  obsfind config set embedding.model_name all-MiniLM-L6-v2
  obsfind config set indexing.reindex_on_startup true
  obsfind config set qdrant.embedded false`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			// Determine the config file path
			cfgPath := configPath
			if cfgPath == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get user home directory: %w", err)
				}
				cfgPath = filepath.Join(homeDir, ".config", "obsfind", "config.yaml")
			}

			// Check if the config file exists
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				return fmt.Errorf("config file not found at %s. Use 'obsfind config init' to create one", cfgPath)
			}

			// Check if we can load the configuration
			_, err := config.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Set up viper with the existing config
			viper.SetConfigFile(cfgPath)
			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read config file: %w", err)
			}

			// Set the new value
			viper.Set(key, parseValue(value))

			// Save the configuration
			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Printf("Set %s = %s in %s\n", key, value, cfgPath)
			return nil
		},
	}

	return cmd
}

// newConfigTemplateCommand creates a command to generate template configs
func newConfigTemplateCommand() *cobra.Command {
	var outputPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "template [type]",
		Short: "Generate a configuration template for specific setups",
		Long: `Generate a configuration template for specific setups.
Available types:
  1. standard - Standard configuration for local use
  2. server   - Configuration optimized for server deployment 
  3. docker   - Configuration for Docker environment
  4. large    - Configuration optimized for large vaults`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateType := args[0]

			// Determine output path
			finalPath := outputPath
			if finalPath == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get user home directory: %w", err)
				}
				configDir := filepath.Join(homeDir, ".config", "obsfind")
				finalPath = filepath.Join(configDir, fmt.Sprintf("config-%s.yaml", templateType))
			}

			// Check if file already exists
			if _, err := os.Stat(finalPath); err == nil && !force {
				return fmt.Errorf("file already exists at %s. Use --force to overwrite", finalPath)
			}

			// Create config based on template type
			cfg := config.DefaultConfig()

			switch templateType {
			case "standard":
				// Standard config is just the default

			case "server":
				// Server deployment optimizations
				cfg.API.Host = "0.0.0.0" // Listen on all interfaces
				cfg.API.Port = 8080
				cfg.Daemon.Host = "0.0.0.0"
				cfg.Embedding.BatchSize = 32
				cfg.Embedding.MaxAttempts = 5
				cfg.Indexing.BatchSize = 100
				cfg.FileWatcher.ScanInterval = 900 // 15 minutes

			case "docker":
				// Docker environment
				cfg.Embedding.ServerURL = "http://host.docker.internal:11434"
				cfg.API.Host = "0.0.0.0"
				cfg.Daemon.Host = "0.0.0.0"
				cfg.Qdrant.Host = "host.docker.internal"

			case "large":
				// For large vaults
				cfg.Embedding.BatchSize = 64
				cfg.Indexing.BatchSize = 200
				cfg.Indexing.MaxChunkSize = 1500
				cfg.Indexing.WindowSize = 750
				cfg.FileWatcher.DebounceTime = 1000 // 1 second
				cfg.FileWatcher.MaxEventQueue = 5000

			default:
				return fmt.Errorf("unknown template type: %s. Use 'standard', 'server', 'docker', or 'large'", templateType)
			}

			// Write the config
			if err := config.WriteConfig(&cfg, finalPath); err != nil {
				return fmt.Errorf("failed to write config template: %w", err)
			}

			fmt.Printf("Created %s config template at: %s\n", templateType, finalPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Path where the config template should be created")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing file if it exists")

	return cmd
}

// newVaultCommand creates the vault management command
func newVaultCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage vault paths",
		Long:  `Add, remove, or list vault paths for indexing.`,
	}

	// Add subcommands
	cmd.AddCommand(
		newVaultListCommand(),
		newVaultAddCommand(),
		newVaultRemoveCommand(),
	)

	return cmd
}

// newVaultListCommand creates a command to list configured vault paths
func newVaultListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured vault paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Get vault paths
			paths := cfg.GetVaultPaths()

			if len(paths) == 0 {
				fmt.Println("No vault paths configured.")
				return nil
			}

			fmt.Println("Configured vault paths:")
			for i, path := range paths {
				fmt.Printf("%d. %s\n", i+1, path)
			}

			return nil
		},
	}

	return cmd
}

// newVaultAddCommand creates a command to add a vault path
func newVaultAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "Add a vault path to the configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultPath := args[0]

			// Resolve to absolute path
			absPath, err := filepath.Abs(vaultPath)
			if err != nil {
				return fmt.Errorf("failed to resolve absolute path: %w", err)
			}

			// Check if path exists
			info, err := os.Stat(absPath)
			if err != nil {
				return fmt.Errorf("path does not exist or is not accessible: %w", err)
			}

			if !info.IsDir() {
				return fmt.Errorf("path is not a directory: %s", absPath)
			}

			// Load configuration
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Add vault path
			cfg.AddVaultPath(absPath)

			// Save configuration
			configToUse := configPath
			if configToUse == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get user home directory: %w", err)
				}
				configToUse = filepath.Join(homeDir, ".config", "obsfind", "config.yaml")
			}

			if err := config.WriteConfig(cfg, configToUse); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Added vault path: %s\n", absPath)
			return nil
		},
	}

	return cmd
}

// newVaultRemoveCommand creates a command to remove a vault path
func newVaultRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [path]",
		Short: "Remove a vault path from the configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultPath := args[0]

			// Resolve to absolute path
			absPath, err := filepath.Abs(vaultPath)
			if err != nil {
				return fmt.Errorf("failed to resolve absolute path: %w", err)
			}

			// Load configuration
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Get current paths
			currentPaths := cfg.GetVaultPaths()

			// Check if path exists in config
			found := false
			newPaths := make([]string, 0, len(currentPaths))

			for _, path := range currentPaths {
				if path == absPath {
					found = true
				} else {
					newPaths = append(newPaths, path)
				}
			}

			if !found {
				return fmt.Errorf("vault path not found in configuration: %s", absPath)
			}

			// Make sure we have at least one vault path
			if len(newPaths) == 0 {
				return fmt.Errorf("cannot remove the last vault path; at least one vault path is required")
			}

			// Update configuration
			cfg.Paths.VaultPaths = newPaths
			cfg.Paths.VaultPath = newPaths[0] // Update for backward compatibility

			// Save configuration
			configToUse := configPath
			if configToUse == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get user home directory: %w", err)
				}
				configToUse = filepath.Join(homeDir, ".config", "obsfind", "config.yaml")
			}

			if err := config.WriteConfig(cfg, configToUse); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Removed vault path: %s\n", absPath)
			return nil
		},
	}

	return cmd
}

// newLogsCommand creates the logs command to view daemon logs
func newLogsCommand() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View daemon logs",
		Long:  `View and follow daemon logs. Use the --follow flag to continuously monitor logs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the log file path
			logFilePath, err := getDaemonLogPath()
			if err != nil {
				return fmt.Errorf("failed to determine log file path: %w", err)
			}

			// Check if log file exists
			if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
				return fmt.Errorf("log file not found at: %s", logFilePath)
			}

			// If follow flag is set, use tail -f (macOS/Linux) or equivalent for Windows
			if follow {
				fmt.Printf("Following log file: %s\n", logFilePath)
				fmt.Println("Press Ctrl+C to exit")

				var cmd *exec.Cmd
				if isWindows() {
					// PowerShell equivalent of tail -f for Windows
					cmd = exec.Command("powershell", "-Command",
						fmt.Sprintf("Get-Content -Path \"%s\" -Wait", logFilePath))
				} else {
					// Use tail -f for macOS/Linux
					cmd = exec.Command("tail", "-f", logFilePath)
				}

				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				return cmd.Run()
			} else {
				// Just show the current logs
				fmt.Printf("Showing logs from: %s\n\n", logFilePath)

				var cmd *exec.Cmd
				if isWindows() {
					// Use PowerShell to display file content on Windows
					cmd = exec.Command("powershell", "-Command",
						fmt.Sprintf("Get-Content -Path \"%s\"", logFilePath))
				} else {
					// Use cat for macOS/Linux
					cmd = exec.Command("cat", logFilePath)
				}

				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				return cmd.Run()
			}
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")

	return cmd
}

// getDaemonLogPath returns the path to the daemon log file
func getDaemonLogPath() (string, error) {
	// First, try to use the built-in function from consts package
	logPath, err := consts.GetDaemonLogFilePath()
	if err == nil {
		return logPath, nil
	}

	// If that fails, try to derive the path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, consts.DefaultConfigDirPath, consts.LogDirectoryName, consts.DefaultDaemonLogFileName), nil
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

// parseValue attempts to parse string values into appropriate types
func parseValue(value string) interface{} {
	// Try to parse as boolean
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// Try to parse as integer
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}

	// Try to parse as float
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}

	// If value starts with [ and ends with ], treat as array
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		// Strip the brackets
		items := value[1 : len(value)-1]
		// Split by comma
		parts := strings.Split(items, ",")

		// Create a slice of strings
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			// Trim whitespace and quotes
			part = strings.Trim(part, " \t\"'")
			if part != "" {
				result = append(result, part)
			}
		}

		return result
	}

	// Default to string
	return value
}
