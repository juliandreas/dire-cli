# Dire CLI Commands

## Basic Commands

### `dire`

Translate all missing keys in your i18n files.

```bash
dire
```

This is the main command that will:

- Scan your configured i18n files
- Identify missing translations
- Apply glossary and memory translations
- Use AI to generate translations for missing keys
- Save the results back to your files

### `dire init`

Create a configuration file (`.dire.yaml`) in your project root.

```bash
dire init
```

This command will:

- Create a `.dire.yaml` configuration file
- Set up default and example settings for your project

## Translation Options

### `dire --keys <key>`

Translate specific key value or object. You can specify multiple keys separated by commas.

```bash
dire --keys auth.login
dire --keys user.profile.name
dire --keys navigation.menu
dire --keys "auth.login,auth.register,auth.logout"
dire --keys "user.profile.name,user.profile.email,user.profile.phone"
```

Use this when you want to:

- Translate only specific keys instead of all missing ones
- Focus on particular sections of your translations

### `dire --sourced`

Only apply glossary and memory translations without using AI. No API-key is required for this command.

```bash
dire --sourced
```

This command will:

- Use only your existing glossary entries
- Apply memory translations from previous runs
- Skip AI-powered translation generation
- Useful for quick updates or when you want to avoid API costs

### `dire --context <context>`

Add context for better AI translations.

```bash
dire --context "Dashboard UI"
dire --context "E-commerce checkout flow"
dire --context "Mobile app navigation"
```

This provides additional context to the AI model for:

- More accurate translations
- Domain-specific terminology
- Consistent tone and style
- Better understanding of the translation context

### `dire --stub`

Create placeholder translations with empty strings. No API-key is required for this command.

```bash
dire --stub
```

This command will:

- Create empty string placeholders for all missing translations
- Skip AI translation generation
- Useful for setting up translation file structure
- Help identify which keys need translation without using API calls

### `dire --no-trim`

Preserve whitespace in translations.

```bash
dire --no-trim
```

By default, the application trims whitespace from translations. Use this flag to:

- Preserve leading/trailing spaces
- Maintain exact formatting
- Keep intentional whitespace in your translations

## Miscellaneous Commands

### `dire --debug`

Enable debug output for debugging.

```bash
dire --debug
```

### `dire --help`

Show help information and available commands.

```bash
dire --help
```

### `dire --version`

Show the current version of dire.

```bash
dire --version
```
