package main

import (
	"context"
	"fmt"
	"obsfind/src/pkg/cmd"
	consts2 "obsfind/src/pkg/consts"
	loggingutil2 "obsfind/src/pkg/loggingutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	configPath string
	debug      bool
	daemonize  bool
	version    = "0.1.0" // Will be set during build
)

func main() {
	// Create the root command
	rootCmd := createRootCommand()

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// createRootCommand sets up the root command and its flags
func createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "obsfindd",
		Short: "ObsFind daemon for semantic search",
		Long: `Daemon process for ObsFind that provides semantic search capabilities.
It watches your vault, indexes content, and serves search queries.`,
		Version: version,
		RunE:    runDaemon,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode")
	rootCmd.PersistentFlags().BoolVar(&daemonize, "daemon", false, "Run as daemon in background")

	return rootCmd
}

// setupLogging configures the logging based on runtime mode
func setupLogging(ctx context.Context) (context.Context, error) {
	// Set log level based on debug flag or environment variable
	if debug || os.Getenv("DEBUG") != "" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	var logger loggingutil2.Logger

	// Configure logging output
	if cmd.IsDaemonized() {
		fileLogger, err := setupFileLogging()
		if err != nil {
			return ctx, err
		}
		logger = fileLogger
	} else {
		// Console logging for interactive mode
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr}
		zerologLogger := zerolog.New(consoleWriter).With().Timestamp().Logger()

		// Adapt zerolog to our Logger interface
		logger = loggingutil2.NewZerologAdapter(zerologLogger)
	}

	// Store logger in context
	ctx = loggingutil2.Set(ctx, logger)
	return ctx, nil
}

// We use the ZerologAdapter from the loggingutil package
// to adapt zerolog loggers to our Logger interface

// setupFileLogging configures logging to a file
func setupFileLogging() (loggingutil2.Logger, error) {
	// Ensure log directory exists
	_, err := consts2.EnsureLogDirectoryExists()
	if err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Get log file path
	logFilePath, err := consts2.GetDaemonLogFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get log file path: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, consts2.LogFilePermissions)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	zerologLogger := zerolog.New(file).With().Timestamp().Logger()
	return loggingutil2.NewZerologAdapter(zerologLogger), nil
}

// findConfigPath tries to locate config file if not explicitly provided
func findConfigPath(ctx context.Context) string {
	logger := loggingutil2.Get(ctx)

	if configPath != "" {
		return configPath
	}

	// Try standard locations from consts package
	locations := consts2.ConfigFileLocations()

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			logger.Debug("Found config file", "path", path)
			return path
		}
	}

	return ""
}

// handleDaemonization manages the process daemonization if requested
func handleDaemonization() (bool, error) {
	if !daemonize || cmd.IsDaemonized() {
		return false, nil
	}

	shouldExit, err := cmd.DaemonizeProcess()
	if err != nil {
		return false, fmt.Errorf("failed to daemonize: %w", err)
	}

	if shouldExit {
		fmt.Println("ObsFind daemon started in background")
		return true, nil
	}

	return false, nil
}

// setupSignalHandling establishes handlers for graceful shutdown
func setupSignalHandling(ctx context.Context) {
	logger := loggingutil2.Get(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		logger.Info("Received shutdown signal", "signal", sig.String())

		// Let daemon know it should gracefully shut down
		cmd.TriggerShutdown()

		// Force exit after a timeout if graceful shutdown fails
		forceExit := make(chan struct{})
		go func() {
			time.Sleep(10 * time.Second)
			close(forceExit)
		}()

		select {
		case <-cmd.ShutdownComplete():
			logger.Info("Graceful shutdown completed")
		case <-forceExit:
			logger.Warn("Forcing shutdown after timeout")
		}

		os.Exit(0)
	}()
}

func runDaemon(_ *cobra.Command, _ []string) error {
	// Create a base context
	ctx := context.Background()

	// Set up logging
	var err error
	ctx, err = setupLogging(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up logging: %v\n", err)
		os.Exit(1)
	}

	logger := loggingutil2.Get(ctx)

	// Handle daemonization if requested
	shouldExit, err := handleDaemonization()
	if err != nil {
		logger.Error("Failed to daemonize", "error", err)
		os.Exit(1)
	}
	if shouldExit {
		os.Exit(0)
	}

	// Set up signal handling for graceful shutdown
	setupSignalHandling(ctx)

	// Find config file if not specified
	effectiveConfigPath := findConfigPath(ctx)

	// Run the daemon - need to adapt to the existing API that doesn't use our custom context
	return cmd.RunDaemon(effectiveConfigPath, debug)
}
