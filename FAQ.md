# Frequently Asked Questions

## Getting Started

### How do I get started with Dire?

1. Run `dire init` to create a `.dire.yaml` configuration file
2. Edit the config to specify your project details and translation file locations
3. Set up API keys as environment variables (e.g., `DIRE_CLAUDE_API_KEY`)
4. Run `dire` from the directory containing your `.dire.yaml` file to start translating missing keys

### What file formats does Dire support?

Dire currently supports JSON files with nested object structures. Each locale should be a separate JSON file (e.g., `en.json`, `fr.json`, `de.json`).

### Which providers are supported?

Dire supports:

- **DeepL** - Set `DIRE_DEEPL_API_KEY`
- **OpenAI** - Set `DIRE_OPENAI_API_KEY`
- **Claude** - Set `DIRE_CLAUDE_API_KEY`
- **Google Gemini** - Set `DIRE_GEMINI_API_KEY`
- **Mistral** - Set `DIRE_MISTRAL_API_KEY`
- **DeepSeek** - Set `DIRE_DEEPSEEK_API_KEY`

Configure providers in `.dire.yaml` and set your active provider.

## Configuration

### Why do I need to use `en-US` instead of `en`?

Dire requires full BCP-47 locale codes (language + region). Bare language codes like `en` are ambiguous (US, GB, AU, …). Use:

- ✅ `en-US`, `en-GB`, `fr-FR`, `de-DE`
- ❌ `en`, `fr`, `de`

### How do I configure multiple locales?

In your `.dire.yaml` file, map filename patterns to locale codes in the `files.locales` section:

```yaml
files:
  locales:
    "en.json": "en-US"
    "fr.json": "fr-FR"
    "de.json": "de-DE"
```

## Usage Modes

### When should I use `--sourced` vs regular translation?

- **`--sourced`**: Apply only glossary and translation memory without any API costs. Good for quick updates using existing translations.
- **Regular mode**: Use the configured provider to generate new translations for missing keys. Costs API usage but handles new content.

### What does `--stub` mode do?

`--stub` creates empty string placeholders for all missing translation keys without using any API. This is useful when you want to:

- Set up translation file structure with all keys present
- Mark certain keys as "not ready for translation" initially
- Create a complete file structure before manual or provider translation

**Important**: Keys with empty string values are normally ignored during translation. To translate these stubbed keys later, use `--include-stubs` with your regular translation command:

```bash
# First, create stubs for all missing keys
dire --stub

# Later, translate the stubbed (empty) values
dire --include-stubs
```

## Features

### How does the glossary feature work?

A **glossary** is your predefined dictionary of key terms and their exact translations across languages. It ensures that important terms are always translated consistently, regardless of context.

Define these terms in your `.dire.yaml`:

```yaml
glossary:
  - { en-US: "dashboard", fr-FR: "tableau de bord", de-DE: "Dashboard" }
  - { en-US: "API", fr-FR: "API", de-DE: "API" }
  - { en-US: "checkout", fr-FR: "validation", de-DE: "Kasse" }
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

### What happens to whitespace and key ordering in translations?

**Whitespace handling:**

- **Default**: Dire trims leading/trailing whitespace from translations
- **Preserve formatting**: Use `--no-trim` to maintain exact whitespace when needed

**Key ordering:**

- **Always alphabetical**: Dire automatically sorts all translation keys alphabetically for consistency
- This is an opinionated design choice to ensure predictable file structure and easier diffs
- There is no flag to disable alphabetical sorting

## Troubleshooting

### Why am I getting "configuration file not found" errors?

1. Run `dire init` to create a `.dire.yaml` file
2. Make sure you're in the correct directory
3. In development mode, ensure config is at `demo/.dire.yaml`

### My API key isn't working - what should I check?

Verify you're using the correct environment variable name:

- `DIRE_CLAUDE_API_KEY` for Claude
- `DIRE_OPENAI_API_KEY` for OpenAI
- `DIRE_GEMINI_API_KEY` for Gemini
- And so on...

Also ensure your active provider in `.dire.yaml` matches the API key you've set.

### Can I use multiple flags together?

Some flag combinations are **not allowed**:

- `--stub` and `--sourced` together
- `--stub` and `--include-stubs` together

Some combinations **work fine**:

- `--sourced` and `--include-stubs`
- `--keys` with most other flags

## Design Choices

### Is Dire opinionated about file formatting?

Yes! Dire makes several opinionated decisions to ensure consistency:

**File Structure:**

- **Alphabetical key sorting**: All translation keys are automatically sorted alphabetically
- **Consistent formatting**: JSON output follows a standardized format
- **Whitespace normalization**: Leading/trailing whitespace is trimmed by default (use `--no-trim` to override)

**Why these choices?**

- Ensures predictable file structure across teams
- Makes Git diffs more meaningful and easier to review
- Reduces merge conflicts caused by inconsistent formatting
- Maintains consistency even when multiple developers work on translations

## Limitations

### Are there any response size limits?

Yes, API responses are capped at 1MB to prevent memory issues. For extremely large translation jobs, consider breaking them into smaller batches.

## Best Practices

### What's the recommended translation workflow?

1. **Setup**: Run `dire init` and configure your `.dire.yaml`
2. **Glossary**: Add key terms to your glossary for consistency
3. **Translate**: Run `dire` to translate missing keys (automatically applies glossary and memory, then uses the configured provider for new content)
4. **Refinement**: Use `dire --rephrase` to improve specific translations

**Alternative approaches:**

- Use `dire --sourced` for updates when you only want glossary/memory translations (no provider costs)
- Use `dire --keys` to focus on specific translation keys

### Any tips for better translations?

- Use descriptive key names (e.g., `checkout.payment.creditCard` vs `cc`)
- Add context with `--context` for domain-specific translations
- Maintain a comprehensive glossary for consistent terminology
- Review and refine translations using `--rephrase` mode
