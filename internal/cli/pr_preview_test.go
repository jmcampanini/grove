package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
