package logger

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"time"
)

// SmartWriter implements a buffered writer similar to glog.
// It buffers logs in memory and flushes them to the underlying writer when:
// 1. The buffer is full.
// 2. A specific flush interval elapses (e.g., 1s).
// 3. A log with level "error" or "fatal" is written.
// 4. Sync() or Close() is called.
type SmartWriter struct {
	writer        io.Writer
	bufWriter     *bufio.Writer
	mu            sync.Mutex
	flushInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewSmartWriter creates a new SmartWriter
func NewSmartWriter(w io.Writer, flushInterval time.Duration) *SmartWriter {
	sw := &SmartWriter{
		writer:        w,
		bufWriter:     bufio.NewWriterSize(w, 256*1024), // 256KB buffer
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
	}

	// Start background flusher
	sw.wg.Add(1)
	go sw.runFlusher()

	return sw
}

// Write implements io.Writer
func (sw *SmartWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Check if this is an error or fatal log
	// Zerolog JSON format: "level":"error" or "level":"fatal"
	// We do a simple bytes check.
	isError := bytes.Contains(p, []byte("\"level\":\"error\"")) ||
		bytes.Contains(p, []byte("\"level\":\"fatal\""))

	n, err = sw.bufWriter.Write(p)

	// If error/fatal, or buffer is getting full (handled by bufio internally, but we can force flush),
	// we flush immediately.
	// bufio.Writer automatically flushes when buffer is full.
	// We only need to handle the explicit error flush here.
	if isError {
		_ = sw.bufWriter.Flush()
	}

	return n, err
}

// Sync flushes the buffer
func (sw *SmartWriter) Sync() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.bufWriter.Flush()
}

// Close flushes and stops the background flusher
func (sw *SmartWriter) Close() error {
	close(sw.stopChan)
	sw.wg.Wait()
	return sw.Sync()
}

func (sw *SmartWriter) runFlusher() {
	defer sw.wg.Done()
	ticker := time.NewTicker(sw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sw.Sync()
		case <-sw.stopChan:
			return
		}
	}
}
