package api

import (
	"context"
	"errors"
	"fmt"
	"obsfind/src/pkg/config"
	indexer2 "obsfind/src/pkg/indexer"
	model2 "obsfind/src/pkg/model"
	"strings"
	"time"

	pb "github.com/qdrant/go-client/qdrant"
	"github.com/rs/zerolog/log"
)

// Service represents the API service layer
type Service struct {
	// Core service components
	indexer      *indexer2.Service
	embedder     model2.Embedder
	qdrantClient model2.QdrantClient
	config       *config.Config

	// Status tracking
	status struct {
		StartTime      time.Time
		DocumentCount  int
		IsIndexing     bool
		IndexedDocs    int
		WatchedDirs    []string
		LastIndexTime  time.Time
		EmbeddingModel string
	}
}

// NewService creates a new API service
func NewService(indexer *indexer2.Service, embedder model2.Embedder, qdrantClient model2.QdrantClient, config *config.Config) *Service {
	return &Service{
		// Store core service components
		indexer:      indexer,
		embedder:     embedder,
		qdrantClient: qdrantClient,
		config:       config,

		// Initialize status tracking
		status: struct {
			StartTime      time.Time
			DocumentCount  int
			IsIndexing     bool
			IndexedDocs    int
			WatchedDirs    []string
			LastIndexTime  time.Time
			EmbeddingModel string
		}{
			StartTime:      time.Now(),
			DocumentCount:  0,
			IsIndexing:     false,
			IndexedDocs:    0,
			WatchedDirs:    config.GetVaultPaths(),
			LastIndexTime:  time.Time{},
			EmbeddingModel: config.Embedding.ModelName,
		},
	}
}

// NewPlaceholderService creates a service with placeholder components
// Used for testing or when full services aren't available
func NewPlaceholderService() *Service {
	return &Service{
		status: struct {
			StartTime      time.Time
			DocumentCount  int
			IsIndexing     bool
			IndexedDocs    int
			WatchedDirs    []string
			LastIndexTime  time.Time
			EmbeddingModel string
		}{
			StartTime:      time.Now(),
			DocumentCount:  0,
			IsIndexing:     false,
			IndexedDocs:    0,
			WatchedDirs:    []string{},
			LastIndexTime:  time.Time{},
			EmbeddingModel: "nomic-embed-text",
		},
	}
}

// GetStatus returns the current daemon status
func (s *Service) GetStatus() (*StatusResponse, error) {
	var indexStats indexer2.Stats

	if s.indexer == nil {
		// In placeholder mode, create simulated stats
		indexStats = indexer2.Stats{
			TotalDocuments:   s.status.DocumentCount,
			IndexedDocuments: s.status.IndexedDocs,
			Status:           "idle",
		}
		if s.status.IsIndexing {
			indexStats.Status = "indexing"
		}
	} else {
		// Get real stats from the indexer
		indexStats = s.indexer.GetStats()

		// Clear the CollectionInfo field which is causing serialization issues
		// We'll extract the relevant info and place it in the Config map instead
		if indexStats.CollectionInfo != nil {
			// Extract useful info before clearing
			if indexStats.CollectionInfo.VectorsCount != nil {
				indexStats.TotalDocuments = int(*indexStats.CollectionInfo.VectorsCount)
			}

			// Clear the problematic field
			indexStats.CollectionInfo = nil
		}
	}

	// Build configuration map
	configMap := make(map[string]string)

	if s.config != nil {
		// Add key configuration values
		configMap["embedding_model"] = s.config.Embedding.ModelName
		configMap["vector_dimensions"] = fmt.Sprintf("%d", s.config.Embedding.Dimensions)
		configMap["chunking_strategy"] = s.config.Indexing.ChunkStrategy
		configMap["max_chunk_size"] = fmt.Sprintf("%d", s.config.Indexing.MaxChunkSize)

		// Add Qdrant configuration
		if s.config.Qdrant.Embedded {
			configMap["qdrant_mode"] = "embedded"
			configMap["qdrant_data_path"] = s.config.Qdrant.DataPath
		} else {
			configMap["qdrant_mode"] = "external"
			configMap["qdrant_server"] = fmt.Sprintf("%s:%d", s.config.Qdrant.Host, s.config.Qdrant.Port)
		}

		// Add daemon information
		configMap["daemon_api"] = fmt.Sprintf("%s:%d", s.config.API.Host, s.config.API.Port)
	} else {
		// Placeholder values
		configMap["embedding_model"] = s.status.EmbeddingModel
		configMap["vector_dimensions"] = "768"
		configMap["chunking_strategy"] = "hybrid"
		configMap["qdrant_mode"] = "embedded"
	}

	// Get version from build info (use "dev" for now)
	version := "dev"

	return &StatusResponse{
		Status:     "running",
		Uptime:     time.Since(s.status.StartTime).String(),
		StartTime:  s.status.StartTime,
		IndexStats: indexStats,
		Version:    version,
		Config:     configMap,
	}, nil
}

