// Package httputil provides HTTP client and server utilities for the ObsFind application,
// including error handling utilities and response processing.
package httputil

import (
	"context"
	"io"
	"net/http"

	"obsfind/src/pkg/loggingutil"
)

// CloseBodyWithContext is a utility function to safely close an HTTP response body
// and properly handle any error that occurs during closing.
// It logs the error with appropriate context but does not return it.
// This function is designed to be used in defer statements after HTTP requests.
//
// Example usage:
//
//	resp, err := http.Get(url)
//	if err != nil {
//		return err
//	}
//	defer httputil.CloseBodyWithContext(ctx, resp)
func CloseBodyWithContext(ctx context.Context, resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	if err := resp.Body.Close(); err != nil {
		// We don't want to return the error from a deferred function
		// as it would mask the main function's return values.
		// Instead, we log it or handle it in a way that doesn't affect the caller.
		logger := loggingutil.Get(ctx)
		logger.Warn("Error closing response body", "error", err)
	}
}

// CloseBody is a utility function to safely close an HTTP response body
// without requiring a context. It uses a default logger for errors.
// This function is designed to be used in defer statements after HTTP requests.
//
// Example usage:
//
//	resp, err := http.Get(url)
//	if err != nil {
//		return err
//	}
//	defer httputil.CloseBody(resp)
func CloseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	if err := resp.Body.Close(); err != nil {
		// Use a background context with the default logger
		ctx := context.Background()
		logger := loggingutil.Get(ctx)
		logger.Warn("Error closing response body", "error", err)
	}
}

// LogCloseWithContext is similar to CloseBodyWithContext but works with any io.Closer interface.
// It handles closing the resource and logs any error that occurs.
// This function makes it easy to properly close any io.Closer resource
// in a defer statement without having to handle the error explicitly.
//
// Example usage:
//
//	file, err := os.Open(filename)
//	if err != nil {
//		return err
//	}
//	defer httputil.LogCloseWithContext(ctx, file, "file")
func LogCloseWithContext(ctx context.Context, closer io.Closer, resourceName string) {
	if closer == nil {
		return
	}

	if err := closer.Close(); err != nil {
		logger := loggingutil.Get(ctx)
		logger.Warn("Error closing resource", "resource", resourceName, "error", err)
	}
}

// LogClose is similar to CloseBody but works with any io.Closer interface.
// It handles closing the resource and logs any error that occurs.
// This function makes it easy to properly close any io.Closer resource
// in a defer statement without having to handle the error explicitly.
//
// Example usage:
//
//	file, err := os.Open(filename)
//	if err != nil {
//		return err
//	}
//	defer httputil.LogClose(file, "file")
func LogClose(closer io.Closer, resourceName string) {
	if closer == nil {
		return
	}

	if err := closer.Close(); err != nil {
		// Use a background context with the default logger
		ctx := context.Background()
		logger := loggingutil.Get(ctx)
		logger.Warn("Error closing resource", "resource", resourceName, "error", err)
	}
}
