package qdrant

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Logger defines the logging interface for the Qdrant client
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// defaultLogger implements the Logger interface using the standard log package
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[DEBUG] "+msg+" %v", args...)
	} else {
		log.Printf("[DEBUG] " + msg)
	}
}

func (l *defaultLogger) Info(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[INFO] "+msg+" %v", args...)
	} else {
		log.Printf("[INFO] " + msg)
	}
}

func (l *defaultLogger) Warn(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[WARN] "+msg+" %v", args...)
	} else {
		log.Printf("[WARN] " + msg)
	}
}

func (l *defaultLogger) Error(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[ERROR] "+msg+" %v", args...)
	} else {
		log.Printf("[ERROR] " + msg)
	}
}

// Client wraps the Qdrant client for ObsFind usage
type Client struct {
	mu          sync.RWMutex
	conn        *grpc.ClientConn
	collections pb.CollectionsClient
	points      pb.PointsClient
	config      *Config
	embedded    *EmbeddedServer
	logger      Logger
}

// Config holds Qdrant connection configuration
type Config struct {
	Host           string
	Port           int
	APIKey         string
	Embedded       bool
	DataPath       string
	Collection     string
	DefaultTimeout time.Duration
}

// ClientOption defines a function that configures a client
type ClientOption func(*Client)

// WithLogger sets a custom logger for the client
func WithLogger(logger Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithTimeout sets a default timeout for operations
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.config.DefaultTimeout = timeout
	}
}

// NewClient creates a new Qdrant client and automatically connects
func NewClient(config *Config, options ...ClientOption) (*Client, error) {
	// Use default timeout if not set
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	client := &Client{
		config: config,
		logger: &defaultLogger{},
	}

	// Apply all options
	for _, option := range options {
		option(client)
	}

	// If using embedded mode, start the embedded server
	if config.Embedded {
		embedded, err := NewEmbeddedServer(config.DataPath, config.Port)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedded server: %w", err)
		}

		client.embedded = embedded
	}

	client.logger.Debug("Created Qdrant client", "host", config.Host, "port", config.Port, "embedded", config.Embedded)

	// Auto-connect to the server - simplified approach matching tmp/simple-insert.go
	// This ensures we're connected before any operations
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultTimeout)
	defer cancel()

	// First try to connect - important step for ensuring we can do operations
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant during initialization: %w", err)
	}

	client.logger.Info("Successfully initialized and connected Qdrant client",
		"host", config.Host,
		"port", config.Port,
		"collection", config.Collection)

	return client, nil
}

// waitForServerReady waits for the embedded server to be ready
func (c *Client) waitForServerReady(ctx context.Context) error {
	c.logger.Debug("Waiting for embedded server to start")

	// Create a timeout context if not provided
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.DefaultTimeout)
		defer cancel()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for embedded server to start: %w", ctx.Err())
		case <-ticker.C:
			if c.embedded.IsRunning() {
				c.logger.Info("Embedded server is running")
				return nil
			}
		}
	}
}

// Connect establishes connection to Qdrant
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already connected, do nothing
	if c.conn != nil {
		c.logger.Debug("Already connected to Qdrant")
		return nil
	}

	// If using embedded mode, start the server
	if c.config.Embedded && c.embedded != nil {
		c.logger.Info("Starting embedded Qdrant server")
		if err := c.embedded.Start(); err != nil {
			return fmt.Errorf("failed to start embedded server: %w", err)
		}

		// Wait for server to start
		if err := c.waitForServerReady(ctx); err != nil {
			return err
		}
	}

	// Connect to Qdrant
	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	c.logger.Info("Connecting to Qdrant", "address", addr)

	// Use context with timeout if not provided
	dialCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, c.config.DefaultTimeout)
		defer cancel()
	}

	// Simplified connection setup matching the working implementation in tmp/simple-insert.go
	conn, err := grpc.DialContext(
		dialCtx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Remove WithBlock to avoid hanging if server is not responsive
	)
	if err != nil {
		return fmt.Errorf("failed to connect to Qdrant at %s: %w", addr, err)
	}

	// On error after this point, clean up connection
	defer func() {
		if err != nil && conn != nil {
			c.logger.Debug("Cleaning up connection due to error")
			conn.Close()
		}
	}()

	// Initialize clients
	c.conn = conn
	c.collections = pb.NewCollectionsClient(conn)
	c.points = pb.NewPointsClient(conn)

	// Verify connection with a ping
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	_, pingErr := c.collections.List(pingCtx, &pb.ListCollectionsRequest{})
	if pingErr != nil {
		c.logger.Error("Failed to ping Qdrant after connection", "error", pingErr)
		// Close the connection
		conn.Close()
		c.conn = nil
		c.collections = nil
		c.points = nil
		return fmt.Errorf("connected to Qdrant but failed to verify connection: %w", pingErr)
	}

	c.logger.Info("Successfully connected to Qdrant", "address", addr)
	return nil
}

