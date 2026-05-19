package agentutils

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// ClipboardImage provides image clipboard operations.
type ClipboardImage struct{}

// ImageClipboardFormat represents supported image clipboard formats.
type ImageClipboardFormat string

const (
	ClipboardPNG  ImageClipboardFormat = "image/png"
	ClipboardJPEG ImageClipboardFormat = "image/jpeg"
	ClipboardTIFF ImageClipboardFormat = "image/tiff"
	ClipboardBMP  ImageClipboardFormat = "image/bmp"
)

// EncodeForClipboard encodes image data for clipboard transfer using OSC 52 with image support.
// The data should be raw bytes of the image in the given format.
func (ci *ClipboardImage) EncodeForClipboard(data []byte, format ImageClipboardFormat) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
}

// EncodeForKittyClipboard encodes image data for the Kitty terminal's clipboard protocol.
func (ci *ClipboardImage) EncodeForKittyClipboard(data []byte, format ImageClipboardFormat) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	mimeType := string(format)
	// Kitty uses a= (action) parameter in the APC sequence
	return fmt.Sprintf("\x1b_Ga=c,f=%d,%d;%s\x07", len(mimeType), len(encoded), encoded)
}

// DecodeFromClipboard decodes base64 clipboard content back to image bytes.
func (ci *ClipboardImage) DecodeFromClipboard(encoded string) ([]byte, error) {
	// Strip OSC 52 wrapper if present
	encoded = strings.TrimPrefix(encoded, "\x1b]52;c;")
	encoded = strings.TrimSuffix(encoded, "\x07")
	encoded = strings.TrimSuffix(encoded, "\x1b\\")

	return base64.StdEncoding.DecodeString(encoded)
}

// WriteImageToFile writes image data to a temporary file and returns the path.
func WriteImageToFile(data []byte, ext string) (string, error) {
	file, err := os.CreateTemp("", "rho-img-*"+ext)
	if err != nil {
		return "", fmt.Errorf("cannot create temp file: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("cannot write image: %w", err)
	}
	return file.Name(), nil
}
