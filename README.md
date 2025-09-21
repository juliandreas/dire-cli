# Dire

<p align="center">
<img src="https://raw.githubusercontent.com/juliandreas/dire-cli/refs/heads/main/assets/dire.png" alt="Dire Logo">
</p>

<p align="center">
    <a href="https://github.com/juliandreas/dire-cli/releases"><img src="https://img.shields.io/github/release/juliandreas/dire-cli" alt="Latest Release"></a>
    <a href="https://github.com/juliandreas/dire-cli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-freeware-black" alt="License"></a>
</p>

<p align="center">
Dire (French for "to say", pronounced <em>/diʁ/</em>) is a Go-based CLI that automatically processes missing translations in your i18n files by using AI, glossary, and pre-determined translations
</p>

<p align="center">
BYOK (Bring Your Own Key) - Use your own AI provider API keys for maximum control and cost transparency
</p>

<p align="center">
<img src="https://raw.githubusercontent.com/juliandreas/dire-cli/refs/heads/main/assets/demo.gif" alt="Dire Demo" width="800">
</p>

## Features

- **High-performance Go engine:** native binary for maximum speed
- **Multi-provider AI support:** Claude, OpenAI, Gemini, Mistral, and DeepSeek
- **CI/CD integration:** lint translation completeness in build pipelines with `--check` flag
- **Style customization:** configure tone, formality, and domain-specific terminology
- **Intelligent translation sourcing:** automatic reuse through glossary definitions and existing translations in your files
- **Translation rephrasing:** generate alternative phrasings for existing translations to improve quality
- **Optimized translation engine:** smart batching, bi-directional translation pairs, and concurrent processing for maximum efficiency and quality
- **Nested object support:** handles complex JSON structures and deeply nested translations

## Installation

```bash
npm install -g dire
```

## Quick Start

1. **Install the CLI**

   ```bash
   # Install globally (recommended for CLI usage)
   npm install -g dire

   # Or install as a project dependency
   npm install dire
   ```

2. **Initialize Configuration**

   ```bash
   dire init
   ```

   This creates a `.dire.yaml` file in your project root. Edit it to match your project structure.

3. **Set Up API Keys**

   See [Supported Providers](#supported-providers) section below for setup details.

   Note: Dire automatically loads environment variables from any `.env*` files in your current directory (e.g., `.env`, `.env.local`, `.env.production`). Files are loaded with proper precedence where `.env.local` overrides `.env`.

4. **Run Translation**

   ```bash
   dire
   ```

   That's it! dire will automatically detect missing translations and use AI to fill them in.

## Common Commands

| Command                                          | Description                                                                        |
| ------------------------------------------------ | ---------------------------------------------------------------------------------- |
| `dire --keys <key>`                              | Translate specific key(s): `dire --keys "auth.login,auth.register"`                |
| `dire --rephrase --keys <key> --locale <locale>` | Generate alternative phrasings: `dire --rephrase --keys auth.login --locale en-US` |
| `dire --context <context>`                       | Add context for better translations: `dire --context "E-commerce checkout"`        |
| `dire --stub`                                    | Create placeholder translations (empty strings)                                    |
| `dire --check`                                   | Lint translation completeness for CI/CD                                            |

See [COMMANDS.md](COMMANDS.md) for the complete command reference.

## Supported Providers

Set your API key in any `.env*` file in your project root:

| Provider                             | Environment Variable    |
| ------------------------------------ | ----------------------- |
| [DeepL](https://www.deepl.com/)      | `DIRE_DEEPL_API_KEY`    |
| [Claude](https://www.anthropic.com/) | `DIRE_CLAUDE_API_KEY`   |
| [OpenAI](https://openai.com/)        | `DIRE_OPENAI_API_KEY`   |
| [Gemini](https://gemini.google.com/) | `DIRE_GEMINI_API_KEY`   |
| [Mistral](https://mistral.ai/)       | `DIRE_MISTRAL_API_KEY`  |
| [DeepSeek](https://deepseek.com/)    | `DIRE_DEEPSEEK_API_KEY` |

Example `.env` file:

```bash
DIRE_CLAUDE_API_KEY=sk-ant-api03-your-key-here
```

## Requirements

- Node.js 20 or higher
- API key for at least one supported AI provider (if using the AI features)

## License

All rights reserved. This software is provided for use only and may not be copied, modified, or redistributed without explicit permission.
