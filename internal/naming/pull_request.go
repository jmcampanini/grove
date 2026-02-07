package naming

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// PullRequestTemplateData contains data available to the branch template.
type PullRequestTemplateData struct {
	BranchName string // PR's head branch (e.g., "feature/add-auth")
	Number     int    // PR number (e.g., 123)
}

// PullRequestNamer handles PR worktree directory name operations.
type PullRequestNamer struct {
	branchTemplate *template.Template
	slugifyOpts    SlugifyOptions
	worktreePrefix string
}

// NewPullRequestNamer creates a namer from pull request and slugify config.
// Returns an error if the template is invalid or produces invalid branch names.
func NewPullRequestNamer(prCfg config.PullRequestConfig, slugCfg config.SlugifyConfig) (*PullRequestNamer, error) {
	tmpl, err := template.New("branch").Parse(prCfg.BranchTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid branch_template: %w", err)
	}

	var buf bytes.Buffer
	testData := PullRequestTemplateData{BranchName: "test/branch", Number: 1}
	if err := tmpl.Execute(&buf, testData); err != nil {
		return nil, fmt.Errorf("branch_template uses invalid field: %w", err)
	}

	if ok, reason := isValidBranchName(buf.String()); !ok {
		return nil, fmt.Errorf("branch_template produces invalid branch name: %s", reason)
	}

	return &PullRequestNamer{
		branchTemplate: tmpl,
		worktreePrefix: prCfg.WorktreePrefix,
		slugifyOpts: SlugifyOptions{
			CollapseDashes:     slugCfg.CollapseDashes,
			HashLength:         slugCfg.HashLength,
			Lowercase:          slugCfg.Lowercase,
			MaxLength:          slugCfg.MaxLength,
			ReplaceNonAlphaNum: slugCfg.ReplaceNonAlphanum,
			TrimDashes:         slugCfg.TrimDashes,
		},
	}, nil
}

// GenerateBranchName executes the template with PR data to produce a local branch name.
func (n *PullRequestNamer) GenerateBranchName(pr PullRequestTemplateData) (string, error) {
	var buf bytes.Buffer
	if err := n.branchTemplate.Execute(&buf, pr); err != nil {
		return "", fmt.Errorf("failed to generate branch name: %w", err)
	}
	return buf.String(), nil
}

// GenerateWorktreeName applies slugify and smart prefix detection to create a worktree directory name.
// If the slugified name already starts with worktreePrefix, the prefix is not added again.
func (n *PullRequestNamer) GenerateWorktreeName(branchName string) string {
	slug := Slugify(branchName, n.slugifyOpts)
	if slug == "" {
		return ""
	}

	// Smart detection: skip prefix if slug already starts with it
	if strings.HasPrefix(slug, n.worktreePrefix) {
		return slug
	}

	return n.worktreePrefix + slug
}

// isValidBranchName validates git branch name with simplified rules.
// Returns (true, "") if valid, or (false, reason) describing why it's invalid.
// Edge cases not covered here will fail at git worktree creation time
// with clear git error messages.
func isValidBranchName(name string) (bool, string) {
	if name == "" {
		return false, "empty"
	}

	if strings.HasPrefix(name, "-") {
		return false, "starts with '-'"
	}

	if strings.Contains(name, "..") {
		return false, "contains '..'"
	}

	for _, r := range name {
		if r < 32 || r == 127 {
			return false, "contains control character"
		}
	}

	return true, ""
}
