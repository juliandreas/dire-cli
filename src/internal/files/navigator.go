package files

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/juliandreas/dire/internal/types"
)

// navigator handles path traversal and value setting with array support.
type navigator struct {
	root map[string]any
	path []string
}

// Navigation errors.
var (
	ErrEmptyTranslationPath  = errors.New("cannot set value on empty translation path")
	ErrPathConflict          = errors.New("path conflict (shape mismatch)")
	ErrNilRootMap            = errors.New("cannot set nested value on nil root map")
	ErrEmptyPathSegment      = errors.New("path contains an empty segment")
	ErrInvalidArrayIndex     = errors.New("invalid array index in path")
	ErrArrayIndexOutOfBounds = errors.New("array index out of bounds")
)

// arrayIndexPattern matches array indices in paths like "[0]", "[1]", etc.
var arrayIndexPattern = regexp.MustCompile(`^\[(\d+)\]$`)

// ParseArrayIndex extracts the numeric index from an array path segment like "[0]".
// Returns the index and true if successful, or 0 and false if not an array index.
func ParseArrayIndex(segment string) (int, bool) {
	if matches := arrayIndexPattern.FindStringSubmatch(segment); matches != nil {
		index, _ := strconv.Atoi(matches[1])
		return index, true
	}
	return 0, false
}

// IsArrayIndex checks if a path segment represents an array index.
func IsArrayIndex(segment string) bool {
	return arrayIndexPattern.MatchString(segment)
}

// ValidatePathSegments validates that all path segments are non-empty.
func ValidatePathSegments(path []string) error {
	for i, segment := range path {
		if segment == "" {
			return fmt.Errorf("%w at index %d", ErrEmptyPathSegment, i)
		}
	}
	return nil
}

// SetValue sets a value at the given path, creating intermediate structures as needed.
// This is the tolerant version that will replace non-navigable values.
func SetValue(contents map[string]any, path []string, value any) error {
	if len(path) == 0 {
		return ErrEmptyTranslationPath
	}

	nav := &navigator{root: contents, path: path}
	return nav.setValueInternal(value, false) // false = tolerant mode
}

// SetValueStrict sets a value at the given path with strict type checking.
// Returns an error if path conflicts exist or types don't match.
func SetValueStrict(contents map[string]any, path []string, value any) error {
	if len(path) == 0 {
		return ErrEmptyTranslationPath
	}

	nav := &navigator{root: contents, path: path}
	return nav.setValueInternal(value, true) // true = strict mode
}

// ExtractTranslations extracts all translatable content from the locale data.
func ExtractTranslations(contents map[string]any, path []string, locale types.Locale) []types.LocaleContent {
	var results []types.LocaleContent

	// Sort keys for deterministic traversal
	keys := slices.Collect(maps.Keys(contents))
	slices.Sort(keys)

	for _, key := range keys {
		value := contents[key]
		currentPath := slices.Clone(path)
		currentPath = append(currentPath, key)

		switch val := value.(type) {
		case map[string]any:
			// Recurse into nested maps
			nestedResults := ExtractTranslations(val, currentPath, locale)
			results = append(results, nestedResults...)
		case []any:
			// Handle arrays - check if array contains translatable content
			if slices.ContainsFunc(val, hasTranslatableStrings) {
				// Arrays are emitted as one LocaleContent when any translatable string exists
				// to avoid slicing arrays into leaves here and preserve array structure integrity
				results = append(results, types.LocaleContent{
					Locale: locale,
					Path:   currentPath,
					Value:  value,
				})
			}
		default:
			// Leaf value (strings, numbers, booleans, etc.)
			results = append(results, types.LocaleContent{
				Locale: locale,
				Path:   currentPath,
				Value:  value,
			})
		}
	}

	return results
}

// hasTranslatableStrings recursively checks if a value contains translatable strings.
func hasTranslatableStrings(value any) bool {
	switch val := value.(type) {
	case string:
		return strings.TrimSpace(val) != ""
	case map[string]any:
		for _, val := range val {
			if hasTranslatableStrings(val) {
				return true
			}
		}
	case []any:
		if slices.ContainsFunc(val, hasTranslatableStrings) {
			return true
		}
	}
	return false
}

// setValueInternal sets a value at the navigator's path with optional strict mode.
func (n *navigator) setValueInternal(value any, strict bool) error {
	if err := ValidatePathSegments(n.path); err != nil {
		return err
	}

	if n.root == nil {
		return fmt.Errorf("%w at %s", ErrNilRootMap, strings.Join(n.path, "."))
	}

	// Navigate to parent and set value
	parent, finalSegment, err := n.navigateToParent(strict)
	if err != nil {
		return err
	}

	return n.setFinalValue(parent, finalSegment, value, strict)
}

// navigateToParent navigates to the parent of the target location.
func (n *navigator) navigateToParent(strict bool) (any, string, error) {
	if len(n.path) == 1 {
		// Setting at root level
		return n.root, n.path[0], nil
	}

	var current any = n.root

	// Navigate through all segments except the last one
	for i, segment := range n.path[:len(n.path)-1] {
		var err error
		current, err = n.navigateSegment(current, segment, i, strict)
		if err != nil {
			return nil, "", err
		}
	}

	return current, n.path[len(n.path)-1], nil
}

