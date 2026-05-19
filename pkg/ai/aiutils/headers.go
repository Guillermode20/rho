package aiutils

import (
	"net/http"
	"strings"
)

// HeadersToRecord converts http.Header to a map[string]string.
// Multi-valued headers are joined with ", ".
func HeadersToRecord(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		result[key] = strings.Join(values, ", ")
	}
	return result
}

// HeadersToRecordMulti converts http.Header to a map[string][]string.
func HeadersToRecordMulti(headers http.Header) map[string][]string {
	result := make(map[string][]string, len(headers))
	for key, values := range headers {
		vals := make([]string, len(values))
		copy(vals, values)
		result[key] = vals
	}
	return result
}

// RecordToHeaders converts a map[string]string to http.Header.
func RecordToHeaders(record map[string]string) http.Header {
	headers := make(http.Header)
	for key, value := range record {
		headers.Set(key, value)
	}
	return headers
}

// MergeHeaders merges multiple header maps. Later maps override earlier ones.
// Nil values in the map will delete the header.
func MergeHeaders(sources ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, source := range sources {
		for key, value := range source {
			if value == "" {
				delete(result, key)
			} else {
				result[key] = value
			}
		}
	}
	return result
}

// NormalizeHeaderKey normalizes a header key to canonical form (e.g., "content-type" -> "Content-Type").
func NormalizeHeaderKey(key string) string {
	return http.CanonicalHeaderKey(key)
}

// HasHeader checks if a header exists (case-insensitive).
func HasHeader(headers map[string]string, key string) bool {
	canonical := http.CanonicalHeaderKey(key)
	lower := strings.ToLower(key)
	for k := range headers {
		if http.CanonicalHeaderKey(k) == canonical || strings.ToLower(k) == lower {
			return true
		}
	}
	return false
}

// GetHeader retrieves a header value (case-insensitive).
func GetHeader(headers map[string]string, key string) (string, bool) {
	canonical := http.CanonicalHeaderKey(key)
	lower := strings.ToLower(key)
	for k, v := range headers {
		if http.CanonicalHeaderKey(k) == canonical || strings.ToLower(k) == lower {
			return v, true
		}
	}
	return "", false
}

// SetHeader sets a header value, replacing any existing value with the same canonical key.
func SetHeader(headers map[string]string, key, value string) {
	canonical := http.CanonicalHeaderKey(key)
	// Remove any existing entry with the same canonical key
	for k := range headers {
		if http.CanonicalHeaderKey(k) == canonical && k != canonical {
			delete(headers, k)
		}
	}
	headers[canonical] = value
}

// DeleteHeader removes a header (case-insensitive).
func DeleteHeader(headers map[string]string, key string) {
	canonical := http.CanonicalHeaderKey(key)
	lower := strings.ToLower(key)
	for k := range headers {
		if http.CanonicalHeaderKey(k) == canonical || strings.ToLower(k) == lower {
			delete(headers, k)
		}
	}
}

// CopyHeaders makes a shallow copy of a header map.
func CopyHeaders(headers map[string]string) map[string]string {
	result := make(map[string]string, len(headers))
	for k, v := range headers {
		result[k] = v
	}
	return result
}

// AuthHeader creates a Bearer authorization header value.
func AuthHeader(token string) string {
	return "Bearer " + token
}
