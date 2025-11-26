package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/engine"
	"github.com/juliandreas/dire/internal/engine/rephrase"
	"github.com/juliandreas/dire/internal/events"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/logger"
)

// DisplayTranslationProgress handles the progress display for translation operations.
func DisplayTranslationProgress(ctx context.Context, cfg *config.Config, event *events.Event, timer *Timer) {
	if timer == nil || event == nil || cfg == nil {
		return
	}

	switch event.Type {

	case events.TranslationPairStarted:
		timer.Restart(ctx)

	case events.TranslationPairCompleted:
		timer.Stop()
		completionMsg := formatPairCompletion(event, cfg, event.Duration, event.Attempt)
		fmt.Printf("✓ %s\n", completionMsg)

	case events.TranslationPairCancelled:
		// Silent cancellation - don't show anything
		timer.ClearLine()

	case events.Warning:
		timer.ClearLine()
		fmt.Printf("! %s\n\n", event.Message)

	case events.Error:
		timer.ClearLine()
		fmt.Printf("✗ Error: %s\n\n", event.Message)
	}
}

// formatPairCompletion formats pair completion data.
func formatPairCompletion(event *events.Event, cfg *config.Config, duration time.Duration, attempt int) string {
	// Look up file names from config
	sourceFile := constants.Gray().Render(fmt.Sprintf("(%s)", cfg.GetFileNameForLocale(string(event.Source))))
	targetFile := constants.Gray().Render(fmt.Sprintf("(%s)", cfg.GetFileNameForLocale(string(event.Target))))

	timing := constants.Gray().Render(fmt.Sprintf("(%s)", formatElapsed(duration)))
	retryIndicator := ""
	if attempt > 0 {
		retryIndicator = " " + constants.Gray().Render(fmt.Sprintf("*retry #%d*", attempt))
	}

	return fmt.Sprintf("%s %s → %s %s · %s%s",
		event.Source, sourceFile, event.Target, targetFile, timing, retryIndicator)
}

// DisplayTranslationCompletion shows the final completion message.
func DisplayTranslationCompletion(
	ctx context.Context,
	metrics *engine.Metrics,
	timer *Timer, cfg *config.Config,
	isSourcedMode bool,
	requiresTranslationService bool,
) {
	if metrics == nil || timer == nil {
		if timer != nil {
			timer.Stop()
		}
		fmt.Println("✗ Error: Translation process was interrupted")
		return
	}

	logger.Info("Translation completed successfully",
		"sourcedKeys", metrics.SourcedTranslatedKeys,
		"translatedKeys", metrics.TranslatedKeys,
		"stubKeys", metrics.StubTranslatedKeys,
		"remainingKeys", metrics.RemainingKeys)

	totalTranslated := metrics.SourcedTranslatedKeys + metrics.TranslatedKeys + metrics.StubTranslatedKeys

	// Display success message
	displaySuccessMessage(timer, totalTranslated, requiresTranslationService)

	// Handle remaining keys warning
	handleRemainingKeysWarning(ctx, cfg, timer, metrics, isSourcedMode, totalTranslated)
}

// displaySuccessMessage shows the completion success message.
func displaySuccessMessage(timer *Timer, totalTranslated int, requiresTranslationService bool) {
	if totalTranslated > 0 {
		if requiresTranslationService {
			fmt.Println() // Blank line
		}
		keyCount := constants.Gray().Render(fmt.Sprintf("(%d keys)", totalTranslated))
		timer.Complete(fmt.Sprintf("%s Done! %s", constants.Star(), keyCount), requiresTranslationService)
	} else {
		timer.Complete(constants.Star()+" All good! No missing translations found", false)
	}
}

// handleRemainingKeysWarning displays warning for any remaining untranslated keys.
func handleRemainingKeysWarning(
	ctx context.Context,
	cfg *config.Config,
	timer *Timer,
	metrics *engine.Metrics,
	isSourcedMode bool,
	totalTranslated int,
) {
	if metrics.RemainingKeys > 0 {
		logger.Debug("Translation completed with remaining untranslated keys",
			"remainingKeys", metrics.RemainingKeys,
			"sourcedMode", isSourcedMode,
			"totalTranslated", totalTranslated)

		// Only show remaining keys warning if not in sourced mode
		if !isSourcedMode {
			event := events.NewWarning(fmt.Sprintf("%d keys remain untranslated", metrics.RemainingKeys))
			DisplayTranslationProgress(ctx, cfg, event, timer)
		}
	}
}

// DisplayAppVersion prints the application name and version in gray.
func DisplayAppVersion() {
	fmt.Printf("%s\r\n\r\n", constants.Gray().Render(fmt.Sprintf("%s %s", constants.AppName, constants.AppVersion)))
}

// DisplayRephrases displays the results of rephrase generation.
func DisplayRephrases(result *rephrase.Result) {
	if result == nil {
		fmt.Println("No rephrase options generated")
		return
	}

	// Header with key and locale information
	grayPrefix := constants.Gray().Render("Rephrase options for ")
	key := fmt.Sprintf("%q", result.Key)
	grayMiddle := constants.Gray().Render(" in ")
	locale := string(result.Locale)
	graySuffix := constants.Gray().Render(":")
	fmt.Printf("%s%s%s%s%s\n", grayPrefix, key, grayMiddle, locale, graySuffix)

	// Current value
	fmt.Printf("%s %s\n\n", constants.Gray().Render("Current:"), fmt.Sprintf("%q", result.CurrentValue))

	// Rephrase options list
	if len(result.Rephrases) == 0 {
		fmt.Println(constants.Gray().Render("No rephrase options available"))
		return
	}

	for i, rephrase := range result.Rephrases {
		prefix := constants.Gray().Render(fmt.Sprintf("%d.", i+1))
		fmt.Printf("%s %s\n", prefix, fmt.Sprintf("%q", rephrase))
	}

	fmt.Println() // Blank line at the end
}

// DisplayPruneResults displays the results of a pruning operation.
func DisplayPruneResults(results []files.PruneResult) {
	if len(results) == 0 {
		fmt.Println(constants.Star() + " All good! No orphaned keys found")
		return
	}

	for _, result := range results {
		if result.PrunedCount == 0 {
			continue
		}

		keyStr := "key"
		if result.PrunedCount > 1 {
			keyStr = "keys"
		}
		prunedStr := constants.Gray().Render(fmt.Sprintf("(pruned %d %s)", result.PrunedCount, keyStr))
		fmt.Printf("✓ %s %s\n", result.Locale, prunedStr)
	}
}
