# Dire CLI Commands

## Command Reference Table

| Command                                          | Description                                         | Example                                            |
| ------------------------------------------------ | --------------------------------------------------- | -------------------------------------------------- |
| `dire`                                           | Translate all missing keys in your i18n files       | `dire`                                             |
| `dire init`                                      | Create a configuration file (`.dire.yaml`)          | `dire init`                                        |
| `dire init --force`                              | Overwrite existing configuration file               | `dire init --force`                                |
| `dire --keys <key>`                              | Translate specific key(s)                           | `dire --keys "auth.login,auth.register"`           |
| `dire --rephrase --keys <key> --locale <locale>` | Generate rephrase options for existing translations | `dire --rephrase --keys auth.login --locale en-US` |
| `dire --sourced`                                 | Apply only glossary and memory translations (no AI) | `dire --sourced`                                   |
| `dire --context <context>`                       | Add context for better AI translations              | `dire --context "E-commerce checkout"`             |
| `dire --stub`                                    | Create placeholder translations (empty strings)     | `dire --stub`                                      |
| `dire --include-stubs`                           | Include empty strings as missing translations       | `dire --include-stubs`                             |
| `dire --no-trim`                                 | Preserve whitespace in translations                 | `dire --no-trim`                                   |
| `dire --debug`                                   | Enable debug output                                 | `dire --debug`                                     |
| `dire --help`                                    | Show help information                               | `dire --help`                                      |
| `dire --version`                                 | Show current version                                | `dire --version`                                   |

## What Each Command Does

### `dire` (Main Command)

- Scans your configured i18n files
- Identifies missing translations
- Applies glossary and memory translations
- Uses AI to generate translations for missing keys
- Saves results back to your files

### `dire init`

- Creates a `.dire.yaml` configuration file
- Sets up default and example settings for your project
- Use `--force` flag to overwrite existing configuration

### `dire --keys <key>`

- Translates only specific keys instead of all missing ones
- Focuses on particular sections of your translations
- Supports multiple keys separated by commas

### `dire --sourced`

- Uses only your existing glossary entries
- Reuses existing translations found in your files
- Skips AI-powered translation generation
- No API key required

### `dire --context <context>`

- Provides additional context to the AI model
- Improves translation accuracy
- Ensures domain-specific terminology
- Maintains consistent tone and style

### `dire --stub`

- Creates empty string placeholders for all missing translations
- Skips AI translation generation
- Useful for setting up translation file structure
- No API key required

### `dire --rephrase --keys <key> --locale <locale>`

- Generates rephrase options for existing translations
- Requires both a specific key and target locale
- Helps improve translation quality and alternatives
- Only works with single keys, not multiple keys

### `dire --include-stubs`

- Treats empty string translations as missing translations
- Processes placeholder values that need actual translations
- Useful when working with incomplete translation files

### `dire --no-trim`

- Preserves leading/trailing spaces
- Maintains exact formatting
- Keeps intentional whitespace in translations
