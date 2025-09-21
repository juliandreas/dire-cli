# Roadmap

This document outlines the planned features and improvements for Dire. Items are organized by priority and development timeline.

## Planned Features

### Additional Providers

**Other Provider Considerations**

- Azure Translator
- Google Translate
- Amazon Translate

### Other File Format Support

**YAML/YML Files**

- Support for nested YAML translation files
- Common in Ruby on Rails and other frameworks
- Preserve formatting and comments

**XLIFF (XML Localization Interchange File Format)**

- Industry-standard translation exchange format
- Support for translation memory and workflow metadata
- Common in professional localization workflows

### Enhanced Translation Features

**Multiple Files Per Locale with Glob Pattern Support**

- Support splitting translations across multiple files per locale using glob patterns
- Enable flexible file matching in configuration
- Examples:
  - `"**/en.json": "en-US"` - Find all en.json files in any subdirectory
  - `"components/*/en.json": "en-US"` - Match specific nested structures
  - `"i18n/**/*.en.json": "en-US"` - Match files with locale extensions
- Maintain organization while enabling comprehensive translation across complex project structures

**AI-Powered Translation Review**

- Spell check and grammar review of existing translations using AI
- Quality assessment and improvement suggestions
- Detection of inconsistent terminology across files
- Potential implementations:
  - `dire --review` - Review all existing translations
  - `dire --review --locale fr-FR` - Review specific locale
  - Interactive mode for accepting/rejecting suggestions
- Improve translation quality without full re-translation

### Integration Features

**IDE Integration**

- VS Code extension for inline translation management
- Real-time translation preview
- Direct integration with development workflow

## Timeline

### Medium Term (3-6 months)

- Multiple files per locale with glob pattern support
- AI-powered translation review and quality checking

### Long Term (6+ months)

- Other file format support
- IDE integrations

## Feedback

Have suggestions for the roadmap? We'd love to hear from you:

- Open an issue on GitHub
- Start a discussion in the repository

This roadmap is subject to change based on community feedback, technical constraints, and development priorities.
