package cmd

import (
	"fmt"
	"image/color"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jmcampanini/grove-cli/internal/github"
)

// maxPreviewComments is how many of the most recent comments the preview renders.
const maxPreviewComments = 5

func issueStateColor(iss github.Issue) color.Color {
	switch iss.State {
	case github.IssueStateOpen:
		return colorGreen
	case github.IssueStateClosed:
		if iss.StateReason == github.IssueStateReasonNotPlanned {
			return colorGray
		}
		return colorPurple
	default:
		return colorGray
	}
}

func issueStateText(iss github.Issue) string {
	if iss.State == github.IssueStateClosed && iss.StateReason == github.IssueStateReasonNotPlanned {
		return "NOT PLANNED"
	}
	return strings.ToUpper(string(iss.State))
}

func renderIssueHeader(iss github.Issue) string {
	labelStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Width(10)
	valueStyle := lipgloss.NewStyle().Foreground(colorGray)

	stateStr := lipgloss.NewStyle().
		Background(issueStateColor(iss)).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Padding(0, 1).
		Render(issueStateText(iss))
	titleLine := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("#%d %s", iss.Number, iss.Title)) + "  " + stateStr

	row := func(label, value string) string {
		return labelStyle.Render(label) + valueStyle.Render(value)
	}

	var sb strings.Builder
	fmt.Fprintln(&sb, titleLine)
	fmt.Fprintln(&sb, row("Author", "@"+iss.AuthorLogin))

	if labels := renderLabels(iss.Labels); labels != "" {
		fmt.Fprintln(&sb, row("Labels", labels))
	}
	if len(iss.Assignees) > 0 {
		fmt.Fprintln(&sb, row("Assignees", "@"+strings.Join(iss.Assignees, " @")))
	}
	if iss.Milestone != "" {
		fmt.Fprintln(&sb, row("Milestone", iss.Milestone))
	}
	if iss.URL != "" {
		fmt.Fprintln(&sb, row("URL", hyperlink(iss.URL, iss.URL)))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderIssueComments(comments []github.IssueComment, width int, colorMode string) string {
	if len(comments) == 0 {
		return ""
	}

	shown := comments
	omitted := 0
	if len(shown) > maxPreviewComments {
		omitted = len(shown) - maxPreviewComments
		shown = shown[omitted:]
	}

	metaStyle := lipgloss.NewStyle().Foreground(colorGray)

	var parts []string
	if omitted > 0 {
		parts = append(parts, metaStyle.Render(fmt.Sprintf("… %d earlier comments not shown", omitted)))
	}
	for _, c := range shown {
		meta := lipgloss.NewStyle().Bold(true).Render("@"+c.AuthorLogin) + metaStyle.Render(" · "+relativeTime(c.CreatedAt))
		if body := renderBody(c.Body, width, colorMode); body != "" {
			parts = append(parts, meta+"\n"+body)
		} else {
			parts = append(parts, meta)
		}
	}

	header := sectionHeader(fmt.Sprintf("Comments (%d)", len(comments)))
	return header + "\n" + strings.Join(parts, "\n\n")
}

func renderIssuePreview(w io.Writer, iss github.Issue, width int, colorMode string) error {
	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorPurple).
		Width(width-2).
		Padding(0, 1)

	var sections []string

	sections = append(sections, boxStyle.Render(renderIssueHeader(iss)))

	if body := renderBody(iss.Body, width, colorMode); body != "" {
		sections = append(sections, boxStyle.Render(body))
	}

	if comments := renderIssueComments(iss.Comments, width, colorMode); comments != "" {
		sections = append(sections, boxStyle.Render(comments))
	}

	return writePreviewLine(w, lipgloss.JoinVertical(lipgloss.Left, sections...))
}