// GetConnection returns the internal gRPC connection
func (c *Client) GetConnection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// Close releases resources
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Debug("Closing Qdrant client resources")
	var errs []error

	// Close gRPC connection
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("Error closing gRPC connection", "error", err)
			errs = append(errs, err)
		}
		c.conn = nil
		c.collections = nil
		c.points = nil
	}

	// Stop embedded server if used
	if c.config.Embedded && c.embedded != nil {
		if stopErr := c.embedded.Stop(); stopErr != nil {
			c.logger.Error("Error stopping embedded server", "error", stopErr)
			errs = append(errs, stopErr)
		}
		c.embedded = nil
	}

	if len(errs) == 0 {
		return nil
	}

	// Combine errors if multiple
	if len(errs) == 1 {
		return errs[0]
	}

	// Create combined error message
	errMsg := "multiple errors during close:"
	for i, err := range errs {
		errMsg += fmt.Sprintf(" (%d) %v;", i+1, err)
	}

	return fmt.Errorf(errMsg)
}

// ensureContext creates a context with timeout if no deadline is set
func (c *Client) ensureContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		return context.WithTimeout(ctx, c.config.DefaultTimeout)
	}
	return ctx, func() {}
}

// CreateCollection creates a new collection if it doesn't exist
func (c *Client) CreateCollection(ctx context.Context, collectionName string, dimensions uint64, distance pb.Distance) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Creating collection", "name", collectionName, "dimensions", dimensions, "distance", distance)

	// Check if collection already exists
	exists, err := c.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("failed to check if collection exists: %w", err)
	}

	if exists {
		c.logger.Info("Collection already exists", "name", collectionName)
		return nil
	}

	// Define vector configuration
	vectorConfig := &pb.VectorParams{
		Size:     dimensions,
		Distance: distance,
	}

	// Define collection configuration - using VectorsConfig_Params instead of VectorsConfig_ParamsMap
	// This matches how the vectors are defined in the PointStruct
	createRequest := &pb.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: vectorConfig,
			},
		},
	}

	// Create collection
	_, err = c.collections.Create(ctx, createRequest)
	if err != nil {
		c.logger.Error("Failed to create collection", "name", collectionName, "error", err)
		return fmt.Errorf("failed to create collection %s: %w", collectionName, err)
	}

	c.logger.Info("Created collection", "name", collectionName, "dimensions", dimensions)
	return nil
}

// CollectionExists checks if a collection exists
func (c *Client) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return false, fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Checking if collection exists", "name", collectionName)

	// List collections
	response, err := c.collections.List(ctx, &pb.ListCollectionsRequest{})
	if err != nil {
		c.logger.Error("Failed to list collections", "error", err)
		return false, fmt.Errorf("failed to list collections: %w", err)
	}

	// Check if collection exists
	for _, collection := range response.Collections {
		if collection.Name == collectionName {
			c.logger.Debug("Collection exists", "name", collectionName)
			return true, nil
		}
	}

	c.logger.Debug("Collection does not exist", "name", collectionName)
	return false, nil
}

// DeleteCollection removes a collection
func (c *Client) DeleteCollection(ctx context.Context, collectionName string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Deleting collection", "name", collectionName)

	_, err := c.collections.Delete(ctx, &pb.DeleteCollection{
		CollectionName: collectionName,
	})
	if err != nil {
		c.logger.Error("Failed to delete collection", "name", collectionName, "error", err)
		return fmt.Errorf("failed to delete collection %s: %w", collectionName, err)
	}

	c.logger.Info("Deleted collection", "name", collectionName)
	return nil
}

// ListCollections returns all collections
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Listing collections")

	response, err := c.collections.List(ctx, &pb.ListCollectionsRequest{})
	if err != nil {
		c.logger.Error("Failed to list collections", "error", err)
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	var collections []string
	for _, collection := range response.Collections {
		collections = append(collections, collection.Name)
	}

	c.logger.Debug("Listed collections", "count", len(collections))
	return collections, nil
}

