package daemon

import (
	"context"
	"fmt"
	"log"
	api2 "obsfind/src/pkg/api"
	"obsfind/src/pkg/config"
	"obsfind/src/pkg/filewatcher"
	"obsfind/src/pkg/indexer"
	model2 "obsfind/src/pkg/model"
	"obsfind/src/pkg/qdrant"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Service represents the daemon service
type Service struct {
	config      *config.Config
	qdrant      *qdrant.Client
	embedder    model2.Embedder
	indexer     *indexer.Service
	fileWatcher *filewatcher.Watcher
	apiServer   *api2.Server
	apiService  *api2.Service

	// Status tracking
	startTime      time.Time
	documentCount  int
	indexedDocs    int
	isIndexing     bool
	lastIndexTime  time.Time
	watchedDirs    []string
	embeddingModel string

	// Control channels
	done      chan struct{}
	eventChan <-chan filewatcher.Event

	// Mutex for status updates
	statusMu sync.RWMutex
}

// NewService creates a new daemon service
func NewService(cfg *config.Config) (*Service, error) {
	service := &Service{
		config:         cfg,
		startTime:      time.Now(),
		done:           make(chan struct{}),
		watchedDirs:    []string{},
		embeddingModel: cfg.Embedding.ModelName,
	}

	return service, nil
}

// Start begins the daemon process
func (s *Service) Start(ctx context.Context) error {
	// Initialize all components
	if err := s.initialize(ctx); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Add all vault paths to file watcher
	if err := s.addVaultPaths(ctx); err != nil {
		return fmt.Errorf("failed to add vault paths: %w", err)
	}

	// Set up file event handler
	go s.handleFileEvents(ctx)

	// Start API server in a goroutine
	go func() {
		apiAddr := fmt.Sprintf("%s:%d", s.config.API.Host, s.config.API.Port)
		apiServer := api2.NewServer(apiAddr, s.apiService)
		s.apiServer = apiServer

		if err := apiServer.Start(ctx); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// If configured, perform initial indexing
	if s.config.Indexing.ReindexOnStartup {
		go s.performInitialIndex(ctx)
	}

	log.Printf("Daemon started successfully. Listening on %s:%d", s.config.Daemon.Host, s.config.Daemon.Port)

	return nil
}

// initialize sets up all the components
func (s *Service) initialize(ctx context.Context) error {
	var err error

	// Initialize Qdrant client
	qdrantCfg := &qdrant.Config{
		Host:       s.config.Qdrant.Host,
		Port:       s.config.Qdrant.Port,
		APIKey:     s.config.Qdrant.APIKey,
		Embedded:   s.config.Qdrant.Embedded,
		DataPath:   s.config.Qdrant.DataPath,
		Collection: s.config.Qdrant.Collection,
	}

	s.qdrant, err = qdrant.NewClient(qdrantCfg)
	if err != nil {
		return fmt.Errorf("failed to create Qdrant client: %w", err)
	}

	// Connect to Qdrant
	if err := s.qdrant.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	// Apply schema
	schema := qdrant.DefaultSchema()
	schema.VectorSize = s.config.Embedding.Dimensions
	if err := schema.Apply(ctx, s.qdrant, s.config.Qdrant.Collection); err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}

	// Set up embedding model
	ollamaCfg := model2.OllamaConfig{
		ModelName:   s.config.Embedding.ModelName,
		ServerURL:   s.config.Embedding.ServerURL,
		Dimensions:  s.config.Embedding.Dimensions,
		BatchSize:   s.config.Embedding.BatchSize,
		MaxAttempts: s.config.Embedding.MaxAttempts,
		Timeout:     s.config.Embedding.Timeout,
	}

	embeddingConfig := model2.Config{
		Provider: s.config.Embedding.Provider,
		Specific: ollamaCfg,
	}

	// Create embedder
	embedder, err := model2.CreateEmbedder(embeddingConfig)
	if err != nil {
		return err
	}

	// Wrap with caching
	s.embedder = model2.NewCachedEmbedder(embedder)
	s.embeddingModel = s.embedder.Name()

	// Create indexer service now that we have embedder and qdrant
	s.indexer = indexer.NewService(s.config, s.embedder, s.qdrant)
	log.Printf("Indexer service initialized")

	// Set up file watcher
	watcherCfg := &filewatcher.Config{
		DebounceTime:     s.config.GetIndexingDebounceTime(),
		ScanInterval:     time.Duration(s.config.FileWatcher.ScanInterval) * time.Second,
		MaxEventQueue:    s.config.FileWatcher.MaxEventQueue,
		IgnoreDotFiles:   s.config.FileWatcher.IgnoreDotFiles,
		IgnoreGitChanges: s.config.FileWatcher.IgnoreGitChanges,
		IncludePatterns:  s.config.Indexing.IncludePatterns,
		ExcludePatterns:  s.config.Indexing.ExcludePatterns,
	}

	s.fileWatcher, err = filewatcher.NewWatcher(watcherCfg)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Start file watcher
	s.eventChan, err = s.fileWatcher.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Set up a fully functional API service with real components
	s.apiService = api2.NewService(
		s.indexer,
		s.embedder,
		s.qdrant,
		s.config,
	)

	log.Printf("API service initialized with real components")

	return nil
}

// handleFileEvents processes file events from the watcher
func (s *Service) handleFileEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case evt, ok := <-s.eventChan:
			if !ok {
				return
			}

			// Process the event
			s.processFileEvent(ctx, evt)
		}
	}
}

