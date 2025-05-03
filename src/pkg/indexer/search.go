package indexer

import (
	"context"
	"errors"
	"fmt"
	"obsfind/src/pkg/model"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

// SearchResult represents a single search result
type SearchResult struct {
	Path       string                 `json:"path"`
	Section    string                 `json:"section,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Score      float64                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ChunkIndex int                    `json:"chunk_index"`
}

// SearchOptions provides options for search operations
type SearchOptions struct {
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
	MinScore   float32  `json:"min_score,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// DefaultSearchOptions returns the default search options
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Limit:    10,
		Offset:   0,
		MinScore: 0.6, // Reasonable default threshold
	}
}

// Search performs a semantic search using the given query
func (s *Service) Search(ctx context.Context, query string, options SearchOptions) ([]SearchResult, error) {
	// Generate embedding for the query
	embeddings, err := s.embedder.EmbedBatch(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, errors.New("empty embedding generated for query")
	}

	// Use the embedding directly
	queryVector := embeddings[0]

	// Set up limit and offset
	limit := uint64(options.Limit)
	if limit <= 0 {
		limit = 10
	}

	offset := uint64(options.Offset)

	// Add proper filtering and vector name handling
	// TODO: Implement more advanced filtering once API is stabilized
	searchPoints, err := s.qdrantClient.Search(
		ctx,
		s.config.Qdrant.Collection,
		queryVector,
		limit,
		offset,
		nil, // filter
		nil, // search params
	)

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert to search results
	results := make([]SearchResult, 0, len(searchPoints))
	for _, point := range searchPoints {
		// Skip results below the minimum score threshold
		if options.MinScore > 0 && point.Score < options.MinScore {
			continue
		}

		payload := point.Payload

		// Extract fields from payload
		path, _ := model.GetPayloadString(payload, "path")
		content, _ := model.GetPayloadString(payload, "content")
		title, _ := model.GetPayloadString(payload, "title")
		section, _ := model.GetPayloadString(payload, "section")
		tags, _ := model.GetPayloadStringSlice(payload, "tags")
		chunkIndex, _ := model.GetPayloadInt(payload, "chunk_index")

		// Apply path prefix filter if specified
		if options.PathPrefix != "" && !strings.HasPrefix(path, options.PathPrefix) {
			continue
		}

		// Apply tag filter if specified
		if len(options.Tags) > 0 {
			matched := false
			for _, tag := range options.Tags {
				for _, docTag := range tags {
					if tag == docTag {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Simplify metadata handling for now
		metadata := make(map[string]interface{})

		results = append(results, SearchResult{
			Path:       path,
			Section:    section,
			Title:      title,
			Content:    content,
			Tags:       tags,
			Score:      float64(point.Score),
			Metadata:   metadata,
			ChunkIndex: chunkIndex,
		})
	}

	// Sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// FindSimilar finds documents similar to the referenced path
func (s *Service) FindSimilar(ctx context.Context, path string, options SearchOptions) ([]SearchResult, error) {
	// Read the file content
	content, err := s.qdrantClient.GetPointsByPath(ctx, s.config.Qdrant.Collection, path)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve document: %w", err)
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("document not found in index: %s", path)
	}

	// Get all chunk vectors for the document
	vectors := make([][]float32, 0, len(content))
	for _, point := range content {
		if point.Vectors != nil && point.Vectors.GetVector() != nil {
			vectors = append(vectors, point.Vectors.GetVector().Data)
		}
	}

	if len(vectors) == 0 {
		return nil, fmt.Errorf("no vectors found for document: %s", path)
	}

	// For each vector, find similar documents
	allResults := make([]SearchResult, 0)

	// Set up limit and offset
	limit := uint64(options.Limit)
	if limit <= 0 {
		limit = 10
	}

	offset := uint64(options.Offset)

	// Use the first vector to find similar documents
	// This is a simplification - in a more advanced implementation,
	// we might want to combine results from all vectors
	if len(vectors) > 0 {
		log.Debug().Int("vector_count", len(vectors)).Msg("Finding similar documents")

		// Perform search with proper vector name handling
		searchPoints, err := s.qdrantClient.Search(
			ctx,
			s.config.Qdrant.Collection,
			vectors[0],
			limit,
			offset,
			nil, // filter
			nil, // search params
		)

		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}

		// Convert to search results, excluding the queried path
		for _, point := range searchPoints {
			// Skip results below the minimum score threshold
			if options.MinScore > 0 && point.Score < options.MinScore {
				continue
			}

			payload := point.Payload

			// Extract fields from payload
			pointPath, _ := model.GetPayloadString(payload, "path")

			// Skip self matches
			if pointPath == path {
				continue
			}

			// Apply path prefix filter if specified
			if options.PathPrefix != "" && !strings.HasPrefix(pointPath, options.PathPrefix) {
				continue
			}

			// Apply tag filter if specified
			if len(options.Tags) > 0 {
				tags, _ := model.GetPayloadStringSlice(payload, "tags")
				matched := false
				for _, tag := range options.Tags {
					for _, docTag := range tags {
						if tag == docTag {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
				if !matched {
					continue
				}
			}

			content, _ := model.GetPayloadString(payload, "content")
			title, _ := model.GetPayloadString(payload, "title")
			section, _ := model.GetPayloadString(payload, "section")
			tags, _ := model.GetPayloadStringSlice(payload, "tags")
			chunkIndex, _ := model.GetPayloadInt(payload, "chunk_index")

			// Simplify metadata handling for now
			metadata := make(map[string]interface{})

			allResults = append(allResults, SearchResult{
				Path:       pointPath,
				Section:    section,
				Title:      title,
				Content:    content,
				Tags:       tags,
				Score:      float64(point.Score),
				Metadata:   metadata,
				ChunkIndex: chunkIndex,
			})
		}
	}

	// Sort by score (highest first)
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	return allResults, nil
}
