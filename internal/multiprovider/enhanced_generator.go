package multiprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v57/github"
	gh "github.com/twin2ai/studio/internal/github"
	"github.com/twin2ai/studio/pkg/models"
)

// Platform represents a platform configuration
type Platform struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// PlatformConfig holds the platform configuration
type PlatformConfig struct {
	Platforms []Platform `json:"platforms"`
}

// ProcessIssueWithStructure processes an issue and generates a complete persona package
func (g *Generator) ProcessIssueWithStructure(ctx context.Context, issue *github.Issue) (*models.Persona, *gh.PersonaFiles, error) {
	return g.ProcessIssueWithStructureAndUser(ctx, issue, "")
}

// ProcessIssueWithStructureAndUser processes an issue with optional user-supplied persona
func (g *Generator) ProcessIssueWithStructureAndUser(ctx context.Context, issue *github.Issue, userPersona string) (*models.Persona, *gh.PersonaFiles, error) {
	g.logger.Infof("Processing issue #%d with structured multi-provider generation: %s", *issue.Number, *issue.Title)
	if userPersona != "" {
		g.logger.Info("User-supplied persona detected, will include in synthesis")
	}

	// Combine issue title and body for context
	issueContent := fmt.Sprintf("Title: %s\n\nDescription:\n%s",
		*issue.Title,
		getStringValue(issue.Body))

	// Load template
	template, err := g.loadTemplate()
	if err != nil {
		g.logger.Warnf("Failed to load template: %v", err)
		template = ""
	}

	// Generate personas from all providers in parallel
	responses, err := g.generateFromAllProviders(ctx, issueContent, template)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate personas from providers: %w", err)
	}

	// Store individual responses
	if err := g.storeArtifacts(*issue.Number, responses); err != nil {
		g.logger.Warnf("Failed to store artifacts: %v", err)
	}

	// Extract individual provider contents
	var claudeRaw, geminiRaw, grokRaw, gptRaw string
	for _, resp := range responses {
		if resp.Error == nil {
			switch resp.Provider {
			case "claude":
				claudeRaw = resp.Content
			case "gemini":
				geminiRaw = resp.Content
			case "grok":
				grokRaw = resp.Content
			case "gpt":
				gptRaw = resp.Content
			}
		}
	}

	// Combine all responses into final persona (including user persona if provided)
	fullSynthesis, err := g.combinePersonasWithUser(ctx, responses, userPersona)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to combine personas: %w", err)
	}

	// Store combined result
	if err := g.storeCombinedPersona(*issue.Number, fullSynthesis); err != nil {
		g.logger.Warnf("Failed to store combined persona: %v", err)
	}

	// Use persona name from issue title
	personaName := *issue.Title

	// Skip generating additional versions for now
	g.logger.Info("Skipping prompt-ready, constrained formats, and platform adaptations generation")
	promptReady := fullSynthesis
	constrainedFormats := "# Constrained Formats\n\n*Generation of constrained formats is currently disabled.*"
	platformAdaptations := make(map[string]string)

	// Create PersonaFiles structure
	files := &gh.PersonaFiles{
		ClaudeRaw:           claudeRaw,
		GeminiRaw:           geminiRaw,
		GrokRaw:             grokRaw,
		GPTRaw:              gptRaw,
		UserRaw:             userPersona,
		FullSynthesis:       fullSynthesis,
		PromptReady:         promptReady,
		ConstrainedFormats:  constrainedFormats,
		PlatformAdaptations: platformAdaptations,
	}

	// Create Persona model
	persona := &models.Persona{
		Name:        personaName,
		Content:     fullSynthesis,
		IssueNumber: *issue.Number,
	}

	return persona, files, nil
}

// loadPromptFromFile loads a prompt template from a file
func (g *Generator) loadPromptFromFile(filename string) (string, error) {
	data, err := os.ReadFile(filepath.Join("prompts", filename))
	if err != nil {
		return "", fmt.Errorf("failed to load prompt from %s: %w", filename, err)
	}
	return string(data), nil
}

// loadPlatforms loads the platform configuration from file
func (g *Generator) loadPlatforms() ([]string, error) {
	configPath := filepath.Join("config", "platforms.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load platforms config: %w", err)
	}

	var config PlatformConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse platforms config: %w", err)
	}

	var enabledPlatforms []string
	for _, platform := range config.Platforms {
		if platform.Enabled {
			enabledPlatforms = append(enabledPlatforms, platform.Name)
		}
	}

	if len(enabledPlatforms) == 0 {
		return nil, fmt.Errorf("no platforms enabled in configuration")
	}

	return enabledPlatforms, nil
}

