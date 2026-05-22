package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// ImageProtocol represents a terminal image protocol.
type ImageProtocol int

const (
	ImageProtocolKitty ImageProtocol = iota
	ImageProtocolITerm2
)

// ImageDimensions holds pixel dimensions.
type ImageDimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// CellDimensions holds terminal cell dimensions in pixels.
type CellDimensions struct {
	WidthPx  int `json:"widthPx"`
	HeightPx int `json:"heightPx"`
}

// TerminalCapabilities describes terminal image support.
type TerminalCapabilities struct {
	Images    bool `json:"images"`
	Kitty     bool `json:"kitty"`
	ITerm2    bool `json:"iterm2"`
	Hyperlink bool `json:"hyperlink"`
}

var (
	capabilities     *TerminalCapabilities
	capabilitiesMu   sync.Once
	cellDimensions   CellDimensions
	cellDimensionsMu sync.RWMutex
)

// GetCapabilities returns terminal image capabilities.
func GetCapabilities() *TerminalCapabilities {
	capabilitiesMu.Do(func() {
		capabilities = detectTerminalCapabilities()
	})
	return capabilities
}

// ResetCapabilitiesCache resets the cached capabilities.
func ResetCapabilitiesCache() {
	capabilities = nil
	capabilitiesMu = sync.Once{}
}

// SetCapabilities allows overriding detected capabilities.
func SetCapabilities(c *TerminalCapabilities) {
	capabilities = c
}

func detectTerminalCapabilities() *TerminalCapabilities {
	c := &TerminalCapabilities{}

	// Check TERM for image support
	term := os.Getenv("TERM")
	if strings.Contains(term, "kitty") {
		c.Images = true
		c.Kitty = true
	}
	if strings.Contains(term, "xterm-kitty") {
		c.Images = true
		c.Kitty = true
	}

	// Check iTerm2
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		c.Images = true
		c.ITerm2 = true
	}

	// Check COLORTERM
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		// Assume Kitty protocol support for modern terminals
		if !c.Kitty && !c.ITerm2 {
			c.Images = true
			c.Kitty = true
		}
	}

	// Hyperlinks
	c.Hyperlink = true

	return c
}

// GetCellDimensions returns terminal cell dimensions in pixels.
func GetCellDimensions() CellDimensions {
	cellDimensionsMu.RLock()
	defer cellDimensionsMu.RUnlock()
	return cellDimensions
}

// SetCellDimensions sets terminal cell dimensions from a query response.
func SetCellDimensions(d CellDimensions) {
	cellDimensionsMu.Lock()
	cellDimensions = d
	cellDimensionsMu.Unlock()
}

// CalculateImageRows calculates how many terminal rows an image occupies.
func CalculateImageRows(height, cellHeight int) int {
	if cellHeight <= 0 {
		cellHeight = 24 // default cell height in pixels
	}
	rows := height / cellHeight
	if height%cellHeight != 0 {
		rows++
	}
	return rows
}

// AllocateImageID returns a unique image ID for the Kitty protocol.
var imageIDCounter int32
var imageIDMu sync.Mutex

func AllocateImageID() int {
	imageIDMu.Lock()
	imageIDCounter++
	id := imageIDCounter
	imageIDMu.Unlock()
	return int(id)
}

// EncodeKitty encodes image data for Kitty terminal protocol.
// Format: \x1b_Ga=T,f=100,i=<id>[,m=1];<base64>\x07
func EncodeKitty(data []byte, width, height int, imageID int) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	m := 0 // 0 = last chunk, 1 = more chunks

	// Split into chunks if needed (max 4096 base64 chars per chunk for reliable transmission)
	chunkSize := 4 * 1024
	var result strings.Builder

	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
			m = 0
		} else {
			m = 1
		}

		chunk := encoded[i:end]
		result.WriteString(fmt.Sprintf("\x1b_Ga=T,f=100,i=%d,m=%d;%s\x07", imageID, m, chunk))

		if m == 0 {
			break
		}
	}

	return result.String()
}

// EncodeITerm2 encodes image data for iTerm2 protocol.
// Format: \x1b]1337;File=inline=1;size=<size>;preserveAspectRatio=1;base64,<data>\x07
func EncodeITerm2(data []byte, width, height int) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("\x1b]1337;File=inline=1;size=%d;preserveAspectRatio=1;width=%dpx;height=%dpx:%s\x07",
		len(data), width, height, encoded)
}

