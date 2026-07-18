package naming

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// IssueTemplateData contains data available to the issue branch template.
type IssueTemplateData struct {
	Number    int    // Issue number (e.g., 123)
	Title     string // Issue title verbatim (may contain spaces and punctuation)
	TitleSlug string // Issue title slugified and capped at issue.title_slug_max_length
}

// IssueNamer handles issue branch and worktree directory name operations.
type IssueNamer struct {
	branchTemplate     *template.Template
	numberAnchor       string
	numberAnchorOK     bool
	numberBoundary     string
	slugifyOpts        SlugifyOptions
	stripBranchPrefix  []string
	titleSlugMaxLength int
	worktreePrefix     string
}

// NewIssueNamer creates a namer from issue and slugify config.
// Returns an error if the template is invalid, produces invalid branch names,
// or never references {{.Number}} (matching is anchored on the issue number,
// so a template without it cannot keep worktrees associated with their issues).
func NewIssueNamer(issueCfg config.IssueConfig, slugCfg config.SlugifyConfig) (*IssueNamer, error) {
	tmpl, err := template.New("branch").Parse(issueCfg.BranchTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid branch_template: %w", err)
	}

	var buf bytes.Buffer
	testData := IssueTemplateData{Number: 1, Title: "Test issue", TitleSlug: "test-issue"}
	if err := tmpl.Execute(&buf, testData); err != nil {
		return nil, fmt.Errorf("branch_template uses invalid field: %w", err)
	}

	if ok, reason := isValidBranchName(buf.String()); !ok {
		return nil, fmt.Errorf("branch_template produces invalid branch name: %s", reason)
	}

	if !templateReferencesNumber(tmpl) {
		return nil, fmt.Errorf("branch_template must reference {{.Number}}: issue matching is anchored on the issue number")
	}

	anchor, boundary, anchorOK := numberAnchorPrefix(tmpl)

	return &IssueNamer{
		branchTemplate:     tmpl,
		numberAnchor:       anchor,
		numberAnchorOK:     anchorOK,
		numberBoundary:     boundary,
		slugifyOpts:        SlugifyOptionsFromConfig(slugCfg),
		stripBranchPrefix:  issueCfg.StripBranchPrefix,
		titleSlugMaxLength: issueCfg.TitleSlugMaxLength,
		worktreePrefix:     issueCfg.WorktreePrefix,
	}, nil
}

// TitleSlug converts an issue title into a slug capped at title_slug_max_length.
// Unlike general slugs, no hash suffix is appended on truncation: the issue
// number already guarantees uniqueness.
func (n *IssueNamer) TitleSlug(title string) string {
	opts := n.slugifyOpts
	opts.HashLength = 0
	opts.MaxLength = 0
	slug := Slugify(title, opts)
	if n.titleSlugMaxLength > 0 {
		// Truncate on runes: with replace_non_alphanum disabled, multibyte
		// characters survive slugification and a byte slice could split one.
		if runes := []rune(slug); len(runes) > n.titleSlugMaxLength {
			slug = strings.TrimRight(string(runes[:n.titleSlugMaxLength]), "-")
		}
	}
	return slug
}

// GenerateBranchName executes the template with issue data to produce a local branch name.
func (n *IssueNamer) GenerateBranchName(number int, title string) (string, error) {
	data := IssueTemplateData{
		Number:    number,
		Title:     title,
		TitleSlug: n.TitleSlug(title),
	}
	var buf bytes.Buffer
	if err := n.branchTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to generate branch name: %w", err)
	}
	return buf.String(), nil
}

// GenerateWorktreeName strips the first configured branch prefix, slugifies
// the result, and avoids adding the worktree prefix when it is already present.
func (n *IssueNamer) GenerateWorktreeName(branchName string) string {
	name := branchName
	for _, prefix := range n.stripBranchPrefix {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}

	slug := Slugify(name, n.slugifyOpts)
	if slug == "" {
		return ""
	}

	if strings.HasPrefix(slug, n.worktreePrefix) {
		return slug
	}

	return n.worktreePrefix + slug
}

// MatchesIssueNumber reports whether branchName belongs to the given issue.
// When only literal text precedes {{.Number}} in the template, matching is
// anchored: the literal prefix, then the number, then end-of-string or the
// template's own separator (the literal that follows {{.Number}}). That
// survives issue title edits, cannot confuse issue 123 with 1234, and cannot
// claim unrelated branches like "issue/2fa-improvements" for issue 2.
// Otherwise it falls back to exact comparison with the regenerated branch name,
// which misses when the issue title has changed since the branch was created.
func (n *IssueNamer) MatchesIssueNumber(branchName string, number int, title string) bool {
	if n.numberAnchorOK {
		prefix := n.numberAnchor + strconv.Itoa(number)
		if !strings.HasPrefix(branchName, prefix) {
			return false
		}
		rest := branchName[len(prefix):]
		if rest == "" {
			return true
		}
		if n.numberBoundary != "" {
			return strings.HasPrefix(rest, n.numberBoundary)
		}
		// No literal follows {{.Number}} in the template; accept any
		// non-digit continuation so 123 still cannot claim 1234.
		return rest[0] < '0' || rest[0] > '9'
	}

	expected, err := n.GenerateBranchName(number, title)
	if err != nil {
		return false
	}
	return branchName == expected
}

// numberAnchorPrefix extracts the literal text preceding the first {{.Number}}
// action in the template, plus the boundary: the first rune of the literal
// that immediately follows it (empty when the number is last or followed by
// another action). Returns ok=false when anything other than literal text
// appears before {{.Number}} (e.g. the title comes first), in which case
// anchored matching is unavailable.
func numberAnchorPrefix(tmpl *template.Template) (prefix, boundary string, ok bool) {
	if tmpl.Tree == nil || tmpl.Root == nil {
		return "", "", false
	}

	var b strings.Builder
	for i, node := range tmpl.Root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
			b.Write(n.Text)
		case *parse.ActionNode:
			if !isNumberField(n) {
				return "", "", false
			}
			if i+1 < len(tmpl.Root.Nodes) {
				if txt, isText := tmpl.Root.Nodes[i+1].(*parse.TextNode); isText && len(txt.Text) > 0 {
					boundary = string([]rune(string(txt.Text))[:1])
				}
			}
			return b.String(), boundary, true
		default:
			return "", "", false
		}
	}
	return "", "", false
}

// templateReferencesNumber reports whether {{.Number}} appears among the
// template's top-level actions. Templates using control structures are given
// the benefit of the doubt.
func templateReferencesNumber(tmpl *template.Template) bool {
	if tmpl.Tree == nil || tmpl.Root == nil {
		return false
	}
	for _, node := range tmpl.Root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
			continue
		case *parse.ActionNode:
			if isNumberField(n) {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func isNumberField(action *parse.ActionNode) bool {
	if action.Pipe == nil || len(action.Pipe.Decl) > 0 || len(action.Pipe.Cmds) != 1 {
		return false
	}
	cmd := action.Pipe.Cmds[0]
	if len(cmd.Args) != 1 {
		return false
	}
	field, ok := cmd.Args[0].(*parse.FieldNode)
	return ok && len(field.Ident) == 1 && field.Ident[0] == "Number"
}
