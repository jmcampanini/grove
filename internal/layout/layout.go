// Package layout renders the absolute path of a new worktree from the
// worktree.root and worktree.layout configuration.
package layout

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/naming"
)

// TemplateData holds the values available to worktree.layout.
type TemplateData struct {
	Host  string // remote host, e.g. github.com
	Name  string // worktree directory name rendered by the naming templates
	Owner string // remote owner or nested group path, e.g. jmcampanini
	Repo  string // remote repository name without .git
	Space string // partition segment, e.g. grove or claude
}

var remoteFields = []string{"Host", "Owner", "Repo"}

// LookupEnv reports the value of an environment variable and whether it is set.
type LookupEnv func(string) (string, bool)

// Resolver renders worktree paths for one configuration.
type Resolver struct {
	layout      *template.Template
	needsRemote bool
	root        string
}

// New expands cfg.Root and parses cfg.Layout. Root expansion replaces a leading
// "~" with homeDir and $VAR or ${VAR} with the environment; a referenced
// variable that is unset or empty fails. The expanded root must be absolute.
// The layout must be a valid template over TemplateData fields and must
// reference Name.
func New(cfg config.WorktreeConfig, lookupEnv LookupEnv, homeDir string) (*Resolver, error) {
	root, err := expandRoot(cfg.Root, lookupEnv, homeDir)
	if err != nil {
		return nil, fmt.Errorf("invalid worktree.root: %w", err)
	}

	tmpl, err := template.New("worktree.layout").Option("missingkey=error").Parse(cfg.Layout)
	if err != nil {
		return nil, fmt.Errorf("invalid worktree.layout: %w", err)
	}
	if err := naming.ValidateTemplateFields(tmpl, TemplateData{}); err != nil {
		return nil, fmt.Errorf("invalid worktree.layout: %w", err)
	}
	// Without the name every worktree of a repository renders the same
	// path, so a bare-name lookup could select a worktree that does not
	// carry that name.
	if !usesAnyField(tmpl, []string{"Name"}) {
		return nil, errors.New("invalid worktree.layout: must use {{.Name}} so each worktree has its own path")
	}
	if _, err := renderLayout(tmpl, TemplateData{Host: "host", Name: "name", Owner: "owner", Repo: "repo", Space: "space"}); err != nil {
		return nil, fmt.Errorf("invalid worktree.layout: %w", err)
	}

	return &Resolver{
		layout:      tmpl,
		needsRemote: usesAnyField(tmpl, remoteFields),
		root:        root,
	}, nil
}

// Root returns the expanded absolute root directory.
func (r *Resolver) Root() string {
	return r.root
}

// NeedsRemote reports whether the layout references Host, Owner, or Repo, in
// which case Path requires a parsed remote.
func (r *Resolver) NeedsRemote() bool {
	return r.needsRemote
}

// Path renders the absolute path for a worktree. The remote is ignored when
// NeedsRemote is false.
func (r *Resolver) Path(remote Remote, space, name string) (string, error) {
	if err := config.ValidateSpace(space); err != nil {
		return "", fmt.Errorf("invalid space %q: %w", space, err)
	}
	if name == "" {
		return "", errors.New("worktree name is empty")
	}
	if r.needsRemote && (remote.Host == "" || remote.Owner == "" || remote.Repo == "") {
		return "", errors.New("worktree.layout uses {{.Host}}, {{.Owner}}, or {{.Repo}} but no remote is available")
	}

	rendered, err := renderLayout(r.layout, TemplateData{
		Host:  remote.Host,
		Name:  name,
		Owner: remote.Owner,
		Repo:  remote.Repo,
		Space: space,
	})
	if err != nil {
		return "", fmt.Errorf("worktree.layout: %w", err)
	}
	return filepath.Join(r.root, rendered), nil
}

func renderLayout(tmpl *template.Template, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	rendered := strings.Trim(buf.String(), "/")
	if rendered == "" {
		return "", errors.New("rendered an empty path")
	}
	if strings.HasPrefix(buf.String(), "/") {
		return "", fmt.Errorf("rendered an absolute path %q; the layout is joined under worktree.root", buf.String())
	}
	for _, segment := range strings.Split(rendered, "/") {
		if segment == ".." {
			return "", fmt.Errorf("rendered %q, which escapes worktree.root", buf.String())
		}
	}
	return filepath.Clean(rendered), nil
}

