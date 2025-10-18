# Configuration Reference

This document provides a complete reference for configuring Dire through the `.dire.toml` configuration file.

## Overview

The `.dire.toml` file controls how Dire processes your translation files. While all configuration can be overridden via CLI flags (see [COMMANDS.md](COMMANDS.md#configuration-override-flags)), the TOML file provides a convenient way to store project settings, manage glossaries, and configure multiple providers.

Create a default configuration file by running in the root of your project:

```bash
dire init
```

## Quick Reference

| Section        | Purpose                                        | Required |
| -------------- | ---------------------------------------------- | -------- |
| `[files]`      | Translation file locations and locale mappings | Required |
| `[providers]`  | Translation provider configuration             | Required |
| `[project]`    | Project context for better translations        | Optional |
| `[styleGuide]` | Translation style preferences                  | Optional |
| `[glossary]`   | Custom terminology dictionary                  | Optional |

## Providers Configuration

### Valid Provider Names

Provider names in `.dire.toml` must use **exact lowercase names**. The `providers.active` field accepts:

| Provider Name    | Type       | Environment Variable    |
| ---------------- | ---------- | ----------------------- |
| `deepl`          | Translator | `DIRE_DEEPL_API_KEY`    |
| `google`         | Translator | `DIRE_GOOGLE_API_KEY`   |
| `azure-<region>` | Translator | `DIRE_AZURE_API_KEY`    |
| `claude`         | LLM        | `DIRE_CLAUDE_API_KEY`   |
| `openai`         | LLM        | `DIRE_OPENAI_API_KEY`   |
| `gemini`         | LLM        | `DIRE_GEMINI_API_KEY`   |
| `mistral`        | LLM        | `DIRE_MISTRAL_API_KEY`  |
| `deepseek`       | LLM        | `DIRE_DEEPSEEK_API_KEY` |

**Important:**

- Use lowercase names: `deepl` not `DeepL`, `openai` not `OpenAI`
- See [Azure special case](#azure-special-case) below for region requirements

### Azure Special Case

Azure AI Translator requires a region suffix in the provider name. You **cannot** use just `azure`.

**Valid formats:**

```toml
[providers]
active = "azure-global"        # Global endpoint
active = "azure-swedencentral" # Sweden region
active = "azure-westeurope"    # West Europe region
active = "azure-eastus"        # East US region
```

The region should match your Azure AI Translator resource location. Common regions include:

- `azure-global` - Global multi-region endpoint
- `azure-swedencentral` - Sweden Central
- `azure-westeurope` - West Europe
- `azure-eastus` - East US
- `azure-westus` - West US
- `azure-northeurope` - North Europe
- `azure-southeastasia` - Southeast Asia

### Provider Configuration Examples

#### Using Translation Services (Simple)

Translation services (DeepL, Google, Azure) don't require provider configuration blocks - just set the active provider and API key:

```toml
[providers]
active = "deepl"
```

```bash
# In your .env file
DIRE_DEEPL_API_KEY=your-key-here
```

#### Using LLM Services (Requires Model Configuration)

LLM providers require a configuration block specifying the model and parameters:

```toml
[providers]
active = "claude"

[[providers.configuration]]
name = "claude"
model = "claude-3-5-haiku-latest"
maxTokens = 8192
temperature = 0.2
```

```bash
# In your .env file
DIRE_CLAUDE_API_KEY=your-key-here
```

#### Multiple Provider Setup (Easy Switching)

You can configure multiple providers and switch between them:

```toml
[providers]
# Currently active provider
active = "openai"

# LLM providers available for switching
[[providers.configuration]]
name = "claude"
model = "claude-3-5-haiku-latest"
maxTokens = 8192
temperature = 0.2

[[providers.configuration]]
name = "openai"
model = "gpt-4-turbo"
maxTokens = 16000
temperature = 0.2

[[providers.configuration]]
name = "gemini"
model = "gemini-1.5-flash"
maxTokens = 32000
temperature = 0.2
```

Switch providers by updating the "active" provider in the config or by using the CLI flag:

```bash
dire --provider-active claude
dire --provider-active openai
dire --provider-active deepl
```

### LLM Provider Parameters

When configuring LLM providers, these parameters are required:

| Parameter     | Description                                        | Example Values                               |
| ------------- | -------------------------------------------------- | -------------------------------------------- |
| `name`        | Provider identifier (must match valid names above) | `"claude"`, `"openai"`                       |
| `model`       | Model identifier from provider                     | `"claude-3-5-haiku-latest"`, `"gpt-4-turbo"` |
| `maxTokens`   | Maximum tokens for response                        | `8192`, `16000`, `32000`                     |
| `temperature` | Randomness (0.0-1.0, lower = more consistent)      | `0.2` (recommended)                          |

## Files Configuration

Controls where Dire looks for translation files and how it maps them to locales.

### Basic Structure

```toml
[files]
directory = "./locales"  # Path to translation files (relative or absolute)
indent = 2               # JSON indentation (optional, defaults to 2)

[files.locales]
"en.json" = "en-US"
"fr.json" = "fr-FR"
"de.json" = "de-DE"
```

### Options

| Field       | Type   | Required | Description                                        |
| ----------- | ------ | -------- | -------------------------------------------------- |
| `directory` | string | Yes      | Path to folder containing translation JSON files   |
| `indent`    | number | No       | Number of spaces for JSON indentation (default: 2) |
| `locales`   | map    | Yes      | Mapping of filenames to BCP-47 locale codes        |

### Locale Codes (BCP-47 Format)

Dire requires **full BCP-47 locale codes** (language + region). Bare language codes are not allowed.

**Valid:**

- `en-US` (English - United States)
- `en-GB` (English - United Kingdom)
- `fr-FR` (French - France)
- `de-DE` (German - Germany)
- `es-ES` (Spanish - Spain)
- `pt-BR` (Portuguese - Brazil)
- `zh-CN` (Chinese - China)
- `ja-JP` (Japanese - Japan)

**Invalid:**

- `en` (ambiguous: US, GB, AU?)
- `fr` (ambiguous: FR, CA?)
- `de` (ambiguous: DE, AT, CH?)

### Example: Multiple Locales

```toml
[files]
directory = "./src/i18n"

[files.locales]
"en.json" = "en-US"
"fr.json" = "fr-FR"
"de.json" = "de-DE"
"es.json" = "es-ES"
"pt-br.json" = "pt-BR"
"ja.json" = "ja-JP"
"zh-hans.json" = "zh-CN"
```

## Project Context

Optional section that provides domain-specific context to LLM providers for better translations.

```toml
[project]
name = "E-commerce Platform"
description = "Online shopping platform with user accounts, product catalog, and order management"
domain = "e-commerce"
```

### Options

| Field         | Type   | Description               | Example                                     |
| ------------- | ------ | ------------------------- | ------------------------------------------- |
| `name`        | string | Project name              | `"My App"`, `"Acme CRM"`                    |
| `description` | string | Brief project description | `"Healthcare patient portal"`               |
| `domain`      | string | Industry/domain           | `"healthcare"`, `"fintech"`, `"e-commerce"` |

**Benefits:**

- Helps AI understand technical terminology
- Improves domain-specific translation accuracy
- Maintains consistent tone for your industry

**Note:** Only affects LLM providers (Claude, OpenAI, etc.). Translation services ignore this configuration.

## Style Guide

Controls translation style and formality.

```toml
[styleGuide]
formality = "automatic"  # automatic | formal | informal
```

### Formality Options

| Value       | Description                                 | Use Case                              |
| ----------- | ------------------------------------------- | ------------------------------------- |
| `automatic` | Provider decides based on context (default) | Most applications                     |
| `formal`    | Use formal language and pronouns            | Business, legal, official documents   |
| `informal`  | Use casual language and pronouns            | Social apps, gaming, casual platforms |

### Example: Formal Business Application

```toml
[styleGuide]
formality = "formal"
```

**Note:** Formality support varies by provider. DeepL has the most robust support. LLM providers attempt to follow formality instructions but may be less consistent.

## Glossary

Define custom terminology that should be translated consistently across all locales.

```toml
[glossary]
autoSort = true  # Automatically sort entries alphabetically
entries = [
    { "en-US" = "dashboard", "fr-FR" = "tableau de bord", "de-DE" = "Dashboard" },
    { "en-US" = "API", "fr-FR" = "API", "de-DE" = "API" },
    { "en-US" = "checkout", "fr-FR" = "validation", "de-DE" = "Kasse" }
]
```

### How Glossary Works

1. **Bi-directional matching**: Works when translating from any language to any other language
2. **Case-insensitive matching**: Glossary terms match regardless of case, and the casing pattern is preserved (e.g., `"api"` in glossary matches both `"API"` and `"api"` in source text)
3. **Priority**: Glossary translations override provider translations
4. **No API costs**: Glossary translations don't require provider API calls

### Use Cases

- **Brand names**: Ensure product/feature names are translated consistently
- **Technical terms**: Define domain-specific terminology
- **Legal terms**: Maintain exact legal language
- **Abbreviations**: Control how acronyms are handled

### Auto-Sort

When `autoSort = true`, Dire automatically sorts glossary entries alphabetically by the first locale code in the entry. This keeps your glossary organized and makes it easier to find and maintain terms.

## Complete Example

Here's a complete `.dire.toml` configuration with all sections:

```toml
# Files configuration
[files]
directory = "./locales"
indent = 2 # optional, defaults to 2

# Map file names to locale codes (BCP 47 format required)
[files.locales]
"en.json" = "en-US"
"fr.json" = "fr-FR"
"de.json" = "de-DE"
"es.json" = "es-ES"

# Translation providers configuration
[providers]
# Active provider (use exact lowercase names)
active = "deepl"

# LLM provider configurations (optional, for easy switching)
[[providers.configuration]]
name = "claude"
model = "claude-3-5-haiku-latest"
maxTokens = 8192
temperature = 0.2

[[providers.configuration]]
name = "openai"
model = "gpt-4o-mini"
maxTokens = 16000
temperature = 0.2

[[providers.configuration]]
name = "gemini"
model = "gemini-1.5-flash"
maxTokens = 32000
temperature = 0.2

# Project context - helps AI understand your domain (optional)
[project]
name = "E-commerce Platform"
description = "Online shopping platform with user accounts, product catalog, and order management"
domain = "e-commerce"

# Style guide configuration (optional)
[styleGuide]
formality = "automatic"  # automatic | formal | informal

# Glossary for consistent translations (optional)
[glossary]
autoSort = true
entries = [
    { "en-US" = "API", "fr-FR" = "API", "de-DE" = "API", "es-ES" = "API" },
    { "en-US" = "cart", "fr-FR" = "panier", "de-DE" = "Warenkorb", "es-ES" = "carrito" },
    { "en-US" = "checkout", "fr-FR" = "passer commande", "de-DE" = "zur Kasse", "es-ES" = "finalizar compra" },
    { "en-US" = "dashboard", "fr-FR" = "tableau de bord", "de-DE" = "Dashboard", "es-ES" = "panel" },
    { "en-US" = "PayPal", "fr-FR" = "PayPal", "de-DE" = "PayPal", "es-ES" = "PayPal" },
]
```

## Environment Variables

API keys must be set as environment variables. Dire automatically loads from `.env*` files in your project root.

You only need to set the API key for the provider(s) you're using.

## CLI Override Flags

All configuration options can be overridden via CLI flags. See [COMMANDS.md](COMMANDS.md#configuration-override-flags) for details.

**Example:**

```bash
# Override provider without editing config
dire --provider-active claude

# Override multiple settings
dire --provider-active openai --formality formal --context "Legal documents"
```

## Validation

Dire validates your configuration when you run commands. Common validation errors:

| Error                | Cause                         | Solution                                           |
| -------------------- | ----------------------------- | -------------------------------------------------- |
| Invalid provider     | Using wrong name format       | Use exact lowercase names: `deepl`, not `DeepL`    |
| Azure missing region | Using `azure` without region  | Use `azure-global` or `azure-<region>`             |
| Invalid locale code  | Using bare language codes     | Use BCP-47 format: `en-US`, not `en`               |
| Missing API key      | Environment variable not set  | Set `DIRE_<PROVIDER>_API_KEY` in `.env` file       |
| LLM missing model    | Provider config without model | Add `[[providers.configuration]]` block with model |

## See Also

- [COMMANDS.md](COMMANDS.md) - Complete command reference
- [FAQ.md](FAQ.md) - Common questions and troubleshooting
- [README.md](README.md) - Quick start guide