// SearchResult represents a search result
type SearchResult struct {
	ID       string                 `json:"id"`
	Path     string                 `json:"path"`
	Title    string                 `json:"title,omitempty"`
	Excerpt  string                 `json:"excerpt,omitempty"`
	Content  string                 `json:"content,omitempty"`
	Score    float32                `json:"score"`
	Tags     []string               `json:"tags,omitempty"`
	Section  string                 `json:"section,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Search performs a semantic search using Qdrant for vector similarity search
func (s *Service) Search(ctx context.Context, query string, limit int, filter string) ([]SearchResult, error) {
	// Configure search options
	if limit <= 0 {
		limit = 10
	}

	// Parse filter if provided (e.g., "tags:note,important" or "path:/folder/")
	var pathPrefix string
	var tags []string

	// Parse filter if it's in format "type:value"
	if filter != "" && strings.Contains(filter, ":") {
		filterParts := strings.Split(filter, ":")
		if len(filterParts) == 2 {
			filterType := strings.TrimSpace(filterParts[0])
			filterValue := strings.TrimSpace(filterParts[1])

			switch filterType {
			case "tags":
				tags = strings.Split(filterValue, ",")
				for i, tag := range tags {
					tags[i] = strings.TrimSpace(tag)
				}
			case "path":
				pathPrefix = filterValue
			}
		}
	} else {
		// Direct filter is provided (from CLI parameters)
		// If filter contains a path separator, treat it as a path filter
		if filter != "" && (strings.Contains(filter, "/") || strings.Contains(filter, "\\")) {
			pathPrefix = filter
		}
	}

	// Log the search request
	log.Info().
		Str("query", query).
		Int("limit", limit).
		Str("pathPrefix", pathPrefix).
		Strs("tags", tags).
		Msg("Executing semantic search")

	// Step 1: Generate embedding for the query
	embeddings, err := s.embedder.EmbedBatch(ctx, []string{query})
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("Embedding generation failed")
		// Return an explicit user-friendly error message
		return nil, fmt.Errorf("unable to process search query: embedding service unavailable - please check if Ollama is running")
	}

	if len(embeddings) == 0 {
		log.Warn().Str("query", query).Msg("Empty embedding generated")
		return nil, fmt.Errorf("search processing error: empty embedding generated")
	}

	// Note: We're getting the embedding but not using it directly here
	// because we're delegating the search to the indexer which will use the query text

	// Step 2: Create search options for indexer
	searchOptions := indexer2.SearchOptions{
		Limit:      limit,
		MinScore:   0.6, // Reasonable default
		Tags:       tags,
		PathPrefix: pathPrefix,
	}

	// Step 3: Perform search using indexer
	indexerResults, err := s.indexer.Search(ctx, query, searchOptions)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Handle empty results - return empty slice instead of error
	if len(indexerResults) == 0 {
		log.Info().Msg("No search results found")
		return []SearchResult{}, nil
	}

	// Step 4: Convert indexer results to API results
	results := make([]SearchResult, len(indexerResults))
	for i, r := range indexerResults {
		results[i] = SearchResult{
			ID:       fmt.Sprintf("result-%d", i),
			Path:     r.Path,
			Title:    r.Title,
			Content:  r.Content,
			Excerpt:  extractExcerpt(r.Content, query, 150),
			Score:    float32(r.Score),
			Tags:     r.Tags,
			Section:  r.Section,
			Metadata: r.Metadata,
		}
	}

	log.Info().
		Int("resultCount", len(results)).
		Float32("topScore", float32(indexerResults[0].Score)).
		Msg("Search completed successfully")

	return results, nil
}

// extractExcerpt creates a relevant excerpt from content based on the query
// This function would be used in a real implementation to show the most relevant
// part of the content in search results
func extractExcerpt(content, query string, maxLength int) string {
	// This is a simplified implementation
	// A real implementation would:
	// 1. Break content into sentences
	// 2. Score each sentence based on relevance to query terms
	// 3. Return the highest scoring sentence/section

	if len(content) <= maxLength {
		return content
	}

	// For this example, just return the first part of the content
	excerpt := content
	if len(excerpt) > maxLength {
		excerpt = excerpt[:maxLength-3] + "..."
	}

	return excerpt
}

// FindSimilar finds documents similar to the specified file
func (s *Service) FindSimilar(ctx context.Context, filePath string, limit int) ([]SearchResult, error) {
	// Check if Qdrant collection has data before proceeding
	if s.qdrantClient != nil {
		// Use collection name from config
		collectionName := s.config.Qdrant.Collection

		collectionInfo, err := s.qdrantClient.GetCollectionInfo(ctx, collectionName)
		if err == nil && collectionInfo != nil {
			// Check if either vectors count or points count is zero/nil
			vectorsEmpty := collectionInfo.VectorsCount == nil || *collectionInfo.VectorsCount == 0
			pointsEmpty := collectionInfo.PointsCount == nil || *collectionInfo.PointsCount == 0

			if vectorsEmpty || pointsEmpty {
				return nil, fmt.Errorf("similar search failed: your vault has not been indexed yet - run 'obsfind reindex' to build the search index")
			}
		}
	}

	// Log component status for debugging
	log.Info().
		Bool("embedder_nil", s.embedder == nil).
		Bool("qdrant_nil", s.qdrantClient == nil).
		Bool("indexer_nil", s.indexer == nil).
		Msg("Checking components before similar search")

	// In a real implementation, we would:
	if s.indexer == nil {
		log.Warn().
			Bool("embedder_nil", s.embedder == nil).
			Bool("qdrant_nil", s.qdrantClient == nil).
			Bool("indexer_nil", s.indexer == nil).
			Msg("Using mock data because indexer is nil")
		// Return dummy results in placeholder mode
		return []SearchResult{
			{
				ID:      "similar1",
				Path:    "/path/to/similar1.md",
				Title:   "Similar Document 1",
				Excerpt: "This document is semantically similar to the requested file.",
				Score:   0.88,
				Tags:    []string{"related", "example"},
				Section: "Overview",
			},
			{
				ID:      "similar2",
				Path:    "/path/to/similar2.md",
				Title:   "Similar Document 2",
				Excerpt: "This document is also semantically similar to the requested file.",
				Score:   0.76,
				Tags:    []string{"related"},
				Section: "Details",
			},
		}, nil
	}

	// Validate input
	if limit <= 0 {
		limit = 10
	}

	// Create search options
	searchOptions := indexer2.SearchOptions{
		Limit:    limit,
		MinScore: 0.6, // Reasonable default
	}

	// Execute similar search via indexer
	indexerResults, err := s.indexer.FindSimilar(ctx, filePath, searchOptions)
	if err != nil {
		// Check if this is a "document not found" error
		if strings.Contains(err.Error(), "document not found") {
			log.Warn().Str("path", filePath).Msg("Document not found in index for similar search")
			return []SearchResult{}, nil
		}
		
		// Check if this is a vector embedding error
		if strings.Contains(err.Error(), "no vectors found") {
			log.Warn().Str("path", filePath).Msg("No vectors found for document in similar search")
			return []SearchResult{}, nil
		}
		
		// Log other errors
		log.Error().Err(err).Str("path", filePath).Msg("Similar search failed")
		return nil, fmt.Errorf("similar search failed: %w", err)
	}

	// Handle empty results - return empty slice instead of error
	if len(indexerResults) == 0 {
		log.Info().Msg("No similar documents found")
		return []SearchResult{}, nil
	}

	// Convert indexer results to API results
	results := make([]SearchResult, len(indexerResults))
	for i, r := range indexerResults {
		results[i] = SearchResult{
			ID:       fmt.Sprintf("similar-%d", i),
			Path:     r.Path,
			Title:    r.Title,
			Excerpt:  extractExcerpt(r.Content, "", 150),
			Content:  r.Content,
			Score:    float32(r.Score),
			Tags:     r.Tags,
			Section:  r.Section,
			Metadata: r.Metadata,
		}
	}

	return results, nil
}

// IndexFile indexes or reindexes a specific file
func (s *Service) IndexFile(ctx context.Context, filePath string, force bool) error {
	// In a real implementation, we would:
	if s.indexer == nil {
		// Update status values in placeholder mode
		s.status.IsIndexing = true
		s.status.IndexedDocs++
		s.status.DocumentCount++
		s.status.LastIndexTime = time.Now()

		// Wait a moment to simulate work
		time.Sleep(100 * time.Millisecond)
		s.status.IsIndexing = false

		return nil
	}

	// Delegate to the actual indexer service
	err := s.indexer.IndexFile(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to index file %s: %w", filePath, err)
	}

	// Update our status information
	indexStats := s.indexer.GetStats()
	s.status.IsIndexing = indexStats.Status == "indexing"
	s.status.IndexedDocs = indexStats.IndexedDocuments
	s.status.DocumentCount = indexStats.TotalDocuments
	s.status.LastIndexTime = time.Now()

	return nil
}

// resetCollection handles dropping and recreating a Qdrant collection
func (s *Service) resetCollection(ctx context.Context) error {
	// Get collection name from config
	collectionName := s.config.Qdrant.Collection

	// Drop the collection
	log.Info().Str("collection", collectionName).Msg("Dropping collection for clean reinstall")
	err := s.qdrantClient.DeleteCollection(ctx, collectionName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete collection - continuing anyway")
		// Continue despite error, as the collection might not exist yet
	} else {
		log.Info().Msg("Collection successfully dropped")
	}

	// Recreate with proper schema
	log.Info().Msg("Recreating collection with fresh schema")
	dims := s.config.Embedding.Dimensions
	distance := pb.Distance_Cosine

	// Handle distance metric if specified in config
	if s.config.Qdrant.Distance == "dot" {
		distance = pb.Distance_Dot
	} else if s.config.Qdrant.Distance == "euclid" {
		distance = pb.Distance_Euclid
	}

	err = s.qdrantClient.CreateCollection(ctx, collectionName, uint64(dims), distance)
	if err != nil {
		return fmt.Errorf("failed to recreate collection: %w", err)
	}
	log.Info().Msg("Collection recreated successfully")

	return nil
}

func (s *Service) backgroundReindexAll(force bool) {
	// Create a new context that won't be canceled when the HTTP request completes
	bgCtx := context.Background()

	if s.qdrantClient == nil {
		log.Error().Msg("qdrant client is nil")
		return
	}

	if force {
		if err := s.resetCollection(bgCtx); err != nil {
			log.Error().Err(err).Msg("Failed to reset collection")
			return
		}
	}

	// Start the indexing process
	log.Info().Msg("Starting full reindex process")
	err := s.indexer.IndexVault(bgCtx)
	if err != nil {
		log.Error().Err(err).Msg("Background reindexing failed")
	} else {
		log.Info().Msg("Background reindexing completed successfully")
	}
}

// ReindexAll reindexes all files in watched directories
func (s *Service) ReindexAll(ctx context.Context, force bool) error {
	if s.indexer == nil {
		return errors.New("no indexer configured")
	}

	// Check if indexing is already in progress
	if s.indexer.IsIndexing() {
		return fmt.Errorf("indexing is already in progress")
	}

	go s.backgroundReindexAll(force)

	// Set status to indicate indexing has started
	s.status.IsIndexing = true
	log.Info().Bool("force", force).Msg("Reindexing started in the background")

	return nil
}

// getIndexingStatus creates a status object from the current service state
// This is an internal function to prepare the status response

// GetIndexingStatus returns the current indexing status
func (s *Service) GetIndexingStatus(ctx context.Context) (*IndexingStatus, error) {
	if s.indexer == nil {
		// In placeholder mode, return simulated indexing status
		percentComplete := 0.0
		if s.status.DocumentCount > 0 {
			percentComplete = float64(s.status.IndexedDocs) / float64(s.status.DocumentCount) * 100
		}

		return &IndexingStatus{
			IsIndexing:        s.status.IsIndexing,
			IndexedDocs:       s.status.IndexedDocs,
			TotalDocs:         s.status.DocumentCount,
			PercentComplete:   percentComplete,
			CurrentFile:       "",
			LastIndexedFile:   "",
			IndexingStartTime: s.status.StartTime,
		}, nil
	}

	// Get real indexing status from indexer
	indexStats := s.indexer.GetStats()

	// Determine percent complete
	percentComplete := 0.0
	if indexStats.TotalDocuments > 0 {
		percentComplete = float64(indexStats.IndexedDocuments) / float64(indexStats.TotalDocuments) * 100
	}

	// Find current and last indexed file
	var currentFile, lastIndexedFile string

	if len(indexStats.Documents) > 0 {
		// Get the most recently indexed document
		var mostRecent time.Time
		for _, doc := range indexStats.Documents {
			if doc.UpdatedAt.After(mostRecent) {
				mostRecent = doc.UpdatedAt
				lastIndexedFile = doc.Path
			}
		}
	}

	return &IndexingStatus{
		IsIndexing:        indexStats.Status == "indexing",
		IndexedDocs:       indexStats.IndexedDocuments,
		TotalDocs:         indexStats.TotalDocuments,
		PercentComplete:   percentComplete,
		CurrentFile:       currentFile,
		LastIndexedFile:   lastIndexedFile,
		IndexingStartTime: indexStats.LastRun,
	}, nil
}

// AddWatchedDirectory adds a directory to the watch list
func (s *Service) AddWatchedDirectory(ctx context.Context, path string) error {
	if s.config == nil {
		// In placeholder mode, just update the internal state
		s.status.WatchedDirs = append(s.status.WatchedDirs, path)
		return nil
	}

	// Add directory to config
	s.config.AddVaultPath(path)

	// Update our status as well
	s.status.WatchedDirs = s.config.GetVaultPaths()

	// In a real implementation, we would need to:
	// 1. Save the configuration to disk
	// 2. Update the file watcher to monitor the new directory
	// For now, we'll assume that happens elsewhere or is triggered by config changes

	log.Info().Str("path", path).Msg("Added watched directory")

	return nil
}

// RemoveWatchedDirectory removes a directory from the watch list
func (s *Service) RemoveWatchedDirectory(ctx context.Context, path string) error {
	if s.config == nil {
		// In placeholder mode, just update internal state
		var newDirs []string
		for _, dir := range s.status.WatchedDirs {
			if dir != path {
				newDirs = append(newDirs, dir)
			}
		}
		s.status.WatchedDirs = newDirs

		if len(newDirs) == 0 {
			return fmt.Errorf("cannot remove the last watched directory")
		}

		return nil
	}

	// Get current paths
	currentPaths := s.config.GetVaultPaths()

	// Make sure we don't remove the last path
	if len(currentPaths) <= 1 {
		return fmt.Errorf("cannot remove the last watched directory; at least one directory must be monitored")
	}

	// Filter out the path to remove
	var newPaths []string
	for _, dir := range currentPaths {
		if dir != path {
			newPaths = append(newPaths, dir)
		}
	}

	// Update the config paths
	s.config.Paths.VaultPaths = newPaths
	if len(newPaths) > 0 {
		s.config.Paths.VaultPath = newPaths[0] // Update for backward compatibility
	}

	// Update our status
	s.status.WatchedDirs = newPaths

	// In a real implementation, we would need to:
	// 1. Save the configuration to disk
	// 2. Update the file watcher to stop monitoring this directory
	// For now, we'll assume that happens elsewhere or is triggered by config changes

	log.Info().Str("path", path).Msg("Removed watched directory")

	return nil
}

// SetEmbeddingModel changes the embedding model
func (s *Service) SetEmbeddingModel(ctx context.Context, model string) error {
	if model == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	if s.config == nil || s.embedder == nil {
		// In placeholder mode, just update internal state
		s.status.EmbeddingModel = model
		return nil
	}

	// Update the configuration
	oldModel := s.config.Embedding.ModelName
	s.config.Embedding.ModelName = model

	// Update status
	s.status.EmbeddingModel = model

	log.Info().
		Str("oldModel", oldModel).
		Str("newModel", model).
		Msg("Changed embedding model")

	// In a real implementation, we would:
	// 1. Save the configuration
	// 2. Reinitialize the embedder with the new model
	// 3. Potentially need to reindex to ensure consistent embeddings
	// For now, we'll assume that happens elsewhere or is triggered by config changes

	return nil
}
