package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRPreviewHelpDocumentsClickableMarkdownLinks(t *testing.T) {
	cmd := newPRPreviewCmd(commandServices{loadRuntime: loadCommandRuntime})
	for _, want := range []string{
		"Markdown links in the PR body",
		"clickable terminal hyperlinks",
		"Only open links from PRs/authors you trust",
	} {
		assert.Contains(t, cmd.Long, want)
	}
}

func TestRenderPreviewHandlesAdversarialMarkdown(t *testing.T) {
	body, err := os.ReadFile("testdata/adversarial-pr-markdown.md")
	require.NoError(t, err)

	pr := github.PullRequest{
		AuthorLogin: "untrusted-contributor",
		BaseRefName: "main",
		Body:        string(body),
		BranchName:  "adversarial-markdown",
		Number:      83,
		State:       github.PRStateOpen,
		Title:       "Adversarial markdown",
		URL:         "https://github.com/owner/repo/pull/83",
	}

	theme, err := render.ResolvePreviewTheme("never", strings.NewReader(""), io.Discard)
	require.NoError(t, err)
	renderer := render.New(testLogger(), theme)

	var buf bytes.Buffer
	require.NotPanics(t, func() {
		err = renderer.PullRequest(&buf, pr, nil, nil, 80)
	})
	require.NoError(t, err)

	output := buf.String()
	for _, want := range []string{
		"Untrusted pull request",
		"Named entity: click /javascript&colon;alert(1)",
		"Numeric entity: click /&#106;avascript:alert(1)",
		"Image destination: Image: pixel → /javascript&colon;alert(1)",
		"https://example.com/docs?q=1&safe=true",
		"Rendering must continue after the adversarial destinations.",
	} {
		assert.Contains(t, output, want)
	}
	assert.NotContains(t, output, "\x1b")
	assert.LessOrEqual(t, len(output), 4096)
}
