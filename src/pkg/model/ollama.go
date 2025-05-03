package model

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms/ollama"
)

// OllamaConfig holds configuration for Ollama embeddings
type OllamaConfig struct {
	ModelName   string
	ServerURL   string
	Dimensions  int
	BatchSize   int
	MaxAttempts int
	Timeout     int
}

// OllamaEmbedder uses Ollama for generating embeddings
type OllamaEmbedder struct {
	client      *ollama.LLM
	modelName   string
	dimensions  int
	batchSize   int
	maxAttempts int
	timeout     time.Duration
	mutex       sync.Mutex
}

// NewOllamaEmbedder creates a new Ollama-based embedder
func NewOllamaEmbedder(config OllamaConfig) (*OllamaEmbedder, error) {
	// Initialize Ollama client with appropriate options
	client, err := ollama.New(
		ollama.WithModel(config.ModelName),
		ollama.WithServerURL(config.ServerURL),
		// Use embedding-only mode for better performance
		ollama.WithRunnerEmbeddingOnly(true),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Ollama client: %w", err)
	}

	return &OllamaEmbedder{
		client:      client,
		modelName:   config.ModelName,
		dimensions:  config.Dimensions,
		batchSize:   config.BatchSize,
		maxAttempts: config.MaxAttempts,
		timeout:     time.Duration(config.Timeout) * time.Second,
	}, nil
}

// Embed generates a vector embedding for a single text
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return make([]float32, e.dimensions), nil
	}

	// Calculate a dynamic timeout based on text length
	dynamicTimeout := e.timeout
	if len(text) > 5000 {
		// Add 1 second per 5000 chars beyond the first 5000
		additionalTime := time.Duration((len(text)-5000)/5000+1) * time.Second
		dynamicTimeout += additionalTime
	}

	var embeddings [][]float32
	var err error

	for attempt := 0; attempt < e.maxAttempts; attempt++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, dynamicTimeout)

		// Log the embedding attempt for debugging
		fmt.Printf("Embedding single text (length: %d, timeout: %v, attempt: %d/%d)\n",
			len(text), dynamicTimeout, attempt+1, e.maxAttempts)

		embeddings, err = e.client.CreateEmbedding(timeoutCtx, []string{text})
		cancel()

		if err == nil && len(embeddings) > 0 && len(embeddings[0]) > 0 {
			break
		}

		fmt.Printf("Embedding attempt failed: %v\n", err)

		// Check if context was canceled by parent
		if ctx.Err() != nil {
			return nil, fmt.Errorf("embedding canceled by parent context: %w", ctx.Err())
		}

		// Exponential backoff before retry with a longer delay
		if attempt < e.maxAttempts-1 {
			backoffTime := time.Duration(500*(1<<attempt)) * time.Millisecond
			fmt.Printf("Retrying in %v...\n", backoffTime)
			time.Sleep(backoffTime)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create embedding after %d attempts: %w", e.maxAttempts, err)
	}

	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}

	// Return the first embedding
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Process in batches for better performance
	var allEmbeddings [][]float32

	// Calculate a more appropriate timeout based on text length
	// For very large batches, we need more time
	var longestText int
	for _, text := range texts {
		if len(text) > longestText {
			longestText = len(text)
		}
	}

	// Dynamic timeout: at least e.timeout, but scaled up for large texts
	// Base timeout + additional time proportional to text length
	dynamicTimeout := e.timeout
	if longestText > 5000 {
		// Add 1 second per 5000 chars beyond the first 5000
		additionalTime := time.Duration((longestText-5000)/5000+1) * time.Second
		dynamicTimeout += additionalTime
	}

	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]

		var embeddings [][]float32
		var err error

		// Try with retries
		for attempt := 0; attempt < e.maxAttempts; attempt++ {
			// Use the dynamic timeout that scales with content size
			timeoutCtx, cancel := context.WithTimeout(ctx, dynamicTimeout)

			// Log the embedding attempt for debugging
			fmt.Printf("Embedding batch %d/%d (size: %d, timeout: %v, attempt: %d/%d)\n",
				i/e.batchSize+1, (len(texts)+e.batchSize-1)/e.batchSize,
				len(batch), dynamicTimeout, attempt+1, e.maxAttempts)

			embeddings, err = e.client.CreateEmbedding(timeoutCtx, batch)
			cancel()

			if err == nil && len(embeddings) == len(batch) {
				break
			}

			fmt.Printf("Embedding attempt failed: %v\n", err)

			// Check if context was canceled by parent
			if ctx.Err() != nil {
				return nil, fmt.Errorf("embedding canceled by parent context: %w", ctx.Err())
			}

			// Exponential backoff before retry with a longer delay for large texts
			if attempt < e.maxAttempts-1 {
				backoffTime := time.Duration(500*(1<<attempt)) * time.Millisecond
				fmt.Printf("Retrying in %v...\n", backoffTime)
				time.Sleep(backoffTime)
			}
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create batch embeddings after %d attempts: %w", e.maxAttempts, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// Dimensions returns the dimensionality of the embeddings
func (e *OllamaEmbedder) Dimensions() int {
	return e.dimensions
}

// Name returns the model name
func (e *OllamaEmbedder) Name() string {
	return e.modelName
}

// Close releases resources used by the embedder
func (e *OllamaEmbedder) Close() error {
	// Clean up any resources if needed
	return nil
}

// Register the Ollama embedder factory
func init() {
	RegisterEmbedder("ollama", func(cfg Config) (Embedder, error) {
		ollamaCfg, ok := cfg.Specific.(OllamaConfig)
		if !ok {
			return nil, fmt.Errorf("invalid configuration for Ollama embedder")
		}
		return NewOllamaEmbedder(ollamaCfg)
	})
}

// IsOllamaAvailable checks if Ollama is available and the model is installed
func IsOllamaAvailable(ctx context.Context, serverURL, modelName string) (bool, error) {
	client, err := ollama.New(
		ollama.WithModel(modelName),
		ollama.WithServerURL(serverURL),
	)

	if err != nil {
		return false, fmt.Errorf("failed to initialize Ollama client: %w", err)
	}

	// Attempt to create a simple embedding as a test
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = client.CreateEmbedding(timeoutCtx, []string{"test"})
	if err != nil {
		return false, fmt.Errorf("failed to create test embedding: %w", err)
	}

	return true, nil
}

// GetOllamaInstallInstructions returns instructions for installing Ollama
func GetOllamaInstallInstructions() string {
	return `Ollama is required for local embedding generation.

Installation:
- macOS: brew install ollama
- Linux: curl -fsSL https://ollama.com/install.sh | sh
- Windows: Download from https://ollama.com/download

Once installed, run:
ollama pull nomic-embed-text

Then restart ObsFind.`
}
