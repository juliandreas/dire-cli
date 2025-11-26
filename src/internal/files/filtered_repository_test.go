package files_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// mockRepositoryWithError is a minimal mock that returns errors from all methods.
type mockRepositoryWithError struct {
	err error
}

// Ensure mockRepositoryWithError implements files.Repository.
var _ files.Repository = (*mockRepositoryWithError)(nil)

var errRepositoryError = errors.New("repository error")

func TestFilteredRepository_WithoutFilters(t *testing.T) {
	t.Parallel()

	// Create mock base repository
	//nolint:exhaustruct // Test mock only needs specific fields
	baseRepo := &testutil.MockRepository{
		Missing: []types.MissingTranslation{
			{Path: []string{"auth", "login"}, SourceValue: "Login", SourceLocale: "en", TargetLocale: "da"},
			{Path: []string{"cart", "title"}, SourceValue: "Cart", SourceLocale: "en", TargetLocale: "da"},
		},
	}

	// Create filtered repository with no filters
	filteredRepo := files.NewFilteredRepository(baseRepo, []string{})

	// Should return the original repository (no wrapper)
	if filteredRepo != baseRepo {
		t.Errorf("Expected same repository when no filters provided")
	}
}

func TestFilteredRepository_WithFilters_LeafKeys(t *testing.T) {
	t.Parallel()

	// Create mock base repository with leaf-level missing translations
	//nolint:exhaustruct // Test mock only needs specific fields
	baseRepo := &testutil.MockRepository{
		Missing: []types.MissingTranslation{
			{Path: []string{"auth", "login"}, SourceValue: "Login", SourceLocale: "en", TargetLocale: "da"},
			{Path: []string{"auth", "logout"}, SourceValue: "Logout", SourceLocale: "en", TargetLocale: "da"},
			{Path: []string{"cart", "title"}, SourceValue: "Cart", SourceLocale: "en", TargetLocale: "da"},
		},
	}

	// Create filtered repository targeting "auth.login"
	filteredRepo := files.NewFilteredRepository(baseRepo, []string{"auth.login"})

	ctx := context.Background()
	missing, err := filteredRepo.GetMissing(ctx)
	if err != nil {
		t.Fatalf("GetMissing failed: %v", err)
	}

	// Should only return auth.login translation
	expected := 1
	if len(missing) != expected {
		t.Errorf("Expected %d missing translations, got %d", expected, len(missing))
	}

	if len(missing) > 0 {
		expectedPath := []string{"auth", "login"}
		if !slices.Equal(missing[0].Path, expectedPath) {
			t.Errorf("Expected path %v, got %v", expectedPath, missing[0].Path)
		}
	}
}

func TestFilteredRepository_WithFilters_ObjectExpansion(t *testing.T) {
	t.Parallel()

	// Create mock base repository with object-level missing translation
	authObject := map[string]any{
		"login":  "Login",
		"logout": "Logout",
		"signup": "Sign up",
	}

	//nolint:exhaustruct // Test mock only needs specific fields
	baseRepo := &testutil.MockRepository{
		Missing: []types.MissingTranslation{
			{Path: []string{"auth"}, SourceValue: authObject, SourceLocale: "en", TargetLocale: "da"},
			{Path: []string{"cart"}, SourceValue: "Cart", SourceLocale: "en", TargetLocale: "da"},
		},
	}

	// Create filtered repository targeting "auth.login"
	filteredRepo := files.NewFilteredRepository(baseRepo, []string{"auth.login"})

	ctx := context.Background()
	missing, err := filteredRepo.GetMissing(ctx)
	if err != nil {
		t.Fatalf("GetMissing failed: %v", err)
	}

	// Should expand auth object and return only auth.login
	expected := 1
	if len(missing) != expected {
		t.Errorf("Expected %d missing translations, got %d", expected, len(missing))
	}

	if len(missing) > 0 {
		expectedPath := []string{"auth", "login"}
		if !slices.Equal(missing[0].Path, expectedPath) {
			t.Errorf("Expected path %v, got %v", expectedPath, missing[0].Path)
		}
		if missing[0].SourceValue != "Login" {
			t.Errorf("Expected 'Login' value, got %v", missing[0].SourceValue)
		}
	}
}

