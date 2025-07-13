package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
)

// PersonaFiles represents all the files that make up a complete persona package
type PersonaFiles struct {
	// Raw AI outputs
	ClaudeRaw  string
	GeminiRaw  string
	GrokRaw    string
	GPTRaw     string
	UserRaw    string // Optional user-supplied persona
	
	// Synthesized versions
	FullSynthesis      string // The complete synthesized persona
	PromptReady        string // 500-1000 word condensed version
	ConstrainedFormats string // Various constrained format versions
	
	// Platform-specific adaptations
	PlatformAdaptations map[string]string // Key: platform name, Value: adapted content
}

// CreateStructuredPersonaPR creates a pull request with the new folder structure
func (c *Client) CreateStructuredPersonaPR(ctx context.Context, issueNumber int, personaName string, files PersonaFiles) (*github.PullRequest, error) {
	// Create branch name
	sanitizedName := strings.ToLower(strings.ReplaceAll(personaName, " ", "-"))
	sanitizedName = strings.ReplaceAll(sanitizedName, "/", "-")
	branchName := fmt.Sprintf("persona/%s-%d", sanitizedName, issueNumber)

	// Get default branch of personas repo
	repo, _, err := c.client.Repositories.Get(ctx, c.personasOwner, c.personasRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to get personas repo: %w", err)
	}
	defaultBranch := repo.GetDefaultBranch()

	// Get base branch ref
	baseRef, _, err := c.client.Git.GetRef(ctx, c.personasOwner, c.personasRepo, "refs/heads/"+defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get base ref: %w", err)
	}

	// Create new branch
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + branchName),
		Object: &github.GitObject{
			SHA: baseRef.Object.SHA,
		},
	}

	_, _, err = c.client.Git.CreateRef(ctx, c.personasOwner, c.personasRepo, newRef)
	if err != nil {
		if strings.Contains(err.Error(), "Reference already exists") {
			c.logger.Warnf("Branch %s already exists, continuing...", branchName)
		} else {
			return nil, fmt.Errorf("failed to create branch: %w", err)
		}
	}

	// Create folder structure
	folderName := strings.ToLower(strings.ReplaceAll(personaName, " ", "_"))
	folderName = strings.ReplaceAll(folderName, "/", "_")
	baseFolder := fmt.Sprintf("personas/%s", folderName)

	// Track all file operations for the commit
	fileOperations := []struct {
		path    string
		content string
		message string
	}{
		// Raw AI outputs
		{fmt.Sprintf("%s/raw/claude.md", baseFolder), files.ClaudeRaw, "Claude's raw output"},
		{fmt.Sprintf("%s/raw/gemini.md", baseFolder), files.GeminiRaw, "Gemini's raw output"},
		{fmt.Sprintf("%s/raw/grok.md", baseFolder), files.GrokRaw, "Grok's raw output"},
		{fmt.Sprintf("%s/raw/gpt.md", baseFolder), files.GPTRaw, "GPT-4's raw output"},
		
		// Main files
		{fmt.Sprintf("%s/synthesized.md", baseFolder), files.FullSynthesis, "Full synthesized persona"},
		{fmt.Sprintf("%s/prompt_ready.md", baseFolder), files.PromptReady, "Prompt-ready condensed version"},
		{fmt.Sprintf("%s/constrained_formats.md", baseFolder), files.ConstrainedFormats, "Constrained format versions"},
		
		// README for the persona folder
		{fmt.Sprintf("%s/README.md", baseFolder), c.generatePersonaReadme(personaName, issueNumber), "Persona overview"},
	}
	
	// Add user-supplied persona if provided
	if files.UserRaw != "" {
		fileOperations = append(fileOperations, struct {
			path    string
			content string
			message string
		}{fmt.Sprintf("%s/raw/user_supplied.md", baseFolder), files.UserRaw, "User-supplied persona"})
	}

	// Add platform-specific adaptations
	for platform, content := range files.PlatformAdaptations {
		sanitizedPlatform := strings.ToLower(strings.ReplaceAll(platform, " ", "_"))
		filePath := fmt.Sprintf("%s/platforms/%s.md", baseFolder, sanitizedPlatform)
		fileOperations = append(fileOperations, struct {
			path    string
			content string
			message string
		}{filePath, content, fmt.Sprintf("%s platform adaptation", platform)})
	}

	// Create all files
	for _, op := range fileOperations {
		fileOpts := &github.RepositoryContentFileOptions{
			Message: github.String(fmt.Sprintf("Add %s for persona: %s", op.message, personaName)),
			Content: []byte(op.content),
			Branch:  github.String(branchName),
		}

		// Check if file exists
		existingFile, _, _, err := c.client.Repositories.GetContents(
			ctx, c.personasOwner, c.personasRepo, op.path,
			&github.RepositoryContentGetOptions{Ref: branchName})

		if err == nil && existingFile != nil {
			// File exists, update it
			fileOpts.SHA = existingFile.SHA
			_, _, err = c.client.Repositories.UpdateFile(
				ctx, c.personasOwner, c.personasRepo, op.path, fileOpts)
			if err != nil {
				c.logger.Warnf("Failed to update file %s: %v", op.path, err)
				continue
			}
		} else {
			// File doesn't exist, create it
			_, _, err = c.client.Repositories.CreateFile(
				ctx, c.personasOwner, c.personasRepo, op.path, fileOpts)
			if err != nil {
				c.logger.Warnf("Failed to create file %s: %v", op.path, err)
				continue
			}
		}
		c.logger.Debugf("Created/updated file: %s", op.path)
	}

	// Create pull request
	includesUserPersona := ""
	if files.UserRaw != "" {
		includesUserPersona = "\n- **User-supplied persona** included in synthesis"
	}
	
	prBody := fmt.Sprintf(`This PR adds a comprehensive persona package for: **%s**

## 📁 Structure
This persona includes:
- **Raw outputs** from all 4 AI providers (Claude, Gemini, Grok, GPT-4)%s
- **Synthesized version** combining the best of all outputs
- **Prompt-ready version** (500-1000 words) for immediate use
- **Constrained formats** for various use cases
- **Platform-specific adaptations** for different environments

## 📍 Files
- %s/raw/ - Individual AI provider outputs
- %s/synthesized.md - Full synthesized persona
- %s/prompt_ready.md - Condensed version for prompts
- %s/constrained_formats.md - Various format constraints
- %s/platforms/ - Platform-specific versions

## 🔗 Source
Created from issue: %s/%s#%d

---
*This is an automated PR created by [Studio](https://github.com/twin2ai/studio)*`,
		personaName, includesUserPersona, baseFolder, baseFolder, baseFolder, baseFolder, baseFolder,
		c.issuesOwner, c.issuesRepo, issueNumber)

	pr := &github.NewPullRequest{
		Title: github.String(fmt.Sprintf("Add persona package: %s", personaName)),
		Body:  github.String(prBody),
		Head:  github.String(branchName),
		Base:  github.String(defaultBranch),
	}

	pullRequest, _, err := c.client.PullRequests.Create(
		ctx, c.personasOwner, c.personasRepo, pr)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// Add labels to PR
	_, _, err = c.client.Issues.AddLabelsToIssue(
		ctx, c.personasOwner, c.personasRepo,
		*pullRequest.Number, []string{"persona", "automated", "studio", "structured"})
	if err != nil {
		c.logger.Warnf("Failed to add labels to PR: %v", err)
	}

	// Comment on original issue with link to PR
	prURL := pullRequest.GetHTMLURL()
	comment := fmt.Sprintf(`✅ Persona package generated successfully!

📦 **Complete persona package created with:**
- Raw outputs from all 4 AI providers
- Synthesized full persona
- Prompt-ready condensed version
- Constrained format variations
- Platform-specific adaptations

View the generated persona package: %s

The persona has been created in the [twin2ai/personas](https://github.com/twin2ai/personas) repository.`, prURL)

	_, _, err = c.client.Issues.CreateComment(
		ctx, c.issuesOwner, c.issuesRepo, issueNumber,
		&github.IssueComment{Body: github.String(comment)})
	if err != nil {
		c.logger.Warnf("Failed to comment on issue: %v", err)
	}

	return pullRequest, nil
}

