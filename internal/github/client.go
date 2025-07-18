package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Client struct {
	client        *github.Client
	issuesOwner   string
	issuesRepo    string
	personasOwner string
	personasRepo  string
	label         string
	logger        *logrus.Logger
}

func (c *Client) GetClient() *github.Client {
	return c.client
}

func NewClient(token, issuesOwner, issuesRepo, personasOwner, personasRepo, label string, logger *logrus.Logger) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client:        github.NewClient(tc),
		issuesOwner:   issuesOwner,
		issuesRepo:    issuesRepo,
		personasOwner: personasOwner,
		personasRepo:  personasRepo,
		label:         label,
		logger:        logger,
	}
}

func (c *Client) GetPersonaIssues(ctx context.Context) ([]*github.Issue, error) {
	opts := &github.IssueListByRepoOptions{
		State:  "open",
		Labels: []string{c.label},
	}

	issues, _, err := c.client.Issues.ListByRepo(ctx, c.issuesOwner, c.issuesRepo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", err)
	}

	c.logger.Infof("Found %d issues with label '%s' in %s/%s",
		len(issues), c.label, c.issuesOwner, c.issuesRepo)
	return issues, nil
}

func (c *Client) CreatePersonaPullRequest(ctx context.Context, issueNumber int, personaName, personaContent string) (*github.PullRequest, error) {
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
		// Branch might already exist, try to update it
		if strings.Contains(err.Error(), "Reference already exists") {
			c.logger.Warnf("Branch %s already exists, continuing...", branchName)
		} else {
			return nil, fmt.Errorf("failed to create branch: %w", err)
		}
	}

	// Create file path - using .md extension for markdown files
	fileName := strings.ToLower(strings.ReplaceAll(personaName, " ", "_"))
	fileName = strings.ReplaceAll(fileName, "/", "_")
	filePath := fmt.Sprintf("personas/%s.md", fileName)

	fileOpts := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Add persona: %s", personaName)),
		Content: []byte(personaContent),
		Branch:  github.String(branchName),
	}

	// Add reference to original issue in commit message
	issueRef := fmt.Sprintf("\n\nCreated from issue: %s/%s#%d",
		c.issuesOwner, c.issuesRepo, issueNumber)
	fileOpts.Message = github.String(*fileOpts.Message + issueRef)

	// Check if file exists
	existingFile, _, _, err := c.client.Repositories.GetContents(
		ctx, c.personasOwner, c.personasRepo, filePath,
		&github.RepositoryContentGetOptions{Ref: branchName})

	if err == nil && existingFile != nil {
		// File exists, update it
		fileOpts.SHA = existingFile.SHA
		_, _, err = c.client.Repositories.UpdateFile(
			ctx, c.personasOwner, c.personasRepo, filePath, fileOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to update file: %w", err)
		}
	} else {
		// File doesn't exist, create it
		_, _, err = c.client.Repositories.CreateFile(
			ctx, c.personasOwner, c.personasRepo, filePath, fileOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}
	}

	// Create pull request
	prBody := fmt.Sprintf(`This PR adds a new persona: **%s**

## Source
Created from issue: %s/%s#%d

## Summary
This persona was automatically generated by Studio based on the requirements specified in the source issue.

---
*This is an automated PR created by [Studio](https://github.com/twin2ai/studio)*`,
		personaName, c.issuesOwner, c.issuesRepo, issueNumber)

	pr := &github.NewPullRequest{
		Title: github.String(fmt.Sprintf("Add persona: %s", personaName)),
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
		*pullRequest.Number, []string{"persona", "automated", "studio"})
	if err != nil {
		c.logger.Warnf("Failed to add labels to PR: %v", err)
	}

	// Comment on original issue with link to PR
	prURL := pullRequest.GetHTMLURL()
	comment := fmt.Sprintf(`✅ Persona generated successfully!

View the generated persona: %s

The persona has been created in the [twin2ai/personas](https://github.com/twin2ai/personas) repository.`, prURL)

	_, _, err = c.client.Issues.CreateComment(
		ctx, c.issuesOwner, c.issuesRepo, issueNumber,
		&github.IssueComment{Body: github.String(comment)})
	if err != nil {
		c.logger.Warnf("Failed to comment on issue: %v", err)
	}

	return pullRequest, nil
}

func (c *Client) GetPersonaPullRequests(ctx context.Context) ([]*github.PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State: "open", // Only get open PRs - closed PRs will be ignored
	}

	prs, _, err := c.client.PullRequests.List(ctx, c.personasOwner, c.personasRepo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pull requests: %w", err)
	}

	// Filter for Studio-created PRs (those with "studio" label or created by Studio)
	var studioPRs []*github.PullRequest
	for _, pr := range prs {
		// Double-check that PR is actually open (API should already filter this)
		if pr.State != nil && *pr.State == "open" && c.isStudioPR(pr) {
			studioPRs = append(studioPRs, pr)
		}
	}

	c.logger.Infof("Found %d open Studio PRs in %s/%s",
		len(studioPRs), c.personasOwner, c.personasRepo)
	return studioPRs, nil
}

