package llm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/juliandreas/dire/internal/files"
)

// llmNavigator handles path traversal with LLM-specific array extension logic.
type llmNavigator struct {
	root map[string]any
	path []string
}

// LLM-specific navigation errors.
var (
	ErrLLMArrayExtension = errors.New("failed to extend array for LLM processing")
	ErrEmptyPath         = errors.New("cannot set value on empty path")
	ErrNilRoot           = errors.New("cannot set nested value on nil root map")
	ErrLLMPathConflict   = errors.New("path conflict during LLM processing")
)

// SetValue sets a value at the given path with LLM-specific array extension capabilities.
// This function handles complex array paths that may require extending arrays beyond their current bounds.
func SetValue(contents map[string]any, path []string, value any) error {
	if len(path) == 0 {
		return ErrEmptyPath
	}

	if err := files.ValidatePathSegments(path); err != nil {
		return err
	}
	// Extra guard: any bracketed segment must be a valid numeric index
	for i, seg := range path {
		if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
			if _, ok := files.ParseArrayIndex(seg); !ok {
				return fmt.Errorf("%w at index %d: invalid array segment %q", ErrLLMPathConflict, i, seg)
			}
		}
	}

	nav := &llmNavigator{root: contents, path: path}
	return nav.setValue(value)
}

// setValue sets a value with LLM-specific array extension logic.
func (n *llmNavigator) setValue(value any) error {
	if n.root == nil {
		return fmt.Errorf("%w at %s", ErrNilRoot, strings.Join(n.path, "."))
	}

	// Navigate to parent and set value
	parent, finalSegment, err := n.navigateToParent()
	if err != nil {
		return err
	}

	return n.setFinalValue(parent, finalSegment, value)
}

// navigateToParent navigates to the parent with array extension capabilities.
func (n *llmNavigator) navigateToParent() (any, string, error) {
	if len(n.path) == 1 {
		// Setting at root level
		return n.root, n.path[0], nil
	}

	var current any = n.root

	// Navigate through all segments except the last one
	for i, segment := range n.path[:len(n.path)-1] {
		var err error
		current, err = n.navigateSegment(current, segment, i)
		if err != nil {
			return nil, "", err
		}
	}

	return current, n.path[len(n.path)-1], nil
}

// navigateSegment navigates through a single path segment with extension capabilities.
func (n *llmNavigator) navigateSegment(current any, segment string, index int) (any, error) {
	// Check if this is an array index
	if arrayIndex, isArray := files.ParseArrayIndex(segment); isArray {
		return n.navigateArrayIndex(current, arrayIndex, index)
	}

	// Handle map navigation
	return n.navigateMapKey(current, segment, index)
}

// navigateArrayIndex navigates into an array with extension capabilities.
func (n *llmNavigator) navigateArrayIndex(current any, index, pathIndex int) (any, error) {
	arr, ok := current.([]any)
	if !ok {
		pathSoFar := strings.Join(n.path[:pathIndex+1], ".")
		return nil, fmt.Errorf("%w at %s: expected array, got %T", ErrLLMPathConflict, pathSoFar, current)
	}

	if index < len(arr) {
		return arr[index], nil
	}

	// Array needs extension
	extendedArr, err := n.extendArrayForNavigation(arr, index, pathIndex)
	if err != nil {
		return nil, err
	}

	return extendedArr[index], nil
}

// navigateMapKey navigates into a map with extension capabilities.
func (n *llmNavigator) navigateMapKey(current any, key string, pathIndex int) (any, error) {
	currentMap, ok := current.(map[string]any)
	if !ok {
		pathSoFar := strings.Join(n.path[:pathIndex+1], ".")
		return nil, fmt.Errorf("%w at %s: expected map, got %T", ErrLLMPathConflict, pathSoFar, current)
	}

	if next, exists := currentMap[key]; exists {
		// Check if we can navigate into the existing value
		switch next.(type) {
		case map[string]any, []any:
			return next, nil
		default:
			// Existing value is not navigable - replace with appropriate structure
			return n.createIntermediateStructure(currentMap, key, pathIndex)
		}
	}

	// Key doesn't exist - create intermediate structure
	return n.createIntermediateStructure(currentMap, key, pathIndex)
}

