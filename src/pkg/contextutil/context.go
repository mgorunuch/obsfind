// Package contextutil provides utilities for working with context.Context objects,
// including type-safe storage and retrieval of values.
package contextutil

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// AppCtx is a specialized context wrapper for the ObsFind application.
// It provides type-safe access to context values and additional utilities
// for context management in the application.
type AppCtx struct {
	context.Context
}

// NewAppCtx creates a new AppCtx wrapper around an existing context.
// If ctx is nil, a background context will be used as the base.
func NewAppCtx(ctx context.Context) *AppCtx {
	if ctx == nil {
		ctx = context.Background()
	}
	return &AppCtx{Context: ctx}
}

// WithValue returns a new AppCtx with the given key-value pair added.
// This is a convenience wrapper around context.WithValue that maintains
// the AppCtx type.
func (c *AppCtx) WithValue(key, val interface{}) *AppCtx {
	return &AppCtx{Context: context.WithValue(c.Context, key, val)}
}

// WithCancel returns a new AppCtx and a cancel function.
// The returned context is a child of the receiver with cancellation capabilities.
func (c *AppCtx) WithCancel() (*AppCtx, context.CancelFunc) {
	ctx, cancel := context.WithCancel(c.Context)
	return &AppCtx{Context: ctx}, cancel
}

// WithDeadline returns a new AppCtx with the given deadline and a cancel function.
// The returned context is a child of the receiver with deadline and cancellation capabilities.
func (c *AppCtx) WithDeadline(deadline time.Time) (*AppCtx, context.CancelFunc) {
	ctx, cancel := context.WithDeadline(c.Context, deadline)
	return &AppCtx{Context: ctx}, cancel
}

// WithTimeout returns a new AppCtx with the given timeout and a cancel function.
// The returned context is a child of the receiver with timeout and cancellation capabilities.
func (c *AppCtx) WithTimeout(timeout time.Duration) (*AppCtx, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Context, timeout)
	return &AppCtx{Context: ctx}, cancel
}

// ctxKey is a private type used as a key for context values to avoid collisions.
type ctxKey[T any] struct{}

// SetTyped stores a typed value in the context and returns a new context with the value.
// This function provides type safety for context values using Go generics.
func SetTyped[T any](ctx context.Context, val T) context.Context {
	return context.WithValue(ctx, ctxKey[T]{}, val)
}

// RetrieveTyped retrieves a typed value from the context.
// It will panic if the value doesn't exist or is of the wrong type.
// Use TryRetrieveTyped if you want to handle missing values more gracefully.
func RetrieveTyped[T any](ctx context.Context) T {
	val, ok := ctx.Value(ctxKey[T]{}).(T)
	if !ok {
		typeName := reflect.TypeOf((*T)(nil)).Elem().String()
		panic(fmt.Sprintf("contextutil: value of type %s not found in context", typeName))
	}
	return val
}

// TryRetrieveTyped attempts to retrieve a typed value from the context.
// It returns the value and a boolean indicating whether the value was found
// and is of the correct type.
func TryRetrieveTyped[T any](ctx context.Context) (T, bool) {
	val, ok := ctx.Value(ctxKey[T]{}).(T)
	return val, ok
}

// MustAppCtx converts a regular context.Context to an AppCtx.
// If the context is already an AppCtx, it returns it directly.
// Otherwise, it wraps the context in a new AppCtx.
func MustAppCtx(ctx context.Context) *AppCtx {
	if appCtx, ok := ctx.(*AppCtx); ok {
		return appCtx
	}
	return NewAppCtx(ctx)
}

// Background returns a new AppCtx with context.Background() as the base.
func Background() *AppCtx {
	return NewAppCtx(context.Background())
}

// TODO returns a new AppCtx with context.TODO() as the base.
func TODO() *AppCtx {
	return NewAppCtx(context.TODO())
}
