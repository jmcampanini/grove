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

type templateFieldValidator struct {
	allowed      map[string]struct{}
	names        []string
	templateName string
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
		validator := templateFieldValidator{
			allowed:      allowed,
			names:        names,
			templateName: associated.Name(),
		}
		if err := validator.validate(associated.Root); err != nil {
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

func (v *templateFieldValidator) validate(node parse.Node) error {
	if isNilParseNode(node) {
		return nil
	}

	switch node := node.(type) {
	case *parse.ListNode:
		return v.validateNodes(node.Nodes)
	case *parse.ActionNode:
		return v.validate(node.Pipe)
	case *parse.IfNode:
		return v.validateBranch(&node.BranchNode)
	case *parse.RangeNode:
		return v.validateBranch(&node.BranchNode)
	case *parse.WithNode:
		return v.validateBranch(&node.BranchNode)
	case *parse.TemplateNode:
		return v.validate(node.Pipe)
	case *parse.PipeNode:
		return v.validatePipe(node)
	case *parse.CommandNode:
		return v.validateNodes(node.Args)
	case *parse.FieldNode:
		return v.validateField(node)
	case *parse.ChainNode:
		return v.validateChain(node)
	case *parse.VariableNode:
		return v.validateVariable(node)
	}
	return nil
}

func isNilParseNode(node parse.Node) bool {
	if node == nil {
		return true
	}
	return reflect.ValueOf(node).IsNil()
}

func (v *templateFieldValidator) validateNodes(nodes []parse.Node) error {
	for _, node := range nodes {
		if err := v.validate(node); err != nil {
			return err
		}
	}
	return nil
}

func (v *templateFieldValidator) validatePipe(pipe *parse.PipeNode) error {
	for _, declaration := range pipe.Decl {
		if err := v.validate(declaration); err != nil {
			return err
		}
	}
	for _, command := range pipe.Cmds {
		if err := v.validate(command); err != nil {
			return err
		}
	}
	return nil
}

func (v *templateFieldValidator) validateField(field *parse.FieldNode) error {
	if len(field.Ident) == 0 {
		return nestedFieldError(v.templateName, field.String())
	}
	if _, ok := v.allowed[field.Ident[0]]; !ok {
		return unavailableFieldError(v.templateName, "."+field.Ident[0], v.names)
	}
	if len(field.Ident) > 1 {
		return nestedFieldError(v.templateName, field.String())
	}
	return nil
}

func (v *templateFieldValidator) validateChain(chain *parse.ChainNode) error {
	if err := v.validate(chain.Node); err != nil {
		return err
	}
	if len(chain.Field) > 0 {
		return nestedFieldError(v.templateName, chain.String())
	}
	return nil
}

func (v *templateFieldValidator) validateVariable(variable *parse.VariableNode) error {
	if len(variable.Ident) <= 1 {
		return nil
	}
	if variable.Ident[0] != "$" {
		return nestedFieldError(v.templateName, variable.String())
	}
	if _, ok := v.allowed[variable.Ident[1]]; !ok {
		return unavailableFieldError(v.templateName, "."+variable.Ident[1], v.names)
	}
	if len(variable.Ident) > 2 {
		return nestedFieldError(v.templateName, variable.String())
	}
	return nil
}

func unavailableFieldError(templateName, field string, names []string) error {
	return fmt.Errorf("template %q uses unavailable field %s; available fields: %s", templateName, field, strings.Join(names, ", "))
}

func nestedFieldError(templateName, field string) error {
	return fmt.Errorf("template %q uses nested field access %s; template fields are scalar", templateName, field)
}

func (v *templateFieldValidator) validateBranch(branch *parse.BranchNode) error {
	if err := v.validate(branch.Pipe); err != nil {
		return err
	}
	if err := v.validate(branch.List); err != nil {
		return err
	}
	return v.validate(branch.ElseList)
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
