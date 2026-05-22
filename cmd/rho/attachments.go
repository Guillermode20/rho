package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/earendil-works/rho/pkg/agent/utils"
	"github.com/earendil-works/rho/pkg/ai"
)

// ProcessedFiles represents the processed text content and image attachments.
type ProcessedFiles struct {
	Text   string
	Images []ai.ImageContent
}

// ProcessFileOptions configures file processing.
type ProcessFileOptions struct {
	AutoResizeImages bool
}

// processFileArguments processes @file CLI arguments into text content and image attachments.
func processFileArguments(fileArgs []string, options ProcessFileOptions) (ProcessedFiles, error) {
	autoResizeImages := options.AutoResizeImages
	var textBuilder strings.Builder
	var images []ai.ImageContent
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	for _, fileArg := range fileArgs {
		// Expand and resolve path (handles ~ expansion and resolves path)
		expanded := agentutils.ExpandTildePath(fileArg)
		absolutePath := agentutils.ResolvePath(expanded, cwd)

		// Check if file exists
		fileInfo, err := os.Stat(absolutePath)
		if err != nil {
			return ProcessedFiles{}, fmt.Errorf("file not found: %s", absolutePath)
		}

		if fileInfo.IsDir() {
			return ProcessedFiles{}, fmt.Errorf("path is a directory: %s", absolutePath)
		}

		// Check if empty
		if fileInfo.Size() == 0 {
			continue
		}

		// Read file
		content, err := os.ReadFile(absolutePath)
		if err != nil {
			return ProcessedFiles{}, fmt.Errorf("could not read file %s: %w", absolutePath, err)
		}

		mimeType := agentutils.DetectMIMEType(absolutePath)

		if isSupportedImageMimeType(mimeType) {
			// Handle image file
			base64Content := base64.StdEncoding.EncodeToString(content)

			// Decode original dimensions
			config, _, err := image.DecodeConfig(bytes.NewReader(content))
			if err != nil {
				// If decode config fails, try to treat as text or skip, but let's report error as in pi
				return ProcessedFiles{}, fmt.Errorf("could not decode image config for %s: %w", absolutePath, err)
			}

			var attachment ai.ImageContent
			var dimensionNote string

			if autoResizeImages {
				resizer := &agentutils.ImageResizer{}
				opts := agentutils.DefaultResizeOptions()
				resizedBytes, err := resizer.Resize(content, opts)
				if err != nil {
					textBuilder.WriteString(fmt.Sprintf("<file name=\"%s\">[Image omitted: could not be resized below the inline image size limit.]</file>\n", absolutePath))
					continue
				}

				resizedConfig, _, err := image.DecodeConfig(bytes.NewReader(resizedBytes))
				if err != nil {
					// Fallback
					resizedConfig = config
				}

				base64Resized := base64.StdEncoding.EncodeToString(resizedBytes)
				scale := float64(config.Width) / float64(resizedConfig.Width)

				if config.Width > resizedConfig.Width || config.Height > resizedConfig.Height {
					dimensionNote = fmt.Sprintf("[Image: original %dx%d, displayed at %dx%d. Multiply coordinates by %.2f to map to original image.]",
						config.Width, config.Height, resizedConfig.Width, resizedConfig.Height, scale)
				}

				// The resizer encodes to PNG unless original was JPEG
				targetMime := "image/png"
				if strings.HasSuffix(mimeType, "jpeg") || strings.HasSuffix(mimeType, "jpg") {
					targetMime = "image/jpeg"
				}

				attachment = ai.ImageContent{
					Type:     "image",
					MimeType: targetMime,
					Data:     base64Resized,
				}
			} else {
				attachment = ai.ImageContent{
					Type:     "image",
					MimeType: mimeType,
					Data:     base64Content,
				}
			}

			images = append(images, attachment)

			// Add text reference to image with optional dimension note
			if dimensionNote != "" {
				textBuilder.WriteString(fmt.Sprintf("<file name=\"%s\">%s</file>\n", absolutePath, dimensionNote))
			} else {
				textBuilder.WriteString(fmt.Sprintf("<file name=\"%s\"></file>\n", absolutePath))
			}
		} else {
			// Handle text file
			textBuilder.WriteString(fmt.Sprintf("<file name=\"%s\">\n%s\n</file>\n", absolutePath, string(content)))
		}
	}

	return ProcessedFiles{
		Text:   textBuilder.String(),
		Images: images,
	}, nil
}

func isSupportedImageMimeType(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	}
	return false
}
