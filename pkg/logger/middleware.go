package logger

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// RequestIDHeader is the HTTP header for request ID
	RequestIDHeader = "X-Request-ID"
)

// GinMiddleware is a Gin middleware that adds request ID to context
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get request ID from header
		requestID := c.GetHeader(RequestIDHeader)

		// If not present, generate a new one
		if requestID == "" {
			requestID = GenerateRequestID()
		}

		// Set response header
		c.Header(RequestIDHeader, requestID)

		// Create context with request ID
		ctx := WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)

		// Log request start
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		Info(ctx).
			Str("method", method).
			Str("path", path).
			Str("ip", c.ClientIP()).
			Msg("Request started")

		// Process request
		c.Next()

		// Log request end
		duration := time.Since(start)
		status := c.Writer.Status()

		Info(ctx).
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Dur("duration", duration).
			Msg("Request completed")
	}
}

// HTTPMiddleware is a standard HTTP middleware
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get request ID from header
		requestID := r.Header.Get(RequestIDHeader)

		// If not present, generate a new one
		if requestID == "" {
			requestID = GenerateRequestID()
		}

		// Set response header
		w.Header().Set(RequestIDHeader, requestID)

		// Create context with request ID
		ctx := WithRequestID(r.Context(), requestID)

		// Log request start
		start := time.Now()

		Info(ctx).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Msg("Request started")

		// Process request
		next.ServeHTTP(w, r.WithContext(ctx))

		// Log request end
		duration := time.Since(start)

		Info(ctx).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", duration).
			Msg("Request completed")
	})
}

// WebSocketContext creates a context with request ID for WebSocket connections
func WebSocketContext(r *http.Request) context.Context {
	// Try to get request ID from query parameter or header
	requestID := r.URL.Query().Get("request_id")
	if requestID == "" {
		requestID = r.Header.Get(RequestIDHeader)
	}
	if requestID == "" {
		requestID = GenerateRequestID()
	}

	return WithRequestID(context.Background(), requestID)
}
