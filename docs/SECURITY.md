# Security

This document explains the security profile of the package, including how your data and API keys are handled.

## Summary

**TL;DR:** The package follows the same security practices as trusted packages like `esbuild`, `@swc/core`, `prisma`, and `puppeteer`. Your API keys and data are handled securely with no telemetry or data collection.

## Runtime Security

### API Key Handling

Your API keys are:

- Read only from environment variables (e.g., `DIRE_CLAUDE_API_KEY`, `DIRE_OPENAI_API_KEY`)
- Never logged to disk or console
- Kept in memory only during translation operations
- Not transmitted anywhere except directly to your configured AI provider (Claude, OpenAI, Gemini, or Mistral)

### Data Transmission

- **Only translation strings and context** are sent to AI providers
- No file paths, system information, or metadata are transmitted
- All connections use HTTPS directly to official AI provider APIs
- **No third-party analytics or telemetry** - the application never "phones home"

### File System Access

The application's file system access is limited and transparent:

- Only reads/writes locale JSON files in directories specified in your `.dire.toml` configuration
- Never accesses files outside your project directory
- Does not read sensitive files (SSH keys, credentials, browser data, etc.)

### Logging & Storage

- No persistent logging of translations or API interactions
- No local caching of API responses beyond the current session

### Configuration Security

- All sensitive configuration (API keys) must be stored in environment variables
- The `.dire.toml` configuration file contains only non-sensitive project settings (locale paths, provider selection, etc.)
- You should add `.env` files to your `.gitignore` if storing environment variables in files

## Binary Code Signing

**All Windows executables are digitally signed** using a trusted certificate authority.

### What This Means for Security

Code signing provides additional security guarantees:

1. **Authenticity** - Verifies the binary was published by the author
2. **Integrity** - Ensures the binary hasn't been tampered with or modified after signing
3. **Trust** - Windows Defender and SmartScreen recognize the signature and don't flag the executable as potentially unwanted software
4. **Transparency** - You can verify the signature by right-clicking the executable and viewing its "Digital Signatures" tab

### How to Verify Code Signing

**On Windows:**

1. After installation, navigate to `node_modules/dire/bin/`
2. Right-click on `dire-cli.exe`
3. Select "Properties" → "Digital Signatures" tab
4. You should see a valid signature from the author

**Why This Matters:**

- Unsigned executables often trigger Windows Defender warnings
- Code signing demonstrates we take distribution security seriously
- It provides an additional layer of verification beyond just trusting the npm package

## Installation Security Warnings

Like `esbuild`, `@swc/core`, `prisma`, and `puppeteer`, this package downloads platform-specific binaries during installation. Security scanners may flag this process with warnings about install scripts, network access, file system access, shell access, and external URLs.

**These warnings are expected and standard for packages that download native binaries.**

The installation process:

1. Detects your platform (OS and architecture)
2. Downloads only the matching binary from our [GitHub releases](https://github.com/juliandreas/dire-cli/releases)
3. Extracts it to the package's `bin/` directory
4. Sets executable permissions (Unix/Mac)
5. Cleans up temporary files

The install script is [fully transparent and auditable](https://github.com/juliandreas/dire-cli) in our repository. It only writes to the package directory and downloads from pinned release URLs.

## Questions?

See our [FAQ](./FAQ.md) or open an issue on [GitHub](https://github.com/juliandreas/dire-cli/issues).