// generatePersonaReadme creates a README file for the persona folder
func (c *Client) generatePersonaReadme(personaName string, issueNumber int) string {
	return fmt.Sprintf(`# %s Persona

This folder contains a comprehensive persona package generated by Studio.

## Contents

### 📝 Raw Outputs (/raw)
- **claude.md** - Claude Opus 4's interpretation
- **gemini.md** - Gemini 2.5 Pro's interpretation
- **grok.md** - Grok 2's interpretation
- **gpt.md** - GPT-4 Turbo's interpretation
- **user_supplied.md** - User-provided persona (if supplied)

### 🎯 Main Files
- **synthesized.md** - Full synthesized persona combining best elements from all AI providers
- **prompt_ready.md** - Condensed 500-1000 word version optimized for immediate use in prompts
- **constrained_formats.md** - Various constrained versions (tweet-length, one-liner, etc.)

### 🌐 Platform Adaptations (/platforms)
Platform-specific versions optimized for different environments and use cases.

## Usage

1. **For general use**: Start with prompt_ready.md for a balanced, comprehensive persona
2. **For detailed work**: Use synthesized.md for the full persona with all nuances
3. **For specific platforms**: Check the /platforms folder for optimized versions
4. **For constraints**: See constrained_formats.md for length-limited versions

## Source
Generated from issue #%d in the twin2ai/personas repository.

---
*Created by [Studio](https://github.com/twin2ai/studio) - Multi-AI Persona Generation Pipeline*`, 
		personaName, issueNumber)
}