// createIntermediateStructure creates the appropriate intermediate structure for LLM processing.
func (n *llmNavigator) createIntermediateStructure(
	parent map[string]any, key string, pathIndex int,
) (any, error) {
	if pathIndex+1 >= len(n.path) || !files.IsArrayIndex(n.path[pathIndex+1]) {
		// Next segment is map key, create map
		newMap := make(map[string]any)
		parent[key] = newMap
		return newMap, nil
	}

	// Next segment is array index, create array with sufficient capacity
	newArr := n.createPreallocatedArray(pathIndex)
	parent[key] = newArr
	return newArr, nil
}

// setFinalValue sets the value at the final location with extension capabilities.
func (n *llmNavigator) setFinalValue(parent any, finalSegment string, value any) error {
	// Check if final segment is array index
	if arrayIndex, isArray := files.ParseArrayIndex(finalSegment); isArray {
		return n.setArrayValue(parent, arrayIndex, value)
	}

	// Set in map
	return n.setMapValue(parent, finalSegment, value)
}

// setArrayValue sets a value in an array with extension capabilities.
func (n *llmNavigator) setArrayValue(parent any, index int, value any) error {
	arr, ok := parent.([]any)
	if !ok {
		pathStr := strings.Join(n.path, ".")
		return fmt.Errorf("%w at %s: expected array, got %T", ErrLLMPathConflict, pathStr, parent)
	}

	// If index is beyond array length, extend the array by updating the parent container
	if index >= len(arr) {
		// Validate we have a proper path structure (need at least array container and array index)
		const minPathLengthForArrayExtension = 2 // array container + array index
		if len(n.path) < minPathLengthForArrayExtension {
			pathStr := strings.Join(n.path, ".")
			return fmt.Errorf("%w at %s: path too short for array extension (need at least 2 segments)",
				ErrLLMArrayExtension, pathStr)
		}

		// Create extended array
		newArr := make([]any, index+1)
		copy(newArr, arr)

		// The last element is the array index (e.g., "[5]")
		// The second-to-last is the array container (e.g., "items")
		// Everything before that is the path to the container
		arrayContainerPath := n.path[:len(n.path)-2] // Path to the array container's parent
		arrayContainerKey := n.path[len(n.path)-2]   // The array container name

		// Update the parent container to point to the extended array
		if err := n.updateArrayContainer(newArr, arrayContainerPath, arrayContainerKey); err != nil {
			pathStr := strings.Join(n.path, ".")
			return fmt.Errorf("%w at %s: %v", ErrLLMArrayExtension, pathStr, err)
		}
		arr = newArr
	}

	arr[index] = value
	return nil
}

// extendArrayForNavigation extends an array during navigation with proper initialization.
func (n *llmNavigator) extendArrayForNavigation(arr []any, index, pathIndex int) ([]any, error) {
	// Validate pathIndex to prevent underflow/overflow
	// pathIndex points to the array index segment (e.g., "[0]")
	// The array container must be at pathIndex-1
	if pathIndex <= 0 || pathIndex >= len(n.path) {
		return nil, fmt.Errorf("%w: invalid pathIndex %d for array extension (path length %d)",
			ErrLLMArrayExtension, pathIndex, len(n.path))
	}

	newArr := make([]any, index+1)
	copy(newArr, arr)

	// Initialize the new element based on what comes next in the path
	newArr[index] = n.initializeElementForPath(pathIndex)

	// Update the array container reference
	// pathIndex points to the array index (e.g., "[0]")
	// The array container is one level up at pathIndex-1
	// Container path is everything before the array container
	// Container key is the array container itself
	arrayContainerPath := n.path[:pathIndex-1]
	arrayContainerKey := n.path[pathIndex-1]

	if err := n.updateArrayContainer(newArr, arrayContainerPath, arrayContainerKey); err != nil {
		pathSoFar := strings.Join(n.path[:pathIndex+1], ".")
		return nil, fmt.Errorf("%w during navigation at %s: %v", ErrLLMArrayExtension, pathSoFar, err)
	}

	return newArr, nil
}

