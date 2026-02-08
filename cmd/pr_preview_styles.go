package cmd

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jmcampanini/grove-cli/internal/github"
)

var (
	colorCyan   = lipgloss.Color("44")
	colorGray   = lipgloss.Color("245")
	colorGreen  = lipgloss.Color("76")
	colorPurple = lipgloss.Color("99")
	colorRed    = lipgloss.Color("196")
	colorYellow = lipgloss.Color("214")
)

const maxPreviewFiles = 30

func stateColor(state github.PRState) lipgloss.Color {
	switch state {
	case github.PRStateOpen:
		return colorGreen
	case github.PRStateDraft:
		return colorYellow
	case github.PRStateMerged:
		return colorPurple
	case github.PRStateClosed:
		return colorRed
	default:
		return colorGray
	}
}

func formatFileEntry(f github.PullRequestFile) string {
	added := lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", f.Additions))
	deleted := lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", f.Deletions))
	return fmt.Sprintf("  %s %s %s", f.Path, added, deleted)
}

func renderFileList(files []github.PullRequestFile, totalChanged int) string {
	var sb strings.Builder
	displayCount := min(len(files), maxPreviewFiles)
	for _, f := range files[:displayCount] {
		sb.WriteString(formatFileEntry(f))
		sb.WriteString("\n")
	}
	if remaining := totalChanged - displayCount; remaining > 0 {
		sb.WriteString(fmt.Sprintf("  … and %d more\n", remaining))
	}
	return sb.String()
}

func wrapBody(body string, width int) string {
	if body == "" {
		return ""
	}
	bodyWidth := max(width-4, 20)
	var lines []string
	for _, paragraph := range strings.Split(body, "\n") {
		if len(paragraph) <= bodyWidth {
			lines = append(lines, paragraph)
			continue
		}
		words := strings.Fields(paragraph)
		var line string
		for _, word := range words {
			if line == "" {
				line = word
			} else if len(line)+1+len(word) <= bodyWidth {
				line += " " + word
			} else {
				lines = append(lines, line)
				line = word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// --- A1: Bordered Card ---

func renderCard(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorPurple).
		Width(width-2).
		Padding(0, 1)

	stateStr := lipgloss.NewStyle().
		Foreground(stateColor(pr.State)).
		Bold(true).
		Render(strings.ToLower(string(pr.State)))

	header := fmt.Sprintf("%s  %s\n%s  @%s",
		lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title)),
		stateStr,
		lipgloss.NewStyle().Foreground(colorGray).Render(pr.BranchName),
		pr.AuthorLogin,
	)

	stats := lipgloss.NewStyle().Foreground(colorGray).Render(
		fmt.Sprintf("%d files  %s  %s",
			pr.FilesChanged,
			lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", pr.LinesAdded)),
			lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", pr.LinesDeleted)),
		),
	)

	fileSection := stats + "\n" + renderFileList(files, pr.FilesChanged)

	var sections []string
	sections = append(sections, boxStyle.Render(header))
	sections = append(sections, boxStyle.Render(fileSection))
	if pr.Body != "" {
		sections = append(sections, boxStyle.Render(wrapBody(pr.Body, width-4)))
	}

	_, err := fmt.Fprintln(w, lipgloss.JoinVertical(lipgloss.Left, sections...))
	return err
}

// --- A2: Compact Dashboard ---

func renderDashboard(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	labelStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Width(10)
	valueStyle := lipgloss.NewStyle().Foreground(colorGray)
	divider := lipgloss.NewStyle().Foreground(colorPurple).Render(strings.Repeat("─", width))

	stateStr := lipgloss.NewStyle().Foreground(stateColor(pr.State)).Bold(true).Render(strings.ToLower(string(pr.State)))

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("PR #%d", pr.Number)))
	sb.WriteString("\n")
	sb.WriteString(divider)
	sb.WriteString("\n")

	rows := []struct{ label, value string }{
		{"Title", pr.Title},
		{"Author", pr.AuthorLogin},
		{"Branch", pr.BranchName},
		{"State", stateStr},
		{"Changed", fmt.Sprintf("%d files  %s  %s",
			pr.FilesChanged,
			lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", pr.LinesAdded)),
			lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", pr.LinesDeleted)),
		)},
	}

	for _, r := range rows {
		sb.WriteString(labelStyle.Render(r.label))
		sb.WriteString(valueStyle.Render(r.value))
		sb.WriteString("\n")
	}

	sb.WriteString(divider)
	sb.WriteString("\n")
	sb.WriteString(renderFileList(files, pr.FilesChanged))

	if pr.Body != "" {
		sb.WriteString(divider)
		sb.WriteString("\n")
		sb.WriteString(wrapBody(pr.Body, width))
		sb.WriteString("\n")
	}

	_, err := fmt.Fprint(w, sb.String())
	return err
}