// GetCollectionInfo retrieves detailed information about a collection
func (c *Client) GetCollectionInfo(ctx context.Context, collectionName string) (*pb.CollectionInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Getting collection info", "name", collectionName)

	response, err := c.collections.Get(ctx, &pb.GetCollectionInfoRequest{
		CollectionName: collectionName,
	})

	if err != nil {
		c.logger.Error("Failed to get collection info", "name", collectionName, "error", err)
		return nil, fmt.Errorf("failed to get collection info for %s: %w", collectionName, err)
	}

	// Log important details from the collection info
	if response != nil && response.Result != nil {
		info := response.Result

		// Format vector count
		vectorsCount := "nil"
		if info.VectorsCount != nil {
			vectorsCount = fmt.Sprintf("%d", *info.VectorsCount)
		}

		// Format points count
		pointsCount := "nil"
		if info.PointsCount != nil {
			pointsCount = fmt.Sprintf("%d", *info.PointsCount)
		}

		// Format segments count (not a pointer)
		segmentsCount := fmt.Sprintf("%d", info.SegmentsCount)

		c.logger.Info("Got collection info",
			"name", collectionName,
			"status", info.Status,
			"vectors_count", vectorsCount,
			"points_count", pointsCount,
			"segments", segmentsCount)
	} else {
		c.logger.Warn("Received nil response or nil result for collection", "name", collectionName)
	}

	return response.Result, nil
}

// Point represents a vector with payload
type Point struct {
	ID      string
	Vector  []float32
	Payload map[string]interface{}
}

// convertToPointStruct converts Point to pb.PointStruct
func convertToPointStruct(point Point) (*pb.PointStruct, error) {
	// Convert payload to structured value
	payload, err := convertPayloadToStructured(point.Payload)
	if err != nil {
		return nil, err
	}

	// Ensure we have a valid vector
	if len(point.Vector) == 0 {
		return nil, fmt.Errorf("point vector is empty")
	}

	// Log vector details for debugging
	log.Printf("Converting point: ID=%s, Vector Length=%d, Payload Keys=%v",
		point.ID,
		len(point.Vector),
		func() []string {
			keys := make([]string, 0, len(point.Payload))
			for k := range point.Payload {
				keys = append(keys, k)
			}
			return keys
		}())

	var pointID *pb.PointId
	if point.ID == "" {
		// Use int ID if string ID is empty
		pointID = &pb.PointId{
			PointIdOptions: &pb.PointId_Num{
				Num: 1, // Default to 1 if no ID provided
			},
		}
	} else {
		pointID = &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: point.ID,
			},
		}
	}

	// Create struct exactly matching the working implementation in tmp/simple-insert.go
	return &pb.PointStruct{
		Id: pointID,
		Vectors: &pb.Vectors{
			VectorsOptions: &pb.Vectors_Vector{
				Vector: &pb.Vector{
					Data: point.Vector,
				},
			},
		},
		Payload: payload,
	}, nil
}

// convertPayloadToStructured converts map to structured value
func convertPayloadToStructured(payload map[string]interface{}) (map[string]*pb.Value, error) {
	result := make(map[string]*pb.Value)

	for k, v := range payload {
		value, err := convertToValue(v)
		if err != nil {
			return nil, err
		}
		result[k] = value
	}

	return result, nil
}

