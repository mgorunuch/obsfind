// Package httputil provides HTTP client and server utilities for the ObsFind application,
// including request/response handling, status checking, and JSON processing.
package httputil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"obsfind/src/pkg/loggingutil"
)

// Response wraps an HTTP response and provides methods for processing it
type Response struct {
	*http.Response
	err error
}

// Error returns any error that occurred during request execution
func (r *Response) Error() error {
	return r.err
}

// CheckStatus checks if the HTTP status is OK (200) and returns an error if not
func (r *Response) CheckStatus() *Response {
	if r.err != nil {
		return r
	}

	if r.StatusCode != http.StatusOK {
		r.err = fmt.Errorf("server returned error: %s", r.Status)
	}

	return r
}

// ParseJSON parses the response body into the given target type
func (r *Response) ParseJSON(target interface{}) error {
	if r.err != nil {
		return r.err
	}

	defer CloseBody(r.Response)

	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return nil
}

// Text returns the response body as a string
func (r *Response) Text() (string, error) {
	if r.err != nil {
		return "", r.err
	}

	defer CloseBody(r.Response)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(data), nil
}

// Bytes returns the response body as a byte slice
func (r *Response) Bytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}

	defer CloseBody(r.Response)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// HTTP request methods with fluent interface
// ==========================================

// HttpResponse is a generic response that includes the parsed data of type T.
// It wraps an HTTP response and provides methods for accessing the parsed data
// and any errors that occurred during request execution or parsing.
type HttpResponse[T any] struct {
	*http.Response
	err  error
	data T
}

// Error returns any error that occurred during request execution or response processing.
func (r *HttpResponse[T]) Error() error {
	return r.err
}

// Data returns the parsed data of type T and any error that occurred.
// This is the primary method to retrieve the typed result from an HTTP request.
func (r *HttpResponse[T]) Data() (T, error) {
	if r.err != nil {
		return r.data, r.err
	}
	return r.data, nil
}

// Get sends an HTTP GET request and returns a Response object.
// This function is for raw responses without automatic parsing.
// It handles URL construction with query parameters and error handling during request creation and execution.
func Get(ctx context.Context, client *http.Client, baseURL, path string, queryParams url.Values) *Response {
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Add query parameters if provided
	requestPath := path
	if queryParams != nil && len(queryParams) > 0 {
		requestPath = fmt.Sprintf("%s?%s", path, queryParams.Encode())
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+requestPath, nil)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+requestPath, "method", "GET")
		return &Response{err: fmt.Errorf("failed to create request: %w", err)}
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &Response{err: fmt.Errorf("failed to connect to server: %w", err)}
	}

	return &Response{Response: resp}
}

// GetTyped sends an HTTP GET request and parses the response into the specified type T.
// It handles URL construction, error handling, HTTP status code checking, and automatic JSON parsing.
// The generic type parameter T determines the type of the parsed response data.
func GetTyped[T any](ctx context.Context, client *http.Client, baseURL, path string, queryParams url.Values) *HttpResponse[T] {
	var result T
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Add query parameters if provided
	requestPath := path
	if queryParams != nil && len(queryParams) > 0 {
		requestPath = fmt.Sprintf("%s?%s", path, queryParams.Encode())
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+requestPath, nil)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+requestPath, "method", "GET")
		return &HttpResponse[T]{err: fmt.Errorf("failed to create request: %w", err), data: result}
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &HttpResponse[T]{err: fmt.Errorf("failed to connect to server: %w", err), data: result}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		defer CloseBodyWithContext(ctx, resp)
		logger.Warn("HTTP request failed with non-OK status",
			"status", resp.Status,
			"status_code", resp.StatusCode,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("server returned error: %s", resp.Status),
			data:     result,
		}
	}

	// Parse the response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		defer CloseBodyWithContext(ctx, resp)
		logger.Error("Failed to parse JSON response",
			"error", err,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("failed to parse JSON response: %w", err),
			data:     result,
		}
	}

	return &HttpResponse[T]{Response: resp, data: result}
}

