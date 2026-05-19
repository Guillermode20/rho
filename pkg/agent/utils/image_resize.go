package agentutils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
)

// ResizeOptions configures image resizing.
type ResizeOptions struct {
	MaxWidth  int // Maximum width in pixels
	MaxHeight int // Maximum height in pixels
	MaxSize   int // Maximum dimension (preserves aspect ratio)
	Quality   int // JPEG quality (1-100), only used for JPEG output
}

// DefaultResizeOptions returns sensible defaults for image resizing.
func DefaultResizeOptions() ResizeOptions {
	return ResizeOptions{
		MaxWidth:  2000,
		MaxHeight: 2000,
		Quality:   85,
	}
}

// ImageResizer resizes images while preserving aspect ratio.
type ImageResizer struct{}

// Resize resizes image data to fit within the specified dimensions.
func (ir *ImageResizer) Resize(data []byte, opts ResizeOptions) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot decode image: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Calculate new dimensions
	newW, newH := ir.calculateDimensions(w, h, opts)

	// Skip if no resize needed
	if newW == w && newH == h {
		return data, nil
	}

	// Resize using bilinear interpolation
	resized := ir.bilinearResize(img, newW, newH)

	// Encode
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		if opts.Quality <= 0 {
			opts.Quality = 85
		}
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: opts.Quality})
	default:
		err = png.Encode(&buf, resized)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot encode resized image: %w", err)
	}

	return buf.Bytes(), nil
}

func (ir *ImageResizer) calculateDimensions(w, h int, opts ResizeOptions) (int, int) {
	if opts.MaxSize > 0 {
		if w > h && w > opts.MaxSize {
			ratio := float64(opts.MaxSize) / float64(w)
			return opts.MaxSize, int(math.Round(float64(h) * ratio))
		}
		if h > opts.MaxSize {
			ratio := float64(opts.MaxSize) / float64(h)
			return int(math.Round(float64(w) * ratio)), opts.MaxSize
		}
		return w, h
	}

	if opts.MaxWidth > 0 && w > opts.MaxWidth {
		ratio := float64(opts.MaxWidth) / float64(w)
		w = opts.MaxWidth
		h = int(math.Round(float64(h) * ratio))
	}
	if opts.MaxHeight > 0 && h > opts.MaxHeight {
		ratio := float64(opts.MaxHeight) / float64(h)
		h = opts.MaxHeight
		w = int(math.Round(float64(w) * ratio))
	}
	return w, h
}

// bilinearResize performs bilinear interpolation resize.
func (ir *ImageResizer) bilinearResize(src image.Image, dstW, dstH int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			// Map destination pixel to source coordinates
			sx := float64(dx) * float64(srcW) / float64(dstW)
			sy := float64(dy) * float64(srcH) / float64(dstH)

			// Get four neighboring pixels
			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			x1 := x0 + 1
			y1 := y0 + 1

			if x0 < 0 {
				x0 = 0
			}
			if y0 < 0 {
				y0 = 0
			}
			if x1 >= srcW {
				x1 = srcW - 1
			}
			if y1 >= srcH {
				y1 = srcH - 1
			}

			// Fractional parts
			xFrac := sx - float64(x0)
			yFrac := sy - float64(y0)

			// Bilinear interpolation
			r, g, b, a := ir.bilinearPixel(src, x0, y0, x1, y1, xFrac, yFrac)
			off := dst.PixOffset(dx, dy)
			dst.Pix[off+0] = r
			dst.Pix[off+1] = g
			dst.Pix[off+2] = b
			dst.Pix[off+3] = a
		}
	}

	return dst
}

func (ir *ImageResizer) bilinearPixel(src image.Image, x0, y0, x1, y1 int, xFrac, yFrac float64) (uint8, uint8, uint8, uint8) {
	// Get colors of four neighbors
	c00 := colorAt(src, x0, y0)
	c01 := colorAt(src, x0, y1)
	c10 := colorAt(src, x1, y0)
	c11 := colorAt(src, x1, y1)

	// Interpolate along X
	c0 := lerpColor(c00, c10, xFrac)
	c1 := lerpColor(c01, c11, xFrac)

	// Interpolate along Y
	result := lerpColor(c0, c1, yFrac)

	return result.R, result.G, result.B, result.A
}

type rgbaColor struct {
	R, G, B, A uint8
}

func colorAt(img image.Image, x, y int) rgbaColor {
	r, g, b, a := img.At(x, y).RGBA()
	return rgbaColor{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

func lerpColor(c1, c2 rgbaColor, t float64) rgbaColor {
	return rgbaColor{
		R: uint8(float64(c1.R) + t*(float64(c2.R)-float64(c1.R))),
		G: uint8(float64(c1.G) + t*(float64(c2.G)-float64(c1.G))),
		B: uint8(float64(c1.B) + t*(float64(c2.B)-float64(c1.B))),
		A: uint8(float64(c1.A) + t*(float64(c2.A)-float64(c1.A))),
	}
}

// ResizeImageFile resizes an image file in place.
func ResizeImageFile(path string, opts ResizeOptions) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	resizer := &ImageResizer{}
	resized, err := resizer.Resize(data, opts)
	if err != nil {
		return err
	}
	return os.WriteFile(path, resized, 0644)
}
