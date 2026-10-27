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
	"github.com/jmcampanini/grove/internal/listformat"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/jmcampanini/grove/internal/pr"
	"github.com/spf13/cobra"
)

func newPRListCmd(services commandServices) *cobra.Command {
	var flags listFlags

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open pull requests",
		Long: `List the open pull requests of the current repository, newest first, on
stdout or in the --output-file path. Draft pull requests are not listed.
Nothing in the repository or on GitHub is modified and nothing prompts.

By default the output is a styled table: number, title, author, branch,
state, a Local checkmark when a worktree exists for the pull request, and
when it was updated. With --fzf each line is
<number>\t<searchable>\t<display>, a machine-readable format that is never
styled, whatever --color says.

` + helpListFormats + `

grove defines the record fields from the GitHub GraphQL API. They are the
--json and --jsonl keys and, in this order, the --csv columns:

  number             pull request number
  title              pull request title
  author             author login, empty when GitHub reports no author
  branch             head branch name
  baseBranch         base branch name
  isCrossRepository  true when the head branch is in a fork
  labels             list of label names, at most the first 100
  createdAt          when the pull request was opened, RFC 3339 in UTC
  updatedAt          when the pull request was last updated, RFC 3339 in
                     UTC
  url                pull request URL
  worktree           absolute path of the local worktree for the pull
                     request, or null when there is none

With no open pull requests, the table prints "No open pull requests
found.", --fzf and --jsonl print nothing, --csv prints only the header
line, or nothing with --no-header, and --json prints {"items": [],
"hasMore": false, "complete": true}.

` + helpListScope("pull requests") + `
GitHub cannot exclude drafts from the pages, so hasMore can be true when
only drafts remain; resuming then returns no records.

` + helpListOutputFile + `

` + helpGH + `

` + helpFailure,
		Example: `  grove pr list --fzf | fzf --delimiter '\t' --with-nth 3 --accept-nth 1
  grove pr list --json --all | jq -r '.items[].branch'
  grove pr list --csv --limit 100 --output-file pulls.csv
  grove pr list --jsonl --cursor <cursor> >>pulls.jsonl`,
		Args: cobra.MatchAll(cobra.NoArgs, flags.validate),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return services.runPRList(cmd, flags)
		},
	}
	flags.register(cmd)
	return cmd
}

func (services commandServices) runPRList(cmd *cobra.Command, flags listFlags) error {
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

	namer, err := naming.NewPullRequestNamer(cfg.PullRequest, cfg.Naming)
	if err != nil {
		return fmt.Errorf("failed to create PR namer: %w", err)
	}

	list, err := gh.ListOpenPullRequests(flags.request())
	if err != nil {
		return err
	}

	worktrees, err := rt.GitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := pr.NewMatcher(namer, rt.Logger)
	matches := matcher.MatchAll(list.PullRequests, worktrees)
	result := listResult[prRecord]{
		hasMore:    list.HasMore,
		nextCursor: list.NextCursor,
		records:    prRecords(matches),
	}

	// Report first, so a failed report fails the run before the output
	// file is in place.
	if err := reportMoreRecords(cmd, flags, "pull requests", result); err != nil {
		return err
	}
	switch flags.format() {
	case listFormatTable:
		return outputPRListTable(styledStdout(cmd), matches)
	case listFormatFzf:
		return outputPRListFzf(cmd.OutOrStdout(), matches)
	default:
		return writeListPayload(cmd, flags, result, prRecordColumns)
	}
}

// prRecordColumns are the CSV columns of prRecord, in order.
var prRecordColumns = []string{
	"number", "title", "author", "branch", "baseBranch", "isCrossRepository",
	"labels", "createdAt", "updatedAt", "url", "worktree",
}

// prRecord is one pull request of the --json, --jsonl, and --csv output.
// The field order is the JSON key order and matches prRecordColumns.
type prRecord struct {
	Number            int       `json:"number"`
	Title             string    `json:"title"`
	Author            string    `json:"author"`
	Branch            string    `json:"branch"`
	BaseBranch        string    `json:"baseBranch"`
	IsCrossRepository bool      `json:"isCrossRepository"`
	Labels            []string  `json:"labels"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	URL               string    `json:"url"`
	Worktree          *string   `json:"worktree"`
}

func prRecords(matches []pr.Match) []prRecord {
	records := make([]prRecord, 0, len(matches))
	for _, match := range matches {
		record := prRecord{
			Author:            match.PR.AuthorLogin,
			BaseBranch:        match.PR.BaseRefName,
			Branch:            match.PR.BranchName,
			CreatedAt:         match.PR.CreatedAt.UTC(),
			IsCrossRepository: match.PR.IsCrossRepository,
			Labels:            labelNames(match.PR.Labels),
			Number:            match.PR.Number,
			Title:             match.PR.Title,
			UpdatedAt:         match.PR.UpdatedAt.UTC(),
			URL:               match.PR.URL,
		}
		if match.HasWorktree() {
			record.Worktree = &match.WorktreePath
		}
		records = append(records, record)
	}
	return records
}

func (r prRecord) csvRow() []string {
	return []string{
		strconv.Itoa(r.Number),
		r.Title,
		r.Author,
		r.Branch,
		r.BaseBranch,
		strconv.FormatBool(r.IsCrossRepository),
		compactJSONCell(r.Labels),
		r.CreatedAt.Format(time.RFC3339),
		r.UpdatedAt.Format(time.RFC3339),
		r.URL,
		optionalCell(r.Worktree),
	}
}

func outputPRListTable(w io.Writer, matches []pr.Match) error {
	if len(matches) == 0 {
		_, err := fmt.Fprintln(w, "No open pull requests found.")
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
			localMarker = "\u2713" // checkmark
		}

		state := strings.ToLower(string(match.PR.State))
		updated := humanize.Time(match.PR.UpdatedAt)

		rows[i] = []string{
			fmt.Sprintf("%d", match.PR.Number),
			listformat.TruncateString(match.PR.Title, 40),
			match.PR.AuthorLogin,
			listformat.TruncateString(match.PR.BranchName, 30),
			state,
			localMarker,
			updated,
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
		Headers("#", "Title", "Author", "Branch", "State", "Local", "Updated").
		Rows(rows...)

	_, err := fmt.Fprintln(w, t)
	return err
}

func outputPRListFzf(w io.Writer, matches []pr.Match) error {
	for _, match := range matches {
		number := fmt.Sprintf("%d", match.PR.Number)

		state := strings.ToLower(string(match.PR.State))
		searchable := listformat.SanitizeFzfField(fmt.Sprintf("%d %s %s %s %s",
			match.PR.Number,
			match.PR.Title,
			match.PR.BranchName,
			match.PR.AuthorLogin,
			state,
		))

		localPrefix := ""
		if match.HasWorktree() {
			localPrefix = "\u2713 " // checkmark with space
		}
		display := listformat.SanitizeFzfField(fmt.Sprintf("%s#%d %s [%s] %s",
			localPrefix,
			match.PR.Number,
			match.PR.Title,
			match.PR.AuthorLogin,
			match.PR.BranchName,
		))

		_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", number, searchable, display)
		if err != nil {
			return err
		}
	}
	return nil
}
