# Known Issues

## Claude API Performance with Large Batches

**Issue**: When using Claude as the AI provider, processing large batches of translations can take significantly longer than expected.

**Details**:

- This appears to be related to Claude's API rate limiting and processing characteristics
- Other providers typically handle large batches more efficiently
- The slowdown is most noticeable when processing hundreds of translation

**Status**: Under investigation for potential optimization improvements.

## Single Translation File Per Locale Limitation

**Issue**: The application currently only supports one translation file per locale. If you have multiple translation files for the same locale, only one can be processed.

**Details**:

- This limitation affects projects with split translation files (e.g., `auth.json`, `navigation.json`, `forms.json` for the same locale)
- Only the translation file specified in the `.dire.yaml` file for each locale will be processed

**Status**: Planned for future development to support multiple files per locale.
