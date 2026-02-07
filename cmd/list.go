package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

var fzfFlag bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long: `List all git worktrees in the workspace.

By default, outputs one absolute path per line to stdout.

With --fzf, outputs tab-separated format suitable for fzf integration:
  <path>\t<display>

Example with fzf:
  grove list --fzf | fzf --delimiter '\t' --with-nth 2 --accept-nth 1

Or for older fzf versions:
  grove list --fzf | fzf --delimiter '\t' --with-nth 2 | cut -f1`,
	Args: cobra.NoArgs,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVar(&fzfFlag, "fzf", false, "Output in fzf-compatible format")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return fmt.Errorf("grove must be run inside a git repository")
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return fmt.Errorf("failed to get main worktree path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loader := config.NewDefaultLoader()
	loadResult, err := loader.Load(configPaths)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := loadResult.Config

	// Recreate the git client using the config timeout
	gitClient = git.New(false, cwd, cfg.Git.Timeout)

	worktrees, err := gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	var mainWT *git.Worktree
	var others []git.Worktree
	for i := range worktrees {
		if worktrees[i].AbsolutePath == mainWorktreePath {
			mainWT = &worktrees[i]
		} else {
			others = append(others, worktrees[i])
		}
	}

	namer := naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Slugify)

	sort.Slice(others, func(i, j int) bool {
		return others[i].AbsolutePath < others[j].AbsolutePath
	})

	prPrefix := cfg.PullRequest.WorktreePrefix

	if mainWT != nil {
		if err := outputWorktree(cmd, *mainWT, namer, prPrefix, fzfFlag); err != nil {
			return err
		}
	}
	for _, wt := range others {
		if err := outputWorktree(cmd, wt, namer, prPrefix, fzfFlag); err != nil {
			return err
		}
	}

	return nil
}

func outputWorktree(cmd *cobra.Command, wt git.Worktree, namer *naming.LocalBranchNamer, prPrefix string, fzf bool) error {
	if fzf {
		path, display := formatWorktree(wt, namer, prPrefix)
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", path, display)
		return err
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), wt.AbsolutePath)
	return err
}

func formatWorktree(wt git.Worktree, namer *naming.LocalBranchNamer, prPrefix string) (path, display string) {
	name := getDisplayName(namer, wt.AbsolutePath)
	name = formatWorktreeName(name, filepath.Base(wt.AbsolutePath), prPrefix)

	switch wt.Ref.Type() {
	case git.WorktreeRefTypeBranch:
		branch, _ := wt.Ref.FullBranch()
		return wt.AbsolutePath, fmt.Sprintf("local branch %s %s", name, branch.Name)
	case git.WorktreeRefTypeTag:
		tag, _ := wt.Ref.FullTag()
		return wt.AbsolutePath, fmt.Sprintf("tag %s %s", name, tag.Name)
	case git.WorktreeRefTypeCommit:
		commit := wt.Ref.Commit()
		shortSHA := shortSHASafe(commit.SHA, 7)
		return wt.AbsolutePath, fmt.Sprintf("detached %s %s", name, shortSHA)
	default:
		// Unknown ref type - still show useful output
		commit := wt.Ref.Commit()
		shortSHA := shortSHASafe(commit.SHA, 7)
		return wt.AbsolutePath, fmt.Sprintf("unknown %s %s", name, shortSHA)
	}
}

// getDisplayName returns the display name for a worktree.
// If the basename has the configured prefix, strip it.
// Otherwise, wrap in brackets to indicate non-standard naming.
func getDisplayName(namer *naming.LocalBranchNamer, absPath string) string {
	basename := filepath.Base(absPath)
	if namer.HasPrefix(basename) {
		return namer.ExtractFromAbsolutePath(absPath)
	}
	// Non-standard worktree name - mark with brackets
	return "[" + basename + "]"
}

// shortSHASafe safely truncates a SHA to the specified length.
// Returns the full SHA if shorter than maxLen, or "(no sha)" if empty.
func shortSHASafe(sha string, maxLen int) string {
	if sha == "" {
		return "(no sha)"
	}
	if len(sha) <= maxLen {
		return sha
	}
	return sha[:maxLen]
}

// formatWorktreeName adds a [PR] marker to the display name if the worktree
// directory name starts with the PR prefix.
func formatWorktreeName(displayName, dirName, prPrefix string) string {
	if prPrefix != "" && strings.HasPrefix(dirName, prPrefix) {
		return "[PR] " + displayName
	}
	return displayName
}
