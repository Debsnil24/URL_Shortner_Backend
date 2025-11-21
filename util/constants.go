package util

// URL status constants
const (
	StatusActive = "active"
	StatusPaused = "paused"
)

// ReservedPaths are paths that should not be treated as short codes
var ReservedPaths = []string{"swagger", "api", "auth", "favicon.ico", "robots.txt"}

