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
	branchTemplate          *template.Template
	recreatedBranchTemplate *template.Template
	slugifyOpts             SlugifyOptions
	worktreePrefix          string
}

// NewPullRequestNamer creates a namer from pull request and slugify config.
// Returns an error if the template is invalid or produces invalid branch names.
func NewPullRequestNamer(prCfg config.PullRequestConfig, slugCfg config.SlugifyConfig) (*PullRequestNamer, error) {
	branchTmpl, err := parseBranchTemplate("branch_template", prCfg.BranchTemplate)
	if err != nil {
		return nil, err
	}

	var recreatedTmpl *template.Template
	if prCfg.RecreatedBranchTemplate != "" {
		recreatedTmpl, err = parseBranchTemplate("recreated_branch_template", prCfg.RecreatedBranchTemplate)
		if err != nil {
			return nil, err
		}
	}

	return &PullRequestNamer{
		branchTemplate:          branchTmpl,
		recreatedBranchTemplate: recreatedTmpl,
		worktreePrefix:          prCfg.WorktreePrefix,
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

func parseBranchTemplate(name, raw string) (*template.Template, error) {
	tmpl, err := template.New(name).Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", name, err)
	}

	testData := PullRequestTemplateData{BranchName: "test/branch", Number: 1}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, testData); err != nil {
		return nil, fmt.Errorf("%s uses invalid field: %w", name, err)
	}

	if ok, reason := isValidBranchName(buf.String()); !ok {
		return nil, fmt.Errorf("%s produces invalid branch name: %s", name, reason)
	}

	return tmpl, nil
}

// GenerateBranchName executes the template with PR data to produce a local branch name.
func (n *PullRequestNamer) GenerateBranchName(pr PullRequestTemplateData) (string, error) {
	var buf bytes.Buffer
	if err := n.branchTemplate.Execute(&buf, pr); err != nil {
		return "", fmt.Errorf("failed to generate branch name: %w", err)
	}
	return buf.String(), nil
}

// GenerateRecreatedBranchName generates a branch name for a recreated (deleted) PR branch.
func (n *PullRequestNamer) GenerateRecreatedBranchName(pr PullRequestTemplateData) (string, error) {
	if n.recreatedBranchTemplate == nil {
		return "", fmt.Errorf("no recreated branch template configured")
	}
	var buf bytes.Buffer
	if err := n.recreatedBranchTemplate.Execute(&buf, pr); err != nil {
		return "", fmt.Errorf("failed to generate recreated branch name: %w", err)
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
