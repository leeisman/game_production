package tests

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/frankieli/game_product/pkg/logger"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// User model for testing
type TestUser struct {
	ID   uint
	Name string
}

func TestGormLoggingIntegration(t *testing.T) {
	// 1. Create a temporary log file
	tmpfile, err := ioutil.TempFile("", "integration_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// 2. Initialize our logger to write to this file
	logger.Init(logger.Config{
		Level:  "info",
		Format: "json",
		Output: tmpfile,
	})

	// 3. Initialize GORM with our logger adapter
	gormLog := logger.NewGormLogger()
	gormLog.LogLevel = gormlogger.Info

	// Use SQLite in-memory database for testing
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// 4. Perform DB operations
	// AutoMigrate
	if err := db.AutoMigrate(&TestUser{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create
	user := TestUser{Name: "Test User"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Query
	var result TestUser
	if err := db.First(&result, user.ID).Error; err != nil {
		t.Fatalf("Failed to find user: %v", err)
	}

	// Simulate Slow Query (if possible, or just rely on normal query logging)
	// SQLite might be too fast to trigger slow query easily without sleep,
	// but we are testing that *any* log is written.

	// 5. Read log file
	// Give a tiny bit of time just in case (though Async=false should be immediate)
	// Now we use SmartWriter by default, so we need to Flush or wait
	logger.Flush()

	content, err := ioutil.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	logOutput := string(content)

	t.Logf("Log Output:\n%s", logOutput)

	// 6. Verify logs
	// Check for Create SQL
	if !strings.Contains(logOutput, "INSERT INTO") {
		t.Errorf("Expected log to contain INSERT statement")
	}

	// Check for Select SQL
	if !strings.Contains(logOutput, "SELECT * FROM") {
		t.Errorf("Expected log to contain SELECT statement")
	}

	// Check for JSON format fields
	if !strings.Contains(logOutput, "\"rows\":") {
		t.Errorf("Expected log to contain 'rows' field")
	}
	if !strings.Contains(logOutput, "\"elapsed_ms\":") {
		t.Errorf("Expected log to contain 'elapsed_ms' field")
	}
}

// MockWriter captures writes for verification
type MockWriter struct {
	Buffer bytes.Buffer
}

func (m *MockWriter) Write(p []byte) (n int, err error) {
	return m.Buffer.Write(p)
}

func TestSmartWriter_ImmediateFlushOnError(t *testing.T) {
	mockOutput := &MockWriter{}
	// Create SmartWriter with a long flush interval to ensure auto-flush doesn't interfere
	sw := logger.NewSmartWriter(mockOutput, 10*time.Second)

	// 1. Write Info log (should be buffered)
	infoLog := []byte(`{"level":"info","message":"test info"}` + "\n")
	n, err := sw.Write(infoLog)
	assert.NoError(t, err)
	assert.Equal(t, len(infoLog), n)

	// Verify buffer is NOT flushed yet
	assert.Equal(t, 0, mockOutput.Buffer.Len(), "Info log should be buffered, not flushed immediately")

	// 2. Write Error log (should trigger immediate flush)
	errorLog := []byte(`{"level":"error","message":"test error"}` + "\n")
	n, err = sw.Write(errorLog)
	assert.NoError(t, err)
	assert.Equal(t, len(errorLog), n)

	// Verify buffer IS flushed (contains both logs)
	expectedOutput := string(infoLog) + string(errorLog)
	assert.Equal(t, expectedOutput, mockOutput.Buffer.String(), "Error log should trigger immediate flush of all buffered logs")
}

func TestSmartWriter_AutoFlush(t *testing.T) {
	mockOutput := &MockWriter{}
	// Create SmartWriter with a short flush interval
	sw := logger.NewSmartWriter(mockOutput, 100*time.Millisecond)

	// 1. Write Info log
	infoLog := []byte(`{"level":"info","message":"test info"}` + "\n")
	sw.Write(infoLog)

	// Verify buffer is NOT flushed immediately
	assert.Equal(t, 0, mockOutput.Buffer.Len())

	// 2. Wait for auto-flush
	time.Sleep(200 * time.Millisecond)

	// Verify buffer IS flushed
	assert.Equal(t, string(infoLog), mockOutput.Buffer.String(), "Auto-flush should write logs after interval")
}

func TestSmartWriter_ExplicitFlush(t *testing.T) {
	mockOutput := &MockWriter{}
	// Create SmartWriter with a long flush interval
	sw := logger.NewSmartWriter(mockOutput, 10*time.Second)

	// 1. Write Info log
	infoLog := []byte(`{"level":"info","message":"test info"}` + "\n")
	sw.Write(infoLog)

	// Verify buffer is NOT flushed immediately
	assert.Equal(t, 0, mockOutput.Buffer.Len())

	// 2. Call Sync (Flush)
	err := sw.Sync()
	assert.NoError(t, err)

	// Verify buffer IS flushed immediately
	assert.Equal(t, string(infoLog), mockOutput.Buffer.String(), "Explicit Sync() should flush buffer immediately")
}
