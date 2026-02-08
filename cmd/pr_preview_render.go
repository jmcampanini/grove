package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
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

const (
	targetHighActivityFiles = 3
	maxPreviewFiles         = 30

	iconArchive  = "\uea98" // nf-cod-archive
	iconBell     = "\ueaa2" // nf-cod-bell
	iconCheck    = "\ueab2" // nf-cod-pass
	iconCircle   = "\uea71" // nf-cod-circle-filled
	iconComment  = "\uea6b" // nf-cod-comment
	iconCross    = "\ueab8" // nf-cod-error
	iconDraft    = "\uea73" // nf-cod-circle-outline
	iconGitRef   = "\uebcb" // nf-cod-git-commit
	iconMerge    = "\ueafe" // nf-cod-git-merge
	iconOpened   = "\uea74" // nf-cod-issue-opened
	iconPending  = "\uf017" // nf-fa-clock
	iconReopened = "\ueb0b" // nf-cod-issue-reopened
	iconTag      = "\uea66" // nf-cod-tag
	iconZap      = "\uea86" // nf-cod-zap
)

func hyperlink(url, text string) string {
	if lipgloss.ColorProfile() == termenv.Ascii {
		return text
	}
	return termenv.Hyperlink(url, text)
}

func stateColor(state github.PRState) lipgloss.TerminalColor {
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

func checkIcon(conclusion github.CheckConclusion) string {
	switch conclusion {
	case github.CheckConclusionSuccess:
		return colorIcon(iconCheck, colorGreen)
	case github.CheckConclusionFailure, github.CheckConclusionTimedOut, github.CheckConclusionActionRequired:
		return colorIcon(iconCross, colorRed)
	case github.CheckConclusionCancelled, github.CheckConclusionSkipped:
		return colorIcon(iconCross, colorGray)
	default:
		return colorIcon(iconPending, colorYellow)
	}
}

func reviewIcon(state github.ReviewState) string {
	switch state {
	case github.ReviewStateApproved:
		return colorIcon(iconCheck, colorGreen)
	case github.ReviewStateChangesRequested:
		return colorIcon(iconCross, colorRed)
	default:
		return colorIcon(iconCircle, colorGray)
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
		return string(sc.Conclusion)
	}
	return strings.ToLower(string(sc.Status))
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
		line := fmt.Sprintf("  %s %s %s",
			checkIcon(sc.Conclusion),
			sc.Name,
			lipgloss.NewStyle().Foreground(colorGray).Render(checkStatusText(sc)),
		)
		if sc.DetailURL != "" {
			link := hyperlink(sc.DetailURL, "[↗ details]")
			line += "  " + lipgloss.NewStyle().Foreground(colorCyan).Render(link)
		}
		lines = append(lines, line)
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

func reviewActionText(state github.ReviewState) string {
	switch state {
	case github.ReviewStateApproved:
		return "approved"
	case github.ReviewStateChangesRequested:
		return "requested changes"
	case github.ReviewStateCommented:
		return "commented"
	case github.ReviewStateDismissed:
		return "review dismissed"
	default:
		return strings.ToLower(string(state))
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

var (
	testDirSegments  = []string{"/test/", "/tests/", "/testdata/"}
	testFileSuffixes = []string{"_test.go", "Test.java", "Tests.java", "IT.java"}
)

func isTestFile(path string) bool {
	base := filepath.Base(path)
	if slices.ContainsFunc(testFileSuffixes, func(s string) bool { return strings.HasSuffix(base, s) }) {
		return true
	}
	prefixed := "/" + path
	return slices.ContainsFunc(testDirSegments, func(s string) bool { return strings.Contains(prefixed, s) })
}

func timelineEventIcon(eventType github.TimelineEventType, details string) string {
	switch eventType {
	case github.TimelineEventReviewed:
		switch details {
		case "approved":
			return colorIcon(iconCheck, colorGreen)
		case "changes requested":
			return colorIcon(iconCross, colorRed)
		default:
			return colorIcon(iconComment, colorGray)
		}
	case github.TimelineEventCommented:
		return colorIcon(iconComment, colorCyan)
	case github.TimelineEventCommitted:
		return colorIcon(iconGitRef, colorPurple)
	case github.TimelineEventForcePushed:
		return colorIcon(iconZap, colorYellow)
	case github.TimelineEventLabeled:
		return colorIcon(iconTag, colorCyan)
	case github.TimelineEventMerged:
		return colorIcon(iconMerge, colorPurple)
	case github.TimelineEventClosed:
		return colorIcon(iconArchive, colorRed)
	case github.TimelineEventReopened:
		return colorIcon(iconReopened, colorGreen)
	case github.TimelineEventReadyForReview:
		return colorIcon(iconBell, colorGreen)
	case github.TimelineEventConvertToDraft:
		return colorIcon(iconDraft, colorYellow)
	case github.TimelineEventReviewRequested:
		return colorIcon(iconBell, colorCyan)
	default:
		return colorIcon(iconCircle, colorGray)
	}
}

func timelineEventMessage(e github.TimelineEvent) string {
	switch e.Type {
	case github.TimelineEventReviewed:
		return fmt.Sprintf("@%s %s", e.Actor, e.Details)
	case github.TimelineEventCommented:
		return fmt.Sprintf("@%s commented", e.Actor)
	case github.TimelineEventCommitted:
		if e.Details != "" {
			return fmt.Sprintf("@%s committed: %s", e.Actor, e.Details)
		}
		return fmt.Sprintf("@%s committed", e.Actor)
	case github.TimelineEventForcePushed:
		return fmt.Sprintf("@%s force pushed", e.Actor)
	case github.TimelineEventLabeled:
		return fmt.Sprintf("@%s added %s", e.Actor, e.Details)
	case github.TimelineEventMerged:
		return fmt.Sprintf("@%s merged", e.Actor)
	case github.TimelineEventClosed:
		return fmt.Sprintf("@%s closed", e.Actor)
	case github.TimelineEventReopened:
		return fmt.Sprintf("@%s reopened", e.Actor)
	case github.TimelineEventReadyForReview:
		return fmt.Sprintf("@%s marked ready for review", e.Actor)
	case github.TimelineEventConvertToDraft:
		return fmt.Sprintf("@%s converted to draft", e.Actor)
	case github.TimelineEventReviewRequested:
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

func fileDiffURL(prURL, path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%s/files#diff-%s", prURL, hex.EncodeToString(h[:]))
}

func formatFileEntry(f github.PullRequestFile, comments int, prURL string, contentWidth int, cw fileColumnWidths) string {
	added := lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("%+*d", cw.addWidth, f.Additions))
	delStr := fmt.Sprintf("-%d", f.Deletions)
	deleted := lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("%*s", cw.delWidth, delStr))

	var linkCol string
	if prURL != "" {
		link := hyperlink(fileDiffURL(prURL, f.Path), "[↗]")
		linkCol = "  " + lipgloss.NewStyle().Foreground(colorCyan).Render(link)
	}

	var right string
	if cw.commentWidth > 0 {
		if comments > 0 {
			right = fmt.Sprintf("%s %*d%s  %s  %s", iconComment, cw.commentWidth, comments, linkCol, added, deleted)
		} else {
			commentColWidth := lipgloss.Width(fmt.Sprintf("%s %*d", iconComment, cw.commentWidth, 0))
			right = fmt.Sprintf("%s%s  %s  %s", strings.Repeat(" ", commentColWidth), linkCol, added, deleted)
		}
	} else {
		if linkCol != "" {
			right = fmt.Sprintf("%s  %s  %s", linkCol, added, deleted)
		} else {
			right = fmt.Sprintf("%s  %s", added, deleted)
		}
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

	row := func(label, value string) string {
		return labelStyle.Render(label) + valueStyle.Render(value)
	}

	var sb strings.Builder
	fmt.Fprintln(&sb, titleLine)
	fmt.Fprintln(&sb, row("Author", "@"+pr.AuthorLogin))
	fmt.Fprintln(&sb, row("Branch", branchInfo(pr.BranchName, pr.BaseRefName)))
	fmt.Fprintln(&sb, row("Changed", formatStats(pr)))

	if labels := renderLabels(pr.Labels); labels != "" {
		fmt.Fprintln(&sb, row("Labels", labels))
	}

	if pr.URL != "" {
		fmt.Fprintln(&sb, row("URL", hyperlink(pr.URL, pr.URL)))
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
	var commented, uncommented []scoredFile
	for _, sf := range scored {
		if sf.comments > 0 {
			commented = append(commented, sf)
		} else {
			uncommented = append(uncommented, sf)
		}
	}

	result := commented
	remaining := targetHighActivityFiles - len(result)
	if remaining > 0 {
		result = append(result, uncommented[:min(remaining, len(uncommented))]...)
	}
	return result
}

func renderHighActivity(scored []scoredFile, prURL string, contentWidth int) (string, []scoredFile) {
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
		sb.WriteString(formatFileEntry(sf.file, sf.comments, prURL, contentWidth, cw))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n"), shown
}

func renderFileList(files []github.PullRequestFile, fileComments map[string]int, totalChanged int, haPaths map[string]bool, prURL string, contentWidth int) string {
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
		sb.WriteString(formatFileEntry(f, fileComments[f.Path], prURL, contentWidth, cw))
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
	// TODO: replace with structured debug logging when a logging framework is added
	renderer, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: markdown renderer init failed, using plain text: %v\n", err)
		return wrapBody(body, width)
	}
	rendered, err := renderer.Render(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: markdown rendering failed, using plain text: %v\n", err)
		return wrapBody(body, width)
	}
	return strings.TrimSpace(rendered)
}

type collapsedEvent struct {
	count int
	event github.TimelineEvent
}

func isConsecutiveCommit(prev collapsedEvent, next github.TimelineEvent) bool {
	return prev.event.Type == github.TimelineEventCommitted &&
		next.Type == github.TimelineEventCommitted &&
		prev.event.Actor == next.Actor
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
			Type:      github.TimelineEventOpened,
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
		case e.Type == github.TimelineEventOpened:
			icon = colorIcon(iconOpened, colorGreen)
			msg = fmt.Sprintf("@%s opened this PR", e.Actor)
		case e.Type == github.TimelineEventCommitted && ce.count > 1:
			icon = colorIcon(iconGitRef, colorPurple)
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

	highActivity, shownFiles := renderHighActivity(scored, pr.URL, contentWidth)
	if highActivity != "" {
		sections = append(sections, boxStyle.Render(highActivity))
	}

	haPaths := make(map[string]bool, len(shownFiles))
	for _, sf := range shownFiles {
		haPaths[sf.file.Path] = true
	}

	sections = append(sections, boxStyle.Render(renderFileList(files, fileComments, pr.FilesChanged, haPaths, pr.URL, contentWidth)))

	if body := renderBody(pr.Body, width, colorMode); body != "" {
		sections = append(sections, boxStyle.Render(body))
	}

	if timelineContent := renderTimeline(pr, timeline); timelineContent != "" {
		sections = append(sections, boxStyle.Render(timelineContent))
	}

	_, err := fmt.Fprintln(w, lipgloss.JoinVertical(lipgloss.Left, sections...))
	return err
}
