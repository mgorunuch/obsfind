package indexer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"obsfind/src/pkg/config"
	"obsfind/src/pkg/markdown"
	model2 "obsfind/src/pkg/model"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "github.com/qdrant/go-client/qdrant"
	"github.com/rs/zerolog/log"
)

// Common errors
var (
	ErrIndexingInProgress = errors.New("indexing is already in progress")
	ErrInvalidPath        = errors.New("invalid path")
	ErrEmbeddingFailed    = errors.New("failed to generate embeddings")
	ErrStorageFailed      = errors.New("failed to store embeddings")
)

// DocumentStatus represents the indexing status of a document
type DocumentStatus struct {
	Path      string    `json:"path"`
	Indexed   bool      `json:"indexed"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Stats provides statistics about the indexing process
type Stats struct {
	TotalDocuments   int                `json:"total_documents"`
	IndexedDocuments int                `json:"indexed_documents"`
	FailedDocuments  int                `json:"failed_documents"`
	Status           string             `json:"status"` // "idle", "indexing", "error"
	Documents        []DocumentStatus   `json:"documents,omitempty"`
	LastError        string             `json:"last_error,omitempty"`
	LastRun          time.Time          `json:"last_run,omitempty"`
	CollectionInfo   *pb.CollectionInfo `json:"-"` // Exclude from JSON to avoid serialization issues
}

// Service handles the indexing of documents
type Service struct {
	config         *config.Config
	embedder       model2.Embedder
	qdrantClient   model2.QdrantClient
	parser         *markdown.Parser
	mutex          sync.RWMutex
	isIndexing     bool
	indexingCtx    context.Context
	cancelIndexing context.CancelFunc
	stats          Stats
}

// NewService creates a new indexer service
func NewService(cfg *config.Config, embedder model2.Embedder, qdrantClient model2.QdrantClient) *Service {
	return &Service{
		config:       cfg,
		embedder:     embedder,
		qdrantClient: qdrantClient,
		parser:       markdown.NewParser(),
		stats: Stats{
			Status: "idle",
		},
	}
}

// GetStats returns the current indexing statistics
func (s *Service) GetStats() Stats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Get the latest collection info
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collInfo, err := s.qdrantClient.GetCollectionInfo(ctx, s.config.Qdrant.Collection)
	if err == nil {
		s.stats.CollectionInfo = collInfo
	}

	return s.stats
}

// IsIndexing returns true if an indexing operation is in progress
func (s *Service) IsIndexing() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.isIndexing
}

// IndexVault indexes the entire vault
func (s *Service) IndexVault(ctx context.Context) error {
	s.mutex.Lock()
	if s.isIndexing {
		s.mutex.Unlock()
		return ErrIndexingInProgress
	}

	s.isIndexing = true
	s.indexingCtx, s.cancelIndexing = context.WithCancel(ctx)
	s.stats.Status = "indexing"
	s.stats.LastRun = time.Now()
	s.stats.Documents = []DocumentStatus{}
	s.mutex.Unlock()

	defer func() {
		s.mutex.Lock()
		s.isIndexing = false
		if s.stats.FailedDocuments > 0 {
			s.stats.Status = "error"
		} else {
			s.stats.Status = "idle"
		}
		s.mutex.Unlock()
	}()

	// Get all vault paths
	vaultPaths := s.config.GetVaultPaths()

	// Process each vault path
	for _, vaultPath := range vaultPaths {
		// Walk the vault directory
		err := filepath.WalkDir(vaultPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidPath, err)
			}

			// Check if the context is cancelled
			select {
			case <-s.indexingCtx.Done():
				return s.indexingCtx.Err()
			default:
			}

			// Skip directories and non-markdown files
			if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}

			// Index the file
			docStatus := DocumentStatus{
				Path:      path,
				UpdatedAt: time.Now(),
			}

			s.mutex.Lock()
			s.stats.TotalDocuments++
			s.mutex.Unlock()

			if err := s.indexFile(s.indexingCtx, path, vaultPath); err != nil {
				docStatus.Error = err.Error()

				s.mutex.Lock()
				s.stats.FailedDocuments++
				s.mutex.Unlock()

				log.Error().Err(err).Str("path", path).Msg("Failed to index file")
			} else {
				docStatus.Indexed = true

				s.mutex.Lock()
				s.stats.IndexedDocuments++
				s.mutex.Unlock()

				log.Debug().Str("path", path).Msg("Indexed file successfully")
			}

			s.mutex.Lock()
			s.stats.Documents = append(s.stats.Documents, docStatus)
			s.mutex.Unlock()

			return nil
		})

		if err != nil {
			// Log the error but continue with other vault paths
			log.Error().Err(err).Str("vaultPath", vaultPath).Msg("Error indexing vault path")
		}
	}

	return nil
}

// IndexFile indexes a single file
func (s *Service) IndexFile(ctx context.Context, path string) error {
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		return fmt.Errorf("%w: not a markdown file", ErrInvalidPath)
	}

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}

	// Determine the base vault path for this file
	basePath := s.findBaseVaultPath(path)

	return s.indexFile(ctx, path, basePath)
}

// findBaseVaultPath determines which vault path contains the given file path
func (s *Service) findBaseVaultPath(path string) string {
	vaultPaths := s.config.GetVaultPaths()

	// If there's only one vault path, use it
	if len(vaultPaths) == 1 {
		return vaultPaths[0]
	}

	// Find the most specific vault path that contains this file
	var bestMatch string
	bestMatchLen := 0

	for _, vaultPath := range vaultPaths {
		// Check if this path is a prefix of the file path
		if strings.HasPrefix(path, vaultPath) {
			// If this is a longer match than our current best, use it
			if len(vaultPath) > bestMatchLen {
				bestMatch = vaultPath
				bestMatchLen = len(vaultPath)
			}
		}
	}

	// If we found a match, return it
	if bestMatch != "" {
		return bestMatch
	}

	// If no match, use the first vault path as a fallback
	if len(vaultPaths) > 0 {
		return vaultPaths[0]
	}

	// Fallback to empty string if no vault paths (should never happen)
	return ""
}

// CancelIndexing cancels an ongoing indexing operation
func (s *Service) CancelIndexing() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isIndexing && s.cancelIndexing != nil {
		s.cancelIndexing()
		s.stats.Status = "idle"
	}
}

// indexFile indexes a single file (internal implementation)
func (s *Service) indexFile(ctx context.Context, path string, basePath string) error {
	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the markdown
	doc, err := s.parser.Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Choose chunking strategy based on config
	var chunks []*markdown.Chunk
	switch s.config.Indexing.ChunkStrategy {
	case "header":
		chunks = s.parser.ChunkByHeaders(doc)
	case "sliding_window":
		chunks = s.parser.ChunkBySlidingWindow(doc, s.config.Indexing.MaxChunkSize, s.config.Indexing.WindowOverlap)
	case "hybrid":
		chunks = s.parser.ChunkHybrid(doc, s.config.Indexing.MaxChunkSize, s.config.Indexing.WindowOverlap)
	default:
		chunks = s.parser.ChunkHybrid(doc, s.config.Indexing.MaxChunkSize, s.config.Indexing.WindowOverlap)
	}

	if len(chunks) == 0 {
		log.Warn().Str("path", path).Msg("No chunks generated for file")
		return nil
	}

	// Prepare texts for embedding
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// Generate embeddings
	embeddings, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEmbeddingFailed, err)
	}

	log.Printf("Found %d embeddings", len(embeddings))

	if len(embeddings) != len(chunks) {
		return fmt.Errorf("%w: expected %d embeddings, got %d",
			ErrEmbeddingFailed, len(chunks), len(embeddings))
	}

	// Prepare points for Qdrant
	points := make([]*pb.PointStruct, len(chunks))

	// If no base path was provided, try to determine it
	if basePath == "" {
		basePath = s.findBaseVaultPath(path)
	}

	// Get relative path from the vault base
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		// If we can't get a relative path, use the full path
		relPath = path
	}

	// Store the vault path for proper attribution
	vaultName := filepath.Base(basePath)

	for i, chunk := range chunks {
		// Get a unique ID for the chunk - include vault name to avoid collisions
		id := model2.HashString(fmt.Sprintf("%s:%s#%d", vaultName, relPath, i))

		// Create payload with metadata
		payload := map[string]interface{}{
			"path":         relPath,
			"full_path":    path,
			"vault_path":   basePath,
			"vault_name":   vaultName,
			"text":         chunk.Content,
			"content":      chunk.ContentOnly,
			"title":        doc.Title,
			"section":      chunk.Section,
			"tags":         doc.Tags,
			"chunk_index":  i,
			"total_chunks": len(chunks),
		}

		// Add frontmatter to payload
		for k, v := range doc.Frontmatter {
			payload["fm_"+k] = v
		}

		// Keep vectors as float32 (they already are from EmbedBatch)
		vector := embeddings[i]

		points[i] = &pb.PointStruct{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{
					Uuid: id,
				},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{
						Data: vector,
					},
				},
			},
			Payload: model2.StructToPayload(payload),
		}
	}

	// Store in Qdrant
	err = s.qdrantClient.UpsertPoints(ctx, s.config.Qdrant.Collection, points)
	log.Print("Upsert points ok")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStorageFailed, err)
	}

	return nil
}

// recordError records an error in the stats
func (s *Service) recordError(errMsg string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.stats.LastError = errMsg
	s.stats.Status = "error"
}
