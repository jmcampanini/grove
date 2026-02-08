package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jmcampanini/grove-cli/internal/github"
)

var (
	colorGreen  = lipgloss.Color("76")
	colorGray   = lipgloss.Color("245")
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

// --- Group B stubs (implemented in Phase 5) ---

func renderContext(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}

func renderBoard(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}

func renderTimeline(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}