// generatePromptReadyVersion creates a 500-1000 word condensed version
func (g *Generator) generatePromptReadyVersion(ctx context.Context, synthesizedPersona, personaName string) (string, error) {
	g.logger.Infof("Generating prompt-ready version for %s", personaName)

	// Load prompt template
	promptTemplate, err := g.loadPromptFromFile("prompt_ready_generation.txt")
	if err != nil {
		g.logger.Warnf("Failed to load prompt template: %v, using fallback", err)
		promptTemplate = `Create a prompt-ready condensed version of the following synthesized persona in 500-1000 words. This version should:

1. Capture the essential characteristics and behaviors
2. Be immediately usable in AI prompts
3. Focus on the most distinctive and important traits
4. Include key speaking patterns, values, and expertise
5. Be formatted for easy copy-paste into prompts

Start your response immediately with the condensed persona content. Do NOT include any preambles or meta-commentary.

SYNTHESIZED PERSONA:
{{SYNTHESIZED_PERSONA}}

Create the condensed version now:`
	}

	// Replace placeholder with synthesized persona
	prompt := strings.ReplaceAll(promptTemplate, "{{SYNTHESIZED_PERSONA}}", synthesizedPersona)

	// Use Gemini with lower temperature for synthesis task
	condensed, err := g.gemini.GeneratePersonaSynthesis(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate prompt-ready version: %w", err)
	}

	return condensed, nil
}

// generateConstrainedFormats creates various length-constrained versions
func (g *Generator) generateConstrainedFormats(ctx context.Context, synthesizedPersona, personaName string) (string, error) {
	g.logger.Infof("Generating constrained formats for %s", personaName)

	// Load prompt template
	promptTemplate, err := g.loadPromptFromFile("constrained_formats_generation.txt")
	if err != nil {
		g.logger.Warnf("Failed to load prompt template: %v, using fallback", err)
		promptTemplate = `Create multiple constrained format versions of the following synthesized persona. Generate each of these formats:

1. **One-Liner** (max 100 characters): A single sentence capturing the essence
2. **Tweet-Length** (max 280 characters): Core identity in tweet format
3. **Elevator Pitch** (30 seconds / ~75 words): Quick introduction
4. **Short Bio** (100-150 words): Brief professional biography
5. **Executive Summary** (200-300 words): Key points for quick reference
6. **Single Paragraph** (150-200 words): Flowing narrative description

Format your response as a Markdown document with clear headers for each version. Start immediately with the content.

SYNTHESIZED PERSONA:
{{SYNTHESIZED_PERSONA}}

Generate the constrained versions now:`
	}

	// Replace placeholder with synthesized persona
	prompt := strings.ReplaceAll(promptTemplate, "{{SYNTHESIZED_PERSONA}}", synthesizedPersona)

	// Use Gemini with lower temperature for synthesis task
	constrained, err := g.gemini.GeneratePersonaSynthesis(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate constrained formats: %w", err)
	}

	return constrained, nil
}

// generatePlatformAdaptations creates platform-specific versions
func (g *Generator) generatePlatformAdaptations(ctx context.Context, synthesizedPersona, personaName string) (map[string]string, error) {
	g.logger.Infof("Generating platform adaptations for %s", personaName)

	// Load platforms from configuration
	platforms, err := g.loadPlatforms()
	if err != nil {
		g.logger.Warnf("Failed to load platforms config: %v, using defaults", err)
		// Fallback to default platforms
		platforms = []string{
			"ChatGPT",
			"Claude",
			"Gemini",
			"Character.AI",
			"Discord Bot",
			"Twitter/X",
			"LinkedIn",
			"Email Assistant",
		}
	}

	g.logger.Infof("Generating adaptations for %d platforms", len(platforms))

	// Load prompt template
	promptTemplate, err := g.loadPromptFromFile("platform_adaptation.txt")
	if err != nil {
		g.logger.Warnf("Failed to load prompt template: %v, using fallback", err)
		promptTemplate = `Adapt the following synthesized persona specifically for use on {{PLATFORM}}. Consider:

1. Platform-specific constraints and features
2. Optimal formatting for the platform
3. Relevant use cases and interactions
4. Platform culture and expectations
5. Technical limitations or opportunities

Start your response immediately with the adapted persona. Do NOT include any preambles.

SYNTHESIZED PERSONA TO ADAPT:
{{SYNTHESIZED_PERSONA}}

Create the {{PLATFORM}} adaptation now:`
	}

	adaptations := make(map[string]string)

	// Generate adaptations for each platform
	for _, platform := range platforms {
		// Replace placeholders
		prompt := strings.ReplaceAll(promptTemplate, "{{PLATFORM}}", platform)
		prompt = strings.ReplaceAll(prompt, "{{SYNTHESIZED_PERSONA}}", synthesizedPersona)

		adapted, err := g.gemini.GeneratePersonaSynthesis(ctx, prompt)
		if err != nil {
			g.logger.Warnf("Failed to generate %s adaptation: %v", platform, err)
			continue
		}

		adaptations[platform] = adapted
	}

	return adaptations, nil
}

