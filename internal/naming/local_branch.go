package naming

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jmcampanini/grove/internal/config"
)

// LocalBranchTemplateData contains the values available to a local branch template.
type LocalBranchTemplateData struct {
	PhraseSlug string
}

// LocalWorktreeTemplateData contains the values available to a local worktree template.
type LocalWorktreeTemplateData struct {
	BranchSlug string
}

type LocalBranchNamer struct {
	branchTemplate        *template.Template
	maxLength             int
	slugifyOpts           SlugifyOptions
	stripPrefixes         []string
	worktreeLiteralPrefix string
	worktreeTemplate      *template.Template
}

func NewLocalBranchNamer(localCfg config.LocalBranchConfig, namingCfg config.NamingConfig) (*LocalBranchNamer, error) {
	branchTemplate, err := parseNameTemplate(
		"branch_template",
		localCfg.BranchTemplate,
		LocalBranchTemplateData{PhraseSlug: "test-phrase"},
		namingCfg.MaxLength,
		isValidBranchName,
	)
	if err != nil {
		return nil, err
	}

	worktreeTemplate, err := parseNameTemplate(
		"worktree_template",
		localCfg.WorktreeTemplate,
		LocalWorktreeTemplateData{BranchSlug: "test-branch"},
		namingCfg.MaxLength,
		isValidWorktreeName,
	)
	if err != nil {
		return nil, err
	}

	return &LocalBranchNamer{
		branchTemplate:        branchTemplate,
		maxLength:             namingCfg.MaxLength,
		slugifyOpts:           SlugifyOptionsFromConfig(namingCfg),
		stripPrefixes:         namingCfg.StripPrefixes,
		worktreeLiteralPrefix: leadingLiteral(worktreeTemplate),
		worktreeTemplate:      worktreeTemplate,
	}, nil
}

func (n *LocalBranchNamer) GenerateBranchName(phrase string) (string, error) {
	phraseSlug := Slugify(phrase, n.slugifyOpts)
	if phraseSlug == "" {
		return "", fmt.Errorf("phrase produces %w", ErrEmptySlug)
	}

	name, err := renderName(
		n.branchTemplate,
		LocalBranchTemplateData{PhraseSlug: phraseSlug},
		n.maxLength,
		isValidBranchName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate branch name: %w", err)
	}
	return name, nil
}

func (n *LocalBranchNamer) GenerateWorktreeName(branch string) (string, error) {
	branchSlug := slugifyBranch(branch, n.stripPrefixes, n.slugifyOpts)
	if branchSlug == "" {
		return "", fmt.Errorf("branch produces %w", ErrEmptySlug)
	}

	name, err := renderName(
		n.worktreeTemplate,
		LocalWorktreeTemplateData{BranchSlug: branchSlug},
		n.maxLength,
		isValidWorktreeName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate worktree name: %w", err)
	}
	return name, nil
}

func (n *LocalBranchNamer) WorktreeLiteralPrefix() string {
	return n.worktreeLiteralPrefix
}

func (n *LocalBranchNamer) ExtractFromAbsolutePath(absPath string) string {
	basename := filepath.Base(absPath)
	return strings.TrimPrefix(basename, n.worktreeLiteralPrefix)
}

func (n *LocalBranchNamer) HasPrefix(name string) bool {
	return strings.HasPrefix(name, n.worktreeLiteralPrefix)
}
