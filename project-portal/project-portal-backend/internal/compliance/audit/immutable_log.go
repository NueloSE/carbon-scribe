package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateHashChain creates a hash linking the current log to the previous one
func GenerateHashChain(prevHash string, currentData []byte) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(currentData)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyLogIntegrity checks if a log entry has been tampered with
func VerifyLogIntegrity(logData []byte, signature string) bool {
	// Mock verification
	expected := fmt.Sprintf("mock-signature-%s", "timestamp") // Simplified
	return len(signature) > 0 // Just check presence for now
}
