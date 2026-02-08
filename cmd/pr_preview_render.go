package cmd

import (
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/muesli/termenv"
)

var (
	colorCyan   = lipgloss.AdaptiveColor{Light: "30", Dark: "44"}
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

func colorIcon(icon string, color lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Foreground(color).Render(icon)
}

func checkIcon(conclusion string) string {
	switch conclusion {
	case "success":
		return colorIcon("✓", colorGreen)
	case "failure", "timed_out", "action_required":
		return colorIcon("✗", colorRed)
	case "cancelled", "skipped":
		return colorIcon("–", colorGray)
	default:
		return colorIcon("◯", colorYellow)
	}
}

func reviewIcon(state string) string {
	switch state {
	case "APPROVED":
		return colorIcon("✓", colorGreen)
	case "CHANGES_REQUESTED":
		return colorIcon("✗", colorRed)
	default:
		return colorIcon("●", colorGray)
	}
}

func labelColor(hexColor string) lipgloss.Color {
	h := strings.TrimPrefix(hexColor, "#")
	if len(h) != 6 {
		return colorGray
	}
	if _, err := hex.DecodeString(h); err != nil {
		return colorGray
	}
	return lipgloss.Color("#" + h)
}

func checkStatusText(sc github.StatusCheck) string {
	if sc.Conclusion != "" {
		return sc.Conclusion
	}
	return strings.ToLower(sc.Status)
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

func sectionHeader(text string) string {
	return lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(text)
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

func deduplicateReviews(reviews []github.Review) []github.Review {
	latest := make(map[string]github.Review, len(reviews))
	for _, r := range reviews {
		if existing, ok := latest[r.AuthorLogin]; !ok || r.SubmittedAt.After(existing.SubmittedAt) {
			latest[r.AuthorLogin] = r
		}
	}
	deduped := slices.Collect(maps.Values(latest))
	slices.SortFunc(deduped, func(a, b github.Review) int {
		return a.SubmittedAt.Compare(b.SubmittedAt)
	})
	return deduped
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

func renderReviewLines(reviews []github.Review) string {
	if len(reviews) == 0 {
		return ""
	}
	deduped := deduplicateReviews(reviews)
	var lines []string
	for _, r := range deduped {
		lines = append(lines, fmt.Sprintf("  %s @%s %s",
			reviewIcon(r.State),
			r.AuthorLogin,
			lipgloss.NewStyle().Foreground(colorGray).Render(reviewActionText(r.State)),
		))
	}
	return strings.Join(lines, "\n")
}

const maxHighActivityFiles = 3

var testFileSuffixes = []string{"_test.go", "Test.java", "Tests.java", "IT.java"}
var testDirSegments = []string{"/test/", "/tests/", "/testdata/"}

func isTestFile(path string) bool {
	base := filepath.Base(path)
	if slices.ContainsFunc(testFileSuffixes, func(s string) bool { return strings.HasSuffix(base, s) }) {
		return true
	}
	prefixed := "/" + path
	return slices.ContainsFunc(testDirSegments, func(s string) bool { return strings.Contains(prefixed, s) })
}

func timelineEventIcon(eventType, details string) string {
	switch eventType {
	case "reviewed":
		switch details {
		case "approved":
			return colorIcon("✓", colorGreen)
		case "changes requested":
			return colorIcon("✗", colorRed)
		default:
			return colorIcon("●", colorGray)
		}
	case "commented":
		return colorIcon("💬", colorCyan)
	case "committed":
		return colorIcon("⊙", colorPurple)
	case "force_pushed":
		return colorIcon("⚡", colorYellow)
	case "labeled":
		return colorIcon("🏷", colorCyan)
	case "merged":
		return colorIcon("●", colorPurple)
	case "closed":
		return colorIcon("●", colorRed)
	case "reopened":
		return colorIcon("●", colorGreen)
	case "ready_for_review":
		return colorIcon("▶", colorGreen)
	case "convert_to_draft":
		return colorIcon("◑", colorYellow)
	case "review_requested":
		return colorIcon("👁", colorCyan)
	default:
		return colorIcon("·", colorGray)
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

type fileColumnWidths struct {
	addWidth     int
	commentWidth int
	delWidth     int
}

func computeFileColumnWidths(files []github.PullRequestFile, fileComments map[string]int) fileColumnWidths {
	var w fileColumnWidths
	for _, f := range files {
		if aw := len(strconv.Itoa(f.Additions)) + 1; aw > w.addWidth {
			w.addWidth = aw
		}
		if dw := len(strconv.Itoa(f.Deletions)) + 1; dw > w.delWidth {
			w.delWidth = dw
		}
		if c := fileComments[f.Path]; c > 0 {
			if cw := len(strconv.Itoa(c)); cw > w.commentWidth {
				w.commentWidth = cw
			}
		}
	}
	return w
}

func formatFileEntry(f github.PullRequestFile, comments int, contentWidth int, cw fileColumnWidths) string {
	added := lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("%+*d", cw.addWidth, f.Additions))
	delStr := fmt.Sprintf("-%d", f.Deletions)
	deleted := lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("%*s", cw.delWidth, delStr))

	var right string
	if cw.commentWidth > 0 {
		if comments > 0 {
			right = fmt.Sprintf("💬 %*d  %s  %s", cw.commentWidth, comments, added, deleted)
		} else {
			commentColWidth := lipgloss.Width(fmt.Sprintf("💬 %*d", cw.commentWidth, 0))
			right = fmt.Sprintf("%s  %s  %s", strings.Repeat(" ", commentColWidth), added, deleted)
		}
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
	gap := max(contentWidth-2-leftWidth-rightWidth, 2)

	return fmt.Sprintf("  %s%s%s", left, strings.Repeat(" ", gap), right)
}

func renderHeader(pr github.PullRequest) string {
	labelStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Width(10)
	valueStyle := lipgloss.NewStyle().Foreground(colorGray)

	stateStr := lipgloss.NewStyle().
		Background(stateColor(pr.State)).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Padding(0, 1).
		Render(strings.ToUpper(string(pr.State)))
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

func renderChecks(pr github.PullRequest) string {
	var parts []string
	if ciLines := renderChecksLines(pr.StatusChecks); ciLines != "" {
		parts = append(parts, sectionHeader("Checks")+"\n"+ciLines)
	}
	if reviewLines := renderReviewLines(pr.Reviews); reviewLines != "" {
		parts = append(parts, sectionHeader("Reviews")+"\n"+reviewLines)
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
	slices.SortFunc(scored, func(a, b scoredFile) int {
		return b.score - a.score
	})
	return scored
}

func selectHighActivityFiles(scored []scoredFile) []scoredFile {
	var commented, rest []scoredFile
	for _, sf := range scored {
		if sf.comments > 0 {
			commented = append(commented, sf)
		} else {
			rest = append(rest, sf)
		}
	}

	shown := commented
	remaining := maxHighActivityFiles - len(shown)
	if remaining > 0 && len(rest) > 0 {
		fill := min(remaining, len(rest))
		shown = append(shown, rest[:fill]...)
	}
	return shown
}

func renderHighActivity(scored []scoredFile, contentWidth int) (string, []scoredFile) {
	shown := selectHighActivityFiles(scored)
	if len(shown) == 0 {
		return "", nil
	}

	shownComments := make(map[string]int, len(shown))
	var shownFiles []github.PullRequestFile
	for _, sf := range shown {
		shownFiles = append(shownFiles, sf.file)
		if sf.comments > 0 {
			shownComments[sf.file.Path] = sf.comments
		}
	}
	cw := computeFileColumnWidths(shownFiles, shownComments)

	header := sectionHeader(fmt.Sprintf("High Activity Files (%d)", len(shown)))
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	for _, sf := range shown {
		sb.WriteString(formatFileEntry(sf.file, sf.comments, contentWidth, cw))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n"), shown
}

func renderFileList(files []github.PullRequestFile, fileComments map[string]int, totalChanged int, haPaths map[string]bool, contentWidth int) string {
	var displayFiles []github.PullRequestFile
	for _, f := range files {
		if haPaths[f.Path] {
			continue
		}
		if len(displayFiles) >= maxPreviewFiles {
			break
		}
		displayFiles = append(displayFiles, f)
	}
	cw := computeFileColumnWidths(displayFiles, fileComments)

	header := sectionHeader(fmt.Sprintf("Files (%d)", totalChanged))
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")

	for _, f := range displayFiles {
		sb.WriteString(formatFileEntry(f, fileComments[f.Path], contentWidth, cw))
		sb.WriteString("\n")
	}

	remaining := totalChanged - len(displayFiles) - len(haPaths)
	if remaining > 0 {
		sb.WriteString(fmt.Sprintf("  … and %d more\n", remaining))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderBody(body string, width int, colorMode string) string {
	if body == "" {
		return ""
	}
	opts := []glamour.TermRendererOption{
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(max(width-4, 20)),
	}
	switch colorMode {
	case "always":
		opts = append(opts, glamour.WithColorProfile(termenv.ANSI256))
	case "never":
		opts = append(opts, glamour.WithColorProfile(termenv.Ascii))
	}
	renderer, err := glamour.NewTermRenderer(opts...)
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

func isConsecutiveCommit(prev collapsedEvent, next github.TimelineEvent) bool {
	return prev.event.Type == "committed" && next.Type == "committed" && prev.event.Actor == next.Actor
}

func collapseTimeline(events []github.TimelineEvent) []collapsedEvent {
	var collapsed []collapsedEvent
	for _, e := range events {
		n := len(collapsed)
		if n > 0 && isConsecutiveCommit(collapsed[n-1], e) {
			collapsed[n-1].count++
			collapsed[n-1].event.CreatedAt = e.CreatedAt
			collapsed[n-1].event.Details = e.Details
		} else {
			collapsed = append(collapsed, collapsedEvent{count: 1, event: e})
		}
	}
	return collapsed
}

func renderTimeline(pr github.PullRequest, timeline []github.TimelineEvent) string {
	var events []github.TimelineEvent

	if !pr.CreatedAt.IsZero() {
		events = append(events, github.TimelineEvent{
			Actor:     pr.AuthorLogin,
			CreatedAt: pr.CreatedAt,
			Type:      "opened",
		})
	}
	events = append(events, timeline...)

	slices.SortFunc(events, func(a, b github.TimelineEvent) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	collapsed := collapseTimeline(events)
	if len(collapsed) == 0 {
		return ""
	}

	header := sectionHeader("Activity")
	timeStyle := lipgloss.NewStyle().Foreground(colorGray).Width(16).Align(lipgloss.Right)
	lineStyle := lipgloss.NewStyle().Foreground(colorPurple)

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	for i, ce := range collapsed {
		e := ce.event
		var icon, msg string
		switch {
		case e.Type == "opened":
			icon = colorIcon("●", colorGreen)
			msg = fmt.Sprintf("@%s opened this PR", e.Actor)
		case e.Type == "committed" && ce.count > 1:
			icon = colorIcon("⊙", colorPurple)
			msg = fmt.Sprintf("@%s pushed %d commits", e.Actor, ce.count)
		default:
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

func renderPreview(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, fileComments map[string]int, timeline []github.TimelineEvent, width int, colorMode string) error {
	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorPurple).
		Width(width-2).
		Padding(0, 1)

	var sections []string

	sections = append(sections, boxStyle.Render(renderHeader(pr)))

	if checksContent := renderChecks(pr); checksContent != "" {
		sections = append(sections, boxStyle.Render(checksContent))
	}

	scored := scoreFiles(files, fileComments)
	contentWidth := width - 4

	highActivity, shownFiles := renderHighActivity(scored, contentWidth)
	if highActivity != "" {
		sections = append(sections, boxStyle.Render(highActivity))
	}

	haPaths := make(map[string]bool, len(shownFiles))
	for _, sf := range shownFiles {
		haPaths[sf.file.Path] = true
	}

	sections = append(sections, boxStyle.Render(renderFileList(files, fileComments, pr.FilesChanged, haPaths, contentWidth)))

	if body := renderBody(pr.Body, width, colorMode); body != "" {
		sections = append(sections, boxStyle.Render(body))
	}

	if timelineContent := renderTimeline(pr, timeline); timelineContent != "" {
		sections = append(sections, boxStyle.Render(timelineContent))
	}

	_, err := fmt.Fprintln(w, lipgloss.JoinVertical(lipgloss.Left, sections...))
	return err
}