func (c *Client) isStudioPR(pr *github.PullRequest) bool {
	// Check if it's a Studio PR by branch name pattern or body content
	if pr.Head != nil && pr.Head.Ref != nil {
		if strings.HasPrefix(*pr.Head.Ref, "persona/") {
			return true
		}
	}

	// Check if PR body contains Studio signature
	if pr.Body != nil && strings.Contains(*pr.Body, "Studio") {
		return true
	}

	return false
}

func (c *Client) GetPRComments(ctx context.Context, prNumber int) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{
		Sort:      github.String("created"),
		Direction: github.String("desc"),
	}

	comments, _, err := c.client.Issues.ListComments(ctx, c.personasOwner, c.personasRepo, prNumber, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR comments: %w", err)
	}

	c.logger.Infof("Found %d comments on PR #%d", len(comments), prNumber)
	return comments, nil
}

func (c *Client) UpdatePersonaPR(ctx context.Context, prNumber int, personaName, personaContent, branchName, filePath string) error {
	// Update the file in the PR branch
	fileOpts := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Update persona: %s (addressing feedback)", personaName)),
		Content: []byte(personaContent),
		Branch:  github.String(branchName),
	}

	// Get existing file to get SHA for update
	existingFile, _, _, err := c.client.Repositories.GetContents(
		ctx, c.personasOwner, c.personasRepo, filePath,
		&github.RepositoryContentGetOptions{Ref: branchName})
	if err != nil {
		return fmt.Errorf("failed to get existing file: %w", err)
	}

	fileOpts.SHA = existingFile.SHA

	// Update the file
	_, _, err = c.client.Repositories.UpdateFile(
		ctx, c.personasOwner, c.personasRepo, filePath, fileOpts)
	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}

	// Add a comment to the PR indicating the update
	comment := fmt.Sprintf(`🔄 **Persona Updated**

The persona has been regenerated and updated to address the feedback provided.

---
*Updated automatically by [Studio](https://github.com/twin2ai/studio)*`)

	_, _, err = c.client.Issues.CreateComment(
		ctx, c.personasOwner, c.personasRepo, prNumber,
		&github.IssueComment{Body: github.String(comment)})
	if err != nil {
		c.logger.Warnf("Failed to comment on PR after update: %v", err)
	}

	return nil
}

func (c *Client) MarkCommentAsAddressed(ctx context.Context, prNumber int, commentBody string) error {
	// Add a reaction to indicate the comment has been addressed
	// This is a simple way to track which comments have been processed
	comment := fmt.Sprintf(`✅ **Feedback Addressed**

The following feedback has been incorporated into the updated persona:

> %s

The persona has been regenerated with your suggestions in mind.

---
*Comment addressed by [Studio](https://github.com/twin2ai/studio)*`, commentBody)

	_, _, err := c.client.Issues.CreateComment(
		ctx, c.personasOwner, c.personasRepo, prNumber,
		&github.IssueComment{Body: github.String(comment)})
	if err != nil {
		return fmt.Errorf("failed to create addressed comment: %w", err)
	}

	return nil
}

// GetFileContent retrieves the content of a file from the personas repository
func (c *Client) GetFileContent(ctx context.Context, filePath string) (string, error) {
	c.logger.Debugf("Fetching file content from personas repo: %s", filePath)

	fileContent, _, _, err := c.client.Repositories.GetContents(
		ctx, c.personasOwner, c.personasRepo, filePath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get file %s: %w", filePath, err)
	}

	if fileContent == nil {
		return "", fmt.Errorf("file %s not found", filePath)
	}

	// Decode the content (GitHub returns base64 encoded content)
	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode file content: %w", err)
	}

	c.logger.Debugf("Successfully fetched file content: %d characters", len(content))
	return content, nil
}

// GetPRStatus retrieves the status of a pull request
func (c *Client) GetPRStatus(ctx context.Context, prNumber int) (string, error) {
	c.logger.Debugf("Checking status of PR #%d", prNumber)

	pr, _, err := c.client.PullRequests.Get(ctx, c.personasOwner, c.personasRepo, prNumber)
	if err != nil {
		return "", fmt.Errorf("failed to get PR #%d: %w", prNumber, err)
	}

	if pr.State == nil {
		return "", fmt.Errorf("PR #%d has no state", prNumber)
	}

	status := *pr.State
	c.logger.Debugf("PR #%d status: %s", prNumber, status)
	return status, nil
}

