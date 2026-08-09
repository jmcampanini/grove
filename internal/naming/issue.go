package naming

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// IssueBranchTemplateData contains the values available to an issue branch template.
type IssueBranchTemplateData struct {
	Number    int
	TitleSlug string
}

// IssueWorktreeTemplateData contains the values available to an issue worktree template.
type IssueWorktreeTemplateData struct {
	BranchSlug string
	Number     int
	TitleSlug  string
}

// IssueNamer handles issue branch and worktree directory name operations.
type IssueNamer struct {
	branchTemplate        *template.Template
	maxLength             int
	numberAnchor          string
	numberAnchorOK        bool
	numberBoundary        string
	slugifyOpts           SlugifyOptions
	stripPrefixes         []string
	worktreeLiteralPrefix string
	worktreeTemplate      *template.Template
}

// NewIssueNamer creates a namer and validates both issue templates.
func NewIssueNamer(issueCfg config.IssueConfig, namingCfg config.NamingConfig) (*IssueNamer, error) {
	branchTemplate, err := parseNameTemplate(
		"branch_template",
		issueCfg.BranchTemplate,
		IssueBranchTemplateData{Number: 1, TitleSlug: "test-issue"},
		namingCfg.MaxLength,
		isValidBranchName,
	)
	if err != nil {
		return nil, err
	}
	if !templateHasDirectNumberAction(branchTemplate) {
		return nil, errors.New("branch_template must directly render {{.Number}}: issue matching requires the complete issue number")
	}

	worktreeTemplate, err := parseNameTemplate(
		"worktree_template",
		issueCfg.WorktreeTemplate,
		IssueWorktreeTemplateData{BranchSlug: "test-branch", Number: 1, TitleSlug: "test-issue"},
		namingCfg.MaxLength,
		isValidWorktreeName,
	)
	if err != nil {
		return nil, err
	}

	anchor, boundary, anchorOK := numberAnchorPrefix(branchTemplate)
	return &IssueNamer{
		branchTemplate:        branchTemplate,
		maxLength:             namingCfg.MaxLength,
		numberAnchor:          anchor,
		numberAnchorOK:        anchorOK,
		numberBoundary:        boundary,
		slugifyOpts:           SlugifyOptionsFromConfig(namingCfg),
		stripPrefixes:         namingCfg.StripPrefixes,
		worktreeLiteralPrefix: leadingLiteral(worktreeTemplate),
		worktreeTemplate:      worktreeTemplate,
	}, nil
}

// TitleSlug converts an issue title into an uncapped slug.
func (n *IssueNamer) TitleSlug(title string) string {
	return Slugify(title, n.slugifyOpts)
}

// GenerateBranchName renders and validates the issue branch name.
func (n *IssueNamer) GenerateBranchName(number int, title string) (string, error) {
	name, err := n.renderBranchName(number, title)
	if err != nil {
		return "", fmt.Errorf("failed to generate branch name: %w", err)
	}
	if !n.preservesIssueNumber(name, number, title) {
		return "", fmt.Errorf("failed to generate branch name: final name %q does not preserve complete issue number %d; increase naming.max_length or place {{.Number}} earlier", name, number)
	}
	return name, nil
}

func (n *IssueNamer) renderBranchName(number int, title string) (string, error) {
	return renderName(
		n.branchTemplate,
		IssueBranchTemplateData{Number: number, TitleSlug: n.TitleSlug(title)},
		n.maxLength,
		isValidBranchName,
	)
}

func (n *IssueNamer) preservesIssueNumber(name string, number int, title string) bool {
	if n.numberAnchorOK && !n.matchesNumberAnchor(name, number) {
		return false
	}

	marker, ok := issueNumberMarker(name, len(strconv.Itoa(number)))
	if !ok {
		return false
	}
	probe, ok := issueNumberProbe(n.branchTemplate, marker)
	if !ok {
		return false
	}
	probeName, err := renderName(
		probe,
		IssueBranchTemplateData{Number: number, TitleSlug: n.TitleSlug(title)},
		n.maxLength,
		isValidBranchName,
	)
	return err == nil && strings.Contains(probeName, marker)
}

func issueNumberMarker(name string, length int) (string, bool) {
	for markerRune := rune(0xE000); markerRune <= 0xF8FF; markerRune++ {
		marker := strings.Repeat(string(markerRune), length)
		if !strings.Contains(name, marker) {
			return marker, true
		}
	}
	return "", false
}

