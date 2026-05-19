package agentutils

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// EXIFOrientation represents the EXIF orientation flag.
type EXIFOrientation int

const (
	OrientationNormal     EXIFOrientation = 1
	OrientationFlipH      EXIFOrientation = 2
	OrientationRotate180  EXIFOrientation = 3
	OrientationFlipV      EXIFOrientation = 4
	OrientationRotate90   EXIFOrientation = 5
	OrientationRotate90FH EXIFOrientation = 6
	OrientationRotate270  EXIFOrientation = 7
	OrientationRotate90FV EXIFOrientation = 8
)

// ReadEXIFOrientation reads the EXIF orientation tag from JPEG/WebP/TIFF image bytes.
// Returns OrientationNormal (1) if no orientation tag is found.
func ReadEXIFOrientation(data []byte) EXIFOrientation {
	if len(data) < 4 {
		return OrientationNormal
	}

	// JPEG: starts with 0xFF 0xD8 0xFF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return readJPEGOrientation(data)
	}

	// TIFF (and some camera RAW): starts with II (0x4949) or MM (0x4D4D)
	if (data[0] == 0x49 && data[1] == 0x49) || (data[0] == 0x4D && data[1] == 0x4D) {
		return readTIFFOrientation(data)
	}

	return OrientationNormal
}

// OrientationDegrees returns the rotation in degrees clockwise.
func (o EXIFOrientation) Degrees() int {
	switch o {
	case OrientationRotate90, OrientationRotate90FH:
		return 90
	case OrientationRotate180, OrientationFlipH, OrientationFlipV:
		return 180
	case OrientationRotate270, OrientationRotate90FV:
		return 270
	default:
		return 0
	}
}

// NeedsFlip returns true if the image needs to be flipped.
func (o EXIFOrientation) NeedsFlip() (h, v bool) {
	switch o {
	case OrientationFlipH:
		return true, false
	case OrientationFlipV:
		return false, true
	case OrientationRotate90FH:
		return true, false
	case OrientationRotate90FV:
		return false, true
	}
	return false, false
}

// AutoRotateDimensions returns the display dimensions after applying orientation.
func (o EXIFOrientation) AutoRotateDimensions(w, h int) (int, int) {
	if o.Degrees() == 90 || o.Degrees() == 270 {
		return h, w
	}
	return w, h
}

func readJPEGOrientation(data []byte) EXIFOrientation {
	// Scan for APP1 (EXIF) marker: 0xFF 0xE1
	for i := 2; i < len(data)-8; {
		if data[i] != 0xFF {
			break
		}
		marker := data[i+1]
		if marker == 0xE1 {
			// APP1 - EXIF
			length := int(data[i+2])<<8 | int(data[i+3])
			if i+length > len(data) {
				break
			}
			// EXIF header: "Exif\0\0"
			if bytes.Equal(data[i+4:i+8], []byte("Exif")) {
				return readTIFFOrientation(data[i+12 : i+length])
			}
		}
		// Skip to next marker
		segLen := int(data[i+2])<<8 | int(data[i+3])
		if segLen < 2 {
			break
		}
		i += segLen + 2
	}
	return OrientationNormal
}

func readTIFFOrientation(data []byte) EXIFOrientation {
	if len(data) < 14 {
		return OrientationNormal
	}

	// Determine byte order
	bigEndian := data[0] == 0x4D && data[1] == 0x4D
	var bo binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		bo = binary.BigEndian
	}

	// Read IFD offset (offset 4, 4 bytes)
	ifdOffset := int(bo.Uint32(data[4:8]))
	if ifdOffset+2 > len(data) {
		return OrientationNormal
	}

	// Number of directory entries
	numEntries := int(bo.Uint16(data[ifdOffset : ifdOffset+2]))
	pos := ifdOffset + 2

	for i := 0; i < numEntries && pos+12 <= len(data); i++ {
		tag := bo.Uint16(data[pos : pos+2])
		// Tag 0x0112 = Orientation
		if tag == 0x0112 {
			// Orientation value is in the 12-byte IFD entry
			// Type (2 bytes), Count (4 bytes), Value/Offset (4 bytes)
			val := bo.Uint16(data[pos+8 : pos+10])
			return EXIFOrientation(val)
		}
		pos += 12
	}

	return OrientationNormal
}

// String returns a human-readable description of the orientation.
func (o EXIFOrientation) String() string {
	switch o {
	case OrientationNormal:
		return "Normal"
	case OrientationFlipH:
		return "Horizontal flip"
	case OrientationRotate180:
		return "Rotate 180°"
	case OrientationFlipV:
		return "Vertical flip"
	case OrientationRotate90:
		return "Rotate 90° CW + flip"
	case OrientationRotate90FH:
		return "Rotate 90° CW"
	case OrientationRotate270:
		return "Rotate 270° CW"
	case OrientationRotate90FV:
		return "Rotate 270° CW + flip"
	default:
		return fmt.Sprintf("Unknown (%d)", o)
	}
}