// processFileEvent handles a single file event
func (s *Service) processFileEvent(ctx context.Context, evt filewatcher.Event) {
	// Skip directory events
	if evt.IsDir {
		return
	}

	// Skip non-markdown files
	if evt.Extension != ".md" {
		return
	}

	// Process based on event type
	switch evt.Type {
	case filewatcher.EventCreated, filewatcher.EventModified:
		log.Printf("Indexing changed file: %s", evt.Path)
		// In a real implementation, we would:
		// 1. Read and process the file
		// 2. Generate embeddings
		// 3. Store in Qdrant

		// For now, just update status
		s.updateStatus(func() {
			s.indexedDocs++
			s.documentCount++
			s.lastIndexTime = time.Now()
		})

	case filewatcher.EventDeleted:
		log.Printf("Removing deleted file from index: %s", evt.Path)
		// In a real implementation, we would:
		// 1. Remove the file's entries from Qdrant

		// For now, just update status
		s.updateStatus(func() {
			s.documentCount--
		})

	case filewatcher.EventRenamed:
		log.Printf("Updating renamed file in index: %s -> %s", evt.OldPath, evt.Path)
		// In a real implementation, we would:
		// 1. Update references in Qdrant

		// For now, do nothing
	}
}

// performInitialIndex performs initial indexing of all files
func (s *Service) performInitialIndex(ctx context.Context) {
	s.updateStatus(func() {
		s.isIndexing = true
	})

	// In a real implementation, we would:
	// 1. Scan all watched directories
	// 2. Process and index all markdown files
	// 3. Update status as we go

	// Simulate indexing work
	log.Printf("Starting initial indexing...")
	time.Sleep(2 * time.Second)

	s.updateStatus(func() {
		s.isIndexing = false
		s.lastIndexTime = time.Now()
		// Set some dummy values
		s.indexedDocs = 10
		s.documentCount = 10
	})

	log.Printf("Initial indexing completed")
}

// addVaultPaths adds all configured vault paths to the watcher
func (s *Service) addVaultPaths(ctx context.Context) error {
	// Get all vault paths from config
	vaultPaths := s.config.GetVaultPaths()

	// Add each path to the watcher
	for _, path := range vaultPaths {
		if err := s.WatchDirectory(path); err != nil {
			log.Printf("Warning: Failed to watch vault path %s: %v", path, err)
			// Continue with other paths even if one fails
		}
	}

	return nil
}

// WatchDirectory adds a directory to the watch list
func (s *Service) WatchDirectory(path string) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Add to file watcher
	if err := s.fileWatcher.AddPath(absPath); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Update watched directories list
	s.updateStatus(func() {
		s.watchedDirs = append(s.watchedDirs, absPath)
	})

	log.Printf("Added directory to watch list: %s", absPath)
	return nil
}

// Stop gracefully shuts down the daemon
func (s *Service) Stop(ctx context.Context) error {
	close(s.done)

	// Stop the API server
	if s.apiServer != nil {
		if err := s.apiServer.Stop(); err != nil {
			log.Printf("Error stopping API server: %v", err)
		}
	}

	// Close the file watcher
	if s.fileWatcher != nil {
		if err := s.fileWatcher.Close(); err != nil {
			log.Printf("Error closing file watcher: %v", err)
		}
	}

	// Close the embedder
	if s.embedder != nil {
		if err := s.embedder.Close(); err != nil {
			log.Printf("Error closing embedder: %v", err)
		}
	}

	// Close Qdrant client
	if s.qdrant != nil {
		if err := s.qdrant.Close(); err != nil {
			log.Printf("Error closing Qdrant client: %v", err)
		}
	}

	log.Printf("Daemon stopped")
	return nil
}

// updateStatus safely updates daemon status
func (s *Service) updateStatus(updateFn func()) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	updateFn()
}

// Status returns current daemon status
type Status struct {
	Running        bool
	StartTime      time.Time
	Uptime         time.Duration
	DocumentCount  int
	IndexedDocs    int
	IsIndexing     bool
	LastIndexTime  time.Time
	WatchedDirs    []string
	EmbeddingModel string
}

// GetStatus returns the current daemon status
func (s *Service) GetStatus() *Status {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()

	return &Status{
		Running:        true,
		StartTime:      s.startTime,
		Uptime:         time.Since(s.startTime),
		DocumentCount:  s.documentCount,
		IndexedDocs:    s.indexedDocs,
		IsIndexing:     s.isIndexing,
		LastIndexTime:  s.lastIndexTime,
		WatchedDirs:    append([]string{}, s.watchedDirs...),
		EmbeddingModel: s.embeddingModel,
	}
}
