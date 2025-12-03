package logger

import (
	"io"
	"os"
)

// LevelAsyncWriter is a writer that handles different log levels differently.
// It wraps an AsyncWriter for non-critical logs (Debug/Info) to prevent blocking,
// but writes critical logs (Warn/Error/Fatal) synchronously to ensure they are not lost.
type LevelAsyncWriter struct {
	asyncWriter *AsyncWriter
	syncWriter  io.Writer
}

// NewLevelAsyncWriter creates a new LevelAsyncWriter
func NewLevelAsyncWriter(w io.Writer, bufSize int) *LevelAsyncWriter {
	if w == nil {
		w = os.Stdout
	}
	return &LevelAsyncWriter{
		asyncWriter: NewAsyncWriter(w, bufSize),
		syncWriter:  w,
	}
}

// Write implements io.Writer.
// It parses the log level from the JSON message (zerolog default) and decides how to write.
// Note: This assumes JSON format where "level":"..." is present.
func (l *LevelAsyncWriter) Write(p []byte) (n int, err error) {
	// Fast path: check for "level":"error", "level":"warn", "level":"fatal"
	// This is a heuristic scan. Zerolog puts level at the beginning usually.
	// We scan the first 100 bytes to be safe and fast.
	limit := len(p)
	if limit > 100 {
		limit = 100
	}
	header := p[:limit]

	// Check for critical levels
	isCritical := false
	// We look for the level field value.
	// In JSON: "level":"error"
	// In Console: ERR, WRN, FTL (but we usually use this with JSON for file)

	// Simple bytes.Contains check is fast enough
	// We check for the specific JSON patterns zerolog uses
	if contains(header, []byte(`"level":"error"`)) ||
		contains(header, []byte(`"level":"warn"`)) ||
		contains(header, []byte(`"level":"fatal"`)) ||
		contains(header, []byte(`"level":"panic"`)) ||
		contains(header, []byte(`"level":"info"`)) { // Treat Info as critical per user request
		isCritical = true
	}

	if isCritical {
		// Critical logs: Write synchronously to ensure persistence
		return l.syncWriter.Write(p)
	}

	// Non-critical (Debug/Info): Write asynchronously (may drop if full)
	return l.asyncWriter.Write(p)
}

// Close closes the underlying async writer
func (l *LevelAsyncWriter) Close() error {
	return l.asyncWriter.Close()
}

// Helper for byte slice containment check
func contains(b, sub []byte) bool {
	for i := 0; i < len(b)-len(sub)+1; i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if b[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
