# Platform Configuration

## Overview

Platform adaptations are configured via a JSON file at `config/platforms.json`. This allows easy management of which platforms to generate adaptations for without modifying code.

## Configuration File

### Location
`config/platforms.json`

### Structure
```json
{
  "platforms": [
    {
      "name": "Platform Name",
      "description": "Brief description of the platform",
      "enabled": true/false
    }
  ]
}
```

### Fields
- **name**: The platform name used in generation and file naming
- **description**: Human-readable description (for documentation)
- **enabled**: Whether to generate adaptations for this platform

## Default Platforms

### Enabled by Default
1. **ChatGPT** - OpenAI's conversational AI assistant
2. **Claude** - Anthropic's AI assistant
3. **Gemini** - Google's AI assistant
4. **Character.AI** - Platform for creating and chatting with AI characters
5. **Discord Bot** - AI-powered Discord bot integration
6. **Twitter/X** - Social media platform adaptation
7. **LinkedIn** - Professional networking platform
8. **Email Assistant** - AI-powered email communication

### Available but Disabled
1. **Slack Bot** - Workplace communication integration
2. **Telegram Bot** - Messaging app bot integration
3. **Reddit** - Social news aggregation platform
4. **Voice Assistant** - Voice-based AI interaction

## Managing Platforms

### Enable a Platform
Change `"enabled": false` to `"enabled": true` in the configuration file.

### Disable a Platform
Change `"enabled": true` to `"enabled": false` in the configuration file.

### Add a New Platform
Add a new entry to the platforms array:
```json
{
  "name": "New Platform",
  "description": "Description of the new platform",
  "enabled": true
}
```

## File Output

Enabled platforms will have adaptation files created at:
```
personas/[persona_name]/platforms/[platform_name].md
```

Platform names are converted to lowercase with spaces replaced by underscores.

Examples:
- "ChatGPT" → `chatgpt.md`
- "Discord Bot" → `discord_bot.md`
- "Twitter/X" → `twitter_x.md`

## Customizing Adaptations

To customize how personas are adapted for each platform:

1. Edit `prompts/platform_adaptation.txt` to modify the adaptation prompt
2. The prompt uses `{{PLATFORM}}` placeholder for the platform name
3. Each platform uses the same prompt template

## Platform-Specific Considerations

The adaptation prompt instructs the AI to consider:
1. Platform-specific constraints and features
2. Optimal formatting for the platform
3. Relevant use cases and interactions
4. Platform culture and expectations
5. Technical limitations or opportunities

## Fallback Behavior

If the configuration file cannot be loaded:
1. A warning is logged
2. Default platforms (the original 8) are used
3. Generation continues normally

## Best Practices

1. **Start Small**: Enable only platforms you actually need
2. **Test First**: Enable one new platform at a time to test quality
3. **Document Changes**: Update platform descriptions when adding new ones
4. **Consider Context**: Some personas may not suit all platforms
5. **Monitor Performance**: More platforms = longer generation time

## Example: Adding Instagram

```json
{
  "name": "Instagram",
  "description": "Visual social media platform",
  "enabled": true
}
```

This will generate `personas/[name]/platforms/instagram.md` with an Instagram-optimized version.