// convertToValue converts Go value to pb.Value
func convertToValue(v interface{}) (*pb.Value, error) {
	switch val := v.(type) {
	case nil:
		return &pb.Value{
			Kind: &pb.Value_NullValue{
				NullValue: pb.NullValue_NULL_VALUE,
			},
		}, nil
	case bool:
		return &pb.Value{
			Kind: &pb.Value_BoolValue{
				BoolValue: val,
			},
		}, nil
	case string:
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: val,
			},
		}, nil
	case float64:
		return &pb.Value{
			Kind: &pb.Value_DoubleValue{
				DoubleValue: val,
			},
		}, nil
	case float32:
		return &pb.Value{
			Kind: &pb.Value_DoubleValue{
				DoubleValue: float64(val),
			},
		}, nil
	case int:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{
				IntegerValue: int64(val),
			},
		}, nil
	case int64:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{
				IntegerValue: val,
			},
		}, nil
	case []interface{}:
		items := make([]*pb.Value, len(val))
		for i, item := range val {
			v, err := convertToValue(item)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return &pb.Value{
			Kind: &pb.Value_ListValue{
				ListValue: &pb.ListValue{
					Values: items,
				},
			},
		}, nil
	case []string:
		items := make([]*pb.Value, len(val))
		for i, item := range val {
			items[i] = &pb.Value{
				Kind: &pb.Value_StringValue{
					StringValue: item,
				},
			}
		}
		return &pb.Value{
			Kind: &pb.Value_ListValue{
				ListValue: &pb.ListValue{
					Values: items,
				},
			},
		}, nil
	case map[string]interface{}:
		structVal, err := convertPayloadToStructured(val)
		if err != nil {
			return nil, err
		}
		return &pb.Value{
			Kind: &pb.Value_StructValue{
				StructValue: &pb.Struct{
					Fields: structVal,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
}

// BatchConfig controls the batching behavior
type BatchConfig struct {
	// BatchSize is the maximum number of items per batch
	BatchSize int
	// MaxConcurrent is the maximum number of concurrent batch operations
	MaxConcurrent int
}

// DefaultBatchConfig returns the default batching configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		BatchSize:     100,
		MaxConcurrent: 4,
	}
}

// batchProcess is a generic function to process items in batches
// The processBatch function is called for each batch of items
func batchProcess[T any](
	ctx context.Context,
	items []T,
	batchConfig BatchConfig,
	processBatch func(ctx context.Context, batch []T) error,
) error {
	if len(items) == 0 {
		return nil
	}

	// If batch size is 0 or negative, process all items in one batch
	if batchConfig.BatchSize <= 0 {
		return processBatch(ctx, items)
	}

	// If there are fewer items than the batch size, process all items in one batch
	if len(items) <= batchConfig.BatchSize {
		return processBatch(ctx, items)
	}

	// Divide items into batches
	var batches [][]T
	for i := 0; i < len(items); i += batchConfig.BatchSize {
		end := i + batchConfig.BatchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	// Limit concurrency
	maxConcurrent := batchConfig.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if maxConcurrent > len(batches) {
		maxConcurrent = len(batches)
	}

	// Process batches with limited concurrency
	errors := make(chan error, len(batches))
	semaphore := make(chan struct{}, maxConcurrent)

	// Start goroutines for each batch
	for i, batch := range batches {
		batchIndex := i
		batchItems := batch

		// Acquire semaphore slot
		semaphore <- struct{}{}

		go func() {
			defer func() { <-semaphore }() // Release semaphore slot

			// Check if context is canceled
			if ctx.Err() != nil {
				errors <- ctx.Err()
				return
			}

			// Process this batch
			err := processBatch(ctx, batchItems)
			if err != nil {
				errors <- fmt.Errorf("batch %d failed: %w", batchIndex, err)
				return
			}

			errors <- nil
		}()
	}

	// Collect results from all batches
	var errs []error
	for i := 0; i < len(batches); i++ {
		err := <-errors
		if err != nil {
			errs = append(errs, err)
		}
	}

	// Return combined errors if any
	if len(errs) > 0 {
		var combinedErr error
		for _, err := range errs {
			if combinedErr == nil {
				combinedErr = err
			} else {
				combinedErr = fmt.Errorf("%v; %v", combinedErr, err)
			}
		}
		return combinedErr
	}

	return nil
}

// UpsertPoints adds or updates points in the collection according to model.QdrantClient interface
func (c *Client) UpsertPoints(ctx context.Context, collectionName string, points []*pb.PointStruct) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	if len(points) == 0 {
		return nil
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Upserting points", "collection", collectionName, "count", len(points))

	// Process single batch (no batching) - simplified approach like in tmp/simple-insert.go
	if len(points) <= 100 {
		// Log the first point structure for debugging
		if len(points) > 0 {
			c.logger.Debug("First point structure",
				"id", points[0].Id,
				"has_vector", points[0].Vectors != nil,
				"vector_length", len(points[0].Vectors.GetVector().Data),
				"payload_keys", getPayloadKeys(points[0].Payload))
		}

		// Direct upsert similar to the working example in tmp/simple-insert.go
		_, err := c.points.Upsert(ctx, &pb.UpsertPoints{
			CollectionName: collectionName,
			Points:         points,
		})
		if err != nil {
			c.logger.Error("Failed to upsert points", "collection", collectionName, "error", err)
			return fmt.Errorf("failed to upsert points: %w", err)
		}

		c.logger.Debug("Upserted points successfully", "collection", collectionName, "count", len(points))
		return nil
	}

	// Process in batches for larger sets
	c.logger.Info("Upserting points in batches", "collection", collectionName, "total_count", len(points))

	batchConfig := DefaultBatchConfig()

	// Define batch processing function - simplified direct approach
	processBatch := func(ctx context.Context, batch []*pb.PointStruct) error {
		_, err := c.points.Upsert(ctx, &pb.UpsertPoints{
			CollectionName: collectionName,
			Points:         batch,
		})
		if err != nil {
			c.logger.Error("Failed to upsert batch", "collection", collectionName, "batch_size", len(batch), "error", err)
			return fmt.Errorf("failed to upsert batch: %w", err)
		}

		c.logger.Debug("Upserted batch successfully", "collection", collectionName, "batch_size", len(batch))
		return nil
	}

	// Process in batches
	err := batchProcess(ctx, points, batchConfig, processBatch)
	if err != nil {
		return fmt.Errorf("batch upsert failed: %w", err)
	}

	c.logger.Info("Completed upserting all points", "collection", collectionName, "total_count", len(points))
	return nil
}

// Helper function to get payload keys for logging
func getPayloadKeys(payload map[string]*pb.Value) []string {
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	return keys
}

// UpsertCustomPoints is a convenience method for working with our custom Point type
func (c *Client) UpsertCustomPoints(ctx context.Context, collectionName string, points []Point) error {
	if len(points) == 0 {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock() // We'll handle explicit unlocking where needed

	if c.conn == nil {
		return fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Converting custom points for upsert", "collection", collectionName, "count", len(points))

	// Process in smaller batches if the input is large
	if len(points) > 100 {
		c.mu.RUnlock() // Unlock before batch processing

		// For larger sets, process in batches of 100 for direct insertion
		var batches [][]Point
		for i := 0; i < len(points); i += 100 {
			end := i + 100
			if end > len(points) {
				end = len(points)
			}
			batches = append(batches, points[i:end])
		}

		c.logger.Info("Processing custom points in batches", "collection", collectionName, "total_count", len(points), "batches", len(batches))

		// Process each batch directly
		for i, batch := range batches {
			// Convert batch to point structs
			pointStructs := make([]*pb.PointStruct, 0, len(batch))
			for _, point := range batch {
				ps, err := convertToPointStruct(point)
				if err != nil {
					c.logger.Error("Failed to convert point", "id", point.ID, "error", err)
					return fmt.Errorf("failed to convert point %s in batch %d: %w", point.ID, i, err)
				}
				pointStructs = append(pointStructs, ps)
			}

			// Log first point in batch for debugging
			if len(pointStructs) > 0 {
				c.logger.Debug("First point in batch",
					"batch", i,
					"id", pointStructs[0].Id,
					"has_vector", pointStructs[0].Vectors != nil,
					"vector_length", len(pointStructs[0].Vectors.GetVector().Data),
					"payload_keys", getPayloadKeys(pointStructs[0].Payload))
			}

			// Direct upsert for this batch
			batchCtx, batchCancel := c.ensureContext(ctx)
			_, err := c.points.Upsert(batchCtx, &pb.UpsertPoints{
				CollectionName: collectionName,
				Points:         pointStructs,
			})
			batchCancel()

			if err != nil {
				c.logger.Error("Failed to upsert batch", "batch", i, "error", err)
				return fmt.Errorf("failed to upsert batch %d: %w", i, err)
			}

			c.logger.Debug("Upserted batch successfully", "batch", i, "size", len(batch))
		}

		c.logger.Info("Completed upserting all custom points", "collection", collectionName, "total_count", len(points))
		return nil
	}

	// For small sets, convert and insert directly
	pointStructs := make([]*pb.PointStruct, 0, len(points))
	for _, point := range points {
		ps, err := convertToPointStruct(point)
		if err != nil {
			c.logger.Error("Failed to convert point", "id", point.ID, "error", err)
			return fmt.Errorf("failed to convert point %s: %w", point.ID, err)
		}
		pointStructs = append(pointStructs, ps)
	}

	// Log first point structure for debugging
	if len(pointStructs) > 0 {
		c.logger.Debug("First point structure",
			"id", pointStructs[0].Id,
			"has_vector", pointStructs[0].Vectors != nil,
			"vector_length", len(pointStructs[0].Vectors.GetVector().Data),
			"payload_keys", getPayloadKeys(pointStructs[0].Payload))
	}

	c.mu.RUnlock() // Unlock before making the gRPC call

	// Direct upsert without going through UpsertPoints again
	ctxDirect, cancelDirect := c.ensureContext(ctx)
	defer cancelDirect()

	_, err := c.points.Upsert(ctxDirect, &pb.UpsertPoints{
		CollectionName: collectionName,
		Points:         pointStructs,
	})

	if err != nil {
		c.logger.Error("Failed to upsert points", "collection", collectionName, "error", err)
		return fmt.Errorf("failed to upsert custom points: %w", err)
	}

	c.logger.Debug("Upserted custom points successfully", "collection", collectionName, "count", len(points))
	return nil
}

// DeletePoints removes points from the collection with batching for large requests
func (c *Client) DeletePoints(ctx context.Context, collectionName string, ids []string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	if len(ids) == 0 {
		return nil
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Deleting points", "collection", collectionName, "count", len(ids))

	// If we have more than 100 IDs, process in batches
	maxBatchSize := 100
	if len(ids) > maxBatchSize {
		c.logger.Info("Deleting points in batches", "collection", collectionName, "total_count", len(ids))

		// Create batch config
		batchConfig := DefaultBatchConfig()
		batchConfig.BatchSize = maxBatchSize

		// Define batch processor
		processBatch := func(ctx context.Context, idBatch []string) error {
			// Call the internal method for each batch
			return c.deletePointsInternal(ctx, collectionName, idBatch)
		}

		// Process in batches
		err := batchProcess(ctx, ids, batchConfig, processBatch)
		if err != nil {
			return fmt.Errorf("batch delete points failed: %w", err)
		}

		c.logger.Info("Completed deleting all points", "collection", collectionName, "total_count", len(ids))
		return nil
	}

	// For small requests, process directly
	return c.deletePointsInternal(ctx, collectionName, ids)
}

// deletePointsInternal handles the actual deletion of points, used by DeletePoints
func (c *Client) deletePointsInternal(ctx context.Context, collectionName string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Convert IDs to PointID
	pointIDs := make([]*pb.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: id,
			},
		}
	}

	// Delete points
	_, err := c.points.Delete(ctx, &pb.DeletePoints{
		CollectionName: collectionName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: pointIDs,
				},
			},
		},
	})
	if err != nil {
		c.logger.Error("Failed to delete points", "collection", collectionName, "count", len(ids), "error", err)
		return fmt.Errorf("failed to delete points: %w", err)
	}

	c.logger.Debug("Deleted points successfully", "collection", collectionName, "count", len(ids))
	return nil
}

