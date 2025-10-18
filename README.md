# Dire

<p align="center">
<img src="https://raw.githubusercontent.com/juliandreas/dire-cli/refs/heads/main/assets/dire.png" alt="Dire Logo">
</p>

<p align="center">
    <a href="https://www.npmjs.com/package/dire"><img src="https://img.shields.io/npm/v/dire" alt="npm version"></a>
    <a href="https://github.com/juliandreas/dire-cli"><img src="https://img.shields.io/badge/language-Go-00ADD8?logo=go" alt="Go"></a>
    <a href="https://github.com/juliandreas/dire-cli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-freeware-black" alt="License"></a>
</p>

<p align="center">
Dire (French for "to say", pronounced /diʁ/) is a Go-based CLI tool that automatically handles translation and maintenance of your i18n JSON files using translation and AI providers, glossary definitions, and translation memory
</p>

<p align="center">
BYOK (Bring Your Own Key) - Use your own provider API keys for maximum control and cost transparency
</p>

<p align="center">
<img src="https://raw.githubusercontent.com/juliandreas/dire-cli/refs/heads/main/assets/demo.gif" alt="Dire Demo" width="800">
</p>

## Features

- **Multi-provider support:** DeepL, Google Translate, Azure AI Translator, Claude, OpenAI, Gemini, Mistral, and DeepSeek
- **Automatic translation reuse:** sourcing from glossary definitions and translation memory
- **Translation rephrasing:** generate alternative phrasings to improve quality
- **Completeness checking:** lint translation files in CI/CD pipelines with `--check` flag
- **Cleanup orphaned keys:** remove translations that no longer exist in your reference locale with `--prune`
- **Context-aware translations:** provide domain-specific context for better results
- **Nested object support:** handles complex JSON structures and deeply nested translations
- **High-performance:** native Go binary with smart batching and concurrent processing

## Installation

```bash
# Option 1: Install as project dependency
npm i dire

# Option 2: Install globally
npm i -g dire

# Option 3: Run without installing
npx dire
```

## Quick Start

1. **Initialize Configuration**

   ```bash
   dire init
   ```

   This creates a `.dire.toml` file in your project root. Edit it to match your project structure. See [CONFIGURATION.md](docs/CONFIGURATION.md) for complete configuration reference.

   Note: The `.dire.toml` config file is optional. You can configure everything via CLI flags (see [Configuration Override Flags](docs/COMMANDS.md#configuration-override-flags)). However, the TOML file offers quality-of-life features like glossary management and multiple pre-configured providers for quick switching.

2. **Set Up API Keys**

   See [Supported Providers](#supported-providers) section below for setup details.

   Note: Dire automatically loads environment variables from any `.env*` files in your current directory (e.g., `.env`, `.env.local`, `.env.production`).

3. **Run Translation**

   ```bash
   dire
   ```

## Common Commands

| Command             | Description                                                         |
| ------------------- | ------------------------------------------------------------------- |
| `dire`              | Translate all missing keys in your i18n files                       |
| `dire --keys <key>` | Translate specific key(s): `dire --keys "auth.login,auth.register"` |
| `dire --sourced`    | Apply only glossary and memory translations                         |
| `dire --stub`       | Create placeholder translations (empty strings)                     |
| `dire --prune`      | Remove orphaned keys from non-reference locales                     |
| `dire --check`      | Lint translation completeness for CI/CD                             |

See [COMMANDS.md](docs/COMMANDS.md) for the complete command reference.

## Supported Providers

Set your API key in any `.env*` file in your project root:

| Provider                                                                                    | Environment Variable    |
| ------------------------------------------------------------------------------------------- | ----------------------- |
| [DeepL](https://www.deepl.com/)                                                             | `DIRE_DEEPL_API_KEY`    |
| [Google Translate](https://cloud.google.com/translate)                                      | `DIRE_GOOGLE_API_KEY`   |
| [Azure AI Translator](https://azure.microsoft.com/en-us/products/ai-services/ai-translator) | `DIRE_AZURE_API_KEY`    |
| [OpenAI](https://openai.com/)                                                               | `DIRE_OPENAI_API_KEY`   |
| [Claude](https://www.anthropic.com/)                                                        | `DIRE_CLAUDE_API_KEY`   |
| [Gemini](https://gemini.google.com/)                                                        | `DIRE_GEMINI_API_KEY`   |
| [Mistral](https://mistral.ai/)                                                              | `DIRE_MISTRAL_API_KEY`  |
| [DeepSeek](https://deepseek.com/)                                                           | `DIRE_DEEPSEEK_API_KEY` |

Example `.env` file:

```bash
DIRE_CLAUDE_API_KEY=sk-ant-api03-your-key-here
```

## Requirements

- Node.js 20 or higher
- API key for at least one supported provider (if using the translation features)

## License

All rights reserved. This software is provided for use only and may not be copied, modified, or redistributed without explicit permission.