// Post sends an HTTP POST request with a JSON body and returns a Response object.
// It automatically marshals the payload to JSON, sets appropriate headers,
// and handles error conditions during request creation and execution.
func Post(ctx context.Context, client *http.Client, baseURL, path string, payload interface{}) *Response {
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Marshal the payload to JSON
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Error("Failed to marshal JSON payload", "error", err)
			return &Response{err: fmt.Errorf("failed to marshal JSON payload: %w", err)}
		}
		body = bytes.NewBuffer(data)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, body)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", "POST")
		return &Response{err: fmt.Errorf("failed to create request: %w", err)}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &Response{err: fmt.Errorf("failed to connect to server: %w", err)}
	}

	return &Response{Response: resp}
}

// PostTyped sends an HTTP POST request with a JSON body and parses the response into the specified type T.
// It marshals the payload to JSON, sets appropriate headers, checks status codes,
// and automatically parses the JSON response into the specified type.
func PostTyped[T any](ctx context.Context, client *http.Client, baseURL, path string, payload interface{}) *HttpResponse[T] {
	var result T
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Marshal the payload to JSON
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Error("Failed to marshal JSON payload", "error", err)
			return &HttpResponse[T]{err: fmt.Errorf("failed to marshal JSON payload: %w", err), data: result}
		}
		body = bytes.NewBuffer(data)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, body)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", "POST")
		return &HttpResponse[T]{err: fmt.Errorf("failed to create request: %w", err), data: result}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &HttpResponse[T]{err: fmt.Errorf("failed to connect to server: %w", err), data: result}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		defer CloseBodyWithContext(ctx, resp)
		logger.Warn("HTTP request failed with non-OK status",
			"status", resp.Status,
			"status_code", resp.StatusCode,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("server returned error: %s", resp.Status),
			data:     result,
		}
	}

	// Parse the response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		defer CloseBodyWithContext(ctx, resp)
		logger.Error("Failed to parse JSON response",
			"error", err,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("failed to parse JSON response: %w", err),
			data:     result,
		}
	}

	return &HttpResponse[T]{Response: resp, data: result}
}

// Delete sends an HTTP DELETE request and returns a Response
func Delete(ctx context.Context, client *http.Client, baseURL, path string) *Response {
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+path, nil)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", "DELETE")
		return &Response{err: fmt.Errorf("failed to create request: %w", err)}
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &Response{err: fmt.Errorf("failed to connect to server: %w", err)}
	}

	return &Response{Response: resp}
}

// DeleteTyped sends an HTTP DELETE request and parses the response into the specified type
func DeleteTyped[T any](ctx context.Context, client *http.Client, baseURL, path string) *HttpResponse[T] {
	var result T
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+path, nil)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", "DELETE")
		return &HttpResponse[T]{err: fmt.Errorf("failed to create request: %w", err), data: result}
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &HttpResponse[T]{err: fmt.Errorf("failed to connect to server: %w", err), data: result}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		defer CloseBodyWithContext(ctx, resp)
		logger.Warn("HTTP request failed with non-OK status",
			"status", resp.Status,
			"status_code", resp.StatusCode,
			"url", req.URL.String(),
			"method", req.Method)

		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("server returned error: %s", resp.Status),
			data:     result,
		}
	}

	// Parse the response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		defer CloseBodyWithContext(ctx, resp)
		logger.Error("Failed to parse JSON response",
			"error", err,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("failed to parse JSON response: %w", err),
			data:     result,
		}
	}

	return &HttpResponse[T]{Response: resp, data: result}
}

