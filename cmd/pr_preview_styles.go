package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
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

// --- Review Style ---

const maxHighActivityFiles = 3

func isTestFile(path string) bool {
	base := filepath.Base(path)

	testFileSuffixes := []string{"_test.go", "Test.java", "Tests.java", "IT.java"}
	for _, suffix := range testFileSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}

	testDirSegments := []string{"/test/", "/tests/", "/testdata/"}
	for _, seg := range testDirSegments {
		if strings.Contains("/"+path, seg) {
			return true
		}
	}
	return false
}

func styleIcon(icon string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render(icon)
}

func timelineEventIcon(eventType, details string) string {
	switch eventType {
	case "reviewed":
		switch details {
		case "approved":
			return styleIcon("✓", colorGreen)
		case "changes requested":
			return styleIcon("✗", colorRed)
		default:
			return styleIcon("●", colorGray)
		}
	case "commented":
		return styleIcon("💬", colorCyan)
	case "committed":
		return styleIcon("⊙", colorPurple)
	case "force_pushed":
		return styleIcon("⚡", colorYellow)
	case "labeled":
		return styleIcon("🏷", colorCyan)
	case "merged":
		return styleIcon("●", colorPurple)
	case "closed":
		return styleIcon("●", colorRed)
	case "reopened":
		return styleIcon("●", colorGreen)
	case "ready_for_review":
		return styleIcon("▶", colorGreen)
	case "convert_to_draft":
		return styleIcon("◑", colorYellow)
	case "review_requested":
		return styleIcon("👁", colorCyan)
	default:
		return styleIcon("·", colorGray)
	}
}

func timelineEventMessage(e github.TimelineEvent) string {
	switch e.Type {
	case "reviewed":
		return fmt.Sprintf("@%s %s", e.Actor, e.Details)
	case "commented":
		return fmt.Sprintf("@%s commented", e.Actor)
	case "committed":
		if e.Details != "" {
			return fmt.Sprintf("@%s committed: %s", e.Actor, e.Details)
		}
		return fmt.Sprintf("@%s committed", e.Actor)
	case "force_pushed":
		return fmt.Sprintf("@%s force pushed", e.Actor)
	case "labeled":
		return fmt.Sprintf("@%s added %s", e.Actor, e.Details)
	case "merged":
		return fmt.Sprintf("@%s merged", e.Actor)
	case "closed":
		return fmt.Sprintf("@%s closed", e.Actor)
	case "reopened":
		return fmt.Sprintf("@%s reopened", e.Actor)
	case "ready_for_review":
		return fmt.Sprintf("@%s marked ready for review", e.Actor)
	case "convert_to_draft":
		return fmt.Sprintf("@%s converted to draft", e.Actor)
	case "review_requested":
		return fmt.Sprintf("@%s requested review from @%s", e.Actor, e.Details)
	default:
		return fmt.Sprintf("@%s %s", e.Actor, e.Type)
	}
}

func formatReviewFileEntry(f github.PullRequestFile, comments int, contentWidth int) string {
	added := lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("+%d", f.Additions))
	deleted := lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("-%d", f.Deletions))

	var right string
	if comments > 0 {
		right = fmt.Sprintf("💬 %d  %s  %s", comments, added, deleted)
	} else {
		right = fmt.Sprintf("%s  %s", added, deleted)
	}

	name := filepath.Base(f.Path)
	dir := filepath.Dir(f.Path)
	var left string
	if dir == "." {
		left = name
	} else {
		left = name + " " + lipgloss.NewStyle().Foreground(colorGray).Render(dir)
	}

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	// 2 for leading indent, 2 for minimum gap
	gap := contentWidth - 2 - leftWidth - rightWidth
	if gap < 2 {
		gap = 2
	}

	return fmt.Sprintf("  %s%s%s", left, strings.Repeat(" ", gap), right)
}