// --- A3: Minimal GitHub ---

func renderMinimal(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	stateStr := lipgloss.NewStyle().
		Foreground(stateColor(pr.State)).
		Render(strings.ToLower(string(pr.State)))

	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  %s\n", title, stateStr))
	sb.WriteString(lipgloss.NewStyle().Foreground(colorGray).Render(
		fmt.Sprintf("@%s → %s", pr.AuthorLogin, pr.BranchName)))
	sb.WriteString("\n\n")

	statsLine := fmt.Sprintf("%d files changed  %s  %s",
		pr.FilesChanged,
		lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", pr.LinesAdded)),
		lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", pr.LinesDeleted)),
	)
	sb.WriteString(statsLine)
	sb.WriteString("\n")

	fileBlock := renderFileList(files, pr.FilesChanged)
	if fileBlock != "" {
		fileBorder := lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorPurple).
			PaddingLeft(1)
		sb.WriteString(fileBorder.Render(strings.TrimRight(fileBlock, "\n")))
		sb.WriteString("\n")
	}

	if pr.Body != "" {
		sb.WriteString("\n")
		sb.WriteString(wrapBody(pr.Body, width))
		sb.WriteString("\n")
	}

	_, err := fmt.Fprint(w, sb.String())
	return err
}

// --- Shared helpers for Group B ---

func checkIcon(conclusion string) string {
	switch conclusion {
	case "success":
		return lipgloss.NewStyle().Foreground(colorGreen).Render("✓")
	case "failure", "timed_out", "action_required":
		return lipgloss.NewStyle().Foreground(colorRed).Render("✗")
	case "cancelled", "skipped":
		return lipgloss.NewStyle().Foreground(colorGray).Render("–")
	default:
		return lipgloss.NewStyle().Foreground(colorYellow).Render("◯")
	}
}

func reviewIcon(state string) string {
	switch state {
	case "APPROVED":
		return lipgloss.NewStyle().Foreground(colorGreen).Render("✓")
	case "CHANGES_REQUESTED":
		return lipgloss.NewStyle().Foreground(colorRed).Render("✗")
	default:
		return lipgloss.NewStyle().Foreground(colorGray).Render("●")
	}
}

