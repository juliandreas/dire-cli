# Dire CLI Commands

Dire is both a translation tool and an i18n file maintenance system. Commands are organized by their primary purpose.

## Setup & Configuration

Commands for initializing and configuring Dire.

| Command             | Description                                | Example             |
| ------------------- | ------------------------------------------ | ------------------- |
| `dire init`         | Create a configuration file (`.dire.toml`) | `dire init`         |
| `dire init --force` | Overwrite existing configuration file      | `dire init --force` |

## Translation Commands

Commands for generating and managing translations in your i18n files.

| Command                                          | Description                                               | Example                                            |
| ------------------------------------------------ | --------------------------------------------------------- | -------------------------------------------------- |
| `dire`                                           | Translate all missing keys in your i18n files             | `dire`                                             |
| `dire --keys <key>`                              | Translate specific key(s)                                 | `dire --keys "auth.login,auth.register"`           |
| `dire --sourced`                                 | Apply only glossary and memory translations (no provider) | `dire --sourced`                                   |
| `dire --glossary`                                | Apply only glossary translations (no provider)            | `dire --glossary`                                  |
| `dire --glossary --overwrite`                    | Overwrite existing translations with glossary values      | `dire --glossary --overwrite`                      |
| `dire --memory`                                  | Apply only memory translations (no provider)              | `dire --memory`                                    |
| `dire --stub`                                    | Create placeholder translations (empty strings)           | `dire --stub`                                      |
| `dire --context <context>`                       | Add context for better AI translations                    | `dire --context "E-commerce checkout"`             |
| `dire --rephrase --keys <key> --locale <locale>` | Generate rephrase options for existing translations       | `dire --rephrase --keys auth.login --locale en-US` |
| `dire --include-stubs`                           | Include empty strings as missing translations             | `dire --include-stubs`                             |
| `dire --no-trim`                                 | Preserve whitespace in translations                       | `dire --no-trim`                                   |

## File Maintenance Commands

Commands for managing and maintaining i18n file structure and completeness.

| Command                      | Description                                     | Example                      |
| ---------------------------- | ----------------------------------------------- | ---------------------------- |
| `dire --prune`               | Remove orphaned keys from non-reference locales | `dire --prune`               |
| `dire --check`               | Lint translation completeness (CI/CD mode)      | `dire --check`               |
| `dire --check --format json` | Output lint results in JSON format              | `dire --check --format json` |

## General Options

Universal flags that work across all commands.

| Command          | Description           | Example          |
| ---------------- | --------------------- | ---------------- |
| `dire --debug`   | Enable debug output   | `dire --debug`   |
| `dire --help`    | Show help information | `dire --help`    |
| `dire --version` | Show current version  | `dire --version` |

## Configuration Override Flags

These flags override settings from your `.dire.toml` configuration file:

| Flag                                 | Description                    | Example                                               |
| ------------------------------------ | ------------------------------ | ----------------------------------------------------- |
| `--directory <path>`                 | Override files.directory       | `dire --directory ./i18n`                             |
| `--locales <filename=language-code>` | Override files.locales mapping | `dire --locales en.json=en-US,fr.json=fr-FR`          |
| `--indent <number>`                  | Override files.indent          | `dire --indent 4`                                     |
| `--formality <level>`                | Override styleGuide.formality  | `dire --formality formal`                             |
| `--project-name <name>`              | Override project.name          | `dire --project-name "My App"`                        |
| `--project-description <text>`       | Override project.description   | `dire --project-description "Online store for shoes"` |
| `--project-domain <domain>`          | Override project.domain        | `dire --project-domain "e-commerce"`                  |
| `--provider-active <name>`           | Override providers.active      | `dire --provider-active claude`                       |
| `--provider-model <model>`           | Override active provider model | `dire --provider-model claude-3-5-sonnet`             |
| `--provider-temperature <float>`     | Override provider temperature  | `dire --provider-temperature 0.3`                     |
| `--provider-max-tokens <number>`     | Override provider max tokens   | `dire --provider-max-tokens 4096`                     |

## What Each Command Does

### Setup & Configuration

#### `dire init`

- Creates a `.dire.toml` configuration file
- Sets up default and example settings for your project
- Use `--force` flag to overwrite existing configuration

### Translation Commands

#### `dire` (Main Command)

- Scans your configured i18n files
- Identifies missing translations
- Applies glossary and memory translations
- Uses the configured provider to generate translations for missing keys
- Saves results back to your files

#### `dire --keys <key>`

- Translates only specific keys instead of all missing ones
- Focuses on particular sections of your translations
- Supports multiple keys separated by commas

#### `dire --sourced`

- Uses both glossary entries and translation memory
- Reuses existing translations found in your files
- Skips provider-powered translation generation
- Convenience flag equivalent to using `--glossary --memory` together
- No API key required

#### `dire --glossary`

- Uses only your configured glossary entries
- Applies exact matches from your glossary configuration
- Skips translation memory and provider-powered translation
- No API key required

#### `dire --glossary --overwrite`

- Overwrites existing translations with glossary values
- Requires `--glossary` flag (cannot be used with `--sourced` or `--memory`)
- Useful for enforcing glossary terms across all translation files
- Updates translations even when they already exist
- No API key required

**Use Cases:**

- **Terminology updates**: Apply glossary changes to existing translations
- **Consistency enforcement**: Ensure all files use the same glossary terms
- **Brand alignment**: Update translations to match new brand guidelines

#### `dire --memory`

- Reuses only existing translations found in your files (translation memory)
- Finds matches based on source text across different keys
- Skips glossary and provider-powered translation
- No API key required

#### `dire --stub`

- Creates empty string placeholders for all missing translations
- Skips provider translation generation
- Useful for setting up translation file structure
- No API key required

#### `dire --context <context>`

- Provides additional context to improve translation quality
- Improves translation accuracy
- Ensures domain-specific terminology
- Maintains consistent tone and style
- **Note**: Only works with LLM services (Claude, OpenAI, etc.), not translation services (DeepL, Google Translate, etc.)

#### `dire --rephrase --keys <key> --locale <locale>`

- Generates rephrase options for existing translations
- Requires both a specific key and target locale
- Helps improve translation quality and alternatives
- Only works with single keys, not multiple keys
- **Note**: Only works with LLM services (Claude, OpenAI, etc.), not translation services (DeepL, Google Translate, etc.)

#### `dire --include-stubs`

- Treats empty string translations as missing translations
- Processes placeholder values that need actual translations
- Useful when working with incomplete translation files

#### `dire --no-trim`

- Preserves leading/trailing spaces
- Maintains exact formatting
- Keeps intentional whitespace in translations

### File Maintenance Commands

#### `dire --prune`

- Removes orphaned translation keys that don't exist in the reference locale
- Identifies the reference locale (the one with the most keys)
- Cleans up keys from other locales that are no longer in the reference
- Keeps your translation files synchronized and maintainable over time
- Essential for ongoing i18n file maintenance as your project evolves
- No API key required

#### `dire --check`

- Lints translation files for completeness issues
- Exits with code 0 if complete, 1 if missing translations found
- Perfect for CI/CD pipelines to prevent incomplete translations from reaching production
- Use `--format json` for machine-readable output
- Respects `--include-stubs` (treats empty strings as missing)
- Supports `--keys` flag to lint specific translation keys only

**Exit Codes:**

- `0` - All translations complete
- `1` - Missing translations detected
- `2` - Configuration or system errors
