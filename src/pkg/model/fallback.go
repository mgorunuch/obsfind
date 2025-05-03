package model

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// HybridEmbedder provides fallback between multiple embedding providers
type HybridEmbedder struct {
	embedders  []Embedder
	current    int
	mutex      sync.RWMutex
	dimensions int
	modelName  string
}

// NewHybridEmbedder creates a new embedder with fallback capability
func NewHybridEmbedder(embedders []Embedder) (*HybridEmbedder, error) {
	if len(embedders) == 0 {
		return nil, fmt.Errorf("no embedders provided")
	}

	return &HybridEmbedder{
		embedders:  embedders,
		current:    0,
		dimensions: embedders[0].Dimensions(),
		modelName:  fmt.Sprintf("hybrid(%s)", embedders[0].Name()),
	}, nil
}

// CreateHybridEmbedderFromConfigs creates a HybridEmbedder from multiple configs
func CreateHybridEmbedderFromConfigs(configs []Config) (*HybridEmbedder, error) {
	var embedders []Embedder

	for _, cfg := range configs {
		embedder, err := CreateEmbedder(cfg)
		if err != nil {
			log.Printf("Failed to initialize embedder %s: %v", cfg.Provider, err)
			continue
		}
		embedders = append(embedders, embedder)
	}

	if len(embedders) == 0 {
		return nil, fmt.Errorf("no valid embedders could be initialized")
	}

	return NewHybridEmbedder(embedders)
}

// Embed tries each embedder until one succeeds
func (e *HybridEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mutex.RLock()
	current := e.current
	e.mutex.RUnlock()

	// Try the current embedder first
	embedding, err := e.embedders[current].Embed(ctx, text)
	if err == nil {
		return embedding, nil
	}

	// Log the error with the current embedder
	log.Printf("Primary embedder %s failed: %v, trying fallbacks",
		e.embedders[current].Name(), err)

	// Try other embedders as fallback
	for i := 0; i < len(e.embedders); i++ {
		if i == current {
			continue
		}

		embedding, err := e.embedders[i].Embed(ctx, text)
		if err == nil {
			// Update the current working embedder
			e.mutex.Lock()
			e.current = i
			e.dimensions = e.embedders[i].Dimensions()
			e.modelName = fmt.Sprintf("hybrid(%s)", e.embedders[i].Name())
			e.mutex.Unlock()

			log.Printf("Switched to embedder %s", e.embedders[i].Name())
			return embedding, nil
		}
	}

	return nil, fmt.Errorf("all embedders failed")
}

// EmbedBatch implements batch embedding with fallback
func (e *HybridEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.mutex.RLock()
	current := e.current
	e.mutex.RUnlock()

	// Try the current embedder first
	embeddings, err := e.embedders[current].EmbedBatch(ctx, texts)
	if err == nil {
		return embeddings, nil
	}

	// Log the error with the current embedder
	log.Printf("Primary embedder %s failed batch operation: %v, trying fallbacks",
		e.embedders[current].Name(), err)

	// Try other embedders as fallback
	for i := 0; i < len(e.embedders); i++ {
		if i == current {
			continue
		}

		embeddings, err := e.embedders[i].EmbedBatch(ctx, texts)
		if err == nil {
			// Update the current working embedder
			e.mutex.Lock()
			e.current = i
			e.dimensions = e.embedders[i].Dimensions()
			e.modelName = fmt.Sprintf("hybrid(%s)", e.embedders[i].Name())
			e.mutex.Unlock()

			log.Printf("Switched to embedder %s for batch operations", e.embedders[i].Name())
			return embeddings, nil
		}
	}

	return nil, fmt.Errorf("all embedders failed for batch operation")
}

// Dimensions returns the dimensionality of the current embedder
func (e *HybridEmbedder) Dimensions() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.dimensions
}

// Name returns the name of the current embedder
func (e *HybridEmbedder) Name() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.modelName
}

// Close releases resources for all embedders
func (e *HybridEmbedder) Close() error {
	var errs []error
	for _, emb := range e.embedders {
		if err := emb.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing embedders: %v", errs)
	}

	return nil
}

// SetupFallbackEmbedder creates a fallback embedder system
// It attempts to set up embedders in order of preference: ollama, api, mock
func SetupFallbackEmbedder(ctx context.Context, cfg *Config) (Embedder, error) {
	var configs []Config

	// Add Ollama embedder if configured
	if ollamaCfg, ok := cfg.Specific.(OllamaConfig); ok {
		configs = append(configs, Config{
			Provider: "ollama",
			Specific: ollamaCfg,
		})
	}

	// If no configs are available, create a simple mock embedder
	if len(configs) == 0 {
		return nil, fmt.Errorf("no valid embedding configurations available")
	}

	// Create hybrid embedder with fallbacks
	embedder, err := CreateHybridEmbedderFromConfigs(configs)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	// Wrap with caching
	return NewCachedEmbedder(embedder), nil
}
