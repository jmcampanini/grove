package naming

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"
	"unicode"
)

// ErrEmptySlug indicates that an input contained no ASCII alphanumeric characters.
var ErrEmptySlug = errors.New("empty slug")

type nameValidator func(string) (bool, string)

func parseNameTemplate(field, source string, testData any, maxLength int, validate nameValidator) (*template.Template, error) {
	tmpl, err := template.New(field).Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", field, err)
	}
	if err := validateTemplateFields(tmpl, testData); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", field, err)
	}
	if _, err := renderName(tmpl, testData, maxLength, validate); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", field, err)
	}
	return tmpl, nil
}

func validateTemplateFields(tmpl *template.Template, testData any) error {
	allowed, names, err := directTemplateFields(testData)
	if err != nil {
		return err
	}

	templates := tmpl.Templates()
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name() < templates[j].Name()
	})
	for _, associated := range templates {
		if associated.Tree == nil || associated.Root == nil {
			continue
		}
		if err := validateTemplateNode(associated.Root, associated.Name(), allowed, names); err != nil {
			return err
		}
	}
	return nil
}

func directTemplateFields(data any) (map[string]struct{}, []string, error) {
	dataType := reflect.TypeOf(data)
	for dataType != nil && dataType.Kind() == reflect.Pointer {
		dataType = dataType.Elem()
	}
	if dataType == nil || dataType.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("template test data must be a struct, got %T", data)
	}

	allowed := make(map[string]struct{}, dataType.NumField())
	names := make([]string, 0, dataType.NumField())
	for i := 0; i < dataType.NumField(); i++ {
		field := dataType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		allowed[field.Name] = struct{}{}
		names = append(names, field.Name)
	}
	sort.Strings(names)
	return allowed, names, nil
}

func validateTemplateNode(node parse.Node, templateName string, allowed map[string]struct{}, names []string) error {
	if isNilParseNode(node) {
		return nil
	}

	switch node := node.(type) {
	case *parse.ListNode:
		return validateTemplateNodes(node.Nodes, templateName, allowed, names)
	case *parse.ActionNode:
		return validateTemplateNode(node.Pipe, templateName, allowed, names)
	case *parse.IfNode:
		return validateTemplateBranch(&node.BranchNode, templateName, allowed, names)
	case *parse.RangeNode:
		return validateTemplateBranch(&node.BranchNode, templateName, allowed, names)
	case *parse.WithNode:
		return validateTemplateBranch(&node.BranchNode, templateName, allowed, names)
	case *parse.TemplateNode:
		return validateTemplateNode(node.Pipe, templateName, allowed, names)
	case *parse.PipeNode:
		return validateTemplatePipe(node, templateName, allowed, names)
	case *parse.CommandNode:
		return validateTemplateNodes(node.Args, templateName, allowed, names)
	case *parse.FieldNode:
		return validateTemplateField(node, templateName, allowed, names)
	case *parse.ChainNode:
		return validateTemplateChain(node, templateName, allowed, names)
	case *parse.VariableNode:
		return validateTemplateVariable(node, templateName, allowed, names)
	}
	return nil
}

func isNilParseNode(node parse.Node) bool {
	if node == nil {
		return true
	}
	return reflect.ValueOf(node).IsNil()
}

func validateTemplateNodes(nodes []parse.Node, templateName string, allowed map[string]struct{}, names []string) error {
	for _, node := range nodes {
		if err := validateTemplateNode(node, templateName, allowed, names); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplatePipe(pipe *parse.PipeNode, templateName string, allowed map[string]struct{}, names []string) error {
	for _, declaration := range pipe.Decl {
		if err := validateTemplateNode(declaration, templateName, allowed, names); err != nil {
			return err
		}
	}
	for _, command := range pipe.Cmds {
		if err := validateTemplateNode(command, templateName, allowed, names); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplateField(field *parse.FieldNode, templateName string, allowed map[string]struct{}, names []string) error {
	if len(field.Ident) == 0 {
		return nestedFieldError(templateName, field.String())
	}
	if _, ok := allowed[field.Ident[0]]; !ok {
		return unavailableFieldError(templateName, "."+field.Ident[0], names)
	}
	if len(field.Ident) > 1 {
		return nestedFieldError(templateName, field.String())
	}
	return nil
}

func validateTemplateChain(chain *parse.ChainNode, templateName string, allowed map[string]struct{}, names []string) error {
	if err := validateTemplateNode(chain.Node, templateName, allowed, names); err != nil {
		return err
	}
	if len(chain.Field) > 0 {
		return nestedFieldError(templateName, chain.String())
	}
	return nil
}

func validateTemplateVariable(variable *parse.VariableNode, templateName string, allowed map[string]struct{}, names []string) error {
	if len(variable.Ident) <= 1 {
		return nil
	}
	if variable.Ident[0] != "$" {
		return nestedFieldError(templateName, variable.String())
	}
	if _, ok := allowed[variable.Ident[1]]; !ok {
		return unavailableFieldError(templateName, "."+variable.Ident[1], names)
	}
	if len(variable.Ident) > 2 {
		return nestedFieldError(templateName, variable.String())
	}
	return nil
}

func unavailableFieldError(templateName, field string, names []string) error {
	return fmt.Errorf("template %q uses unavailable field %s; available fields: %s", templateName, field, strings.Join(names, ", "))
}

func nestedFieldError(templateName, field string) error {
	return fmt.Errorf("template %q uses nested field access %s; template fields are scalar", templateName, field)
}

func validateTemplateBranch(branch *parse.BranchNode, templateName string, allowed map[string]struct{}, names []string) error {
	if err := validateTemplateNode(branch.Pipe, templateName, allowed, names); err != nil {
		return err
	}
	if err := validateTemplateNode(branch.List, templateName, allowed, names); err != nil {
		return err
	}
	return validateTemplateNode(branch.ElseList, templateName, allowed, names)
}

func renderName(tmpl *template.Template, data any, maxLength int, validate nameValidator) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	name := TruncateName(buf.String(), maxLength)
	if ok, reason := validate(name); !ok {
		return "", errors.New(reason)
	}
	return name, nil
}

// leadingLiteral returns all literal text before the template's first action.
func leadingLiteral(tmpl *template.Template) string {
	if tmpl == nil || tmpl.Tree == nil || tmpl.Root == nil {
		return ""
	}

	var literal strings.Builder
	for _, node := range tmpl.Root.Nodes {
		text, ok := node.(*parse.TextNode)
		if !ok {
			break
		}
		literal.Write(text.Text)
	}
	return literal.String()
}

// isValidBranchName validates git branch names with the intentionally simplified rules.
func isValidBranchName(name string) (bool, string) {
	if name == "" {
		return false, "empty"
	}
	if strings.HasPrefix(name, "-") {
		return false, "starts with '-'"
	}
	if strings.Contains(name, "..") {
		return false, "contains '..'"
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return false, "contains control character"
		}
	}
	return true, ""
}

func slugifyBranch(branch string, prefixes []string, opts SlugifyOptions) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(branch, prefix) {
			branch = strings.TrimPrefix(branch, prefix)
			break
		}
	}
	return Slugify(branch, opts)
}

func isValidWorktreeName(name string) (bool, string) {
	if name == "" {
		return false, "worktree name is empty"
	}
	if strings.ContainsRune(name, '/') {
		return false, "worktree name contains '/'"
	}
	if strings.HasPrefix(name, "-") {
		return false, "worktree name starts with '-'"
	}
	if name == "." || name == ".." {
		return false, fmt.Sprintf("worktree name is %q", name)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false, "worktree name contains control character"
		}
	}
	return true, ""
}
