<h1 align="center">Dire</h1>

<p align="center">
Go based CLI to automatically processes missing translations in your i18n files by using AI, glossary, and pre-determined translations
</p>

<p align="center">
<img src="./assets/dire.png" alt="Dire Logo">
</p>

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
   dire --init
   ```

   This creates a `.dire.yaml` file in your project root. Edit it to match your project structure.

3. **Set Up API Keys**

   Add your API key(s) in your `.env` file.

4. **Run Translation**
   ```bash
   dire
   ```
   That's it! dire will automatically detect missing translations and use AI to fill them in.

## Features

- **High-performance Go engine:** native binary for maximum speed
- **Multi-provider AI support:** Claude, OpenAI, Gemini, Mistral, and DeepSeek
- **Smart translation reuse:** automatically reuses existing translations
- **Nested JSON support:** handles complex translation file structures
- **Intelligent batching:** optimized for large translation jobs
- **Style customization:** configure tone, formality, and domain-specific terminology
- **Automatic retries:** handles API failures gracefully

## Supported Providers

| Provider                             | Environment Variable    |
| ------------------------------------ | ----------------------- |
| [Claude](https://www.anthropic.com/) | `DIRE_CLAUDE_API_KEY`   |
| [OpenAI](https://openai.com/)        | `DIRE_OPENAI_API_KEY`   |
| [Gemini](https://gemini.google.com/) | `DIRE_GEMINI_API_KEY`   |
| [Mistral](https://mistral.ai/)       | `DIRE_MISTRAL_API_KEY`  |
| [DeepSeek](https://deepseek.com/)    | `DIRE_DEEPSEEK_API_KEY` |

## Requirements

- Node.js 20 or higher
- API key for at least one supported AI provider

## License

All rights reserved. This software is provided for use only and may not be copied, modified, or redistributed without explicit permission.