// initializeElementForPath initializes an array element based on the next path segment.
func (n *llmNavigator) initializeElementForPath(pathIndex int) any {
	if pathIndex+1 >= len(n.path) {
		return nil
	}

	nextSegment := n.path[pathIndex+1]
	if files.IsArrayIndex(nextSegment) {
		return []any{}
	}
	return make(map[string]any)
}

// createPreallocatedArray creates an array with pre-allocation based on the required index.
func (n *llmNavigator) createPreallocatedArray(pathIndex int) []any {
	requiredIndex, isArray := files.ParseArrayIndex(n.path[pathIndex+1])
	if !isArray {
		return []any{}
	}

	newArr := make([]any, requiredIndex+1)
	n.initializeArrayElement(newArr, requiredIndex, pathIndex)
	return newArr
}

// initializeArrayElement initializes a specific array element based on subsequent path segments.
func (n *llmNavigator) initializeArrayElement(arr []any, index, pathIndex int) {
	if pathIndex+2 >= len(n.path) {
		return
	}

	nextAfterIndex := n.path[pathIndex+2]
	if files.IsArrayIndex(nextAfterIndex) {
		arr[index] = []any{}
		return
	}
	arr[index] = make(map[string]any)
}

// updateArrayContainer updates a specific array container in the structure.
// containerPath: path to the parent object that will contain the array (e.g., ["data", "nested"]).
// containerKey: the key in that parent object where the array will be placed (e.g., "items").
// Result: sets root["data"]["nested"]["items"] = newArr.
func (n *llmNavigator) updateArrayContainer(newArr []any, containerPath []string, containerKey string) error {
	if len(containerPath) == 0 {
		// Array container is at root level
		n.root[containerKey] = newArr
		return nil
	}

	// Navigate to the container node itself (not its parent)
	// This navigates to the object at containerPath where we'll set containerKey
	var current any = n.root
	for i, segment := range containerPath {
		var err error
		current, err = n.navigateSegment(current, segment, i)
		if err != nil {
			pathStr := strings.Join(containerPath[:i+1], ".")
			return fmt.Errorf("failed to navigate to container at %s: %w", pathStr, err)
		}
	}

	// The current node should be a map where we can set the containerKey
	if containerMap, ok := current.(map[string]any); ok {
		containerMap[containerKey] = newArr
		return nil
	}

	// Check if this is an array-of-arrays case (not supported)
	if _, isArrayIndex := files.ParseArrayIndex(containerKey); isArrayIndex {
		if _, isArray := current.([]any); isArray {
			pathStr := strings.Join(append(containerPath, containerKey), ".")
			return fmt.Errorf("%w: arrays-of-arrays are not supported at %s",
				ErrLLMArrayExtension, pathStr)
		}
	}

	pathStr := strings.Join(append(containerPath, containerKey), ".")
	return fmt.Errorf("%w: failed to update array container at %s: container is not a map (got %T)",
		ErrLLMArrayExtension, pathStr, current)
}

// setMapValue sets a value in a map for LLM processing.
func (n *llmNavigator) setMapValue(parent any, key string, value any) error {
	currentMap, ok := parent.(map[string]any)
	if !ok {
		pathStr := strings.Join(n.path, ".")
		return fmt.Errorf("%w at %s: expected map, got %T", ErrLLMPathConflict, pathStr, parent)
	}

	currentMap[key] = value
	return nil
}
