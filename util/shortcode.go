package util

import (
	"crypto/rand"
	"math/big"
)

// GenerateShortCode generates a unique 6-character short code
func GenerateShortCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 6

	// Generate random short code
	shortCode := make([]byte, length)
	for i := range shortCode {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to simple random if crypto/rand fails
			shortCode[i] = charset[randomIndex.Int64()]
		} else {
			shortCode[i] = charset[randomIndex.Int64()]
		}
	}

	return string(shortCode)
}