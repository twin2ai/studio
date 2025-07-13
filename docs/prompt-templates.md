# Prompt Templates

## Overview

Studio uses external prompt template files for generating different persona variations. This allows for easy customization without modifying code.

## Template Files

All prompt templates are stored in the `prompts/` directory:

### 1. `prompt_ready_generation.txt`
Generates the condensed 500-1000 word version from the synthesized persona.

**Input**: `{{SYNTHESIZED_PERSONA}}`
**Output**: Prompt-ready condensed version

### 2. `constrained_formats_generation.txt`
Creates multiple length-constrained versions from the synthesized persona.

**Input**: `{{SYNTHESIZED_PERSONA}}`
**Output**: Various formats (one-liner, tweet, bio, etc.)

### 3. `platform_adaptation.txt`
Adapts the synthesized persona for specific platforms.

**Inputs**: 
- `{{SYNTHESIZED_PERSONA}}`
- `{{PLATFORM}}`
**Output**: Platform-specific adaptation

## Data Flow

```
1. AI Providers → Individual Personas
2. Synthesis → Synthesized Persona
3. Synthesized Persona → Prompt-Ready Version
4. Synthesized Persona → Constrained Formats
5. Synthesized Persona → Platform Adaptations
```

The synthesized persona is the single source for all variations, ensuring consistency.

## Customization

To customize the generation process:

1. Edit the relevant prompt file in `prompts/`
2. Use the placeholder variables:
   - `{{SYNTHESIZED_PERSONA}}` - The combined persona
   - `{{PLATFORM}}` - Platform name (for adaptations)
3. Save the file - changes take effect immediately

## Template Structure

### Placeholders
- `{{SYNTHESIZED_PERSONA}}`: Replaced with the full synthesized persona
- `{{PLATFORM}}`: Replaced with the platform name (ChatGPT, Claude, etc.)

### Best Practices
1. Start with clear instructions
2. Use numbered lists for structured output
3. Include "Start immediately with..." to avoid preambles
4. Be specific about constraints (word counts, format)

## Benefits

1. **Flexibility**: Modify prompts without code changes
2. **Consistency**: All variations derive from synthesized version
3. **Transparency**: Prompts are visible and editable
4. **Version Control**: Track prompt changes in git
5. **Experimentation**: Easy to test different approaches

## Example Customization

To add a new constrained format:

1. Edit `prompts/constrained_formats_generation.txt`
2. Add a new item like:
   ```
   7. **Haiku Format** (3 lines, 5-7-5 syllables): Poetic essence
   ```
3. Save the file
4. Next generation will include the haiku format

## Fallback Behavior

If a prompt file is missing or unreadable:
1. A warning is logged
2. A hardcoded fallback prompt is used
3. Generation continues normally

This ensures the pipeline never fails due to missing prompt files.