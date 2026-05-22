package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessFileArguments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "attachments-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a text file
	textFilePath := filepath.Join(tempDir, "test.txt")
	textContent := "Hello, this is a test text file content."
	if err := os.WriteFile(textFilePath, []byte(textContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a mock PNG image (10x10 red pixel image)
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		t.Fatal(err)
	}
	imageFilePath := filepath.Join(tempDir, "test.png")
	if err := os.WriteFile(imageFilePath, imgBuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	// Test processing files
	fileArgs := []string{textFilePath, imageFilePath}
	opts := ProcessFileOptions{AutoResizeImages: true}
	processed, err := processFileArguments(fileArgs, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check text wrap for text file
	expectedTextPrefix := fmt.Sprintf("<file name=\"%s\">\n%s\n</file>\n", textFilePath, textContent)
	if !strings.Contains(processed.Text, expectedTextPrefix) {
		t.Errorf("expected text wrap for text file, got:\n%s", processed.Text)
	}

	// Check image attachment and reference
	if len(processed.Images) != 1 {
		t.Fatalf("expected 1 image attachment, got %d", len(processed.Images))
	}
	if processed.Images[0].MimeType != "image/png" {
		t.Errorf("expected MIME type image/png, got %s", processed.Images[0].MimeType)
	}
	if processed.Images[0].Data == "" {
		t.Error("expected base64 image data, got empty string")
	}

	imageRefTag := fmt.Sprintf("<file name=\"%s\"></file>", imageFilePath)
	if !strings.Contains(processed.Text, imageRefTag) {
		t.Errorf("expected image file reference %q in text, got:\n%s", imageRefTag, processed.Text)
	}
}
