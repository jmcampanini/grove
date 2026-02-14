package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Interactively remove stale worktrees",
	Long: `Prune identifies worktrees that are likely no longer needed and lets you
remove them interactively.

A worktree is considered prunable if:
  - Its associated PR has been merged
  - Its associated PR has been closed
  - Its upstream tracking branch no longer exists on the remote
  - Its directory no longer exists on disk (orphaned)

The main worktree is never prunable.`,
	Args: cobra.NoArgs,
	RunE: runPrune,
}

func init() {
	pruneCmd.GroupID = "worktree"
	rootCmd.AddCommand(pruneCmd)
}

type prunable struct {
	branchName string
	name       string
	path       string
	reason     string
}

func runPrune(cmd *cobra.Command, _ []string) error {
	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	ghClient, err := rt.newCachedGitHubClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	ctx := &statusContext{
		ghClient:         ghClient,
		gitClient:        git.New(false, rt.mainWorktreePath, rt.cfg.Git.Timeout),
		mainWorktreePath: rt.mainWorktreePath,
	}

	return executePrune(cmd.OutOrStdout(), ctx)
}

func executePrune(w io.Writer, ctx *statusContext) error {
	statuses, err := gatherStatuses(ctx)
	if err != nil {
		return err
	}

	remoteBranches, err := buildRemoteBranchSet(ctx.gitClient)
	if err != nil {
		return err
	}

	prunables := findPrunable(statuses, remoteBranches)
	if len(prunables) == 0 {
		_, err := fmt.Fprintln(w, "Nothing to prune.")
		return err
	}

	selected, err := promptPruneSelection(prunables)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		_, err := fmt.Fprintln(w, "No worktrees selected.")
		return err
	}

	confirmed, err := promptPruneConfirm(len(selected))
	if err != nil {
		return err
	}
	if !confirmed {
		_, err := fmt.Fprintln(w, "Aborted.")
		return err
	}

	return executeRemovals(w, ctx.gitClient, selected)
}

func buildRemoteBranchSet(gitClient git.Git) (map[string]bool, error) {
	remotes, err := gitClient.ListRemotes()
	if err != nil {
		return nil, fmt.Errorf("failed to list remotes: %w", err)
	}

	remoteBranches := make(map[string]bool)
	for _, remote := range remotes {
		branches, err := gitClient.ListRemoteBranches(remote)
		if err != nil {
			return nil, fmt.Errorf("failed to list branches for remote %q: %w", remote, err)
		}
		for _, b := range branches {
			remoteBranches[remote+"/"+b.Name] = true
		}
	}
	return remoteBranches, nil
}

func findPrunable(statuses []worktreeStatus, remoteBranches map[string]bool) []prunable {
	var result []prunable
	for _, ws := range statuses {
		if ws.isMain {
			continue
		}

		if reason := pruneReason(ws, remoteBranches); reason != "" {
			result = append(result, prunable{
				branchName: ws.branchName,
				name:       filepath.Base(ws.absPath),
				path:       ws.absPath,
				reason:     reason,
			})
		}
	}
	return result
}

func pruneReason(ws worktreeStatus, remoteBranches map[string]bool) string {
	if _, err := os.Stat(ws.absPath); os.IsNotExist(err) {
		return "orphaned"
	}

	if ws.pr != nil {
		switch ws.pr.State {
		case github.PRStateMerged:
			return fmt.Sprintf("PR #%d merged", ws.pr.Number)
		case github.PRStateClosed:
			return fmt.Sprintf("PR #%d closed", ws.pr.Number)
		}
	}

	if ws.tracking.upstream != "" && !remoteBranches[ws.tracking.upstream] {
		return "upstream gone"
	}

	return ""
}

func promptPruneSelection(prunables []prunable) ([]prunable, error) {
	options := make([]huh.Option[int], len(prunables))
	for i, p := range prunables {
		label := fmt.Sprintf("%s  %s",
			p.name,
			lipgloss.NewStyle().Foreground(colorGray).Render("("+p.reason+")"),
		)
		options[i] = huh.NewOption(label, i).Selected(true)
	}

	var selected []int
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title("Select worktrees to remove").
				Options(options...).
				Value(&selected),
		),
	).Run()
	if err != nil {
		return nil, err
	}

	var result []prunable
	for _, idx := range selected {
		result = append(result, prunables[idx])
	}
	return result, nil
}

func promptPruneConfirm(count int) (bool, error) {
	noun := "worktree"
	if count > 1 {
		noun = "worktrees"
	}

	var confirmed bool
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Remove %d %s?", count, noun)).
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	).Run()
	if err != nil {
		return false, err
	}
	return confirmed, nil
}

func executeRemovals(w io.Writer, gitClient git.Git, selected []prunable) error {
	var removed []string
	var errs []string

	for _, p := range selected {
		var err error
		if p.reason == "orphaned" {
			err = removeOrphanedWorktree(gitClient, p.branchName)
		} else {
			err = removeWorktreeAndBranch(gitClient, p.path, p.branchName)
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("  %s: %v", p.name, err))
		} else {
			removed = append(removed, p.name)
		}
	}

	if len(removed) > 0 {
		if _, err := fmt.Fprintf(w, "Removed %d worktree(s): %s\n", len(removed), strings.Join(removed, ", ")); err != nil {
			return err
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("some removals failed:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

func removeOrphanedWorktree(gitClient git.Git, branchName string) error {
	if err := gitClient.PruneWorktrees(); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}
	if branchName != "" {
		if err := gitClient.DeleteBranch(branchName, true); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
			}
		}
	}
	return nil
}