func expandRoot(root string, lookupEnv LookupEnv, homeDir string) (string, error) {
	if root == "" {
		return "", errors.New("cannot be empty")
	}

	if root == "~" || strings.HasPrefix(root, "~/") {
		if homeDir == "" {
			return "", errors.New("cannot expand ~: home directory is unknown")
		}
		root = homeDir + root[1:]
	}

	missing := map[string]bool{}
	expanded := expandVariables(root, func(name string) string {
		value, ok := lookupEnv(name)
		if !ok || value == "" {
			missing[name] = true
		}
		return value
	})
	if len(missing) > 0 {
		names := slices.Sorted(maps.Keys(missing))
		return "", fmt.Errorf("%q references unset environment variable %s; set it or configure worktree.root without it", root, strings.Join(names, ", "))
	}

	if !filepath.IsAbs(expanded) {
		return "", fmt.Errorf("%q expands to %q, which is not an absolute path", root, expanded)
	}
	return filepath.Clean(expanded), nil
}

// expandVariables replaces $NAME and ${NAME} using mapping. Names are ASCII
// letters, digits, and underscores. A "$" not followed by a name is kept.
func expandVariables(s string, mapping func(string) string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' || i+1 >= len(s) {
			out.WriteByte(s[i])
			continue
		}

		if s[i+1] == '{' {
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				out.WriteByte(s[i])
				continue
			}
			name := s[i+2 : i+2+end]
			if !isVariableName(name) {
				out.WriteByte(s[i])
				continue
			}
			out.WriteString(mapping(name))
			i += 2 + end
			continue
		}

		end := i + 1
		for end < len(s) && isVariableByte(s[end]) {
			end++
		}
		name := s[i+1 : end]
		if !isVariableName(name) {
			out.WriteByte(s[i])
			continue
		}
		out.WriteString(mapping(name))
		i = end - 1
	}
	return out.String()
}

func isVariableName(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isVariableByte(name[i]) {
			return false
		}
	}
	return true
}

func isVariableByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// usesAnyField reports whether any associated template references one of the
// named top-level fields, as .Field or $.Field.
func usesAnyField(tmpl *template.Template, names []string) bool {
	for _, associated := range tmpl.Templates() {
		if associated.Tree == nil || associated.Root == nil {
			continue
		}
		if nodeUsesField(associated.Root, names) {
			return true
		}
	}
	return false
}

func nodeUsesField(node parse.Node, names []string) bool {
	if node == nil || reflect.ValueOf(node).IsNil() {
		return false
	}
	switch node := node.(type) {
	case *parse.ListNode:
		return nodesUseField(node.Nodes, names)
	case *parse.ActionNode:
		return nodeUsesField(node.Pipe, names)
	case *parse.TemplateNode:
		return nodeUsesField(node.Pipe, names)
	case *parse.PipeNode:
		return pipeUsesField(node, names)
	case *parse.CommandNode:
		return nodesUseField(node.Args, names)
	case *parse.ChainNode:
		return nodeUsesField(node.Node, names)
	case *parse.FieldNode:
		return len(node.Ident) > 0 && slices.Contains(names, node.Ident[0])
	case *parse.VariableNode:
		return len(node.Ident) > 1 && node.Ident[0] == "$" && slices.Contains(names, node.Ident[1])
	}
	return controlNodeUsesField(node, names)
}

func controlNodeUsesField(node parse.Node, names []string) bool {
	switch node := node.(type) {
	case *parse.IfNode:
		return branchUsesField(&node.BranchNode, names)
	case *parse.RangeNode:
		return branchUsesField(&node.BranchNode, names)
	case *parse.WithNode:
		return branchUsesField(&node.BranchNode, names)
	}
	return false
}

func pipeUsesField(pipe *parse.PipeNode, names []string) bool {
	for _, decl := range pipe.Decl {
		if nodeUsesField(decl, names) {
			return true
		}
	}
	for _, cmd := range pipe.Cmds {
		if nodeUsesField(cmd, names) {
			return true
		}
	}
	return false
}

func nodesUseField(nodes []parse.Node, names []string) bool {
	for _, node := range nodes {
		if nodeUsesField(node, names) {
			return true
		}
	}
	return false
}

func branchUsesField(branch *parse.BranchNode, names []string) bool {
	return nodeUsesField(branch.Pipe, names) || nodeUsesField(branch.List, names) || nodeUsesField(branch.ElseList, names)
}