func renderReviewHeader(pr github.PullRequest, width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Width(10)
	valueStyle := lipgloss.NewStyle().Foreground(colorGray)

	stateStr := lipgloss.NewStyle().Foreground(stateColor(pr.State)).Bold(true).Render(strings.ToLower(string(pr.State)))
	titleLine := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title)) + "  " + stateStr

	var sb strings.Builder
	sb.WriteString(titleLine)
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Author"))
	sb.WriteString(valueStyle.Render("@" + pr.AuthorLogin))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Branch"))
	sb.WriteString(valueStyle.Render(branchInfo(pr.BranchName, pr.BaseRefName)))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Changed"))
	sb.WriteString(valueStyle.Render(formatStats(pr)))
	sb.WriteString("\n")

	if labels := renderLabels(pr.Labels); labels != "" {
		sb.WriteString(labelStyle.Render("Labels"))
		sb.WriteString(valueStyle.Render(labels))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderReviewChecks(pr github.PullRequest) string {
	var parts []string
	if ciLines := renderChecksLines(pr.StatusChecks); ciLines != "" {
		ciHeader := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Checks")
		parts = append(parts, ciHeader+"\n"+ciLines)
	}
	if reviewLines := renderReviewLines(pr.Reviews); reviewLines != "" {
		reviewHeader := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Reviews")
		parts = append(parts, reviewHeader+"\n"+reviewLines)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

type scoredFile struct {
	comments int
	file     github.PullRequestFile
	score    int
}

func scoreFiles(files []github.PullRequestFile, fileComments map[string]int) []scoredFile {
	var scored []scoredFile
	for _, f := range files {
		if isTestFile(f.Path) {
			continue
		}
		comments := fileComments[f.Path]
		score := f.Additions + f.Deletions + (comments * 10)
		if score > 0 {
			scored = append(scored, scoredFile{comments: comments, file: f, score: score})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	return scored
}

func renderReviewHighActivity(scored []scoredFile, contentWidth int) string {
	count := min(len(scored), maxHighActivityFiles)
	if count == 0 {
		return ""
	}

	header := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("High Activity")
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	for _, sf := range scored[:count] {
		sb.WriteString(formatReviewFileEntry(sf.file, sf.comments, contentWidth))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderReviewFileList(files []github.PullRequestFile, fileComments map[string]int, totalChanged int, haPaths map[string]bool, contentWidth int) string {
	header := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(fmt.Sprintf("Files (%d)", totalChanged))
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")

	displayed := 0
	for _, f := range files {
		if haPaths[f.Path] {
			continue
		}
		if displayed >= maxPreviewFiles {
			break
		}
		comments := fileComments[f.Path]
		sb.WriteString(formatReviewFileEntry(f, comments, contentWidth))
		sb.WriteString("\n")
		displayed++
	}

	remaining := totalChanged - displayed - len(haPaths)
	if remaining > 0 {
		sb.WriteString(fmt.Sprintf("  … and %d more\n", remaining))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderReviewBody(body string, width int) string {
	if body == "" {
		return ""
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(max(width-4, 20)),
	)
	if err != nil {
		return wrapBody(body, width)
	}
	rendered, err := renderer.Render(body)
	if err != nil {
		return wrapBody(body, width)
	}
	return strings.TrimSpace(rendered)
}

type collapsedEvent struct {
	count int
	event github.TimelineEvent
}

func collapseTimeline(events []github.TimelineEvent) []collapsedEvent {
	var collapsed []collapsedEvent
	for _, e := range events {
		n := len(collapsed)
		if n > 0 && collapsed[n-1].event.Type == "committed" && e.Type == "committed" && collapsed[n-1].event.Actor == e.Actor {
			collapsed[n-1].count++
			collapsed[n-1].event.CreatedAt = e.CreatedAt
			collapsed[n-1].event.Details = e.Details
		} else {
			collapsed = append(collapsed, collapsedEvent{count: 1, event: e})
		}
	}
	return collapsed
}

func renderReviewTimeline(pr github.PullRequest, timeline []github.TimelineEvent, width int) string {
	var events []github.TimelineEvent

	if !pr.CreatedAt.IsZero() {
		events = append(events, github.TimelineEvent{
			Actor:     pr.AuthorLogin,
			CreatedAt: pr.CreatedAt,
			Type:      "opened",
		})
	}
	events = append(events, timeline...)

	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})

	collapsed := collapseTimeline(events)
	if len(collapsed) == 0 {
		return ""
	}

	header := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Activity")
	timeStyle := lipgloss.NewStyle().Foreground(colorGray).Width(16).Align(lipgloss.Right)
	lineStyle := lipgloss.NewStyle().Foreground(colorPurple)

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	for i, ce := range collapsed {
		e := ce.event
		var icon, msg string
		if e.Type == "opened" {
			icon = styleIcon("●", colorGreen)
			msg = fmt.Sprintf("@%s opened this PR", e.Actor)
		} else if e.Type == "committed" && ce.count > 1 {
			icon = styleIcon("⊙", colorPurple)
			msg = fmt.Sprintf("@%s pushed %d commits", e.Actor, ce.count)
		} else {
			icon = timelineEventIcon(e.Type, e.Details)
			msg = timelineEventMessage(e)
		}
		sb.WriteString(fmt.Sprintf("  %s  %s %s\n", timeStyle.Render(relativeTime(e.CreatedAt)), icon, msg))
		if i < len(collapsed)-1 {
			sb.WriteString(fmt.Sprintf("  %s  %s\n", strings.Repeat(" ", 16), lineStyle.Render("│")))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderReview(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, fileComments map[string]int, timeline []github.TimelineEvent, width int) error {
	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorPurple).
		Width(width-2).
		Padding(0, 1)

	var sections []string

	sections = append(sections, boxStyle.Render(renderReviewHeader(pr, width)))

	if checksContent := renderReviewChecks(pr); checksContent != "" {
		sections = append(sections, boxStyle.Render(checksContent))
	}

	scored := scoreFiles(files, fileComments)
	contentWidth := width - 4

	if highActivity := renderReviewHighActivity(scored, contentWidth); highActivity != "" {
		sections = append(sections, boxStyle.Render(highActivity))
	}

	haPaths := make(map[string]bool)
	for i, sf := range scored {
		if i >= maxHighActivityFiles {
			break
		}
		haPaths[sf.file.Path] = true
	}

	sections = append(sections, boxStyle.Render(renderReviewFileList(files, fileComments, pr.FilesChanged, haPaths, contentWidth)))

	if body := renderReviewBody(pr.Body, width); body != "" {
		sections = append(sections, boxStyle.Render(body))
	}

	if timelineContent := renderReviewTimeline(pr, timeline, width); timelineContent != "" {
		sections = append(sections, boxStyle.Render(timelineContent))
	}

	_, err := fmt.Fprintln(w, lipgloss.JoinVertical(lipgloss.Left, sections...))
	return err
}
