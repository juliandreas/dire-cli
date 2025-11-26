package utils

// Line ending constants for consistent usage across the codebase.
const (
	LineEndingLF   = "\n"   // Unix/Linux/macOS
	LineEndingCRLF = "\r\n" // Windows
	LineEndingCR   = "\r"   // Classic Mac (rarely used)
)

// DetectLineEnding analyzes file content to determine the line ending style.
// Returns LineEndingCRLF for Windows (CRLF), LineEndingCR for classic Mac (CR),
// or LineEndingLF for Unix/Linux/modern Mac (LF).
// If ANY CRLF sequences are found, returns LineEndingCRLF to properly handle files with mixed line endings,
// which is important for git with core.autocrlf=true.
func DetectLineEnding(data []byte) string {
	var crlfCount, crCount int

	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				crlfCount++
				i++ // Skip the \n as we've counted this as CRLF
			} else {
				crCount++
			}
		case '\n':
			// LF present; default return covers pure-LF case
		}
	}

	// If we find ANY CRLF sequences, use CRLF (Windows)
	// This handles mixed line ending files properly, which is common
	// when git's core.autocrlf setting is involved
	if crlfCount > 0 {
		return LineEndingCRLF
	} else if crCount > 0 {
		// Classic Mac style (rare nowadays)
		return LineEndingCR
	}

	// Default to Unix line endings (pure LF or no line endings)
	return LineEndingLF
}
