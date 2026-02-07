package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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
	env, err := initFromEnv()
	if err != nil {
		return err
	}
	cfg := env.cfg

	worktrees, err := env.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	var mainWT *git.Worktree
	var others []git.Worktree
	for i := range worktrees {
		if worktrees[i].AbsolutePath == env.mainWorktreePath {
			mainWT = &worktrees[i]
		} else {
			others = append(others, worktrees[i])
		}
	}

	namer := naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Slugify)

	sort.Slice(others, func(i, j int) bool {
		return others[i].AbsolutePath < others[j].AbsolutePath
	})

	prWorktreePrefix := cfg.PullRequest.WorktreePrefix

	if mainWT != nil {
		if err := outputWorktree(cmd, *mainWT, namer, prWorktreePrefix, fzfFlag); err != nil {
			return err
		}
	}
	for _, wt := range others {
		if err := outputWorktree(cmd, wt, namer, prWorktreePrefix, fzfFlag); err != nil {
			return err
		}
	}

	return nil
}

func outputWorktree(cmd *cobra.Command, wt git.Worktree, namer *naming.LocalBranchNamer, prWorktreePrefix string, fzf bool) error {
	if fzf {
		path, display := formatWorktree(wt, namer, prWorktreePrefix)
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", path, display)
		return err
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), wt.AbsolutePath)
	return err
}

func formatWorktree(wt git.Worktree, namer *naming.LocalBranchNamer, prWorktreePrefix string) (path, display string) {
	name := getDisplayName(namer, wt.AbsolutePath)
	name = formatWorktreeName(name, filepath.Base(wt.AbsolutePath), prWorktreePrefix)

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
func formatWorktreeName(displayName, dirName, prWorktreefprefix string) string {
	if prWorktreefprefix != "" && strings.HasPrefix(dirName, prWorktreefprefix) {
		return "[PR] " + displayName
	}
	return displayName
}
