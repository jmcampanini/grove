package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/jmcampanini/grove/internal/worktree"
	"github.com/spf13/cobra"
)

func newPruneCmd(services commandServices) *cobra.Command {
	return &cobra.Command{
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
		Args:    cobra.NoArgs,
		GroupID: "worktree",
		RunE:    services.runPrune,
	}
}

type pruneEvidenceKind int

const (
	_ pruneEvidenceKind = iota
	pruneEvidenceOrphaned
	pruneEvidencePR
	pruneEvidenceUpstreamGone
)

type pruneEvidence struct {
	kind     pruneEvidenceKind
	prNumber int
	prState  github.PRState
	upstream string
}

func (e pruneEvidence) String() string {
	switch e.kind {
	case pruneEvidenceOrphaned:
		return "orphaned"
	case pruneEvidencePR:
		return fmt.Sprintf("PR #%d %s", e.prNumber, strings.ToLower(e.prState.String()))
	case pruneEvidenceUpstreamGone:
		return "upstream gone"
	default:
		return ""
	}
}

type prunable struct {
	branchName string
	dirty      bool
	evidence   pruneEvidence
	headSHA    string
	name       string
	path       string
}

func (services commandServices) runPrune(cmd *cobra.Command, _ []string) error {
	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &worktree.StatusContext{
		GitHubClient:     rt.NewUncachedGitHubClient(),
		GitClient:        git.New(cmd.Context(), false, rt.MainWorktreePath, rt.Config.Git.Timeout, rt.Logger),
		MainWorktreePath: rt.MainWorktreePath,
	}

	return executePrune(cmd.OutOrStdout(), ctx)
}

func executePrune(w io.Writer, ctx *worktree.StatusContext) error {
	statuses, err := gatherPruneStatuses(ctx)
	if err != nil {
		return err
	}
	if err := renderLockedWorktrees(w, statuses); err != nil {
		return err
	}

	remoteBranches, err := buildRemoteBranchSet(ctx.GitClient)
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
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	if len(selected) == 0 {
		_, err := fmt.Fprintln(w, "No worktrees selected.")
		return err
	}

	confirmed, err := promptPruneConfirm(len(selected))
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	if !confirmed {
		_, err := fmt.Fprintln(w, "Aborted.")
		return err
	}

	return executeRemovals(w, ctx.GitClient, selected, func(p prunable) (string, bool, error) {
		return revalidatePrunable(ctx, p)
	})
}