func TestFilteredRepository_MultipleKeyFilters(t *testing.T) {
	t.Parallel()

	// Create mock base repository with multiple translations
	//nolint:exhaustruct // Test mock only needs specific fields
	baseRepo := &testutil.MockRepository{
		Missing: []types.MissingTranslation{
			{Path: []string{"auth", "login"}, SourceValue: "Login", SourceLocale: "en", TargetLocale: "da"},
			{Path: []string{"auth", "logout"}, SourceValue: "Logout", SourceLocale: "en", TargetLocale: "da"},
			{Path: []string{"cart", "title"}, SourceValue: "Cart", SourceLocale: "en", TargetLocale: "da"},
			{Path: []string{"dashboard", "title"}, SourceValue: "Dashboard", SourceLocale: "en", TargetLocale: "da"},
		},
	}

	// Create filtered repository targeting multiple keys
	filteredRepo := files.NewFilteredRepository(baseRepo, []string{"auth.login", "cart.title"})

	ctx := context.Background()
	missing, err := filteredRepo.GetMissing(ctx)
	if err != nil {
		t.Fatalf("GetMissing failed: %v", err)
	}

	// Should return exactly 2 translations: auth.login and cart.title
	expected := 2
	if len(missing) != expected {
		t.Errorf("Expected %d missing translations, got %d", expected, len(missing))
	}

	// Verify we got the right keys (order might vary)
	paths := make([][]string, len(missing))
	for i, m := range missing {
		paths[i] = m.Path
	}

	expectedPaths := [][]string{{"auth", "login"}, {"cart", "title"}}
	for _, expectedPath := range expectedPaths {
		found := false
		for _, path := range paths {
			if slices.Equal(path, expectedPath) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected path %v not found in results %v", expectedPath, paths)
		}
	}
}

func TestFilteredRepository_ErrorPropagation(t *testing.T) {
	t.Parallel()

	// Create mock repository that returns an error
	baseRepo := &mockRepositoryWithError{
		err: errRepositoryError,
	}

	filteredRepo := files.NewFilteredRepository(baseRepo, []string{"auth.login"})

	ctx := context.Background()
	_, err := filteredRepo.GetMissing(ctx)

	// Should propagate the error from the underlying repository
	if err == nil {
		t.Error("Expected error to be propagated, but got nil")
	}

	if !errors.Is(err, errRepositoryError) {
		t.Errorf("Expected error '%s', got '%s'", errRepositoryError, err)
	}
}

func (m *mockRepositoryWithError) Scan(ctx context.Context) (map[types.Locale]*types.LocaleFile, error) {
	return nil, m.err
}

func (m *mockRepositoryWithError) DetermineMissing(
	ctx context.Context, localeFiles map[types.Locale]*types.LocaleFile, includeStubs bool,
) ([]types.MissingTranslation, types.Locale, error) {
	return nil, "", m.err
}

func (m *mockRepositoryWithError) GetMissing(ctx context.Context) ([]types.MissingTranslation, error) {
	return nil, m.err
}

func (m *mockRepositoryWithError) GetMissingWithStubs(ctx context.Context) ([]types.MissingTranslation, error) {
	return nil, m.err
}

func (m *mockRepositoryWithError) GetFilteredContentForLocale(
	ctx context.Context,
	locale types.Locale,
) ([]types.LocaleContent, error) {
	return nil, m.err
}

func (m *mockRepositoryWithError) FinalCleanup(ctx context.Context) error {
	return m.err
}

func TestFilteredRepository_InvalidKeys(t *testing.T) {
	t.Parallel()

	referenceFile := &types.LocaleFile{
		Path:   "",
		Locale: "en",
		Contents: map[string]any{
			"auth": map[string]any{"login": "Login"},
		},
		LineEnding: utils.LineEndingLF, // Default to Unix line endings for tests
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": referenceFile,
	}

	//nolint:exhaustruct // Test mock only needs specific fields
	baseRepo := &testutil.MockRepository{
		LocaleFiles:     localeFiles,
		Missing:         nil,
		ReferenceLocale: "en",
	}
	filteredRepo := files.NewFilteredRepository(baseRepo, []string{"invalid.key"})

	_, _, err := filteredRepo.DetermineMissing(context.Background(), localeFiles, false)

	if !errors.Is(err, errs.ErrInvalidKeys) {
		t.Errorf("Expected ErrInvalidKeys, got %v", err)
	}
}
