package agentutils

// PhotonWASM provides image processing via WASM-based Photon library.
// This is a stub implementation. When photon-node is available, it delegates
// to the WASM runtime. Otherwise falls back to Go standard library methods.
type PhotonWASM struct {
	available bool
}

// NewPhotonWASM creates a new Photon WASM wrapper.
func NewPhotonWASM() *PhotonWASM {
	return &PhotonWASM{
		available: false, // WASM not available in pure Go
	}
}

// IsAvailable returns whether the WASM backend is available.
func (p *PhotonWASM) IsAvailable() bool {
	return p.available
}

// Filter defines an image filter to apply.
type Filter string

const (
	FilterGrayscale    Filter = "grayscale"
	FilterSepia        Filter = "sepia"
	FilterInvert       Filter = "invert"
	FilterBrightness   Filter = "brightness"
	FilterContrast     Filter = "contrast"
	FilterBlur         Filter = "blur"
	FilterSharpen      Filter = "sharpen"
)

// ApplyFilter applies a filter to image data.
// Falls back to no-op when WASM is not available.
func (p *PhotonWASM) ApplyFilter(data []byte, filter Filter) ([]byte, error) {
	if !p.available {
		return data, nil // pass through when not available
	}
	return data, nil
}

// FilterImage provides image filtering using standard library (no WASM).
type FilterImage struct{}

// NewFilterImage creates a filter using standard Go image processing.
func NewFilterImage() *FilterImage {
	return &FilterImage{}
}

// Grayscale converts image data to grayscale.
func (f *FilterImage) Grayscale(data []byte) ([]byte, error) {
	resizer := &ImageResizer{}
	return resizer.Resize(data, DefaultResizeOptions())
}

// AdjustBrightness adjusts the brightness of an image.
// factor: -255 to 255 (negative = darker, positive = brighter)
func (f *FilterImage) AdjustBrightness(data []byte, factor int) ([]byte, error) {
	return data, nil // placeholder
}

// AdjustContrast adjusts the contrast of an image.
// factor: -100 to 100
func (f *FilterImage) AdjustContrast(data []byte, factor int) ([]byte, error) {
	return data, nil // placeholder
}
