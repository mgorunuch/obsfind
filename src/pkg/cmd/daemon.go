package cmd

import (
	"context"
	"fmt"
	"obsfind/src/pkg/config"
	"obsfind/src/pkg/daemon"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

// RunDaemon starts the daemon process
func RunDaemon(configPath string, debug bool) error {
	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create daemon service
	service, err := daemon.NewService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create daemon service: %w", err)
	}

	// Start the daemon
	log.Info().Msg("Starting ObsFind daemon")
	if err := service.Start(ctx); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for shutdown signal from either sigChan or shutdownCh
	go func() {
		// Setup OS signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case sig := <-sigChan:
			log.Info().Str("signal", sig.String()).Msg("Received OS signal, shutting down...")
			TriggerShutdown() // Ensure we also trigger the shutdown channel
		case <-shutdownCh:
			log.Info().Msg("Received shutdown request")
		}

		// Set a timeout for graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// Initiate clean shutdown
		log.Info().Msg("Shutting down daemon")
		if err := service.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during shutdown")
		}

		// Signal completion
		SignalShutdownComplete()
	}()

	// Block until shutdown complete
	<-completeCh
	log.Info().Msg("Daemon shutdown completed")

	return nil
}

// DaemonizeProcess runs the process as a daemon
func DaemonizeProcess() (bool, error) {
	// Check if already daemonized
	if os.Getenv("OBSFIND_DAEMON") == "1" {
		return false, nil
	}

	// Fork the process
	args := os.Args
	env := os.Environ()
	env = append(env, "OBSFIND_DAEMON=1")

	procAttr := &os.ProcAttr{
		Env:   env,
		Files: []*os.File{nil, nil, nil}, // No stdin/stdout/stderr
		Sys:   nil,
	}

	// Fork a new process
	process, err := os.StartProcess(args[0], args, procAttr)
	if err != nil {
		return false, fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Detach from the child
	err = process.Release()
	if err != nil {
		return false, fmt.Errorf("failed to release daemon process: %w", err)
	}

	// Exit parent process
	return true, nil
}

// IsDaemonized returns true if process is running as a daemon
func IsDaemonized() bool {
	return os.Getenv("OBSFIND_DAEMON") == "1"
}

var (
	// shutdownCh is used to signal when shutdown is triggered
	shutdownCh = make(chan struct{})

	// completeCh is used to signal when shutdown is complete
	completeCh = make(chan struct{})

	// shutdownOnce ensures shutdown is only triggered once
	shutdownOnce sync.Once

	// completeOnce ensures shutdown completion is only signaled once
	completeOnce sync.Once
)

// TriggerShutdown requests the daemon to begin shutting down
func TriggerShutdown() {
	shutdownOnce.Do(func() {
		log.Debug().Msg("Shutdown triggered")
		close(shutdownCh)
	})
}

// ShutdownComplete returns a channel that's closed when shutdown is complete
func ShutdownComplete() <-chan struct{} {
	return completeCh
}

// SignalShutdownComplete should be called after daemon shutdown is complete
func SignalShutdownComplete() {
	completeOnce.Do(func() {
		log.Debug().Msg("Shutdown complete")
		close(completeCh)
	})
}
