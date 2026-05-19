package agentutils

import (
	"path/filepath"
	"strings"
)

// MIMEType maps file extensions to MIME types.
var extensionMIMEs = map[string]string{
	".txt":   "text/plain",
	".md":    "text/markdown",
	".html":  "text/html",
	".htm":   "text/html",
	".css":   "text/css",
	".js":    "application/javascript",
	".mjs":   "application/javascript",
	".ts":    "application/typescript",
	".tsx":   "application/typescript",
	".jsx":   "application/javascript",
	".json":  "application/json",
	".xml":   "application/xml",
	".yaml":  "application/x-yaml",
	".yml":   "application/x-yaml",
	".csv":   "text/csv",
	".go":    "text/x-go",
	".py":    "text/x-python",
	".rb":    "text/x-ruby",
	".java":  "text/x-java",
	".c":     "text/x-c",
	".h":     "text/x-c",
	".cpp":   "text/x-c++",
	".hpp":   "text/x-c++",
	".rs":    "text/x-rust",
	".sh":    "application/x-sh",
	".bash":  "application/x-sh",
	".zsh":   "application/x-sh",
	".ps1":   "text/x-powershell",
	".bat":   "application/bat",
	".cmd":   "application/bat",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".bmp":   "image/bmp",
	".ico":   "image/x-icon",
	".svg":   "image/svg+xml",
	".pdf":   "application/pdf",
	".doc":   "application/msword",
	".docx":  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":   "application/vnd.ms-excel",
	".xlsx":  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".zip":   "application/zip",
	".tar":   "application/x-tar",
	".gz":    "application/gzip",
	".bz2":   "application/x-bzip2",
	".xz":    "application/x-xz",
	".7z":    "application/x-7z-compressed",
	".mp3":   "audio/mpeg",
	".wav":   "audio/wav",
	".ogg":   "audio/ogg",
	".mp4":   "video/mp4",
	".avi":   "video/x-msvideo",
	".mov":   "video/quicktime",
	".wasm":  "application/wasm",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".eot":   "application/vnd.ms-fontobject",
	".class": "application/java-vm",
	".jar":   "application/java-archive",
}

// DetectMIMEType detects the MIME type from a file path.
func DetectMIMEType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mime, ok := extensionMIMEs[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// IsImageMIME returns true if the MIME type is an image type.
func IsImageMIME(mime string) bool {
	prefixes := []string{"image/"}
	for _, p := range prefixes {
		if strings.HasPrefix(mime, p) {
			return true
		}
	}
	return false
}

// IsTextMIME returns true if the MIME type is a text type.
func IsTextMIME(mime string) bool {
	prefixes := []string{"text/", "application/json", "application/xml", "application/x-yaml",
		"application/javascript", "application/typescript"}
	for _, p := range prefixes {
		if strings.HasPrefix(mime, p) {
			return true
		}
	}
	return false
}

// SupportedImageMIMETypes returns the set of image MIME types the agent can handle.
func SupportedImageMIMETypes() []string {
	return []string{
		"image/png",
		"image/jpeg",
		"image/gif",
		"image/webp",
	}
}

// FileExtensionFromMIME returns the file extension for a MIME type.
func FileExtensionFromMIME(mime string) string {
	for ext, m := range extensionMIMEs {
		if m == mime {
			return ext
		}
	}
	return ""
}

// IsBinaryMIME returns true for binary (non-text) MIME types.
func IsBinaryMIME(mime string) bool {
	if IsImageMIME(mime) {
		return true
	}
	binaryPrefixes := []string{"application/", "audio/", "video/", "font/", "model/"}
	for _, p := range binaryPrefixes {
		if strings.HasPrefix(mime, p) {
			// Exclude text-like application types
			textApp := []string{"application/json", "application/xml", "application/x-yaml",
				"application/javascript", "application/typescript", "application/wasm"}
			for _, ta := range textApp {
				if mime == ta {
					return false
				}
			}
			return true
		}
	}
	return false
}

// MIMETypeForExtension returns the MIME type for a file extension.
func MIMETypeForExtension(ext string) string {
	return DetectMIMEType("file" + ext)
}
