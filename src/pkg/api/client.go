package api

import (
	"context"
	"net/http"
	"net/url"
	httputil2 "obsfind/src/pkg/httputil"
	"obsfind/src/pkg/indexer"
	"obsfind/src/pkg/loggingutil"
	"strconv"
	"time"
)

// Client is the API client for communicating with the ObsFind daemon
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new API client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Minute}, // Increased timeout to 10 minutes for long operations
	}
}

// Status checks the status of the daemon
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	logger := loggingutil.Get(ctx)
	logger.Debug("Requesting daemon status", "baseURL", c.baseURL)

	status, err := httputil2.GetJSON[StatusResponse](ctx, c.httpClient, c.baseURL, "/api/v1/status", nil)
	if err != nil {
		logger.Error("Failed to get daemon status", "error", err)
		return nil, err
	}

	logger.Debug("Got daemon status", "status", status.Status, "version", status.Version)
	return &status, nil
}

// Health checks if the daemon is running
func (c *Client) Health(ctx context.Context) (bool, error) {
	logger := loggingutil.Get(ctx)
	logger.Debug("Checking daemon health", "baseURL", c.baseURL)

	resp := httputil2.Get(ctx, c.httpClient, c.baseURL, "/api/v1/health", nil)
	if resp.Error() != nil {
		logger.Error("Health check failed", "error", resp.Error())
		return false, resp.Error()
	}
	defer httputil2.CloseBodyWithContext(ctx, resp.Response)

	isHealthy := resp.StatusCode == http.StatusOK
	logger.Debug("Health check completed", "isHealthy", isHealthy)
	return isHealthy, nil
}

// Search performs a semantic search
func (c *Client) Search(ctx context.Context, req *SearchRequest) ([]indexer.SearchResult, error) {
	logger := loggingutil.Get(ctx)
	logger.Debug("Performing semantic search",
		"query", req.Query,
		"limit", req.Limit,
		"pathPrefix", req.PathPrefix,
		"tags", req.Tags)

	// Construct query string for GET request
	values := url.Values{}
	values.Set("q", req.Query)
	if req.Limit > 0 {
		values.Set("limit", strconv.Itoa(req.Limit))
	}
	if req.Offset > 0 {
		values.Set("offset", strconv.Itoa(req.Offset))
	}
	if req.MinScore > 0 {
		values.Set("min_score", strconv.FormatFloat(float64(req.MinScore), 'f', 4, 32))
	}
	for _, tag := range req.Tags {
		values.Add("tag", tag)
	}
	if req.PathPrefix != "" {
		values.Set("path_prefix", req.PathPrefix)
	}

	// Get results directly using the GetJSON helper
	results, err := httputil2.GetJSON[[]indexer.SearchResult](ctx, c.httpClient, c.baseURL, "/api/v1/search/query", values)
	if err != nil {
		logger.Error("Search request failed", "error", err)
		return nil, err
	}

	logger.Debug("Search completed successfully", "resultCount", len(results))
	return results, nil
}

// Similar finds documents similar to the specified file
func (c *Client) Similar(ctx context.Context, req *SimilarRequest) ([]indexer.SearchResult, error) {
	logger := loggingutil.Get(ctx)
	logger.Debug("Finding similar documents",
		"path", req.Path,
		"limit", req.Limit)

	results, err := httputil2.PostJSON[[]indexer.SearchResult](ctx, c.httpClient, c.baseURL, "/api/v1/search/similar", req)
	if err != nil {
		logger.Error("Similar search request failed", "error", err)
		return nil, err
	}

	logger.Debug("Similar search completed successfully", "resultCount", len(results))
	return results, nil
}

// Reindex triggers a full reindexing of the vault
func (c *Client) Reindex(ctx context.Context, force bool) error {
	logger := loggingutil.Get(ctx)
	logger.Info("Requesting vault reindexing", "force", force)

	payload := map[string]bool{
		"force": force,
	}

	// Create a special client with an extended timeout specifically for reindexing
	reindexClient := &http.Client{
		Timeout: 30 * time.Minute, // 30 minute timeout for reindexing
	}

	// Use the typed response interface for better error handling
	resp := httputil2.PostTyped[struct{}](ctx, reindexClient, c.baseURL, "/api/v1/index/all", payload)
	if resp.Error() != nil {
		logger.Error("Reindex request failed", "error", resp.Error())
		return resp.Error()
	}
	defer httputil2.CloseBodyWithContext(ctx, resp.Response)

	logger.Info("Reindexing started successfully")
	return nil
}

// CancelIndexing cancels an ongoing indexing operation
func (c *Client) CancelIndexing(ctx context.Context) error {
	logger := loggingutil.Get(ctx)
	logger.Info("Canceling ongoing indexing operation")

	resp := httputil2.DeleteTyped[struct{}](ctx, c.httpClient, c.baseURL, "/api/v1/index/all")
	if resp.Error() != nil {
		logger.Error("Cancel indexing request failed", "error", resp.Error())
		return resp.Error()
	}
	defer httputil2.CloseBodyWithContext(ctx, resp.Response)

	logger.Info("Indexing cancellation request successful")
	return nil
}

// IndexFile indexes a specific file
func (c *Client) IndexFile(ctx context.Context, filePath string, force bool) error {
	logger := loggingutil.Get(ctx)
	logger.Debug("Indexing file", "path", filePath, "force", force)

	req := IndexFileRequest{
		FilePath: filePath,
		Force:    force,
	}

	resp := httputil2.PostTyped[struct{}](ctx, c.httpClient, c.baseURL, "/api/v1/index/file", req)
	if resp.Error() != nil {
		logger.Error("File indexing request failed", "error", resp.Error(), "path", filePath)
		return resp.Error()
	}
	defer httputil2.CloseBodyWithContext(ctx, resp.Response)

	logger.Debug("File indexing request successful", "path", filePath)
	return nil
}

// GetIndexingStatus gets the current status of the indexing process
func (c *Client) GetIndexingStatus(ctx context.Context) (*IndexingStatus, error) {
	logger := loggingutil.Get(ctx)
	logger.Debug("Requesting indexing status")

	status, err := httputil2.GetJSON[IndexingStatus](ctx, c.httpClient, c.baseURL, "/api/v1/index/status", nil)
	if err != nil {
		logger.Error("Failed to get indexing status", "error", err)
		return nil, err
	}

	logger.Debug("Got indexing status",
		"isIndexing", status.IsIndexing,
		"indexedDocs", status.IndexedDocs,
		"totalDocs", status.TotalDocs)
	return &status, nil
}