func gatherPruneStatuses(ctx *worktree.StatusContext) ([]worktree.Status, error) {
	worktrees, branchMap, err := worktree.LoadWithBranches(ctx.GitClient)
	if err != nil {
		return nil, err
	}

	statuses := make([]worktree.Status, 0, len(worktrees))
	for _, wt := range worktrees {
		status, err := buildPruneWorktreeStatus(ctx, wt, branchMap)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func buildPruneWorktreeStatus(ctx *worktree.StatusContext, wt git.Worktree, branchMap map[string]git.LocalBranch) (worktree.Status, error) {
	status := worktree.NewStatus(ctx.MainWorktreePath, wt, branchMap)
	if status.IsMain || status.Locked {
		return status, nil
	}

	if _, err := os.Stat(status.AbsPath); err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return worktree.Status{}, fmt.Errorf("failed to inspect worktree %q: %w", filepath.Base(status.AbsPath), err)
	}
	if status.BranchName == "" {
		return status, nil
	}

	dirty, err := ctx.GitClient.IsWorktreeDirty(status.AbsPath)
	if err != nil {
		return worktree.Status{}, fmt.Errorf("failed to inspect dirty state for worktree %q: %w", filepath.Base(status.AbsPath), err)
	}
	status.Dirty = dirty

	pr, err := ctx.GitHubClient.GetPullRequestByBranch(status.BranchName)
	if err != nil {
		return worktree.Status{}, fmt.Errorf("failed to get PR state for branch %q: %w", status.BranchName, err)
	}
	status.PR = pr
	if pr != nil {
		status.Kind = "PR"
	} else {
		status.Kind = "local"
	}
	return status, nil
}

func renderLockedWorktrees(w io.Writer, statuses []worktree.Status) error {
	var locked []worktree.Status
	for _, status := range statuses {
		if status.Locked && !status.IsMain {
			locked = append(locked, status)
		}
	}
	if len(locked) == 0 {
		return nil
	}

	if _, err := fmt.Fprintln(w, "Locked worktrees excluded from prune:"); err != nil {
		return err
	}
	for _, status := range locked {
		branch := status.BranchName
		if branch == "" {
			branch = "detached"
		}
		if _, err := fmt.Fprintf(w, "  %s  %s  (locked)\n", filepath.Base(status.AbsPath), branch); err != nil {
			return err
		}
	}
	return nil
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

func findPrunable(statuses []worktree.Status, remoteBranches map[string]bool) []prunable {
	var result []prunable
	for _, ws := range statuses {
		if ws.IsMain || ws.Locked {
			continue
		}

		if evidence, ok := classifyPrunable(ws, remoteBranches); ok {
			result = append(result, prunable{
				branchName: ws.BranchName,
				dirty:      ws.Dirty,
				evidence:   evidence,
				headSHA:    ws.HeadSHA,
				name:       filepath.Base(ws.AbsPath),
				path:       ws.AbsPath,
			})
		}
	}
	return result
}

func classifyPrunable(ws worktree.Status, remoteBranches map[string]bool) (pruneEvidence, bool) {
	if _, err := os.Stat(ws.AbsPath); os.IsNotExist(err) {
		return pruneEvidence{kind: pruneEvidenceOrphaned}, true
	}

	if ws.PR != nil {
		switch ws.PR.State {
		case github.PRStateMerged, github.PRStateClosed:
			return pruneEvidence{
				kind:     pruneEvidencePR,
				prNumber: ws.PR.Number,
				prState:  ws.PR.State,
			}, true
		}
	}

	if ws.Tracking.Upstream != "" && !remoteBranches[ws.Tracking.Upstream] {
		return pruneEvidence{
			kind:     pruneEvidenceUpstreamGone,
			upstream: ws.Tracking.Upstream,
		}, true
	}

	return pruneEvidence{}, false
}

type pruneState struct {
	description string
	evidence    pruneEvidence
}

func revalidatePrunable(ctx *worktree.StatusContext, candidate prunable) (string, bool, error) {
	identityChange, err := revalidatePrunableIdentity(ctx, candidate)
	if err != nil {
		return "", false, err
	}
	if identityChange != "" {
		return identityChange, false, nil
	}

	state, err := currentPruneState(ctx, candidate)
	if err != nil {
		return "", false, err
	}
	if state.evidence != candidate.evidence {
		return state.description, false, nil
	}

	localChange, err := revalidateLocalBarrier(ctx, candidate)
	if err != nil {
		return "", false, err
	}
	if localChange != "" {
		return localChange, false, nil
	}
	return state.description, true, nil
}

func revalidatePrunableIdentity(ctx *worktree.StatusContext, candidate prunable) (string, error) {
	worktrees, err := ctx.GitClient.ListWorktrees()
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	var current *git.Worktree
	for i := range worktrees {
		if worktrees[i].AbsolutePath == candidate.path {
			current = &worktrees[i]
			break
		}
	}
	if current == nil {
		return "worktree is no longer registered", nil
	}
	if current.AbsolutePath == ctx.MainWorktreePath {
		return "path is now the main worktree", nil
	}
	if current.Locked {
		return "worktree is locked", nil
	}

	branchName := worktree.ExtractBranchName(current)
	if branchName != candidate.branchName {
		if branchName == "" {
			return "path is now detached", nil
		}
		return fmt.Sprintf("path now maps to branch %q", branchName), nil
	}

	currentHead := current.CommitSHA()
	if currentHead != candidate.headSHA {
		return fmt.Sprintf(
			"HEAD changed from %s to %s",
			worktree.ShortSHASafe(candidate.headSHA, 7),
			worktree.ShortSHASafe(currentHead, 7),
		), nil
	}
	if candidate.evidence.kind == pruneEvidenceOrphaned {
		return "", nil
	}

	dirty, err := ctx.GitClient.IsWorktreeDirty(candidate.path)
	if err != nil {
		return "", fmt.Errorf("failed to inspect dirty state for worktree %q: %w", candidate.name, err)
	}
	if dirty != candidate.dirty {
		if dirty {
			return "worktree has uncommitted changes", nil
		}
		return "worktree is now clean", nil
	}
	return "", nil
}

func revalidateLocalBarrier(ctx *worktree.StatusContext, candidate prunable) (string, error) {
	if candidate.evidence.kind == pruneEvidenceUpstreamGone {
		branches, err := ctx.GitClient.ListLocalBranches()
		if err != nil {
			return "", fmt.Errorf("failed to list branches: %w", err)
		}
		branch, ok := findLocalBranch(branches, candidate.branchName)
		if !ok {
			return "local branch no longer exists", nil
		}
		if branch.UpstreamName != candidate.evidence.upstream {
			if branch.UpstreamName == "" {
				return "branch no longer has an upstream", nil
			}
			return fmt.Sprintf("upstream is now %q", branch.UpstreamName), nil
		}

		remoteBranches, err := buildRemoteBranchSet(ctx.GitClient)
		if err != nil {
			return "", err
		}
		if remoteBranches[candidate.evidence.upstream] {
			return fmt.Sprintf("upstream %q exists", candidate.evidence.upstream), nil
		}
	}

	_, statErr := os.Stat(candidate.path)
	pathMissing := os.IsNotExist(statErr)
	if statErr != nil && !pathMissing {
		return "", fmt.Errorf("failed to inspect worktree %q: %w", candidate.name, statErr)
	}
	if candidate.evidence.kind == pruneEvidenceOrphaned && !pathMissing {
		return "path now exists", nil
	}
	if candidate.evidence.kind != pruneEvidenceOrphaned && pathMissing {
		return pruneEvidence{kind: pruneEvidenceOrphaned}.String(), nil
	}

	return revalidatePrunableIdentity(ctx, candidate)
}

func currentPruneState(ctx *worktree.StatusContext, candidate prunable) (pruneState, error) {
	if _, err := os.Stat(candidate.path); err != nil {
		if os.IsNotExist(err) {
			evidence := pruneEvidence{kind: pruneEvidenceOrphaned}
			return pruneState{description: evidence.String(), evidence: evidence}, nil
		}
		return pruneState{}, fmt.Errorf("failed to inspect worktree %q: %w", candidate.name, err)
	}
	if candidate.branchName == "" {
		return pruneState{description: "not prunable"}, nil
	}

	branches, err := ctx.GitClient.ListLocalBranches()
	if err != nil {
		return pruneState{}, fmt.Errorf("failed to list branches: %w", err)
	}
	branch, ok := findLocalBranch(branches, candidate.branchName)
	if !ok {
		return pruneState{description: "local branch no longer exists"}, nil
	}

	ws := worktree.Status{
		AbsPath:    candidate.path,
		BranchName: candidate.branchName,
		Tracking:   worktree.TrackingInfo{Upstream: branch.UpstreamName},
	}
	pr, err := ctx.GitHubClient.GetPullRequestByBranch(candidate.branchName)
	if err != nil {
		return pruneState{}, fmt.Errorf("failed to refresh PR state for branch %q: %w", candidate.branchName, err)
	}
	ws.PR = pr

	if evidence, ok := classifyPrunable(ws, nil); ok && evidence.kind == pruneEvidencePR {
		return pruneState{description: evidence.String(), evidence: evidence}, nil
	}

	remoteBranches, err := buildRemoteBranchSet(ctx.GitClient)
	if err != nil {
		return pruneState{}, err
	}
	evidence, ok := classifyPrunable(ws, remoteBranches)
	if ok {
		return pruneState{description: evidence.String(), evidence: evidence}, nil
	}
	return pruneState{description: describeCurrentPruneState(ws, remoteBranches)}, nil
}

func findLocalBranch(branches []git.LocalBranch, name string) (git.LocalBranch, bool) {
	for _, branch := range branches {
		if branch.Name == name {
			return branch, true
		}
	}
	return git.LocalBranch{}, false
}

func describeCurrentPruneState(ws worktree.Status, remoteBranches map[string]bool) string {
	if ws.PR != nil {
		return fmt.Sprintf("PR #%d %s", ws.PR.Number, strings.ToLower(ws.PR.State.String()))
	}
	if ws.Tracking.Upstream != "" && remoteBranches[ws.Tracking.Upstream] {
		return fmt.Sprintf("upstream %q exists", ws.Tracking.Upstream)
	}
	return "not prunable"
}

func pruneKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc", "q"))
	return km
}

func promptPruneSelection(prunables []prunable) ([]prunable, error) {
	options := make([]huh.Option[int], len(prunables))
	for i, p := range prunables {
		label := fmt.Sprintf("%s  %s",
			p.name,
			lipgloss.NewStyle().Foreground(render.ColorGray).Render("("+p.evidence.String()+")"),
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
	).WithKeyMap(pruneKeyMap()).Run()
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
	).WithKeyMap(pruneKeyMap()).Run()
	if err != nil {
		return false, err
	}
	return confirmed, nil
}

type pruneResult struct {
	err        error
	prunable   prunable
	skipReason string
}

type pruneRevalidator func(prunable) (string, bool, error)

func executeRemovals(w io.Writer, gitClient git.Git, selected []prunable, revalidate pruneRevalidator) error {
	results := make([]pruneResult, len(selected))
	for i, p := range selected {
		current, unchanged, err := revalidate(p)
		if err != nil {
			results[i] = pruneResult{
				err:      fmt.Errorf("revalidation failed: %w", err),
				prunable: p,
			}
			continue
		}
		if !unchanged {
			results[i] = pruneResult{
				prunable: p,
				skipReason: fmt.Sprintf(
					"skipped: was %s; current: %s; rerun grove prune",
					p.evidence.String(),
					current,
				),
			}
			continue
		}

		if p.evidence.kind == pruneEvidenceOrphaned {
			err = removeOrphanedWorktree(gitClient, p.path, p.branchName)
		} else {
			err = removeWorktreeAndBranch(gitClient, p.path, p.branchName)
		}
		results[i] = pruneResult{err: err, prunable: p}
	}

	return renderPruneResults(w, results)
}

func renderPruneResults(w io.Writer, results []pruneResult) error {
	failStyle := lipgloss.NewStyle().Foreground(render.ColorRed)
	reasonStyle := lipgloss.NewStyle().Foreground(render.ColorGray)
	skipStyle := lipgloss.NewStyle().Foreground(render.ColorYellow)
	successStyle := lipgloss.NewStyle().Foreground(render.ColorGreen)

	var rows [][]string
	var failed, removed, skipped int
	for _, r := range results {
		icon := successStyle.Render(render.IconCheck)
		status := ""
		switch {
		case r.err != nil:
			icon = failStyle.Render(render.IconCross)
			status = failStyle.Render(r.err.Error())
			failed++
		case r.skipReason != "":
			icon = skipStyle.Render(render.IconPending)
			status = skipStyle.Render(r.skipReason)
			skipped++
		default:
			removed++
		}
		rows = append(rows, []string{
			icon,
			r.prunable.name,
			r.prunable.branchName,
			reasonStyle.Render(r.prunable.evidence.String()),
			status,
		})
	}

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(_, _ int) lipgloss.Style {
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Rows(rows...)

	if _, err := lipgloss.Fprintln(w, t); err != nil {
		return err
	}

	summary := successStyle.Render(fmt.Sprintf("%d removed", removed))
	if skipped > 0 {
		summary += ", " + skipStyle.Render(fmt.Sprintf("%d skipped", skipped))
	}
	if failed > 0 {
		summary += ", " + failStyle.Render(fmt.Sprintf("%d failed", failed))
	}
	if _, err := lipgloss.Fprintln(w, summary); err != nil {
		return err
	}

	if failed > 0 {
		noun := "worktree"
		if failed != 1 {
			noun = "worktrees"
		}
		return fmt.Errorf("prune failed for %d selected %s", failed, noun)
	}
	return nil
}

func removeOrphanedWorktree(gitClient git.Git, path, branchName string) error {
	// Targeted stale-worktree removal requires force; revalidation narrows but cannot eliminate path-recreation races.
	if err := gitClient.RemoveWorktree(path, true); err != nil {
		return fmt.Errorf("failed to remove orphaned worktree %q: %w", filepath.Base(path), err)
	}
	return deleteBranchIfExists(gitClient, branchName)
}
