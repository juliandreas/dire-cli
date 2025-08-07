# Dire CLI Commands

## Command Reference Table

| Command                    | Description                                         | Use Case                                   |
| -------------------------- | --------------------------------------------------- | ------------------------------------------ |
| `dire`                     | Translate all missing keys in your i18n files       | Main command for comprehensive translation |
| `dire init`                | Create a configuration file (`.dire.yaml`)          | Initial project setup                      |
| `dire --keys <key>`        | Translate specific key(s)                           | Focus on specific translation keys         |
| `dire --sourced`           | Apply only glossary and memory translations (no AI) | Quick updates without API costs            |
| `dire --context <context>` | Add context for better AI translations              | Improve translation accuracy               |
| `dire --stub`              | Create placeholder translations (empty strings)     | Set up translation file structure          |
| `dire --no-trim`           | Preserve whitespace in translations                 | Maintain exact formatting                  |
| `dire --debug`             | Enable debug output                                 | Troubleshooting                            |
| `dire --help`              | Show help information                               | Get command reference                      |
| `dire --version`           | Show current version                                | Check installed version                    |

## Detailed Usage Examples

### Basic Translation

```bash
# Translate all missing keys
dire

# Initialize project configuration
dire init
```

### Selective Translation

```bash
# Translate specific keys
dire --keys auth.login
dire --keys user.profile.name
dire --keys navigation.menu

# Translate multiple keys
dire --keys "auth.login,auth.register,auth.logout"
dire --keys "user.profile.name,user.profile.email,user.profile.phone"
```

### Translation Modes

```bash
# Use only existing glossary and memory (no AI)
dire --sourced

# Create empty placeholders
dire --stub

# Add context for better AI translations
dire --context "Dashboard UI"
dire --context "E-commerce checkout flow"
dire --context "Mobile app navigation"
```

### Formatting Options

```bash
# Preserve whitespace
dire --no-trim

# Enable debug mode
dire --debug
```

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

### `dire --keys <key>`

- Translates only specific keys instead of all missing ones
- Focuses on particular sections of your translations
- Supports multiple keys separated by commas

### `dire --sourced`

- Uses only your existing glossary entries
- Applies memory translations from previous runs
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

### `dire --no-trim`

- Preserves leading/trailing spaces
- Maintains exact formatting
- Keeps intentional whitespace in translations
