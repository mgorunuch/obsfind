// Package httputil provides HTTP client and server utilities for the ObsFind application,
// including request/response handling, status checking, and JSON processing.
package httputil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"obsfind/src/pkg/consts"
	"obsfind/src/pkg/loggingutil"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes JSON response with the proper Content-Type header
func WriteJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error but can't really recover at this point
		logger := loggingutil.Get(context.Background())
		logger.Error("Error encoding JSON response", "error", err)
	}
}

// WriteError writes an error response with JSON formatting
func WriteError(w http.ResponseWriter, message string, statusCode int) {
	WriteJSON(w, ErrorResponse{Error: message}, statusCode)
}

// ParseQueryParameter parses a string query parameter from the request
func ParseQueryParameter(r *http.Request, paramName string) (string, bool) {
	value := r.URL.Query().Get(paramName)
	return value, value != ""
}

// ParseIntQueryParameter parses an integer query parameter from the request
func ParseIntQueryParameter(r *http.Request, paramName string, defaultValue int) (int, error) {
	valueStr := r.URL.Query().Get(paramName)
	if valueStr == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid %s parameter", paramName)
	}

	return value, nil
}

// ParseJSONRequest parses the request body into the given target type
func ParseJSONRequest(r *http.Request, target interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

// MethodChecker is a helper to check if the HTTP method is allowed
func MethodChecker(w http.ResponseWriter, r *http.Request, allowedMethods ...string) bool {
	for _, method := range allowedMethods {
		if r.Method == method {
			return true
		}
	}

	logger := loggingutil.Get(context.Background())
	logger.Warn("Method not allowed", "method", r.Method, "path", r.URL.Path)
	WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

// ParseSearchParameters parses common search parameters from a request
func ParseSearchParameters(r *http.Request) (query string, limit int, filter string, err error) {
	// Get query parameter
	query = r.URL.Query().Get(consts.QueryParamQuery)
	if query == "" {
		return "", 0, "", fmt.Errorf("missing query parameter")
	}

	// Parse limit
	limitStr := r.URL.Query().Get(consts.QueryParamLimit)
	limit = consts.DefaultSearchLimit // Default limit
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			return "", 0, "", fmt.Errorf("invalid limit parameter")
		}
	}

	// Get filter parameters
	pathPrefix := r.URL.Query().Get(consts.QueryParamPathPrefix)
	if pathPrefix != "" {
		filter = consts.FilterPrefixPath + pathPrefix
		return
	}

	// Check for tag filters (can be multiple)
	tags := r.URL.Query()[consts.QueryParamTag]
	if len(tags) > 0 {
		filter = consts.FilterPrefixTags + strings.Join(tags, ",")
		return
	}

	// Get generic filter if provided
	filter = r.URL.Query().Get(consts.QueryParamFilter)
	return
}
