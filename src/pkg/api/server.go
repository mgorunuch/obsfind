package api

import (
	"context"
	"fmt"
	"net/http"
	"obsfind/src/pkg/consts"
	"obsfind/src/pkg/contextutil"
	"obsfind/src/pkg/httputil"
	"obsfind/src/pkg/loggingutil"
	"strings"
	"time"
)

// Server represents the API server
type Server struct {
	addr    string
	router  *http.ServeMux
	server  *http.Server
	service *Service
}

// NewServer creates a new API server
func NewServer(addr string, service *Service) *Server {
	router := http.NewServeMux()

	return &Server{
		addr:    addr,
		router:  router,
		service: service,
	}
}

// Start begins listening for requests
func (s *Server) Start(ctx context.Context) error {
	logger := loggingutil.Get(ctx)

	// Set up routes
	s.setupRoutes()

	// Create HTTP server
	s.server = &http.Server{
		Addr:    s.addr,
		Handler: s.router,
	}

	// Start the server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		logger.Info("Starting API server", "addr", s.addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		// Graceful shutdown with timeout
		logger.Info("API server shutdown initiated")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errChan:
		logger.Error("API server error", "error", err)
		return err
	}
}

// Stop stops the server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// setupRoutes configures API endpoints
func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc(consts.APIHealthEndpoint, s.handleHealth)

	// Status endpoint
	s.router.HandleFunc(consts.APIStatusEndpoint, s.handleStatus)

	// Search endpoints
	s.router.HandleFunc(consts.APISearchQuery, s.handleSearchQuery)
	s.router.HandleFunc(consts.APISearchSimilar, s.handleSearchSimilar)

	// Index endpoints
	s.router.HandleFunc(consts.APIIndexFile, s.handleIndexFile)
	s.router.HandleFunc(consts.APIIndexAll, s.handleIndexAll)
	s.router.HandleFunc(consts.APIIndexStatus, s.handleIndexStatus)
}

