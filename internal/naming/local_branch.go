package naming

import (
	"path/filepath"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
)

type LocalBranchNamer struct {
	branchPrefix      string
	slugifyOpts       SlugifyOptions
	stripBranchPrefix []string
	worktreePrefix    string
}

func NewLocalBranchNamer(localBranchCfg config.LocalBranchConfig, slugCfg config.SlugifyConfig) *LocalBranchNamer {
	return &LocalBranchNamer{
		branchPrefix:      localBranchCfg.BranchPrefix,
		slugifyOpts:       SlugifyOptionsFromConfig(slugCfg),
		stripBranchPrefix: localBranchCfg.StripBranchPrefix,
		worktreePrefix:    localBranchCfg.WorktreePrefix,
	}
}

func (n *LocalBranchNamer) GenerateBranchName(phrase string) string {
	slug := Slugify(phrase, n.slugifyOpts)
	if slug == "" {
		return ""
	}
	return n.branchPrefix + slug
}

func (n *LocalBranchNamer) GenerateWorktreeName(branchName string) string {
	if branchName == "" {
		return ""
	}

	name := branchName
	for _, prefix := range n.stripBranchPrefix {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}

	slug := Slugify(name, n.slugifyOpts)
	if slug == "" {
		return ""
	}

	return n.worktreePrefix + slug
}

func (n *LocalBranchNamer) ExtractFromAbsolutePath(absPath string) string {
	basename := filepath.Base(absPath)
	if strings.HasPrefix(basename, n.worktreePrefix) {
		return strings.TrimPrefix(basename, n.worktreePrefix)
	}
	return basename
}

func (n *LocalBranchNamer) HasPrefix(name string) bool {
	return strings.HasPrefix(name, n.worktreePrefix)
}