// navigateSegment navigates through a single path segment.
func (n *navigator) navigateSegment(current any, segment string, index int, strict bool) (any, error) {
	// Check if this is an array index
	if arrayIndex, isArray := ParseArrayIndex(segment); isArray {
		return n.navigateArrayIndex(current, arrayIndex, index)
	}

	// Handle map navigation
	return n.navigateMapKey(current, segment, index, strict)
}

// navigateArrayIndex navigates into an array at the specified index.
func (n *navigator) navigateArrayIndex(current any, index, pathIndex int) (any, error) {
	arr, ok := current.([]any)
	if !ok {
		pathSoFar := strings.Join(n.path[:pathIndex+1], ".")
		return nil, fmt.Errorf("%w at %s: expected array, got %T", ErrPathConflict, pathSoFar, current)
	}

	// Check bounds
	if index >= len(arr) {
		pathSoFar := strings.Join(n.path[:pathIndex+1], ".")
		return nil, fmt.Errorf("%w at %s: index %d out of bounds (length %d)",
			ErrArrayIndexOutOfBounds, pathSoFar, index, len(arr))
	}

	return arr[index], nil
}

// navigateMapKey navigates into a map at the specified key.
func (n *navigator) navigateMapKey(current any, key string, pathIndex int, strict bool) (any, error) {
	currentMap, ok := current.(map[string]any)
	if !ok {
		pathSoFar := strings.Join(n.path[:pathIndex+1], ".")
		return nil, fmt.Errorf("%w at %s: expected map, got %T", ErrPathConflict, pathSoFar, current)
	}

	if next, exists := currentMap[key]; exists {
		// Check if we can navigate into the existing value
		switch next.(type) {
		case map[string]any, []any:
			return next, nil
		default:
			// Existing value is not navigable
			if strict {
				pathSoFar := strings.Join(n.path[:pathIndex+1], ".")
				return nil, fmt.Errorf("%w at %s (existing type: %T)", ErrPathConflict, pathSoFar, next)
			}
			// Tolerant mode: replace with appropriate structure
			return n.createIntermediateStructure(currentMap, key, pathIndex)
		}
	}

	// Key doesn't exist - create intermediate structure
	return n.createIntermediateStructure(currentMap, key, pathIndex)
}

// createIntermediateStructure creates the appropriate intermediate structure.
func (n *navigator) createIntermediateStructure(parent map[string]any, key string, pathIndex int) (any, error) {
	// Determine what to create based on the next segment
	if pathIndex+1 < len(n.path) && IsArrayIndex(n.path[pathIndex+1]) {
		// Next segment is array index, create empty array
		newArr := []any{}
		parent[key] = newArr
		return newArr, nil
	}

	// Next segment is map key, create map
	newMap := make(map[string]any)
	parent[key] = newMap
	return newMap, nil
}

// setFinalValue sets the value at the final location.
func (n *navigator) setFinalValue(parent any, finalSegment string, value any, strict bool) error {
	// Check if final segment is array index
	if arrayIndex, isArray := ParseArrayIndex(finalSegment); isArray {
		return n.setArrayValue(parent, arrayIndex, value, strict)
	}

	// Set in map
	return n.setMapValue(parent, finalSegment, value, strict)
}

// setArrayValue sets a value in an array.
func (n *navigator) setArrayValue(parent any, index int, value any, strict bool) error {
	arr, ok := parent.([]any)
	if !ok {
		pathStr := strings.Join(n.path, ".")
		return fmt.Errorf("%w at %s: expected array, got %T", ErrPathConflict, pathStr, parent)
	}

	// Check bounds
	if index >= len(arr) {
		pathStr := strings.Join(n.path, ".")
		return fmt.Errorf("%w at %s: index %d out of bounds (length %d)", ErrArrayIndexOutOfBounds, pathStr, index, len(arr))
	}

	if strict && arr[index] != nil {
		if err := n.validateTypeCompatibility(arr[index], value); err != nil {
			return err
		}
	}

	arr[index] = value
	return nil
}

// setMapValue sets a value in a map.
func (n *navigator) setMapValue(parent any, key string, value any, strict bool) error {
	currentMap, ok := parent.(map[string]any)
	if !ok {
		pathStr := strings.Join(n.path, ".")
		return fmt.Errorf("%w at %s: expected map, got %T", ErrPathConflict, pathStr, parent)
	}

	if strict {
		if existing, exists := currentMap[key]; exists {
			// Check type compatibility in strict mode
			if err := n.validateTypeCompatibility(existing, value); err != nil {
				return err
			}
		}
	}

	currentMap[key] = value
	return nil
}

// validateTypeCompatibility checks if values are type-compatible in strict mode.
func (n *navigator) validateTypeCompatibility(existing, newValue any) error {
	existingIsComposite := isCompositeType(existing)
	newIsComposite := isCompositeType(newValue)

	// Error if one is composite and the other is not
	if existingIsComposite != newIsComposite {
		pathStr := strings.Join(n.path, ".")
		return fmt.Errorf("%w at %s (existing type: %T, new type: %T)", ErrPathConflict, pathStr, existing, newValue)
	}

	return nil
}

// isCompositeType checks if a value is a composite type (map, slice, or array).
func isCompositeType(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}
