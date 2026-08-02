package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var fzf bool
	cmd := &cobra.Command{
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
		Args:    cobra.NoArgs,
		GroupID: "worktree",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd, fzf)
		},
	}
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Output in fzf-compatible format")
	return cmd
}

type listContext struct {
	cfg              config.Config
	gitClient        git.Git
	mainWorktreePath string
}

func runList(cmd *cobra.Command, fzf bool) error {
	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &listContext{
		cfg:              rt.cfg,
		gitClient:        rt.gitClient,
		mainWorktreePath: rt.mainWorktreePath,
	}

	return executeList(cmd.OutOrStdout(), ctx, fzf)
}

func executeList(w io.Writer, ctx *listContext, fzf bool) error {
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	var mainWT *git.Worktree
	var others []git.Worktree
	for i := range worktrees {
		if worktrees[i].AbsolutePath == ctx.mainWorktreePath {
			mainWT = &worktrees[i]
		} else {
			others = append(others, worktrees[i])
		}
	}

	sort.Slice(others, func(i, j int) bool {
		return others[i].AbsolutePath < others[j].AbsolutePath
	})

	namer := naming.NewLocalBranchNamer(ctx.cfg.LocalBranch, ctx.cfg.Slugify)
	prWorktreePrefix := ctx.cfg.PullRequest.WorktreePrefix

	if mainWT != nil {
		if err := writeWorktree(w, *mainWT, namer, prWorktreePrefix, fzf); err != nil {
			return err
		}
	}
	for _, wt := range others {
		if err := writeWorktree(w, wt, namer, prWorktreePrefix, fzf); err != nil {
			return err
		}
	}

	return nil
}

func writeWorktree(w io.Writer, wt git.Worktree, namer *naming.LocalBranchNamer, prWorktreePrefix string, fzf bool) error {
	if fzf {
		path, display := formatWorktree(wt, namer, prWorktreePrefix)
		_, err := fmt.Fprintf(w, "%s\t%s\n", path, display)
		return err
	}
	_, err := fmt.Fprintln(w, wt.AbsolutePath)
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

func getDisplayName(namer *naming.LocalBranchNamer, absPath string) string {
	basename := filepath.Base(absPath)
	if namer.HasPrefix(basename) {
		return namer.ExtractFromAbsolutePath(absPath)
	}
	// Non-standard worktree name - mark with brackets
	return "[" + basename + "]"
}

func shortSHASafe(sha string, maxLen int) string {
	if sha == "" {
		return "(no sha)"
	}
	if len(sha) <= maxLen {
		return sha
	}
	return sha[:maxLen]
}

func formatWorktreeName(displayName, dirName, prWorktreePrefix string) string {
	if prWorktreePrefix != "" && strings.HasPrefix(dirName, prWorktreePrefix) {
		return "[PR] " + displayName
	}
	return displayName
}
