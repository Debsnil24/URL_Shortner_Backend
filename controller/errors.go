package controller

import "errors"

// Custom error types for better error handling
// These errors can be checked using errors.Is() instead of string comparison
var (
	ErrURLNotFound              = errors.New("URL not found")
	ErrPermissionDenied         = errors.New("permission denied")
	ErrInvalidStatus            = errors.New("status must be either 'active' or 'paused'")
	ErrExpiredLinkUpdate        = errors.New("cannot update URL of expired link. Update expiration to reactivate it")
	ErrNoFieldsProvided         = errors.New("at least one field (url, expiration_preset, or custom_expiration) must be provided")
	ErrBothExpirationProvided   = errors.New("cannot provide both expiration_preset and custom_expiration. Use only one")
	ErrInvalidQRCodeSize        = errors.New("invalid QR code size")
)

