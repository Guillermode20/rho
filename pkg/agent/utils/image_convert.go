package agentutils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// SupportedImageFormat represents an image format.
type SupportedImageFormat string

const (
	FormatPNG  SupportedImageFormat = "png"
	FormatJPEG SupportedImageFormat = "jpeg"
	FormatGIF  SupportedImageFormat = "gif"
	FormatWebP SupportedImageFormat = "webp"
	FormatBMP  SupportedImageFormat = "bmp"
)

// ImageConverter converts between image formats.
type ImageConverter struct{}

// Convert converts image data from one format to another.
func (ic *ImageConverter) Convert(data []byte, fromFormat, toFormat SupportedImageFormat) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s image: %w", fromFormat, err)
	}

	switch toFormat {
	case FormatPNG:
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("cannot encode PNG: %w", err)
		}
		return buf.Bytes(), nil

	case FormatJPEG:
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, fmt.Errorf("cannot encode JPEG: %w", err)
		}
		return buf.Bytes(), nil

	default:
		return nil, fmt.Errorf("unsupported target format: %s", toFormat)
	}
}

// CompressJPEG compresses a JPEG image to the target quality.
func (ic *ImageConverter) CompressJPEG(data []byte, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot decode image: %w", err)
	}
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("cannot encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// CompressPNG compresses a PNG image using best compression.
func (ic *ImageConverter) CompressPNG(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot decode image: %w", err)
	}
	var buf bytes.Buffer
	enc := &png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("cannot encode PNG: %w", err)
	}
	return buf.Bytes(), nil
}

// DetectFormat detects the image format from file extension or magic bytes.
func DetectFormat(data []byte, path string) SupportedImageFormat {
	if path != "" {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".png":
			return FormatPNG
		case ".jpg", ".jpeg":
			return FormatJPEG
		case ".gif":
			return FormatGIF
		case ".webp":
			return FormatWebP
		case ".bmp":
			return FormatBMP
		}
	}
	if len(data) < 8 {
		return ""
	}
	// Magic bytes detection
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return FormatPNG
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return FormatJPEG
	}
	if len(data) >= 6 && string(data[:6]) == "GIF89a" || string(data[:6]) == "GIF87a" {
		return FormatGIF
	}
	if len(data) >= 4 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return FormatWebP
	}
	if len(data) >= 2 && string(data[:2]) == "BM" {
		return FormatBMP
	}
	return ""
}

// ConvertImageFile converts an image file from one format to another.
func ConvertImageFile(srcPath, dstPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	fromFormat := DetectFormat(data, srcPath)
	toFormat := DetectFormat(nil, dstPath)
	converter := &ImageConverter{}
	converted, err := converter.Convert(data, fromFormat, toFormat)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, converted, 0644)
}