// GetPoints retrieves points by IDs with batching for large requests
func (c *Client) GetPoints(ctx context.Context, collectionName string, ids []string) ([]Point, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	if len(ids) == 0 {
		return []Point{}, nil
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Getting points by IDs", "collection", collectionName, "count", len(ids))

	// If we have more than 100 IDs, process in batches
	maxBatchSize := 100
	if len(ids) > maxBatchSize {
		c.logger.Info("Getting points in batches", "collection", collectionName, "total_count", len(ids))

		// Create batch config
		batchConfig := DefaultBatchConfig()
		batchConfig.BatchSize = maxBatchSize

		// Prepare to collect results
		resultChan := make(chan []Point, (len(ids)+maxBatchSize-1)/maxBatchSize)

		// Define batch processor
		processBatch := func(ctx context.Context, idBatch []string) error {
			// Call this same method recursively for each batch
			points, err := c.getPointsInternal(ctx, collectionName, idBatch)
			if err != nil {
				return err
			}
			resultChan <- points
			return nil
		}

		// Process in batches
		err := batchProcess(ctx, ids, batchConfig, processBatch)
		if err != nil {
			return nil, fmt.Errorf("batch get points failed: %w", err)
		}

		// Collect all results
		var allPoints []Point
		batchCount := (len(ids) + maxBatchSize - 1) / maxBatchSize
		for i := 0; i < batchCount; i++ {
			batchPoints := <-resultChan
			allPoints = append(allPoints, batchPoints...)
		}

		c.logger.Info("Completed getting all points", "collection", collectionName, "total_count", len(allPoints))
		return allPoints, nil
	}

	// For small requests, process directly
	return c.getPointsInternal(ctx, collectionName, ids)
}

// getPointsInternal handles the actual retrieval of points, used by GetPoints
func (c *Client) getPointsInternal(ctx context.Context, collectionName string, ids []string) ([]Point, error) {
	if len(ids) == 0 {
		return []Point{}, nil
	}

	// Convert IDs to PointID
	pointIDs := make([]*pb.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: id,
			},
		}
	}

	// Get points
	response, err := c.points.Get(ctx, &pb.GetPoints{
		CollectionName: collectionName,
		Ids:            pointIDs,
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
		WithVectors:    &pb.WithVectorsSelector{SelectorOptions: &pb.WithVectorsSelector_Enable{Enable: true}},
	})
	if err != nil {
		c.logger.Error("Failed to get points", "collection", collectionName, "count", len(ids), "error", err)
		return nil, fmt.Errorf("failed to get points: %w", err)
	}

	// Convert response to Point
	result := make([]Point, len(response.Result))
	for i, p := range response.Result {
		id := ""
		if p.Id != nil && p.Id.GetUuid() != "" {
			id = p.Id.GetUuid()
		}

		vector := []float32{}
		if p.Vectors != nil && p.Vectors.GetVector() != nil {
			vector = p.Vectors.GetVector().Data
		}

		payload := map[string]interface{}{}
		if p.Payload != nil {
			for k, v := range p.Payload {
				payload[k] = convertValueToInterface(v)
			}
		}

		result[i] = Point{
			ID:      id,
			Vector:  vector,
			Payload: payload,
		}
	}

	c.logger.Debug("Got points", "collection", collectionName, "count", len(result))
	return result, nil
}