// Put sends an HTTP PUT request with a JSON body and returns a Response
func Put(ctx context.Context, client *http.Client, baseURL, path string, payload interface{}) *Response {
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Marshal the payload to JSON
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Error("Failed to marshal JSON payload", "error", err)
			return &Response{err: fmt.Errorf("failed to marshal JSON payload: %w", err)}
		}
		body = bytes.NewBuffer(data)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+path, body)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", "PUT")
		return &Response{err: fmt.Errorf("failed to create request: %w", err)}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &Response{err: fmt.Errorf("failed to connect to server: %w", err)}
	}

	return &Response{Response: resp}
}

// PutTyped sends an HTTP PUT request with a JSON body and parses the response into the specified type
func PutTyped[T any](ctx context.Context, client *http.Client, baseURL, path string, payload interface{}) *HttpResponse[T] {
	var result T
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Marshal the payload to JSON
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Error("Failed to marshal JSON payload", "error", err)
			return &HttpResponse[T]{err: fmt.Errorf("failed to marshal JSON payload: %w", err), data: result}
		}
		body = bytes.NewBuffer(data)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+path, body)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", "PUT")
		return &HttpResponse[T]{err: fmt.Errorf("failed to create request: %w", err), data: result}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &HttpResponse[T]{err: fmt.Errorf("failed to connect to server: %w", err), data: result}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		defer CloseBodyWithContext(ctx, resp)
		logger.Warn("HTTP request failed with non-OK status",
			"status", resp.Status,
			"status_code", resp.StatusCode,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("server returned error: %s", resp.Status),
			data:     result,
		}
	}

	// Parse the response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		defer CloseBodyWithContext(ctx, resp)
		logger.Error("Failed to parse JSON response",
			"error", err,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("failed to parse JSON response: %w", err),
			data:     result,
		}
	}

	return &HttpResponse[T]{Response: resp, data: result}
}

// Request is a more flexible function that allows specifying custom headers and a request body
func Request(ctx context.Context, client *http.Client, method, baseURL, path string, body io.Reader, headers map[string]string) *Response {
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", method)
		return &Response{err: fmt.Errorf("failed to create request: %w", err)}
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &Response{err: fmt.Errorf("failed to connect to server: %w", err)}
	}

	return &Response{Response: resp}
}

// RequestTyped is a more flexible function that allows specifying custom headers and a request body
// and parses the response into the specified type
func RequestTyped[T any](ctx context.Context, client *http.Client, method, baseURL, path string, body io.Reader, headers map[string]string) *HttpResponse[T] {
	var result T
	logger := loggingutil.Get(ctx)

	if client == nil {
		client = http.DefaultClient
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err, "url", baseURL+path, "method", method)
		return &HttpResponse[T]{err: fmt.Errorf("failed to create request: %w", err), data: result}
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to connect to server", "error", err, "url", req.URL.String(), "method", req.Method)
		return &HttpResponse[T]{err: fmt.Errorf("failed to connect to server: %w", err), data: result}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		defer CloseBodyWithContext(ctx, resp)
		logger.Warn("HTTP request failed with non-OK status",
			"status", resp.Status,
			"status_code", resp.StatusCode,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("server returned error: %s", resp.Status),
			data:     result,
		}
	}

	// Parse the response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		defer CloseBodyWithContext(ctx, resp)
		logger.Error("Failed to parse JSON response",
			"error", err,
			"url", req.URL.String(),
			"method", req.Method)
		return &HttpResponse[T]{
			Response: resp,
			err:      fmt.Errorf("failed to parse JSON response: %w", err),
			data:     result,
		}
	}

	return &HttpResponse[T]{Response: resp, data: result}
}

// Complete helper functions
// =======================
// These functions provide simple, one-call operations for common HTTP operations
// with automatic JSON parsing and error handling.

// GetJSON makes a GET request and returns the parsed JSON response of type T.
// This is a convenience wrapper around GetTyped that directly returns the parsed data and error.
func GetJSON[T any](ctx context.Context, client *http.Client, baseURL, path string, queryParams url.Values) (T, error) {
	resp := GetTyped[T](ctx, client, baseURL, path, queryParams)
	return resp.Data()
}