// ListPersonaFolders lists all persona folders in the personas repository
func (c *Client) ListPersonaFolders(ctx context.Context) ([]string, error) {
	c.logger.Debug("Listing persona folders from GitHub")

	_, dirContent, _, err := c.client.Repositories.GetContents(
		ctx, c.personasOwner, c.personasRepo, "personas", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get personas directory: %w", err)
	}

	var folderNames []string
	for _, item := range dirContent {
		if item.Type != nil && *item.Type == "dir" && item.Name != nil {
			folderNames = append(folderNames, *item.Name)
		}
	}

	c.logger.Debugf("Found %d persona folders in GitHub", len(folderNames))
	return folderNames, nil
}

// GetFileModTime gets the last modification time of a file from GitHub
func (c *Client) GetFileModTime(ctx context.Context, filePath string) (time.Time, error) {
	c.logger.Debugf("Getting modification time for file: %s", filePath)

	// Get commits for the specific file to find the last modification
	opts := &github.CommitsListOptions{
		Path: filePath,
		ListOptions: github.ListOptions{
			PerPage: 1, // We only need the most recent commit
		},
	}

	commits, _, err := c.client.Repositories.ListCommits(ctx, c.personasOwner, c.personasRepo, opts)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get commits for file %s: %w", filePath, err)
	}

	if len(commits) == 0 {
		return time.Time{}, fmt.Errorf("no commits found for file %s", filePath)
	}

	// Get the commit date from the most recent commit
	if commits[0].Commit == nil || commits[0].Commit.Committer == nil || commits[0].Commit.Committer.Date == nil {
		return time.Time{}, fmt.Errorf("invalid commit data for file %s", filePath)
	}

	modTime := commits[0].Commit.Committer.Date.Time
	c.logger.Debugf("File %s last modified: %v", filePath, modTime)
	return modTime, nil
}

// FileExists checks if a file exists in the GitHub repository
func (c *Client) FileExists(ctx context.Context, filePath string) bool {
	c.logger.Debugf("Checking if file exists: %s", filePath)

	_, _, _, err := c.client.Repositories.GetContents(
		ctx, c.personasOwner, c.personasRepo, filePath, nil)

	exists := err == nil
	c.logger.Debugf("File %s exists: %v", filePath, exists)
	return exists
}

// CreateSynthesisUpdatePR creates a PR to update synthesized.md from raw outputs
func (c *Client) CreateSynthesisUpdatePR(ctx context.Context, personaName, folderName, synthesizedContent string) (*github.PullRequest, error) {
	// Create branch name for synthesis update
	sanitizedName := strings.ToLower(strings.ReplaceAll(personaName, " ", "-"))
	sanitizedName = strings.ReplaceAll(sanitizedName, "/", "-")
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	branchName := fmt.Sprintf("synthesize/%s-%s", sanitizedName, timestamp)

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
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	// File path for synthesized.md
	filePath := fmt.Sprintf("personas/%s/synthesized.md", folderName)

	// Update the file
	fileOpts := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Regenerate synthesized.md for %s", personaName)),
		Content: []byte(synthesizedContent),
		Branch:  github.String(branchName),
	}

	// Get existing file SHA
	existingFile, _, _, err := c.client.Repositories.GetContents(
		ctx, c.personasOwner, c.personasRepo, filePath,
		&github.RepositoryContentGetOptions{Ref: defaultBranch})

	if err != nil {
		return nil, fmt.Errorf("failed to get existing synthesized.md: %w", err)
	}

	if existingFile != nil {
		fileOpts.SHA = existingFile.SHA
	}

	// Update the file
	_, _, err = c.client.Repositories.UpdateFile(
		ctx, c.personasOwner, c.personasRepo, filePath, fileOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to update file: %w", err)
	}

	// Create pull request
	prBody := fmt.Sprintf(`This PR regenerates the synthesized.md for: **%s**

## 🔄 Regeneration Details
Synthesized.md has been regenerated from the existing raw AI outputs.

## 📊 Source Files Used
- **Claude**: personas/%s/raw/claude.md
- **Gemini**: personas/%s/raw/gemini.md
- **Grok**: personas/%s/raw/grok.md
- **GPT-4**: personas/%s/raw/gpt.md

## 🧬 Synthesis Process
1. Retrieved all raw AI outputs from the repository
2. Applied the standard persona combination prompt
3. Used Gemini to synthesize a comprehensive unified persona
4. Temperature: 0.3 for consistent synthesis

## 📝 Changes
- Only synthesized.md is updated
- Raw files remain unchanged
- This preserves the original AI outputs while refreshing the synthesis

---
*This is an automated PR created by [Studio](https://github.com/twin2ai/studio) synthesize command*`,
		personaName, folderName, folderName, folderName, folderName)

	pr := &github.NewPullRequest{
		Title: github.String(fmt.Sprintf("Regenerate synthesized.md for %s", personaName)),
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
		*pullRequest.Number, []string{"persona", "synthesis", "regeneration", "studio"})
	if err != nil {
		c.logger.Warnf("Failed to add labels to PR: %v", err)
	}

	return pullRequest, nil
}