// convertValueToInterface converts pb.Value to Go interface{}
func convertValueToInterface(v *pb.Value) interface{} {
	switch v.Kind.(type) {
	case *pb.Value_NullValue:
		return nil
	case *pb.Value_BoolValue:
		return v.GetBoolValue()
	case *pb.Value_StringValue:
		return v.GetStringValue()
	case *pb.Value_DoubleValue:
		return v.GetDoubleValue()
	case *pb.Value_IntegerValue:
		return v.GetIntegerValue()
	case *pb.Value_ListValue:
		list := v.GetListValue()
		result := make([]interface{}, len(list.Values))
		for i, item := range list.Values {
			result[i] = convertValueToInterface(item)
		}
		return result
	case *pb.Value_StructValue:
		structVal := v.GetStructValue()
		result := make(map[string]interface{})
		for k, val := range structVal.Fields {
			result[k] = convertValueToInterface(val)
		}
		return result
	default:
		return nil
	}
}

// SearchOptions contains search parameters
type SearchOptions struct {
	Limit       uint64
	Offset      uint64
	WithPayload bool
	Filter      *pb.Filter
}

// SearchResult represents a single search result
type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]interface{}
}

// Search performs vector similarity search according to the model.QdrantClient interface
func (c *Client) Search(
	ctx context.Context,
	collectionName string,
	vector []float32,
	limit uint64,
	offset uint64,
	filter *pb.Filter,
	params *pb.SearchParams,
) ([]*pb.ScoredPoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Searching collection",
		"collection", collectionName,
		"vector_length", len(vector),
		"limit", limit,
		"offset", offset,
		"has_filter", filter != nil)

	// Create search request without named vector (use default vector)
	request := &pb.SearchPoints{
		CollectionName: collectionName,
		Vector:         vector,
		// Don't use VectorName for non-named vectors config
		Limit:  limit,
		Offset: &offset,
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
		WithVectors: &pb.WithVectorsSelector{
			SelectorOptions: &pb.WithVectorsSelector_Enable{
				Enable: true,
			},
		},
	}

	// Add filter if provided
	if filter != nil {
		request.Filter = filter
	}

	// Add search params if provided
	if params != nil {
		request.Params = params
	}

	// Execute search
	response, err := c.points.Search(ctx, request)
	if err != nil {
		c.logger.Error("Search failed", "collection", collectionName, "error", err)
		return nil, fmt.Errorf("failed to search in %s: %w", collectionName, err)
	}

	resultCount := 0
	if response != nil && response.Result != nil {
		resultCount = len(response.Result)
	}

	c.logger.Debug("Search completed", "collection", collectionName, "result_count", resultCount)
	return response.Result, nil
}

