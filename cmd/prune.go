package cmd

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
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
)

func newPruneCmd() *cobra.Command {
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
		RunE:    runPrune,
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

func runPrune(cmd *cobra.Command, _ []string) error {
	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &statusContext{
		ghClient:         rt.newUncachedGitHubClient(),
		gitClient:        git.New(cmd.Context(), false, rt.mainWorktreePath, rt.cfg.Git.Timeout, rt.logger),
		mainWorktreePath: rt.mainWorktreePath,
	}

	return executePrune(cmd.OutOrStdout(), ctx)
}

func executePrune(w io.Writer, ctx *statusContext) error {
	statuses, err := gatherPruneStatuses(ctx)
	if err != nil {
		return err
	}
	if err := renderLockedWorktrees(w, statuses); err != nil {
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

	return executeRemovals(w, ctx.gitClient, selected, func(p prunable) (string, bool, error) {
		return revalidatePrunable(ctx, p)
	})
}

func gatherPruneStatuses(ctx *statusContext) ([]worktreeStatus, error) {
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	branches, err := ctx.gitClient.ListLocalBranches()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branchMap := make(map[string]git.LocalBranch, len(branches))
	for _, branch := range branches {
		branchMap[branch.Name] = branch
	}

	statuses := make([]worktreeStatus, 0, len(worktrees))
	for _, worktree := range worktrees {
		status, err := buildPruneWorktreeStatus(ctx, worktree, branchMap)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func buildPruneWorktreeStatus(ctx *statusContext, worktree git.Worktree, branchMap map[string]git.LocalBranch) (worktreeStatus, error) {
	status := newWorktreeStatus(ctx, worktree, branchMap)
	if status.isMain || status.locked {
		return status, nil
	}

	if _, err := os.Stat(status.absPath); err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return worktreeStatus{}, fmt.Errorf("failed to inspect worktree %q: %w", filepath.Base(status.absPath), err)
	}
	if status.branchName == "" {
		return status, nil
	}

	dirty, err := ctx.gitClient.IsWorktreeDirty(status.absPath)
	if err != nil {
		return worktreeStatus{}, fmt.Errorf("failed to inspect dirty state for worktree %q: %w", filepath.Base(status.absPath), err)
	}
	status.dirty = dirty

	pr, err := ctx.ghClient.GetPullRequestByBranch(status.branchName)
	if err != nil {
		return worktreeStatus{}, fmt.Errorf("failed to get PR state for branch %q: %w", status.branchName, err)
	}
	status.pr = pr
	if pr != nil {
		status.kind = "PR"
	} else {
		status.kind = "local"
	}
	return status, nil
}

func renderLockedWorktrees(w io.Writer, statuses []worktreeStatus) error {
	var locked []worktreeStatus
	for _, status := range statuses {
		if status.locked && !status.isMain {
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
		branch := status.branchName
		if branch == "" {
			branch = "detached"
		}
		if _, err := fmt.Fprintf(w, "  %s  %s  (locked)\n", filepath.Base(status.absPath), branch); err != nil {
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

func findPrunable(statuses []worktreeStatus, remoteBranches map[string]bool) []prunable {
	var result []prunable
	for _, ws := range statuses {
		if ws.isMain || ws.locked {
			continue
		}

		if evidence, ok := classifyPrunable(ws, remoteBranches); ok {
			result = append(result, prunable{
				branchName: ws.branchName,
				dirty:      ws.dirty,
				evidence:   evidence,
				headSHA:    ws.headSHA,
				name:       filepath.Base(ws.absPath),
				path:       ws.absPath,
			})
		}
	}
	return result
}

func classifyPrunable(ws worktreeStatus, remoteBranches map[string]bool) (pruneEvidence, bool) {
	if _, err := os.Stat(ws.absPath); os.IsNotExist(err) {
		return pruneEvidence{kind: pruneEvidenceOrphaned}, true
	}

	if ws.pr != nil {
		switch ws.pr.State {
		case github.PRStateMerged, github.PRStateClosed:
			return pruneEvidence{
				kind:     pruneEvidencePR,
				prNumber: ws.pr.Number,
				prState:  ws.pr.State,
			}, true
		}
	}

	if ws.tracking.upstream != "" && !remoteBranches[ws.tracking.upstream] {
		return pruneEvidence{
			kind:     pruneEvidenceUpstreamGone,
			upstream: ws.tracking.upstream,
		}, true
	}

	return pruneEvidence{}, false
}

type pruneState struct {
	description string
	evidence    pruneEvidence
	prunable    bool
}

func revalidatePrunable(ctx *statusContext, candidate prunable) (string, bool, error) {
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
	if !state.prunable || state.evidence != candidate.evidence {
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

func revalidatePrunableIdentity(ctx *statusContext, candidate prunable) (string, error) {
	worktrees, err := ctx.gitClient.ListWorktrees()
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
	if current.AbsolutePath == ctx.mainWorktreePath {
		return "path is now the main worktree", nil
	}
	if current.Locked {
		return "worktree is locked", nil
	}

	branchName := extractBranchName(current)
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
			shortSHASafe(candidate.headSHA, 7),
			shortSHASafe(currentHead, 7),
		), nil
	}
	if candidate.evidence.kind == pruneEvidenceOrphaned {
		return "", nil
	}

	dirty, err := ctx.gitClient.IsWorktreeDirty(candidate.path)
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

func revalidateLocalBarrier(ctx *statusContext, candidate prunable) (string, error) {
	if candidate.evidence.kind == pruneEvidenceUpstreamGone {
		branches, err := ctx.gitClient.ListLocalBranches()
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

		remoteBranches, err := buildRemoteBranchSet(ctx.gitClient)
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

func currentPruneState(ctx *statusContext, candidate prunable) (pruneState, error) {
	if _, err := os.Stat(candidate.path); err != nil {
		if os.IsNotExist(err) {
			evidence := pruneEvidence{kind: pruneEvidenceOrphaned}
			return pruneState{description: evidence.String(), evidence: evidence, prunable: true}, nil
		}
		return pruneState{}, fmt.Errorf("failed to inspect worktree %q: %w", candidate.name, err)
	}
	if candidate.branchName == "" {
		return pruneState{description: "not prunable"}, nil
	}

	branches, err := ctx.gitClient.ListLocalBranches()
	if err != nil {
		return pruneState{}, fmt.Errorf("failed to list branches: %w", err)
	}
	branch, ok := findLocalBranch(branches, candidate.branchName)
	if !ok {
		return pruneState{description: "local branch no longer exists"}, nil
	}

	ws := worktreeStatus{
		absPath:    candidate.path,
		branchName: candidate.branchName,
		tracking:   trackingInfo{upstream: branch.UpstreamName},
	}
	pr, err := ctx.ghClient.GetPullRequestByBranch(candidate.branchName)
	if err != nil {
		return pruneState{}, fmt.Errorf("failed to refresh PR state for branch %q: %w", candidate.branchName, err)
	}
	ws.pr = pr

	if evidence, ok := classifyPrunable(ws, nil); ok && evidence.kind == pruneEvidencePR {
		return pruneState{description: evidence.String(), evidence: evidence, prunable: true}, nil
	}

	remoteBranches, err := buildRemoteBranchSet(ctx.gitClient)
	if err != nil {
		return pruneState{}, err
	}
	evidence, ok := classifyPrunable(ws, remoteBranches)
	if ok {
		return pruneState{description: evidence.String(), evidence: evidence, prunable: true}, nil
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

func describeCurrentPruneState(ws worktreeStatus, remoteBranches map[string]bool) string {
	if ws.pr != nil {
		return fmt.Sprintf("PR #%d %s", ws.pr.Number, strings.ToLower(ws.pr.State.String()))
	}
	if ws.tracking.upstream != "" && remoteBranches[ws.tracking.upstream] {
		return fmt.Sprintf("upstream %q exists", ws.tracking.upstream)
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
			lipgloss.NewStyle().Foreground(colorGray).Render("("+p.evidence.String()+")"),
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
	failStyle := lipgloss.NewStyle().Foreground(colorRed)
	reasonStyle := lipgloss.NewStyle().Foreground(colorGray)
	skipStyle := lipgloss.NewStyle().Foreground(colorYellow)
	successStyle := lipgloss.NewStyle().Foreground(colorGreen)

	var rows [][]string
	var failed, removed, skipped int
	for _, r := range results {
		icon := successStyle.Render(iconCheck)
		status := ""
		switch {
		case r.err != nil:
			icon = failStyle.Render(iconCross)
			status = failStyle.Render(r.err.Error())
			failed++
		case r.skipReason != "":
			icon = skipStyle.Render(iconPending)
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
	if branchName != "" {
		if err := gitClient.DeleteBranch(branchName, true); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
			}
		}
	}
	return nil
}