// ErrorResponse represents an error response
// Kept for backward compatibility, use httputil.ErrorResponse instead
type ErrorResponse struct {
	Error string `json:"error"`
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Create a context with logger for this request
	ctx := contextutil.Background()
	logger := loggingutil.Get(ctx)

	if !httputil.MethodChecker(w, r, http.MethodGet) {
		return
	}

	logger.Debug("Health check request", "remote_addr", r.RemoteAddr)
	httputil.WriteJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

// handleStatus handles daemon status requests
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Create a context with logger for this request
	ctx := contextutil.Background()
	logger := loggingutil.Get(ctx)

	if !httputil.MethodChecker(w, r, http.MethodGet) {
		return
	}

	logger.Debug("Status request", "remote_addr", r.RemoteAddr)

	status, err := s.service.GetStatus()
	if err != nil {
		logger.Error("Failed to get status", "error", err)
		httputil.WriteError(w, fmt.Sprintf("Failed to get status: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Debug("Status request completed successfully")
	httputil.WriteJSON(w, status, http.StatusOK)
}

// handleSearchQuery handles search query requests
func (s *Server) handleSearchQuery(w http.ResponseWriter, r *http.Request) {
	// Create a context with logger for this request
	ctx := r.Context()
	ctx = contextutil.Background() // Use our own context
	logger := loggingutil.Get(ctx)

	// Accept both GET and POST methods for flexibility
	if !httputil.MethodChecker(w, r, http.MethodGet, http.MethodPost) {
		return
	}

	if r.Method == http.MethodGet {
		// Handle GET request with URL parameters
		query, limit, filter, err := httputil.ParseSearchParameters(r)
		if err != nil {
			logger.Warn("Invalid search parameters", "error", err, "remote_addr", r.RemoteAddr)
			httputil.WriteError(w, err.Error(), http.StatusBadRequest)
			return
		}

		logger.Debug("GET search request",
			"query", query,
			"limit", limit,
			"filter", filter,
			"remote_addr", r.RemoteAddr)

		// Execute search
		results, err := s.service.Search(ctx, query, limit, filter)
		if err != nil {
			logger.Error("Search failed", "error", err, "query", query)
			httputil.WriteError(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
			return
		}

		logger.Debug("Search completed successfully", "query", query, "resultCount", len(results))
		httputil.WriteJSON(w, results, http.StatusOK)
		return
	} else if r.Method == http.MethodPost {
		// Parse request body for POST
		var request struct {
			Query      string   `json:"query"`
			Limit      int      `json:"limit,omitempty"`
			Offset     int      `json:"offset,omitempty"`
			MinScore   float32  `json:"min_score,omitempty"`
			Tags       []string `json:"tags,omitempty"`
			PathPrefix string   `json:"path_prefix,omitempty"`
		}

		if err := httputil.ParseJSONRequest(r, &request); err != nil {
			logger.Warn("Invalid request body", "error", err, "remote_addr", r.RemoteAddr)
			httputil.WriteError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if request.Query == "" {
			logger.Warn("Missing query parameter in search request", "remote_addr", r.RemoteAddr)
			httputil.WriteError(w, "Missing query parameter", http.StatusBadRequest)
			return
		}

		if request.Limit == 0 {
			request.Limit = consts.DefaultSearchLimit // Default limit
		}

		// Build the filter string from the POST data
		var filter string
		if request.PathPrefix != "" {
			filter = consts.FilterPrefixPath + request.PathPrefix
		} else if len(request.Tags) > 0 {
			filter = consts.FilterPrefixTags + strings.Join(request.Tags, ",")
		}

		logger.Debug("POST search request",
			"query", request.Query,
			"limit", request.Limit,
			"filter", filter,
			"remote_addr", r.RemoteAddr)

		// Execute search
		results, err := s.service.Search(ctx, request.Query, request.Limit, filter)
		if err != nil {
			logger.Error("Search failed", "error", err, "query", request.Query)
			httputil.WriteError(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
			return
		}

		logger.Debug("Search completed successfully",
			"query", request.Query,
			"resultCount", len(results))
		httputil.WriteJSON(w, results, http.StatusOK)
		return
	}
}

// handleSearchSimilar handles similar document search requests
func (s *Server) handleSearchSimilar(w http.ResponseWriter, r *http.Request) {
	// Create a context with logger for this request
	ctx := contextutil.Background()
	logger := loggingutil.Get(ctx)

	if !httputil.MethodChecker(w, r, http.MethodPost) {
		return
	}

	// Parse request
	var request struct {
		FilePath string `json:"file_path"`
		Limit    int    `json:"limit,omitempty"`
	}

	if err := httputil.ParseJSONRequest(r, &request); err != nil {
		logger.Warn("Invalid request body", "error", err, "remote_addr", r.RemoteAddr)
		httputil.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.FilePath == "" {
		logger.Warn("Missing file_path parameter", "remote_addr", r.RemoteAddr)
		httputil.WriteError(w, "Missing file_path parameter", http.StatusBadRequest)
		return
	}

	if request.Limit == 0 {
		request.Limit = consts.DefaultSearchLimit // Default limit
	}

	logger.Debug("Similar search request",
		"path", request.FilePath,
		"limit", request.Limit,
		"remote_addr", r.RemoteAddr)

	// Execute search
	results, err := s.service.FindSimilar(ctx, request.FilePath, request.Limit)
	if err != nil {
		logger.Error("Similar search failed", "error", err, "path", request.FilePath)
		httputil.WriteError(w, fmt.Sprintf("Similar search failed: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Debug("Similar search completed successfully",
		"path", request.FilePath,
		"resultCount", len(results))
	httputil.WriteJSON(w, results, http.StatusOK)
}

// handleIndexFile handles file indexing requests
func (s *Server) handleIndexFile(w http.ResponseWriter, r *http.Request) {
	// Create a context with logger for this request
	ctx := contextutil.Background()
	logger := loggingutil.Get(ctx)

	if !httputil.MethodChecker(w, r, http.MethodPost) {
		return
	}

	// Parse request
	var request struct {
		FilePath string `json:"file_path"`
		Force    bool   `json:"force,omitempty"`
	}

	if err := httputil.ParseJSONRequest(r, &request); err != nil {
		logger.Warn("Invalid request body", "error", err, "remote_addr", r.RemoteAddr)
		httputil.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.FilePath == "" {
		logger.Warn("Missing file_path parameter", "remote_addr", r.RemoteAddr)
		httputil.WriteError(w, "Missing file_path parameter", http.StatusBadRequest)
		return
	}

	logger.Debug("Index file request",
		"path", request.FilePath,
		"force", request.Force,
		"remote_addr", r.RemoteAddr)

	// Execute indexing
	err := s.service.IndexFile(ctx, request.FilePath, request.Force)
	if err != nil {
		logger.Error("Indexing failed", "error", err, "path", request.FilePath)
		httputil.WriteError(w, fmt.Sprintf("Indexing failed: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Debug("File indexed successfully", "path", request.FilePath)
	httputil.WriteJSON(w, map[string]string{"status": "success"}, http.StatusOK)
}

// handleIndexAll handles full reindexing requests
func (s *Server) handleIndexAll(w http.ResponseWriter, r *http.Request) {
	// Create a context with logger for this request
	ctx := contextutil.Background()
	logger := loggingutil.Get(ctx)

	if !httputil.MethodChecker(w, r, http.MethodPost) {
		return
	}

	// Parse request
	var request struct {
		Force bool `json:"force,omitempty"`
	}

	if err := httputil.ParseJSONRequest(r, &request); err != nil {
		logger.Warn("Invalid request body", "error", err, "remote_addr", r.RemoteAddr)
		httputil.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	logger.Info("Reindex all request", "force", request.Force, "remote_addr", r.RemoteAddr)

	// Execute reindexing
	err := s.service.ReindexAll(ctx, request.Force)
	if err != nil {
		logger.Error("Reindexing failed", "error", err)
		httputil.WriteError(w, fmt.Sprintf("Reindexing failed: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Info("Reindexing started successfully")
	httputil.WriteJSON(w, map[string]string{"status": "reindexing_started"}, http.StatusOK)
}

// handleIndexStatus handles indexing status requests
func (s *Server) handleIndexStatus(w http.ResponseWriter, r *http.Request) {
	// Create a context with logger for this request
	ctx := contextutil.Background()
	logger := loggingutil.Get(ctx)

	if !httputil.MethodChecker(w, r, http.MethodGet) {
		return
	}

	logger.Debug("Index status request", "remote_addr", r.RemoteAddr)

	status, err := s.service.GetIndexingStatus(ctx)
	if err != nil {
		logger.Error("Failed to get indexing status", "error", err)
		httputil.WriteError(w, fmt.Sprintf("Failed to get indexing status: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Debug("Got indexing status successfully",
		"isIndexing", status.IsIndexing,
		"indexedDocs", status.IndexedDocs)
	httputil.WriteJSON(w, status, http.StatusOK)
}
