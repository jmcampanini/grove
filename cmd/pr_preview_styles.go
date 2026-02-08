package cmd

import (
	"io"

	"github.com/jmcampanini/grove-cli/internal/github"
)

func renderCard(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}

func renderDashboard(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}

func renderMinimal(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}

func renderContext(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}

func renderBoard(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}

func renderTimeline(w io.Writer, pr github.PullRequest, files []github.PullRequestFile, width int) error {
	return outputPRPreview(w, pr, files)
}