// RenderImage renders an image in the terminal using the best available protocol.
func RenderImage(data []byte, width, height int) string {
	caps := GetCapabilities()
	if caps.Kitty {
		id := AllocateImageID()
		return EncodeKitty(data, width, height, id)
	}
	if caps.ITerm2 {
		return EncodeITerm2(data, width, height)
	}
	return ""
}

// ImageFallback returns a text placeholder for terminals without image support.
func ImageFallback(path string, width, height int) string {
	return fmt.Sprintf("[Image: %s (%dx%d)]", path, width, height)
}

// DeleteKittyImage deletes a previously transmitted Kitty image.
// Format: \x1b_Ga=d,i=<id>\x07
func DeleteKittyImage(id int) string {
	return fmt.Sprintf("\x1b_Ga=d,i=%d\x07", id)
}

// DeleteAllKittyImages deletes all transmitted Kitty images.
func DeleteAllKittyImages(ids []int) string {
	var result strings.Builder
	for _, id := range ids {
		result.WriteString(DeleteKittyImage(id))
	}
	return result.String()
}

// Hyperlink returns an OSC 8 hyperlink escape sequence.
func Hyperlink(url, text string) string {
	return fmt.Sprintf("\x1b]8;;%s\x07%s\x1b]8;;\x07", url, text)
}

// GetImageDimensions returns dimensions of common image formats by reading headers.
func GetImageDimensions(data []byte) (int, int, bool) {
	// PNG
	if len(data) >= 24 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		w := int(data[16])<<24 | int(data[17])<<16 | int(data[18])<<8 | int(data[19])
		h := int(data[20])<<24 | int(data[21])<<16 | int(data[22])<<8 | int(data[23])
		return w, h, true
	}
	// JPEG
	if len(data) >= 4 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		pos := 2
		for pos+8 < len(data) {
			if data[pos] == 0xFF && (data[pos+1] >= 0xC0 && data[pos+1] <= 0xCF) && data[pos+1] != 0xC4 && data[pos+1] != 0xC8 {
				h := int(data[pos+5])<<8 | int(data[pos+6])
				w := int(data[pos+7])<<8 | int(data[pos+8])
				return w, h, true
			}
			pos += 2 + int(data[pos+2])<<8 + int(data[pos+3])
		}
	}
	// GIF
	if len(data) >= 10 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		w := int(data[6]) | int(data[7])<<8
		h := int(data[8]) | int(data[9])<<8
		return w, h, true
	}
	return 0, 0, false
}

// GetGifDimensions returns dimensions from a GIF header.
func GetGifDimensions(data []byte) (int, int, bool) {
	return GetImageDimensions(data)
}

// GetPngDimensions returns dimensions from a PNG header.
func GetPngDimensions(data []byte) (int, int, bool) {
	return GetImageDimensions(data)
}

// GetJpegDimensions returns dimensions from a JPEG header.
func GetJpegDimensions(data []byte) (int, int, bool) {
	return GetImageDimensions(data)
}

// GetWebpDimensions returns dimensions from a WebP header.
func GetWebpDimensions(data []byte) (int, int, bool) {
	if len(data) >= 30 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		// VP8X or VP8/VP8L
		if data[20] == 'V' && data[21] == 'P' {
			if data[22] == '8' && data[23] == ' ' {
				// VP8
				w := int(data[26]) | int(data[27])<<8
				h := int(data[28]) | int(data[29])<<8
				return w & 0x3fff, h & 0x3fff, true
			}
			if data[22] == '8' && data[23] == 'L' {
				w := int(data[24]) | int(data[25])<<8
				h := int(data[26]) | int(data[27])<<8
				return (w & 0x3fff) + 1, (h & 0x3fff) + 1, true
			}
			if data[22] == '8' && data[23] == 'X' && len(data) >= 30 {
				w := int(data[24]) | int(data[25])<<8 | int(data[26])<<16 | int(data[27])<<24
				h := int(data[28]) | int(data[29])<<8 | int(data[30])<<16 | int(data[31])<<24
				return (w & 0xffffff) + 1, (h & 0xffffff) + 1, true
			}
		}
	}
	return 0, 0, false
}

var _ = strconv.Itoa
