package utils

import (
	"bytes"
	"encoding/json"
)

// MarshalJSON marshals JSON while preserving Unicode characters (prevents \uXXXX escaping).
// This should be used instead of json.Marshal and json.MarshalIndent for all JSON output (not for reading).
//
// Parameters:
//   - v: the value to marshal
//   - indent: if non-empty, uses indented output with this string as indent
func MarshalJSON(val any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // Prevent &, <, > and Unicode escaping

	// Set indent if provided
	if indent != "" {
		encoder.SetIndent("", indent)
	}

	if err := encoder.Encode(val); err != nil {
		return nil, err
	}

	// Remove the trailing newline that Encode adds
	result := bytes.TrimSuffix(buf.Bytes(), []byte{'\n'})
	return result, nil
}
