# Frequently Asked Questions

## Getting Started

### How do I get started with Dire?

1. Run `dire init` in your project directory to create a `.dire.toml` configuration file
2. Edit the config to specify your project details and translation file locations
3. Set up API keys as env variables in your `.env` file (e.g., `DIRE_DEEPL_API_KEY`)
4. Run `dire` from the directory containing your `.dire.toml` file to start translating missing keys

### What file formats does Dire support?

Dire currently supports JSON files with nested object structures. Each locale should be a separate JSON file (e.g., `en.json`, `fr.json`, `de.json`).

### Which providers are supported?

Dire supports:

- **DeepL** - Set `DIRE_DEEPL_API_KEY`
- **Google Translate** - Set `DIRE_GOOGLE_API_KEY`
- **Azure AI Translator** - Set `DIRE_AZURE_API_KEY`
- **OpenAI** - Set `DIRE_OPENAI_API_KEY`
- **Claude** - Set `DIRE_CLAUDE_API_KEY`
- **Google Gemini** - Set `DIRE_GEMINI_API_KEY`
- **Mistral** - Set `DIRE_MISTRAL_API_KEY`
- **DeepSeek** - Set `DIRE_DEEPSEEK_API_KEY`

Configure providers in `.dire.toml` and set your active provider.

## Configuration

### Where can I find the complete configuration reference?

See [CONFIGURATION.md](CONFIGURATION.md) for a comprehensive guide covering:

- Valid provider names
- Provider categories (LLM vs translator services)
- Complete `.dire.toml` file structure
- Glossary configuration
- Project context and style guide options
- Examples for all configuration scenarios

### Why do I need to use `en-US` instead of `en`?

Dire requires full BCP-47 locale codes (language + region). Bare language codes like `en` are ambiguous (US, GB, AU, …). Use:

- ✅ `en-US`, `en-GB`, `fr-FR`, `de-DE`
- ❌ `en`, `fr`, `de`

### How do I configure multiple locales?

In your `.dire.toml` file, map filename patterns to locale codes in the `[files.locales]` section:

```toml
# Map file names/patterns to languages
[files.locales]
"en.json" = "en-US"
"sv.json" = "sv-SE"
"da.json" = "da-DK"
```

## Flags

### What is the `--sourced` flag?

The `--sourced` flag applies translations only from your glossary and translation memory, without making any API calls to translation providers. This means zero API costs.

During normal translation mode, Dire automatically checks glossary and translation memory first before calling the provider. However, use the `--sourced` flag when you want to apply **only** these local translations without any provider involvement - useful for quick updates using your existing translation resources.

### What does the `--stub` flag do?

The `--stub` flag creates empty string placeholders for all missing translation keys without using any API. This is useful when you want to:

- Set up translation file structure with all keys present
- Mark certain keys as "not ready for translation" initially
- Create a complete file structure before manual or provider translation

**Important**: Keys with empty string values are ignored during translation. To translate these stubbed keys later, use `--include-stubs` with your regular translation command:

```bash
# First, create stubs for all missing keys
dire --stub

# Later, translate the stubbed (empty) values
dire --include-stubs
```

## Features

### How does the glossary feature work?

A **glossary** is your predefined dictionary of key terms and their exact translations across languages. It ensures that important terms are always translated consistently, regardless of context. The glossary is **bi-directional** - it works when translating from any language to any other language in your configuration.

Define these terms in your `.dire.toml`:

```toml
# Glossary configuration
[glossary]
autoSort = true
entries = [
    { "en-US" = "dashboard", "fr-FR" = "tableau de bord", "de-DE" = "Dashboard" },
    { "en-US" = "API", "fr-FR" = "API", "de-DE" = "API" },
    { "en-US" = "checkout", "fr-FR" = "validation", "de-DE" = "Kasse" }
]
```

**Why use a glossary?**

- **Brand consistency**: Ensure product names, features, and technical terms are translated identically
- **Override provider**: Glossary entries take priority over provider translations
- **Domain expertise**: Use your team's knowledge of specialized terminology
- **Quality control**: Prevent the provider from making incorrect assumptions about context-specific terms

### Does Dire remember previous translations?

Yes! Dire has **translation memory** - it remembers translations by looking at your existing JSON files.

**How translation memory works:**

- **Scans existing translations**: Dire looks through all your current translation files
- **Finds exact matches**: If the same text exists elsewhere with a translation, it reuses that
- **No API costs**: Reused translations don't require new provider calls
- **Uses your own data**: All memory comes from your existing translation files

**Benefits:**

- **Speed**: Skip re-translating content you've already handled
- **Consistency**: Same text always gets the same translation
- **Cost efficiency**: Reduce API usage by reusing previous work
- **Quality**: Build up a library of vetted translations over time

**Example**: If you translate "Save changes" to "Enregistrer les modifications" in French once, Dire will automatically use that same translation everywhere "Save changes" appears, without calling the provider again.

### Can I add context to improve AI translations?

Use the `--context` flag to provide additional context (max 500 characters):

```bash
dire --context "E-commerce checkout flow for online shopping"
```

Context helps the AI understand the domain and generate more accurate translations.

### How does Dire handle JSON formatting?

**Key ordering:**

- **Always alphabetical**: Dire automatically sorts all translation keys alphabetically for consistency
- This is an opinionated design choice to ensure predictable file structure and easier diffs

**Whitespace handling:**

- **Default**: Dire trims leading/trailing whitespace from translations
- **Preserve formatting**: Use `--no-trim` to maintain exact whitespace when needed

**Indentation:**

- **Default**: Uses 2 spaces for JSON indentation
- **Custom indentation**: Use `--indent` flag to override (e.g., `--indent 4` for 4 spaces)

## Troubleshooting

### Why am I getting "configuration file not found" errors?

1. Run `dire init` to create a `.dire.toml` file
2. Make sure you're in the correct directory

### My API key isn't working - what should I check?

Verify you're using the correct environment variable name:

- `DIRE_DEEPL_API_KEY` for DeepL
- `DIRE_GOOGLE_API_KEY` for Google Translate
- `DIRE_CLAUDE_API_KEY` for Claude
- `DIRE_OPENAI_API_KEY` for OpenAI
- `DIRE_GEMINI_API_KEY` for Gemini
- And so on...

Also ensure your active provider in `.dire.toml` matches the API key you've set.

### Can I use multiple flags together?

Some flag combinations are **not allowed**:

- `--stub` and `--sourced` together
- `--stub` and `--include-stubs` together

Some combinations **work fine**:

- `--sourced` and `--include-stubs`
- `--keys` with most other flags

## Best Practices

### What's the recommended translation workflow?

1. **Setup**: Run `dire init` and configure your `.dire.toml`
2. **Glossary**: Add key terms to your glossary for consistency
3. **Gradual Translation**: Even though you can translate thousands of keys in seconds, we recommend translating gradually so you can review the results:
   - Use `--keys` to target specific keys or objects (e.g., `--keys auth,user.profile`)
   - Review translations before moving to the next batch
4. **Full Translation**: Once comfortable, run `dire` without filters to translate all remaining keys
