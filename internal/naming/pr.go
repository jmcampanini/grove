package naming

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// PRTemplateData contains data available to the branch template.
type PRTemplateData struct {
	BranchName string // PR's head branch (e.g., "feature/add-auth")
	Number     int    // PR number (e.g., 123)
}

// PRWorktreeNamer handles PR worktree directory name operations.
type PRWorktreeNamer struct {
	branchTemplate *template.Template
	slugifyOpts    SlugifyOptions
	worktreePrefix string
}

// NewPRWorktreeNamer creates a namer from PR and slugify config.
// Returns an error if the template is invalid or produces invalid branch names.
func NewPRWorktreeNamer(prCfg config.PRConfig, slugCfg config.SlugifyConfig) (*PRWorktreeNamer, error) {
	// 1. Parse template
	tmpl, err := template.New("branch").Parse(prCfg.BranchTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid branch_template: %w", err)
	}

	// 2. Execute with test data to verify fields exist
	var buf bytes.Buffer
	testData := PRTemplateData{BranchName: "test/branch", Number: 1}
	if err := tmpl.Execute(&buf, testData); err != nil {
		return nil, fmt.Errorf("branch_template uses invalid field: %w", err)
	}

	// 3. Validate output is valid git branch name
	if ok, reason := isValidBranchName(buf.String()); !ok {
		return nil, fmt.Errorf("branch_template produces invalid branch name: %s", reason)
	}

	return &PRWorktreeNamer{
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
func (n *PRWorktreeNamer) GenerateBranchName(pr PRTemplateData) (string, error) {
	var buf bytes.Buffer
	if err := n.branchTemplate.Execute(&buf, pr); err != nil {
		return "", fmt.Errorf("failed to generate branch name: %w", err)
	}
	return buf.String(), nil
}

// GenerateWorktreeName applies slugify and smart prefix detection to create a worktree directory name.
// If the slugified name already starts with worktreePrefix, the prefix is not added again.
func (n *PRWorktreeNamer) GenerateWorktreeName(branchName string) string {
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
