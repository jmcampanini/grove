package naming

import (
	"fmt"
	"text/template"

	"github.com/jmcampanini/grove/internal/config"
)

// PullRequestTemplateData contains the values available to the PR branch template.
type PullRequestTemplateData struct {
	Branch string
	Number int
}

// PullRequestWorktreeTemplateData contains the values available to the PR worktree template.
type PullRequestWorktreeTemplateData struct {
	BranchSlug string
	Number     int
	TitleSlug  string
}

// PullRequestNamer handles PR branch and worktree directory name operations.
type PullRequestNamer struct {
	branchTemplate        *template.Template
	maxLength             int
	slugifyOpts           SlugifyOptions
	stripPrefixes         []string
	worktreeLiteralPrefix string
	worktreeTemplate      *template.Template
}

// NewPullRequestNamer creates a namer and validates both PR templates.
func NewPullRequestNamer(prCfg config.PullRequestConfig, namingCfg config.NamingConfig) (*PullRequestNamer, error) {
	branchTemplate, err := parseNameTemplate(
		"branch_template",
		prCfg.BranchTemplate,
		PullRequestTemplateData{Branch: "test/branch", Number: 1},
		0,
		isValidBranchName,
	)
	if err != nil {
		return nil, err
	}

	worktreeTemplate, err := parseNameTemplate(
		"worktree_template",
		prCfg.WorktreeTemplate,
		PullRequestWorktreeTemplateData{BranchSlug: "test-branch", Number: 1, TitleSlug: "test-pr"},
		namingCfg.MaxLength,
		isValidWorktreeName,
	)
	if err != nil {
		return nil, err
	}

	return &PullRequestNamer{
		branchTemplate:        branchTemplate,
		maxLength:             namingCfg.MaxLength,
		slugifyOpts:           SlugifyOptionsFromConfig(namingCfg),
		stripPrefixes:         namingCfg.StripPrefixes,
		worktreeLiteralPrefix: leadingLiteral(worktreeTemplate),
		worktreeTemplate:      worktreeTemplate,
	}, nil
}

// GenerateBranchName renders and validates the uncapped PR branch name.
func (n *PullRequestNamer) GenerateBranchName(data PullRequestTemplateData) (string, error) {
	name, err := renderName(n.branchTemplate, data, 0, isValidBranchName)
	if err != nil {
		return "", fmt.Errorf("failed to generate branch name: %w", err)
	}
	return name, nil
}

// GenerateWorktreeName renders and validates the capped PR worktree directory name.
func (n *PullRequestNamer) GenerateWorktreeName(number int, title, branch string) (string, error) {
	name, err := renderName(
		n.worktreeTemplate,
		PullRequestWorktreeTemplateData{
			BranchSlug: slugifyBranch(branch, n.stripPrefixes, n.slugifyOpts),
			Number:     number,
			TitleSlug:  Slugify(title, n.slugifyOpts),
		},
		n.maxLength,
		isValidWorktreeName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate worktree name: %w", err)
	}
	return name, nil
}

func (n *PullRequestNamer) WorktreeLiteralPrefix() string {
	return n.worktreeLiteralPrefix
}
