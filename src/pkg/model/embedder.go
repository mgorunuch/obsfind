package model

import (
	"context"
	"fmt"
	"sync"
)

// Embedder represents a service that can generate embeddings for text
type Embedder interface {
	// Embed generates a vector embedding for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the dimensionality of the embeddings
	Dimensions() int

	// Name returns the model name
	Name() string

	// Close releases resources used by the embedder
	Close() error
}

// Config represents generic configuration for embedders
type Config struct {
	Provider string
	Specific interface{}
}

// Factory is a function that creates an embedder from a config
type Factory func(Config) (Embedder, error)

var (
	embedderFactories = make(map[string]Factory)
	factoryMutex      sync.RWMutex
)

// RegisterEmbedder registers an embedder factory for a provider
func RegisterEmbedder(provider string, factory Factory) {
	factoryMutex.Lock()
	defer factoryMutex.Unlock()

	embedderFactories[provider] = factory
}

// CreateEmbedder creates an embedder based on the specified configuration
func CreateEmbedder(config Config) (Embedder, error) {
	factoryMutex.RLock()
	factory, ok := embedderFactories[config.Provider]
	factoryMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unsupported embedder provider: %s", config.Provider)
	}

	return factory(config)
}

// AvailableProviders returns a list of registered embedder providers
func AvailableProviders() []string {
	factoryMutex.RLock()
	defer factoryMutex.RUnlock()

	var providers []string
	for provider := range embedderFactories {
		providers = append(providers, provider)
	}

	return providers
}

// CacheKey represents a key for caching embeddings
type CacheKey struct {
	Text       string
	ModelName  string
	Dimensions int
}

// SimpleEmbeddingCache provides a basic in-memory cache for embeddings
type SimpleEmbeddingCache struct {
	cache map[CacheKey][]float32
	mutex sync.RWMutex
}

// NewSimpleEmbeddingCache creates a new simple embedding cache
func NewSimpleEmbeddingCache() *SimpleEmbeddingCache {
	return &SimpleEmbeddingCache{
		cache: make(map[CacheKey][]float32),
	}
}

// Get retrieves an embedding from the cache
func (c *SimpleEmbeddingCache) Get(key CacheKey) ([]float32, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	embedding, ok := c.cache[key]
	return embedding, ok
}

// Set stores an embedding in the cache
func (c *SimpleEmbeddingCache) Set(key CacheKey, embedding []float32) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache[key] = embedding
}

// Clear empties the cache
func (c *SimpleEmbeddingCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache = make(map[CacheKey][]float32)
}

// CachedEmbedder wraps an embedder with caching functionality
type CachedEmbedder struct {
	embedder Embedder
	cache    *SimpleEmbeddingCache
}

// NewCachedEmbedder creates a new cached embedder
func NewCachedEmbedder(embedder Embedder) *CachedEmbedder {
	return &CachedEmbedder{
		embedder: embedder,
		cache:    NewSimpleEmbeddingCache(),
	}
}

// Embed generates a vector embedding for a single text, with caching
func (e *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Check cache first
	key := CacheKey{
		Text:       text,
		ModelName:  e.embedder.Name(),
		Dimensions: e.embedder.Dimensions(),
	}

	if embedding, found := e.cache.Get(key); found {
		return embedding, nil
	}

	// If not in cache, generate embedding
	embedding, err := e.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// Store in cache
	e.cache.Set(key, embedding)

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts, with caching
func (e *CachedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Check which texts are not in cache
	var uncachedTexts []string
	var uncachedIndices []int
	var results = make([][]float32, len(texts))

	for i, text := range texts {
		key := CacheKey{
			Text:       text,
			ModelName:  e.embedder.Name(),
			Dimensions: e.embedder.Dimensions(),
		}

		if embedding, found := e.cache.Get(key); found {
			results[i] = embedding
		} else {
			uncachedTexts = append(uncachedTexts, text)
			uncachedIndices = append(uncachedIndices, i)
		}
	}

	// Generate embeddings for uncached texts
	if len(uncachedTexts) > 0 {
		embeddings, err := e.embedder.EmbedBatch(ctx, uncachedTexts)
		if err != nil {
			return nil, err
		}

		// Store in cache and results
		for i, embedding := range embeddings {
			text := uncachedTexts[i]
			resultIndex := uncachedIndices[i]

			key := CacheKey{
				Text:       text,
				ModelName:  e.embedder.Name(),
				Dimensions: e.embedder.Dimensions(),
			}

			e.cache.Set(key, embedding)
			results[resultIndex] = embedding
		}
	}

	return results, nil
}

// Dimensions returns the dimensionality of the embeddings
func (e *CachedEmbedder) Dimensions() int {
	return e.embedder.Dimensions()
}

// Name returns the model name
func (e *CachedEmbedder) Name() string {
	return e.embedder.Name()
}

// Close releases resources used by the embedder
func (e *CachedEmbedder) Close() error {
	e.cache.Clear()
	return e.embedder.Close()
}
