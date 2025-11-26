package utils

import "strings"

// PathToKey creates a collision-free key from a JSON path slice.
// Uses null byte separator to prevent collisions between paths like
// ["a.b", "c"] and ["a", "b.c"], which would both produce "a.b.c" with dot separator
// but produce different keys with null byte separator.
//
// JSON keys cannot contain U+0000 (null byte), making this separator safe for all valid JSON.
func PathToKey(path []string) string {
	return strings.Join(path, "\x00")
}
