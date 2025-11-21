package util

import (
	"errors"
	"fmt"

	"github.com/skip2/go-qrcode"
)

// ValidQRCodeSizes defines the allowed QR code sizes in pixels
var ValidQRCodeSizes = []int{256, 512, 1024}

// ValidQRCodeSizesMap provides O(1) lookup for valid QR code sizes
// Used for optimized validation instead of linear search
var ValidQRCodeSizesMap = map[int]bool{
	256:  true,
	512:  true,
	1024: true,
}

// DefaultQRCodeSize is the default size for QR codes
const DefaultQRCodeSize = 256

// ValidateQRCodeSize validates that the provided size is one of the allowed values
// Optimized to use map lookup (O(1)) instead of linear search (O(n))
func ValidateQRCodeSize(size int) error {
	if !ValidQRCodeSizesMap[size] {
		return fmt.Errorf("invalid QR code size: %d. Allowed sizes: %v", size, ValidQRCodeSizes)
	}
	return nil
}

// GenerateQRCode generates a QR code image for the given URL
// Returns PNG image bytes and any error encountered
func GenerateQRCode(url string, size int) ([]byte, error) {
	// Validate size
	if err := ValidateQRCodeSize(size); err != nil {
		return nil, err
	}

	// Validate URL is not empty
	if url == "" {
		return nil, errors.New("URL cannot be empty")
	}

	// Generate QR code with medium error recovery level
	// Error recovery levels: Low, Medium, High, Highest
	// Medium provides good balance between data capacity and error correction
	qrCode, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR code: %w", err)
	}

	// Generate PNG image bytes with the specified size
	// qrCode.PNG() generates a PNG image with default size (256x256)
	// We need to scale it to the requested size
	pngBytes, err := qrCode.PNG(size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code PNG: %w", err)
	}

	return pngBytes, nil
}