func labelColor(hexColor string) lipgloss.Color {
	if hexColor == "" {
		return colorGray
	}
	hex := strings.TrimPrefix(hexColor, "#")
	if len(hex) != 6 {
		return colorGray
	}
	r, err := strconv.ParseInt(hex[0:2], 16, 64)
	if err != nil {
		return colorGray
	}
	g, err := strconv.ParseInt(hex[2:4], 16, 64)
	if err != nil {
		return colorGray
	}
	b, err := strconv.ParseInt(hex[4:6], 16, 64)
	if err != nil {
		return colorGray
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

func checkStatusText(sc github.StatusCheck) string {
	if sc.Status == "PENDING" || sc.Status == "IN_PROGRESS" || sc.Status == "QUEUED" {
		return strings.ToLower(sc.Status)
	}
	if sc.Conclusion == "" {
		return strings.ToLower(sc.Status)
	}
	return sc.Conclusion
}

func branchInfo(branchName, baseRefName string) string {
	if baseRefName == "" {
		return branchName
	}
	return fmt.Sprintf("%s → %s", branchName, baseRefName)
}

func renderLabels(labels []github.Label) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for _, l := range labels {
		parts = append(parts, lipgloss.NewStyle().Foreground(labelColor(l.Color)).Render(l.Name))
	}
	return strings.Join(parts, lipgloss.NewStyle().Foreground(colorGray).Render(" · "))
}

func formatStats(pr github.PullRequest) string {
	return fmt.Sprintf("%d files  %s  %s",
		pr.FilesChanged,
		lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", pr.LinesAdded)),
		lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", pr.LinesDeleted)),
	)
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// --- B1: Full Context Card ---

func renderContext(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorPurple).
		Width(width-2).
		Padding(0, 1)

	stateStr := lipgloss.NewStyle().
		Foreground(stateColor(pr.State)).
		Bold(true).
		Render(strings.ToLower(string(pr.State)))

	header := fmt.Sprintf("%s  %s\n%s  @%s",
		lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title)),
		stateStr,
		lipgloss.NewStyle().Foreground(colorGray).Render(branchInfo(pr.BranchName, pr.BaseRefName)),
		pr.AuthorLogin,
	)

	if labels := renderLabels(pr.Labels); labels != "" {
		header += "\n" + labels
	}

	stats := formatStats(pr)
	fileSection := stats + "\n" + renderFileList(files, pr.FilesChanged)

	var sections []string
	sections = append(sections, boxStyle.Render(header))
	sections = append(sections, boxStyle.Render(fileSection))

	if len(pr.StatusChecks) > 0 || len(pr.Reviews) > 0 {
		var statusParts []string
		if ciLines := renderChecksLines(pr.StatusChecks); ciLines != "" {
			ciHeader := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Checks")
			statusParts = append(statusParts, ciHeader+"\n"+ciLines)
		}
		if reviewLines := renderReviewLines(pr.Reviews); reviewLines != "" {
			reviewHeader := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Reviews")
			statusParts = append(statusParts, reviewHeader+"\n"+reviewLines)
		}

		sections = append(sections, boxStyle.Render(strings.Join(statusParts, "\n\n")))
	}

	if pr.Body != "" {
		sections = append(sections, boxStyle.Render(wrapBody(pr.Body, width-4)))
	}

	_, err := fmt.Fprintln(w, lipgloss.JoinVertical(lipgloss.Left, sections...))
	return err
}

// --- B2: Status Board ---

func renderBoard(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	labelStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Width(10)
	valueStyle := lipgloss.NewStyle().Foreground(colorGray)
	divider := lipgloss.NewStyle().Foreground(colorPurple).Render(strings.Repeat("─", width))

	stateStr := lipgloss.NewStyle().Foreground(stateColor(pr.State)).Bold(true).Render(strings.ToLower(string(pr.State)))

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("PR #%d", pr.Number)))
	sb.WriteString("\n")
	sb.WriteString(divider)
	sb.WriteString("\n")

	rows := []struct{ label, value string }{
		{"Title", pr.Title},
		{"Author", pr.AuthorLogin},
		{"Branch", branchInfo(pr.BranchName, pr.BaseRefName)},
		{"State", stateStr},
		{"Changed", formatStats(pr)},
	}

	if labels := renderLabels(pr.Labels); labels != "" {
		rows = append(rows, struct{ label, value string }{"Labels", labels})
	}

	for _, r := range rows {
		sb.WriteString(labelStyle.Render(r.label))
		sb.WriteString(valueStyle.Render(r.value))
		sb.WriteString("\n")
	}

	if ciLines := renderChecksLines(pr.StatusChecks); ciLines != "" {
		sb.WriteString(divider)
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Checks"))
		sb.WriteString("\n")
		sb.WriteString(ciLines)
		sb.WriteString("\n")
	}

	if reviewLines := renderReviewLines(pr.Reviews); reviewLines != "" {
		sb.WriteString(divider)
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Reviews"))
		sb.WriteString("\n")
		sb.WriteString(reviewLines)
		sb.WriteString("\n")
	}

	sb.WriteString(divider)
	sb.WriteString("\n")
	sb.WriteString(renderFileList(files, pr.FilesChanged))

	if pr.Body != "" {
		sb.WriteString(divider)
		sb.WriteString("\n")
		sb.WriteString(wrapBody(pr.Body, width))
		sb.WriteString("\n")
	}

	_, err := fmt.Fprint(w, sb.String())
	return err
}

