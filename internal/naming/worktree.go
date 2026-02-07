package naming

import (
	"path/filepath"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// LocalBranchNamer handles worktree directory name operations.
type LocalBranchNamer struct {
	prefix            string
	slugifyOpts       SlugifyOptions
	stripBranchPrefix []string
}

// NewLocalBranchNamer creates a namer from config.
func NewLocalBranchNamer(localBranchCfg config.LocalBranchConfig, slugCfg config.SlugifyConfig) *LocalBranchNamer {
	return &LocalBranchNamer{
		prefix:            localBranchCfg.WorktreePrefix,
		stripBranchPrefix: localBranchCfg.StripBranchPrefix,
		slugifyOpts: SlugifyOptions{
			CollapseDashes:     slugCfg.CollapseDashes,
			HashLength:         slugCfg.HashLength,
			Lowercase:          slugCfg.Lowercase,
			MaxLength:          slugCfg.MaxLength,
			ReplaceNonAlphaNum: slugCfg.ReplaceNonAlphanum,
			TrimDashes:         slugCfg.TrimDashes,
		},
	}
}

// Generate creates a worktree name from a branch name.
func (n *LocalBranchNamer) Generate(branchName string) string {
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

	return n.prefix + slug
}

// ExtractFromAbsolutePath returns the display name from an absolute worktree path.
// It extracts the basename and strips the configured prefix if present.
// If the name doesn't have the expected prefix, returns the original basename.
func (n *LocalBranchNamer) ExtractFromAbsolutePath(absPath string) string {
	basename := filepath.Base(absPath)
	if strings.HasPrefix(basename, n.prefix) {
		return strings.TrimPrefix(basename, n.prefix)
	}
	return basename
}

// HasPrefix checks if the given directory name has the configured prefix.
func (n *LocalBranchNamer) HasPrefix(name string) bool {
	return strings.HasPrefix(name, n.prefix)
}
