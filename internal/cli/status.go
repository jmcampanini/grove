package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/jmcampanini/grove/internal/worktree"
	"github.com/spf13/cobra"
)

func newStatusCmd(services commandServices) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show worktree status dashboard",
		Long: `Status shows a rich dashboard of all worktrees with branch tracking,
dirty state, and PR information for worktrees associated with pull requests.`,
		Args:    cobra.NoArgs,
		GroupID: "worktree",
		RunE:    services.runStatus,
	}
}

func (services commandServices) runStatus(cmd *cobra.Command, _ []string) error {
	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}

	ghClient, err := rt.NewCachedGitHubClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	ctx := &worktree.StatusContext{
		GitHubClient:     ghClient,
		GitClient:        rt.GitClient,
		MainWorktreePath: rt.MainWorktreePath,
	}

	statuses, err := gatherStatuses(ctx)
	if err != nil {
		return err
	}

	return renderStatusTable(cmd.OutOrStdout(), statuses)
}

func gatherStatuses(ctx *worktree.StatusContext) ([]worktree.Status, error) {
	worktrees, branchMap, err := worktree.LoadWithBranches(ctx.GitClient)
	if err != nil {
		return nil, err
	}

	var statuses []worktree.Status
	for _, wt := range worktrees {
		ws := buildWorktreeStatus(ctx, wt, branchMap)
		statuses = append(statuses, ws)
	}

	return statuses, nil
}

func buildWorktreeStatus(ctx *worktree.StatusContext, wt git.Worktree, branchMap map[string]git.LocalBranch) worktree.Status {
	ws := worktree.NewStatus(ctx.MainWorktreePath, wt, branchMap)
	if ws.BranchName == "" {
		return ws
	}

	dirty, err := ctx.GitClient.IsWorktreeDirty(wt.AbsolutePath)
	if err == nil {
		ws.Dirty = dirty
	}

	pr, err := ctx.GitHubClient.GetPullRequestByBranch(ws.BranchName)
	if err == nil && pr != nil {
		ws.Kind = "PR"
		ws.PR = pr
	} else if !ws.IsMain {
		ws.Kind = "local"
	}

	return ws
}

func renderStatusTable(w io.Writer, statuses []worktree.Status) error {
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
			filepath.Base(ws.AbsPath),
			ws.BranchName,
			formatDirtyStatus(ws),
			formatTracking(ws.Tracking),
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

func formatDirtyStatus(ws worktree.Status) string {
	if ws.BranchName == "" {
		return ""
	}
	if ws.Dirty {
		return render.ColorIcon(render.IconCross, render.ColorRed) + " dirty"
	}
	return render.ColorIcon(render.IconCheck, render.ColorGreen) + " clean"
}

func formatTracking(ti worktree.TrackingInfo) string {
	if ti.Upstream == "" {
		return ""
	}
	if ti.Ahead == 0 && ti.Behind == 0 {
		return "\u2261" // ≡
	}
	var parts []string
	if ti.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("\u2191%d", ti.Ahead))
	}
	if ti.Behind > 0 {
		parts = append(parts, fmt.Sprintf("\u2193%d", ti.Behind))
	}
	return strings.Join(parts, " ")
}

func formatKind(ws worktree.Status) string {
	switch ws.Kind {
	case "PR":
		return lipgloss.NewStyle().Foreground(render.ColorPurple).Render("PR")
	case "local":
		return lipgloss.NewStyle().Foreground(render.ColorGray).Render("local")
	default:
		return ""
	}
}

func formatPRInfo(ws worktree.Status) string {
	if ws.PR == nil {
		return ""
	}
	pr := ws.PR

	stateStr := lipgloss.NewStyle().Foreground(render.StateColor(pr.State)).Render(strings.ToLower(string(pr.State)))
	info := fmt.Sprintf("#%d %s", pr.Number, stateStr)

	if checksStr := formatChecksSummary(pr.StatusChecks); checksStr != "" {
		info += "  " + checksStr
	}

	if reviewStr := formatReviewSummary(pr.Reviews); reviewStr != "" {
		info += "  " + reviewStr
	}

	info += fmt.Sprintf("  %s %s",
		lipgloss.NewStyle().Foreground(render.ColorGreen).Render(fmt.Sprintf("+%d", pr.LinesAdded)),
		lipgloss.NewStyle().Foreground(render.ColorRed).Render(fmt.Sprintf("-%d", pr.LinesDeleted)),
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
	icon := render.ColorIcon(render.IconCross, render.ColorRed)
	if passed == total {
		icon = render.ColorIcon(render.IconCheck, render.ColorGreen)
	}
	return icon + fmt.Sprintf("%d/%d checks", passed, total)
}

func formatReviewSummary(reviews []github.Review) string {
	if len(reviews) == 0 {
		return ""
	}
	deduped := render.DeduplicateReviews(reviews)
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
		return lipgloss.NewStyle().Foreground(render.ColorRed).Render("changes requested")
	}
	if hasApproval {
		return lipgloss.NewStyle().Foreground(render.ColorGreen).Render("approved")
	}
	noun := "reviews"
	if len(deduped) == 1 {
		noun = "review"
	}
	return lipgloss.NewStyle().Foreground(render.ColorGray).Render(fmt.Sprintf("%d %s", len(deduped), noun))
}