// SearchWithOptions is a convenience method that wraps the standard Search method
func (c *Client) SearchWithOptions(ctx context.Context, collectionName string, vector []float32, options SearchOptions) ([]SearchResult, error) {
	// The Search method already handles mutex locking and context timeout,
	// so we don't need to duplicate that here

	c.logger.Debug("Searching with options",
		"collection", collectionName,
		"vector_length", len(vector),
		"limit", options.Limit,
		"has_filter", options.Filter != nil)

	// Call the interface-compliant Search method
	scoredPoints, err := c.Search(ctx, collectionName, vector, options.Limit, options.Offset, options.Filter, nil)
	if err != nil {
		// Error already logged in the Search method
		return nil, err
	}

	// Convert response to SearchResult
	results := make([]SearchResult, len(scoredPoints))
	for i, r := range scoredPoints {
		id := ""
		if r.Id != nil && r.Id.GetUuid() != "" {
			id = r.Id.GetUuid()
		}

		payload := map[string]interface{}{}
		if r.Payload != nil {
			for k, v := range r.Payload {
				payload[k] = convertValueToInterface(v)
			}
		}

		results[i] = SearchResult{
			ID:      id,
			Score:   r.Score,
			Payload: payload,
		}
	}

	c.logger.Debug("Converted search results", "count", len(results))
	return results, nil
}

// Schema defines the collection schema for ObsFind
type Schema struct {
	VectorSize int
	IndexType  string
}

// DefaultSchema returns the default schema for ObsFind
func DefaultSchema() *Schema {
	return &Schema{
		VectorSize: 768, // Default for nomic-embed-text model
		IndexType:  "hnsw",
	}
}

