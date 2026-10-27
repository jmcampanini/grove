package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/dustin/go-humanize"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/issue"
	"github.com/jmcampanini/grove/internal/listformat"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/spf13/cobra"
)

func newIssueListCmd(services commandServices) *cobra.Command {
	var flags listFlags

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open issues",
		Long: `List the open issues of the current repository on stdout, newest first.
Nothing is modified and nothing prompts.

By default the output is a styled table: number, title, author, labels, a
Local checkmark when a worktree exists for the issue, and when it was
updated. With --fzf each line is <number>\t<searchable>\t<display>, a
machine-readable format that is never styled, whatever --color says.

` + helpListFormats + `

grove defines the record fields from the GitHub GraphQL API. They are the
--json and --jsonl keys and, in this order, the --csv columns:

  number     issue number
  title      issue title
  author     author login, empty when GitHub reports no author
  labels     list of label names, at most the first 100
  createdAt  when the issue was opened, RFC 3339 in UTC
  updatedAt  when the issue was last updated, RFC 3339 in UTC
  url        issue URL
  worktree   absolute path of the local worktree for the issue, or null
             when there is none

With no open issues, the table prints "No open issues found.", --fzf and
--jsonl print nothing, --csv prints only the header line, or nothing with
--no-header, and --json prints {"items": [], "hasMore": false,
"complete": true}.

` + helpListScope("issues") + `

` + helpListOutputFile + `

` + helpGH + `

` + helpFailure,
		Example: `  grove issue list --fzf | fzf --delimiter '\t' --with-nth 3 --accept-nth 1
  grove issue list --json --all | jq -r '.items[].title'
  grove issue list --csv --limit 100 --output-file issues.csv
  grove issue list --jsonl --cursor <cursor> >>issues.jsonl`,
		Args: cobra.MatchAll(cobra.NoArgs, flags.validate),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return services.runIssueList(cmd, flags)
		},
	}
	flags.register(cmd)
	return cmd
}

func (services commandServices) runIssueList(cmd *cobra.Command, flags listFlags) error {
	if err := flags.checkOutputFile(); err != nil {
		return err
	}

	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}
	cfg := rt.Config

	gh, err := rt.NewCachedGitHubClient()
	if err != nil {
		return err
	}

	namer, err := naming.NewIssueNamer(cfg.Issue, cfg.Naming)
	if err != nil {
		return fmt.Errorf("failed to create issue namer: %w", err)
	}

	list, err := gh.ListOpenIssues(flags.request())
	if err != nil {
		return err
	}

	worktrees, err := rt.GitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := issue.NewMatcher(namer, rt.Logger)
	matches := matcher.MatchAll(list.Issues, worktrees)
	result := listResult[issueRecord]{
		hasMore:    list.HasMore,
		nextCursor: list.NextCursor,
		records:    issueRecords(matches),
	}

	switch flags.format() {
	case listFormatTable:
		err = outputIssueListTable(styledStdout(cmd), matches)
	case listFormatFzf:
		err = outputIssueListFzf(cmd.OutOrStdout(), matches)
	default:
		err = writeListPayload(cmd, flags, result, issueRecordColumns)
	}
	if err != nil {
		return err
	}
	return reportMoreRecords(cmd, flags, "issues", result)
}

// issueRecordColumns are the CSV columns of issueRecord, in order.
var issueRecordColumns = []string{"number", "title", "author", "labels", "createdAt", "updatedAt", "url", "worktree"}

