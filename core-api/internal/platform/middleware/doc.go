// Package middleware provides essential HTTP middleware handlers for the SwiftDrop Core API:
// request ID tracing (X-Request-ID propagation to correlation_id), panic recovery with sanitized
// error responses, structured privacy-safe access logging, and request body size limiting.
package middleware
