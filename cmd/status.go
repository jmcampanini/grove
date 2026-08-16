package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show worktree status dashboard",
		Long: `Status shows a rich dashboard of all worktrees with branch tracking,
dirty state, and PR information for worktrees associated with pull requests.`,
		Args:    cobra.NoArgs,
		GroupID: "worktree",
		RunE:    runStatus,
	}
}

type statusContext struct {
	ghClient         github.GitHub
	gitClient        git.Git
	mainWorktreePath string
}

type trackingInfo struct {
	ahead    int
	behind   int
	upstream string
}

type worktreeStatus struct {
	absPath    string
	branchName string
	dirty      bool
	headSHA    string
	isMain     bool
	kind       string
	locked     bool
	pr         *github.PullRequest
	tracking   trackingInfo
}

func runStatus(cmd *cobra.Command, _ []string) error {
	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return err
	}

	ghClient, err := rt.newCachedGitHubClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	ctx := &statusContext{
		ghClient:         ghClient,
		gitClient:        rt.gitClient,
		mainWorktreePath: rt.mainWorktreePath,
	}

	statuses, err := gatherStatuses(ctx)
	if err != nil {
		return err
	}

	return renderStatusTable(cmd.OutOrStdout(), statuses)
}

func gatherStatuses(ctx *statusContext) ([]worktreeStatus, error) {
	worktrees, branchMap, err := loadWorktreesAndBranches(ctx.gitClient)
	if err != nil {
		return nil, err
	}

	var statuses []worktreeStatus
	for _, wt := range worktrees {
		ws := buildWorktreeStatus(ctx, wt, branchMap)
		statuses = append(statuses, ws)
	}

	return statuses, nil
}

func loadWorktreesAndBranches(gitClient git.Git) ([]git.Worktree, map[string]git.LocalBranch, error) {
	worktrees, err := gitClient.ListWorktrees()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	branches, err := gitClient.ListLocalBranches()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branchMap := make(map[string]git.LocalBranch, len(branches))
	for _, branch := range branches {
		branchMap[branch.Name] = branch
	}
	return worktrees, branchMap, nil
}

func buildWorktreeStatus(ctx *statusContext, wt git.Worktree, branchMap map[string]git.LocalBranch) worktreeStatus {
	ws := newWorktreeStatus(ctx, wt, branchMap)
	if ws.branchName == "" {
		return ws
	}

	dirty, err := ctx.gitClient.IsWorktreeDirty(wt.AbsolutePath)
	if err == nil {
		ws.dirty = dirty
	}

	pr, err := ctx.ghClient.GetPullRequestByBranch(ws.branchName)
	if err == nil && pr != nil {
		ws.kind = "PR"
		ws.pr = pr
	} else if !ws.isMain {
		ws.kind = "local"
	}

	return ws
}

func newWorktreeStatus(ctx *statusContext, wt git.Worktree, branchMap map[string]git.LocalBranch) worktreeStatus {
	ws := worktreeStatus{
		absPath: wt.AbsolutePath,
		isMain:  wt.AbsolutePath == ctx.mainWorktreePath,
		locked:  wt.Locked,
	}
	ws.branchName = extractBranchName(&wt)
	ws.headSHA = wt.CommitSHA()
	if b, ok := branchMap[ws.branchName]; ok {
		ws.tracking = trackingInfo{
			ahead:    b.Ahead,
			behind:   b.Behind,
			upstream: b.UpstreamName,
		}
	}
	return ws
}

func renderStatusTable(w io.Writer, statuses []worktreeStatus) error {
	if len(statuses) == 0 {
		_, err := fmt.Fprintln(w, "No worktrees found.")
		return err
	}

	purple := lipgloss.Color("99")
	gray := lipgloss.Color("245")
	lightGray := lipgloss.Color("241")

	headerStyle := lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	oddRowStyle := cellStyle.Foreground(gray)
	evenRowStyle := cellStyle.Foreground(lightGray)

	rows := make([][]string, len(statuses))
	for i, ws := range statuses {
		rows[i] = []string{
			filepath.Base(ws.absPath),
			ws.branchName,
			formatDirtyStatus(ws),
			formatTracking(ws.tracking),
			formatKind(ws),
			formatPRInfo(ws),
		}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row%2 == 0:
				return evenRowStyle
			default:
				return oddRowStyle
			}
		}).
		Headers("Name", "Branch", "Status", "Tracking", "Kind", "PR Info").
		Rows(rows...)

	_, err := lipgloss.Fprintln(w, t)
	return err
}

func formatDirtyStatus(ws worktreeStatus) string {
	if ws.branchName == "" {
		return ""
	}
	if ws.dirty {
		return colorIcon(iconCross, colorRed) + " dirty"
	}
	return colorIcon(iconCheck, colorGreen) + " clean"
}

func formatTracking(ti trackingInfo) string {
	if ti.upstream == "" {
		return ""
	}
	if ti.ahead == 0 && ti.behind == 0 {
		return "\u2261" // ≡
	}
	var parts []string
	if ti.ahead > 0 {
		parts = append(parts, fmt.Sprintf("\u2191%d", ti.ahead))
	}
	if ti.behind > 0 {
		parts = append(parts, fmt.Sprintf("\u2193%d", ti.behind))
	}
	return strings.Join(parts, " ")
}

func formatKind(ws worktreeStatus) string {
	switch ws.kind {
	case "PR":
		return lipgloss.NewStyle().Foreground(colorPurple).Render("PR")
	case "local":
		return lipgloss.NewStyle().Foreground(colorGray).Render("local")
	default:
		return ""
	}
}

func formatPRInfo(ws worktreeStatus) string {
	if ws.pr == nil {
		return ""
	}
	pr := ws.pr

	stateStr := lipgloss.NewStyle().Foreground(stateColor(pr.State)).Render(strings.ToLower(string(pr.State)))
	info := fmt.Sprintf("#%d %s", pr.Number, stateStr)

	if checksStr := formatChecksSummary(pr.StatusChecks); checksStr != "" {
		info += "  " + checksStr
	}

	if reviewStr := formatReviewSummary(pr.Reviews); reviewStr != "" {
		info += "  " + reviewStr
	}

	info += fmt.Sprintf("  %s %s",
		lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", pr.LinesAdded)),
		lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", pr.LinesDeleted)),
	)

	return info
}

func formatChecksSummary(checks []github.StatusCheck) string {
	if len(checks) == 0 {
		return ""
	}
	passed := 0
	for _, c := range checks {
		if c.Conclusion == github.CheckConclusionSuccess {
			passed++
		}
	}
	total := len(checks)
	icon := colorIcon(iconCross, colorRed)
	if passed == total {
		icon = colorIcon(iconCheck, colorGreen)
	}
	return icon + fmt.Sprintf("%d/%d checks", passed, total)
}

func formatReviewSummary(reviews []github.Review) string {
	if len(reviews) == 0 {
		return ""
	}
	deduped := deduplicateReviews(reviews)
	hasApproval := false
	hasChangesRequested := false
	for _, r := range deduped {
		switch r.State {
		case github.ReviewStateApproved:
			hasApproval = true
		case github.ReviewStateChangesRequested:
			hasChangesRequested = true
		}
	}
	if hasChangesRequested {
		return lipgloss.NewStyle().Foreground(colorRed).Render("changes requested")
	}
	if hasApproval {
		return lipgloss.NewStyle().Foreground(colorGreen).Render("approved")
	}
	noun := "reviews"
	if len(deduped) == 1 {
		noun = "review"
	}
	return lipgloss.NewStyle().Foreground(colorGray).Render(fmt.Sprintf("%d %s", len(deduped), noun))
}
