package tests

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/frankieli/game_product/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// This test runs a subprocess that initializes the logger and then panics.
// It verifies that the buffered logs are flushed to the file even after a panic.
func TestLoggerFlushOnPanic(t *testing.T) {
	if os.Getenv("RUN_PANIC_TEST") == "1" {
		doLoggerWork()
		return
	}

	// Compile and run the test binary itself, but with the environment variable set
	cmd := exec.Command(os.Args[0], "-test.run=TestLoggerFlushOnPanic")
	cmd.Env = append(os.Environ(), "RUN_PANIC_TEST=1")

	// We expect the process to fail (panic)
	// We ignore the error because we expect a non-zero exit code due to panic
	_ = cmd.Run()

	// Verify log file content
	logContent, err := os.ReadFile("panic_test.log")
	if err != nil {
		// If file doesn't exist, it means flush failed completely
		t.Fatalf("Failed to read log file: %v", err)
	}

	content := string(logContent)
	assert.Contains(t, content, "This message should be flushed before panic", "Buffered log was not flushed on panic")

	// Cleanup
	os.Remove("panic_test.log")
}

func doLoggerWork() {
	// Initialize logger with a file
	logger.InitWithFile("panic_test.log", "info", "json")

	// Ensure Flush is called on panic via defer
	defer logger.Flush()

	// Write a buffered log
	logger.InfoGlobal().Msg("This message should be flushed before panic")

	// Sleep a bit to ensure it's buffered (though SmartWriter buffers immediately)
	// We want to make sure it DOESN'T auto-flush (default is 1s), so we panic quickly.
	time.Sleep(10 * time.Millisecond)

	// Panic!
	panic("Intentional panic for testing")
}
