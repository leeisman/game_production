package logger

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

var (
	// counter for generating sequential IDs
	counter uint64
)

// GenerateRequestID generates a unique request ID
// Format: timestamp-counter-random
// Example: 20231201102830-000001-a3f2b1
func GenerateRequestID() string {
	// Get current timestamp
	timestamp := time.Now().Format("20060102150405")

	// Get sequential counter
	count := atomic.AddUint64(&counter, 1)

	// Generate random bytes
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	return fmt.Sprintf("%s-%06d-%s", timestamp, count, randomHex)
}

// ShortRequestID generates a shorter request ID (for display)
// Format: counter-random
// Example: 000001-a3f2b1
func ShortRequestID() string {
	count := atomic.AddUint64(&counter, 1)

	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	return fmt.Sprintf("%06d-%s", count, randomHex)
}
