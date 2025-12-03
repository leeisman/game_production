package logger

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"
)

// AsyncWriter is a non-blocking writer that drops messages if the buffer is full
type AsyncWriter struct {
	w       io.Writer
	ch      chan []byte
	done    chan struct{}
	dropped uint64
}

// NewAsyncWriter creates a new AsyncWriter
func NewAsyncWriter(w io.Writer, bufSize int) *AsyncWriter {
	if w == nil {
		w = os.Stdout
	}
	aw := &AsyncWriter{
		w:    w,
		ch:   make(chan []byte, bufSize),
		done: make(chan struct{}),
	}
	go aw.run()
	return aw
}

// Write implements io.Writer
func (a *AsyncWriter) Write(p []byte) (n int, err error) {
	// We must copy p because zerolog reuses the buffer
	pCopy := make([]byte, len(p))
	copy(pCopy, p)

	select {
	case a.ch <- pCopy:
		return len(p), nil
	default:
		// Buffer full, drop message
		atomic.AddUint64(&a.dropped, 1)
		// Return success to avoid zerolog internal error logging
		return len(p), nil
	}
}

// run is the background worker
func (a *AsyncWriter) run() {
	defer close(a.done)
	for p := range a.ch {
		a.w.Write(p)
	}
}

// Close closes the channel and waits for the worker to finish
func (a *AsyncWriter) Close() error {
	close(a.ch)
	<-a.done

	if dropped := atomic.LoadUint64(&a.dropped); dropped > 0 {
		fmt.Fprintf(a.w, "AsyncWriter: dropped %d messages due to buffer overflow\n", dropped)
	}
	return nil
}
