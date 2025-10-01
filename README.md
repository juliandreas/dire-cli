# Dire

<p align="center">
<img src="https://raw.githubusercontent.com/juliandreas/dire-cli/refs/heads/main/assets/dire.png" alt="Dire Logo">
</p>

<p align="center">
    <a href="https://github.com/juliandreas/dire-cli"><img src="https://img.shields.io/badge/language-Go-00ADD8?logo=go" alt="Go"></a>
    <a href="https://www.npmjs.com/package/dire"><img src="https://img.shields.io/npm/v/dire" alt="npm version"></a>
    <a href="https://github.com/juliandreas/dire-cli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-freeware-black" alt="License"></a>
</p>

<p align="center">
Dire (French for "to say", pronounced <em>/diʁ/</em>) is a Go-based CLI that automatically processes missing translations in your i18n files using translation providers, glossary definitions, and translation memory
</p>

<p align="center">
BYOK (Bring Your Own Key) - Use your own provider API keys for maximum control and cost transparency
</p>

<p align="center">
<img src="https://raw.githubusercontent.com/juliandreas/dire-cli/refs/heads/main/assets/demo.gif" alt="Dire Demo" width="800">
</p>

## Features

- **Multi-provider support:** DeepL, Claude, OpenAI, Gemini, Mistral, and DeepSeek
- **Intelligent translation reuse:** automatic sourcing from glossary definitions and translation memory
- **Translation rephrasing:** generate alternative phrasings to improve quality
- **CI/CD integration:** lint translation completeness in build pipelines with `--check` flag
- **Glossary management:** ensure consistent terminology across all translations
- **High-performance:** native Go binary with smart batching and concurrent processing
- **Nested object support:** handles complex JSON structures and deeply nested translations
- **Context-aware translations:** provide domain-specific context for better results

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

   This creates a `.dire.toml` file in your project root. Edit it to match your project structure. You can also reference the [config.template.toml](config.template.toml) for all available options.

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
| `dire --check`      | Lint translation completeness for CI/CD                             |

See [COMMANDS.md](COMMANDS.md) for the complete command reference.

## Supported Providers

Set your API key in any `.env*` file in your project root:

| Provider                             | Environment Variable    |
| ------------------------------------ | ----------------------- |
| [DeepL](https://www.deepl.com/)      | `DIRE_DEEPL_API_KEY`    |
| [OpenAI](https://openai.com/)        | `DIRE_OPENAI_API_KEY`   |
| [Claude](https://www.anthropic.com/) | `DIRE_CLAUDE_API_KEY`   |
| [Gemini](https://gemini.google.com/) | `DIRE_GEMINI_API_KEY`   |
| [Mistral](https://mistral.ai/)       | `DIRE_MISTRAL_API_KEY`  |
| [DeepSeek](https://deepseek.com/)    | `DIRE_DEEPSEEK_API_KEY` |

Example `.env` file:

```bash
DIRE_CLAUDE_API_KEY=sk-ant-api03-your-key-here
```

## Requirements

- Node.js 20 or higher
- API key for at least one supported provider (if using the translation features)

## License

All rights reserved. This software is provided for use only and may not be copied, modified, or redistributed without explicit permission.
