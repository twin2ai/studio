# Structured Persona Generation Pipeline

## Overview

The Studio pipeline has been enhanced to create a comprehensive persona package with multiple versions and formats, organized in a structured folder hierarchy.

## New Folder Structure

Instead of a flat `personas/persona_name.md` file, each persona now gets its own folder:

```
personas/
├── david_attenborough/
│   ├── README.md                    # Persona overview and usage guide
│   ├── raw/                        # Individual AI provider outputs
│   │   ├── claude.md
│   │   ├── gemini.md
│   │   ├── grok.md
│   │   └── gpt.md
│   ├── synthesized.md              # Full combined persona
│   ├── prompt_ready.md             # 500-1000 word condensed version
│   ├── constrained_formats.md      # Various length-constrained versions
│   └── platforms/                  # Platform-specific adaptations
│       ├── chatgpt.md
│       ├── claude.md
│       ├── gemini.md
│       ├── character_ai.md
│       ├── discord_bot.md
│       ├── twitter_x.md
│       ├── linkedin.md
│       └── email_assistant.md
```

## File Descriptions

### 1. **Raw Outputs** (`/raw/`)
- Contains the original, unmodified outputs from each AI provider
- Useful for analysis and understanding different AI interpretations
- Files: `claude.md`, `gemini.md`, `grok.md`, `gpt.md`

### 2. **Synthesized Version** (`synthesized.md`)
- The complete, combined persona using Gemini's synthesis
- Incorporates the best elements from all four AI providers
- Most comprehensive version for detailed use

### 3. **Prompt-Ready Version** (`prompt_ready.md`)
- Condensed 500-1000 word version
- Optimized for immediate use in AI prompts
- Captures essential characteristics and behaviors
- Best for quick implementation

### 4. **Constrained Formats** (`constrained_formats.md`)
Contains multiple length-constrained versions:
- **One-Liner**: Max 100 characters
- **Tweet-Length**: Max 280 characters
- **Elevator Pitch**: ~75 words
- **Short Bio**: 100-150 words
- **Executive Summary**: 200-300 words
- **Single Paragraph**: 150-200 words

### 5. **Platform Adaptations** (`/platforms/`)
Platform-specific versions optimized for:
- ChatGPT
- Claude
- Gemini
- Character.AI
- Discord Bot
- Twitter/X
- LinkedIn
- Email Assistant

## Implementation Details

### New Components

1. **`internal/github/structured_pr.go`**
   - `CreateStructuredPersonaPR()`: Creates PRs with multiple files
   - `PersonaFiles` struct: Holds all persona variations
   - `generatePersonaReadme()`: Creates folder README

2. **`internal/multiprovider/enhanced_generator.go`**
   - `ProcessIssueWithStructure()`: Generates complete persona package
   - `generatePromptReadyVersion()`: Creates condensed version
   - `generateConstrainedFormats()`: Creates length-limited versions
   - `generatePlatformAdaptations()`: Creates platform-specific versions

3. **`internal/pipeline/structured_pipeline.go`**
   - `processNewIssuesWithStructure()`: Handles new issues
   - `processPRCommentsWithStructure()`: Updates all files on feedback
   - `updateStructuredPR()`: Updates entire persona package

### Pipeline Flow

1. **Issue Processing**:
   - Issue with `create-persona` label detected
   - All 4 AI providers generate personas in parallel
   - Gemini synthesizes into full version
   - Additional versions generated (prompt-ready, constrained, platforms)
   - Complete package created in structured folders

2. **PR Creation**:
   - Creates folder structure with all files
   - Includes comprehensive README
   - Labels: `persona`, `automated`, `studio`, `structured`

3. **Feedback Processing**:
   - Comments trigger regeneration of entire package
   - All files updated with improved versions
   - Maintains consistency across all formats

## Benefits

1. **Organization**: Clear folder structure for each persona
2. **Flexibility**: Multiple versions for different use cases
3. **Discoverability**: README guides users to right version
4. **Analysis**: Raw outputs preserved for comparison
5. **Immediate Use**: Prompt-ready version for quick implementation
6. **Platform Optimization**: Tailored versions for specific platforms

## Usage Recommendations

- **Quick Implementation**: Use `prompt_ready.md`
- **Detailed Work**: Use `synthesized.md`
- **Platform-Specific**: Check `/platforms/` folder
- **Length Constraints**: See `constrained_formats.md`
- **Analysis**: Compare `/raw/` outputs

## Future Enhancements

- Version history tracking
- Persona evolution over time
- Cross-persona analysis
- Automated testing of personas
- Usage analytics per version