// issueRecord is one issue of the --json, --jsonl, and --csv output. The
// field order is the JSON key order and matches issueRecordColumns.
type issueRecord struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Author    string    `json:"author"`
	Labels    []string  `json:"labels"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	URL       string    `json:"url"`
	Worktree  *string   `json:"worktree"`
}

func issueRecords(matches []issue.Match) []issueRecord {
	records := make([]issueRecord, 0, len(matches))
	for _, match := range matches {
		record := issueRecord{
			Author:    match.Issue.AuthorLogin,
			CreatedAt: match.Issue.CreatedAt.UTC(),
			Labels:    labelNames(match.Issue.Labels),
			Number:    match.Issue.Number,
			Title:     match.Issue.Title,
			UpdatedAt: match.Issue.UpdatedAt.UTC(),
			URL:       match.Issue.URL,
		}
		if match.HasWorktree() {
			record.Worktree = &match.WorktreePath
		}
		records = append(records, record)
	}
	return records
}

func (r issueRecord) csvRow() []string {
	return []string{
		strconv.Itoa(r.Number),
		r.Title,
		r.Author,
		compactJSONCell(r.Labels),
		r.CreatedAt.Format(time.RFC3339),
		r.UpdatedAt.Format(time.RFC3339),
		r.URL,
		optionalCell(r.Worktree),
	}
}

// labelNames returns the names of the labels, never nil.
func labelNames(labels []github.Label) []string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return names
}

// maxListLabels is how many label names the list shows before collapsing the
// rest into a "+n" overflow count.
const maxListLabels = 2

func formatIssueLabels(labels []github.Label) string {
	if len(labels) == 0 {
		return ""
	}

	shown := min(len(labels), maxListLabels)
	names := make([]string, 0, shown)
	for _, l := range labels[:shown] {
		names = append(names, l.Name)
	}

	s := strings.Join(names, ", ")
	if extra := len(labels) - shown; extra > 0 {
		s = fmt.Sprintf("%s +%d", s, extra)
	}
	return s
}

func allIssueLabels(labels []github.Label) string {
	return strings.Join(labelNames(labels), ", ")
}

func outputIssueListTable(w io.Writer, matches []issue.Match) error {
	if len(matches) == 0 {
		_, err := fmt.Fprintln(w, "No open issues found.")
		return err
	}

	purple := lipgloss.Color("99")
	gray := lipgloss.Color("245")
	lightGray := lipgloss.Color("241")

	headerStyle := lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	oddRowStyle := cellStyle.Foreground(gray)
	evenRowStyle := cellStyle.Foreground(lightGray)

	rows := make([][]string, len(matches))
	for i, match := range matches {
		localMarker := ""
		if match.HasWorktree() {
			localMarker = "✓" // checkmark
		}

		rows[i] = []string{
			fmt.Sprintf("%d", match.Issue.Number),
			listformat.TruncateString(match.Issue.Title, 40),
			match.Issue.AuthorLogin,
			listformat.TruncateString(formatIssueLabels(match.Issue.Labels), 30),
			localMarker,
			humanize.Time(match.Issue.UpdatedAt),
		}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row%2 == 0:
				return evenRowStyle
			default:
				return oddRowStyle
			}
		}).
		Headers("#", "Title", "Author", "Labels", "Local", "Updated").
		Rows(rows...)

	_, err := fmt.Fprintln(w, t)
	return err
}

func outputIssueListFzf(w io.Writer, matches []issue.Match) error {
	for _, match := range matches {
		number := fmt.Sprintf("%d", match.Issue.Number)

		state := strings.ToLower(string(match.Issue.State))
		searchable := listformat.SanitizeFzfField(fmt.Sprintf("%d %s %s %s %s",
			match.Issue.Number,
			match.Issue.Title,
			allIssueLabels(match.Issue.Labels),
			match.Issue.AuthorLogin,
			state,
		))

		localPrefix := ""
		if match.HasWorktree() {
			localPrefix = "✓ " // checkmark with space
		}
		display := listformat.SanitizeFzfField(fmt.Sprintf("%s#%d %s [%s] %s",
			localPrefix,
			match.Issue.Number,
			match.Issue.Title,
			match.Issue.AuthorLogin,
			formatIssueLabels(match.Issue.Labels),
		))

		_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", number, searchable, display)
		if err != nil {
			return err
		}
	}
	return nil
}