// Apply creates or updates the collection according to schema
func (s *Schema) Apply(ctx context.Context, client *Client, collection string) error {
	// Create collection if it doesn't exist
	if err := client.CreateCollection(ctx, collection, uint64(s.VectorSize), pb.Distance_Cosine); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

// CreatePayloadIndex creates an index for a payload field
func (c *Client) CreatePayloadIndex(ctx context.Context, collectionName string, fieldName string, fieldType int) error {
	// Convert the int field type to qdrant.FieldType enum
	var qFieldType pb.FieldType
	switch fieldType {
	case 1: // Text
		qFieldType = pb.FieldType_FieldTypeText
	case 2: // Keyword
		qFieldType = pb.FieldType_FieldTypeKeyword
	case 3: // Integer
		qFieldType = pb.FieldType_FieldTypeInteger
	case 4: // Float
		qFieldType = pb.FieldType_FieldTypeFloat
	default:
		return fmt.Errorf("unsupported field type: %d", fieldType)
	}

	// Create the index request
	createFieldIndexRequest := &pb.CreateFieldIndexCollection{
		CollectionName: collectionName,
		FieldName:      fieldName,
		FieldType:      &qFieldType,
		Wait:           &[]bool{true}[0], // Wait for operation to complete
	}

	// Execute the request
	_, err := c.points.CreateFieldIndex(ctx, createFieldIndexRequest)
	if err != nil {
		return fmt.Errorf("failed to create index for field %s: %w", fieldName, err)
	}

	log.Printf("Created payload index for field %s in collection %s (type: %v)", fieldName, collectionName, qFieldType)
	return nil
}

// GetPointsByPath retrieves points with a specific path with proper pagination
func (c *Client) GetPointsByPath(ctx context.Context, collectionName string, path string) ([]*pb.RetrievedPoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected to Qdrant, call Connect() first")
	}

	ctx, cancel := c.ensureContext(ctx)
	defer cancel()

	c.logger.Debug("Getting points by path", "collection", collectionName, "path", path)

	// Create a match condition for the path field
	matchCondition := &pb.Condition{
		ConditionOneOf: &pb.Condition_Field{
			Field: &pb.FieldCondition{
				Key: "path",
				Match: &pb.Match{
					MatchValue: &pb.Match_Text{
						Text: path,
					},
				},
			},
		},
	}

	// Create a filter with the match condition
	filter := &pb.Filter{
		Must: []*pb.Condition{matchCondition},
	}

	// Paginate through results using scroll API
	var allResults []*pb.RetrievedPoint
	var pointId *pb.PointId = nil
	limit := uint32(100)

	for {
		// Create scroll request with pagination
		request := &pb.ScrollPoints{
			CollectionName: collectionName,
			Filter:         filter,
			Limit:          &limit,
			WithPayload: &pb.WithPayloadSelector{
				SelectorOptions: &pb.WithPayloadSelector_Enable{
					Enable: true,
				},
			},
			WithVectors: &pb.WithVectorsSelector{
				SelectorOptions: &pb.WithVectorsSelector_Enable{
					Enable: true,
				},
			},
			Offset: pointId, // Use the last point ID as offset for pagination
		}

		// Execute scroll request
		response, err := c.points.Scroll(ctx, request)
		if err != nil {
			c.logger.Error("Failed to scroll points", "collection", collectionName, "path", path, "error", err)
			return nil, fmt.Errorf("failed to get points by path: %w", err)
		}

		// Add results to the collection
		allResults = append(allResults, response.Result...)

		// Check if there are more results
		if len(response.Result) == 0 || response.NextPageOffset == nil {
			break
		}

		// Update the point ID for next page
		pointId = response.NextPageOffset

		c.logger.Debug("Retrieved batch of points", "count", len(response.Result), "total_so_far", len(allResults))
	}

	c.logger.Info("Retrieved points by path", "collection", collectionName, "path", path, "count", len(allResults))
	return allResults, nil
}

// EmbeddedServer manages an embedded Qdrant instance
type EmbeddedServer struct {
	cmd      *exec.Cmd
	dataPath string
	port     int
}

// NewEmbeddedServer creates a new embedded Qdrant server
func NewEmbeddedServer(dataPath string, port int) (*EmbeddedServer, error) {
	// Ensure the data directory exists
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create log directory
	logDir := filepath.Join(dataPath, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return &EmbeddedServer{
		dataPath: dataPath,
		port:     port,
	}, nil
}

// Start launches the embedded server
func (s *EmbeddedServer) Start() error {
	// Check if already running
	if s.cmd != nil && s.cmd.Process != nil {
		// Check if process is still running
		if s.IsRunning() {
			return nil
		}
	}

	// For now, this is a placeholder for embedded Qdrant
	// In a real implementation, we would:
	// 1. Download Qdrant binary if not available
	// 2. Launch it with appropriate arguments
	// 3. Monitor the process

	log.Println("Embedded Qdrant is not yet implemented - using external Qdrant")

	// Simulating a running server (will be replaced in a real implementation)
	s.cmd = exec.Command("sleep", "3600")
	return nil
}

// Stop gracefully stops the server
func (s *EmbeddedServer) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	// Send signal to terminate
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		// If interrupt fails, force kill
		if killErr := s.cmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("failed to kill process: %w", killErr)
		}
	}

	// Wait for process to exit
	return s.cmd.Wait()
}

// IsRunning checks if the server is operational
func (s *EmbeddedServer) IsRunning() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}

	// Try to get process state (will return nil if running)
	if s.cmd.ProcessState != nil {
		return false
	}

	// Check if process exists - on Unix we'd use Signal(0)
	// For now, just assume process is running if we get here
	return true
}