func issueNumberProbe(tmpl *template.Template, marker string) (*template.Template, bool) {
	probe := template.New(tmpl.Name())
	replaced := 0
	for _, associated := range tmpl.Templates() {
		if associated.Tree == nil {
			continue
		}
		tree := associated.Copy()
		replaced += walkNumberActions(tree.Root, func(action *parse.ActionNode) {
			field := action.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
			action.Pipe.Cmds[0].Args[0] = &parse.StringNode{
				NodeType: parse.NodeString,
				Pos:      field.Pos,
				Quoted:   strconv.Quote(marker),
				Text:     marker,
			}
		})
		if _, err := probe.AddParseTree(associated.Name(), tree); err != nil {
			return nil, false
		}
	}
	if replaced == 0 {
		return nil, false
	}
	return probe.Lookup(tmpl.Name()), true
}

func walkNumberActions(node parse.Node, visit func(*parse.ActionNode)) int {
	if isNilParseNode(node) {
		return 0
	}

	switch node := node.(type) {
	case *parse.ListNode:
		count := 0
		for _, child := range node.Nodes {
			count += walkNumberActions(child, visit)
		}
		return count
	case *parse.ActionNode:
		if !isNumberField(node) {
			return 0
		}
		if visit != nil {
			visit(node)
		}
		return 1
	case *parse.IfNode:
		return walkNumberActions(node.List, visit) + walkNumberActions(node.ElseList, visit)
	case *parse.RangeNode:
		return walkNumberActions(node.List, visit) + walkNumberActions(node.ElseList, visit)
	case *parse.WithNode:
		return walkNumberActions(node.List, visit) + walkNumberActions(node.ElseList, visit)
	default:
		return 0
	}
}

// GenerateWorktreeName renders and validates the issue worktree directory name.
func (n *IssueNamer) GenerateWorktreeName(number int, title, branch string) (string, error) {
	name, err := renderName(
		n.worktreeTemplate,
		IssueWorktreeTemplateData{
			BranchSlug: slugifyBranch(branch, n.stripPrefixes, n.slugifyOpts),
			Number:     number,
			TitleSlug:  n.TitleSlug(title),
		},
		n.maxLength,
		isValidWorktreeName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate worktree name: %w", err)
	}
	return name, nil
}

func (n *IssueNamer) WorktreeLiteralPrefix() string {
	return n.worktreeLiteralPrefix
}

// MatchesIssueNumber reports whether branch belongs to the given issue.
func (n *IssueNamer) MatchesIssueNumber(branch string, number int, title string) bool {
	if n.numberAnchorOK {
		return n.matchesNumberAnchor(branch, number)
	}

	expected, err := n.GenerateBranchName(number, title)
	return err == nil && branch == expected
}

func (n *IssueNamer) matchesNumberAnchor(branch string, number int) bool {
	prefix := n.numberAnchor + strconv.Itoa(number)
	if !strings.HasPrefix(branch, prefix) {
		return false
	}
	rest := branch[len(prefix):]
	if rest == "" {
		return true
	}
	if n.numberBoundary != "" {
		return strings.HasPrefix(rest, n.numberBoundary)
	}
	return rest[0] < '0' || rest[0] > '9'
}

func numberAnchorPrefix(tmpl *template.Template) (prefix, boundary string, ok bool) {
	if tmpl.Tree == nil || tmpl.Root == nil {
		return "", "", false
	}

	var prefixBuilder strings.Builder
	for i, node := range tmpl.Root.Nodes {
		switch node := node.(type) {
		case *parse.TextNode:
			prefixBuilder.Write(node.Text)
		case *parse.ActionNode:
			if !isNumberField(node) {
				return "", "", false
			}
			if i+1 < len(tmpl.Root.Nodes) {
				if text, isText := tmpl.Root.Nodes[i+1].(*parse.TextNode); isText && len(text.Text) > 0 {
					boundary = string([]rune(string(text.Text))[0])
				}
			}
			return prefixBuilder.String(), boundary, true
		default:
			return "", "", false
		}
	}
	return "", "", false
}

func templateHasDirectNumberAction(tmpl *template.Template) bool {
	for _, associated := range tmpl.Templates() {
		if associated.Tree != nil && walkNumberActions(associated.Root, nil) > 0 {
			return true
		}
	}
	return false
}

func isNumberField(action *parse.ActionNode) bool {
	if action.Pipe == nil || len(action.Pipe.Decl) > 0 || len(action.Pipe.Cmds) != 1 {
		return false
	}
	command := action.Pipe.Cmds[0]
	if len(command.Args) != 1 {
		return false
	}
	field, ok := command.Args[0].(*parse.FieldNode)
	return ok && len(field.Ident) == 1 && field.Ident[0] == "Number"
}
