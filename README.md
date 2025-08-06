<h1 align="center">Dire</h1>

<p align="center">
Go based CLI to automatically translate your i18n JSON files to any language
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
   Add your API keys as environment variables:

   ```bash
   DIRE_CLAUDE_API_KEY=your_api_key
   DIRE_OPENAI_API_KEY=your_api_key
   DIRE_GEMINI_API_KEY=your_api_key
   DIRE_MISTRAL_API_KEY=your_api_key
   DIRE_DEEPSEEK_API_KEY=your_api_key
   ```

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

| Provider                             |
| ------------------------------------ |
| [Claude](https://www.anthropic.com/) |
| [OpenAI](https://openai.com/)        |
| [Gemini](https://gemini.google.com/) |
| [Mistral](https://mistral.ai/)       |
| [DeepSeek](https://deepseek.com/)    |

## Requirements

- Node.js 20 or higher
- API key for at least one supported AI provider

## License

All rights reserved. This software is provided for use only and may not be copied, modified, or redistributed without explicit permission.