// PostJSON makes a POST request with a JSON body and returns the parsed JSON response of type T.
// This is a convenience wrapper around PostTyped that directly returns the parsed data and error.
func PostJSON[T any](ctx context.Context, client *http.Client, baseURL, path string, payload interface{}) (T, error) {
	resp := PostTyped[T](ctx, client, baseURL, path, payload)
	return resp.Data()
}

// DeleteJSON makes a DELETE request and returns the parsed JSON response of type T.
// This is a convenience wrapper around DeleteTyped that directly returns the parsed data and error.
func DeleteJSON[T any](ctx context.Context, client *http.Client, baseURL, path string) (T, error) {
	resp := DeleteTyped[T](ctx, client, baseURL, path)
	return resp.Data()
}

// PutJSON makes a PUT request with a JSON body and returns the parsed JSON response of type T.
// This is a convenience wrapper around PutTyped that directly returns the parsed data and error.
func PutJSON[T any](ctx context.Context, client *http.Client, baseURL, path string, payload interface{}) (T, error) {
	resp := PutTyped[T](ctx, client, baseURL, path, payload)
	return resp.Data()
}

// Backwards compatibility functions
// ================================
// These functions maintain compatibility with previous code that uses the older API design.
// They provide the same functionality but with a different interface.

// DoRequest executes an HTTP request and handles common response processing.
// This is a backward compatibility function that delegates to the Request function.
// It allows specifying a custom HTTP client and provides the raw response without automatic parsing.
func DoRequest(ctx context.Context, httpClient *http.Client, baseURL, method, path string, body io.Reader, headers map[string]string, customClient *http.Client) (*http.Response, error) {
	// Use the provided client or default client
	client := httpClient
	if customClient != nil {
		client = customClient
	}

	resp := Request(ctx, client, method, baseURL, path, body, headers)
	if resp.err != nil {
		return nil, resp.err
	}

	return resp.Response, nil
}

// DoGet performs a GET request to the specified path
func DoGet(ctx context.Context, httpClient *http.Client, baseURL, path string, queryParams url.Values) (*http.Response, error) {
	resp := Get(ctx, httpClient, baseURL, path, queryParams)
	if resp.err != nil {
		return nil, resp.err
	}

	return resp.Response, nil
}

// DoPost performs a POST request with JSON body to the specified path
func DoPost(ctx context.Context, httpClient *http.Client, baseURL, path string, payload interface{}) (*http.Response, error) {
	resp := Post(ctx, httpClient, baseURL, path, payload)
	if resp.err != nil {
		return nil, resp.err
	}

	return resp.Response, nil
}

// DoDelete performs a DELETE request to the specified path
func DoDelete(ctx context.Context, httpClient *http.Client, baseURL, path string) (*http.Response, error) {
	resp := Delete(ctx, httpClient, baseURL, path)
	if resp.err != nil {
		return nil, resp.err
	}

	return resp.Response, nil
}

// DoPut performs a PUT request with JSON body to the specified path
func DoPut(ctx context.Context, httpClient *http.Client, baseURL, path string, payload interface{}) (*http.Response, error) {
	resp := Put(ctx, httpClient, baseURL, path, payload)
	if resp.err != nil {
		return nil, resp.err
	}

	return resp.Response, nil
}

// ParseJSONResponse parses a JSON response into the given target type T using generics.
// It handles HTTP status code checking and proper error context.
// This is a backward compatibility function that provides standalone JSON parsing functionality.
func ParseJSONResponse[T any](resp *http.Response) (T, error) {
	var result T

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("server returned error: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// HandleResponseError checks for HTTP response status errors and returns an appropriate error message.
// It returns nil if the response status code is 200 OK.
func HandleResponseError(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned error: %s", resp.Status)
	}
	return nil
}
