package util

import "strings"

// URL status constants
const (
	StatusActive = "active"
	StatusPaused = "paused"
)

// ReservedPaths are paths that should not be treated as short codes
var ReservedPaths = []string{"swagger", "api", "auth", "favicon.ico", "robots.txt"}

// NormalizeStatus normalizes a status string by trimming whitespace and converting to lowercase
func NormalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

// IsStatusPaused checks if a status is paused (case-insensitive)
func IsStatusPaused(status string) bool {
	return NormalizeStatus(status) == StatusPaused
}

// IsStatusActive checks if a status is active (case-insensitive)
func IsStatusActive(status string) bool {
	return NormalizeStatus(status) == StatusActive
}

// NormalizeEmail normalizes an email address by trimming whitespace and converting to lowercase
// This ensures consistent email comparison across the application
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