// UpdatePersonaWithUserInput updates an existing persona with user-provided content
func (g *Generator) UpdatePersonaWithUserInput(ctx context.Context, personaName string, existingPersona string, userPersona string) (string, error) {
	g.logger.Infof("Updating persona '%s' with user input", personaName)

	// Combine existing and user personas for synthesis
	combinationPrompt := fmt.Sprintf(`You are tasked with creating an improved persona by synthesizing an existing persona with a user-provided update.

The user has provided their own version of the persona that should be intelligently merged with the existing version to create a superior, comprehensive persona that incorporates the best elements of both.

EXISTING PERSONA:
<<<
%s
>>>

USER-PROVIDED UPDATE:
<<<
%s
>>>

Please create a synthesized version that:
1. Incorporates new information from the user's version
2. Preserves valuable details from the existing persona
3. Resolves any conflicts by preferring the user's interpretation
4. Maintains consistency and coherence throughout
5. Results in a richer, more complete persona

Start your response immediately with the synthesized persona content using proper Markdown formatting. Do NOT include any preambles or meta-commentary.`, existingPersona, userPersona)

	// Use Gemini with lower temperature for synthesis
	synthesized, err := g.gemini.GeneratePersonaSynthesis(ctx, combinationPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to synthesize personas: %w", err)
	}

	return synthesized, nil
}

// RegeneratePersonaWithStructuredFeedback regenerates a complete persona package with feedback
func (g *Generator) RegeneratePersonaWithStructuredFeedback(ctx context.Context, issue *github.Issue, existingPersona string, feedback []string) (*models.Persona, *gh.PersonaFiles, error) {
	g.logger.Infof("Regenerating structured persona for issue #%d with feedback", *issue.Number)

	// Combine issue title and body for context
	issueContent := fmt.Sprintf("Title: %s\n\nDescription:\n%s",
		*issue.Title,
		getStringValue(issue.Body))

	// Load template
	template, err := g.loadTemplate()
	if err != nil {
		g.logger.Warnf("Failed to load template: %v", err)
		template = ""
	}

	// Create enhanced prompt with feedback
	feedbackSection := g.formatFeedback(feedback)
	enhancedPrompt := fmt.Sprintf(`%s

IMPORTANT: Please address the following feedback and regenerate the persona:

%s

Previous persona version:
%s

Please create an improved version that addresses all the feedback points above.`,
		issueContent, feedbackSection, existingPersona)

	// Generate from all providers with feedback
	responses, err := g.generateFromAllProviders(ctx, enhancedPrompt, template)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to regenerate personas from providers: %w", err)
	}

	// Store individual responses with feedback suffix
	if err := g.storeArtifactsWithSuffix(*issue.Number, responses, "feedback"); err != nil {
		g.logger.Warnf("Failed to store feedback artifacts: %v", err)
	}

	// Continue with the same structure generation as ProcessIssueWithStructure
	// ... (rest of the implementation follows the same pattern)

	// Extract individual provider contents
	var claudeRaw, geminiRaw, grokRaw, gptRaw string
	for _, resp := range responses {
		if resp.Error == nil {
			switch resp.Provider {
			case "claude":
				claudeRaw = resp.Content
			case "gemini":
				geminiRaw = resp.Content
			case "grok":
				grokRaw = resp.Content
			case "gpt":
				gptRaw = resp.Content
			}
		}
	}

	// Combine all responses into final persona
	fullSynthesis, err := g.combinePersonas(ctx, responses)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to combine feedback personas: %w", err)
	}

	// Store combined result with feedback suffix
	if err := g.storeCombinedPersonaWithSuffix(*issue.Number, fullSynthesis, "feedback"); err != nil {
		g.logger.Warnf("Failed to store combined feedback persona: %v", err)
	}

	// Use persona name from issue title
	personaName := *issue.Title

	// Skip generating additional versions for now
	g.logger.Info("Skipping prompt-ready, constrained formats, and platform adaptations generation")
	promptReady := fullSynthesis
	constrainedFormats := "# Constrained Formats\n\n*Generation of constrained formats is currently disabled.*"
	platformAdaptations := make(map[string]string)

	// Create PersonaFiles structure
	files := &gh.PersonaFiles{
		ClaudeRaw:           claudeRaw,
		GeminiRaw:           geminiRaw,
		GrokRaw:             grokRaw,
		GPTRaw:              gptRaw,
		UserRaw:             "", // No user persona in regeneration
		FullSynthesis:       fullSynthesis,
		PromptReady:         promptReady,
		ConstrainedFormats:  constrainedFormats,
		PlatformAdaptations: platformAdaptations,
	}

	// Create Persona model
	persona := &models.Persona{
		Name:        personaName,
		Content:     fullSynthesis,
		IssueNumber: *issue.Number,
	}

	return persona, files, nil
}