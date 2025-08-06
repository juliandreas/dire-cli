# Known Issues

## Claude API Performance with Large Batches

**Issue**: When using Claude as the AI provider, processing large batches of translations can take significantly longer than expected.

**Details**:

- This appears to be related to Claude's API rate limiting and processing characteristics
- Other providers typically handle large batches more efficiently
- The slowdown is most noticeable when processing hundreds of translation

**Status**: Under investigation for potential optimization improvements.