// --- B3: Activity Timeline ---

type timelineEvent struct {
	icon    string
	message string
	time    time.Time
}

func reviewActionText(state string) string {
	switch state {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "requested changes"
	case "COMMENTED":
		return "commented"
	case "DISMISSED":
		return "review dismissed"
	default:
		return strings.ToLower(state)
	}
}

func renderChecksLines(checks []github.StatusCheck) string {
	if len(checks) == 0 {
		return ""
	}
	var lines []string
	for _, sc := range checks {
		lines = append(lines, fmt.Sprintf("  %s %s %s",
			checkIcon(sc.Conclusion),
			sc.Name,
			lipgloss.NewStyle().Foreground(colorGray).Render(checkStatusText(sc)),
		))
	}
	return strings.Join(lines, "\n")
}

func renderReviewLines(reviews []github.Review) string {
	if len(reviews) == 0 {
		return ""
	}
	var lines []string
	for _, r := range reviews {
		lines = append(lines, fmt.Sprintf("  %s @%s %s",
			reviewIcon(r.State),
			r.AuthorLogin,
			lipgloss.NewStyle().Foreground(colorGray).Render(reviewActionText(r.State)),
		))
	}
	return strings.Join(lines, "\n")
}

func buildTimelineEvents(pr github.PullRequest) []timelineEvent {
	var events []timelineEvent

	if !pr.CreatedAt.IsZero() {
		events = append(events, timelineEvent{
			icon:    lipgloss.NewStyle().Foreground(colorGreen).Render("●"),
			message: fmt.Sprintf("@%s opened this PR", pr.AuthorLogin),
			time:    pr.CreatedAt,
		})
	}

	for _, r := range pr.Reviews {
		events = append(events, timelineEvent{
			icon:    reviewIcon(r.State),
			message: fmt.Sprintf("@%s %s", r.AuthorLogin, reviewActionText(r.State)),
			time:    r.SubmittedAt,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].time.Before(events[j].time)
	})

	return events
}

func renderTimeline(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	stateStr := lipgloss.NewStyle().
		Foreground(stateColor(pr.State)).
		Render(strings.ToLower(string(pr.State)))

	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  %s\n", title, stateStr))
	sb.WriteString(lipgloss.NewStyle().Foreground(colorGray).Render(
		fmt.Sprintf("@%s  %s", pr.AuthorLogin, branchInfo(pr.BranchName, pr.BaseRefName))))
	sb.WriteString("\n")

	if labels := renderLabels(pr.Labels); labels != "" {
		sb.WriteString(labels)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	events := buildTimelineEvents(pr)
	timeStyle := lipgloss.NewStyle().Foreground(colorGray).Width(16).Align(lipgloss.Right)
	lineStyle := lipgloss.NewStyle().Foreground(colorPurple)

	for i, e := range events {
		sb.WriteString(fmt.Sprintf("%s  %s %s\n", timeStyle.Render(relativeTime(e.time)), e.icon, e.message))
		if i < len(events)-1 {
			sb.WriteString(fmt.Sprintf("%s  %s\n", strings.Repeat(" ", 16), lineStyle.Render("│")))
		}
	}

	if ciLines := renderChecksLines(pr.StatusChecks); ciLines != "" {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Checks"))
		sb.WriteString("\n")
		sb.WriteString(ciLines)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(formatStats(pr))
	sb.WriteString("\n")

	fileBlock := renderFileList(files, pr.FilesChanged)
	if fileBlock != "" {
		fileBorder := lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorPurple).
			PaddingLeft(1)
		sb.WriteString(fileBorder.Render(strings.TrimRight(fileBlock, "\n")))
		sb.WriteString("\n")
	}

	if pr.Body != "" {
		sb.WriteString("\n")
		sb.WriteString(wrapBody(pr.Body, width))
		sb.WriteString("\n")
	}

	_, err := fmt.Fprint(w, sb.String())
	return err
}
