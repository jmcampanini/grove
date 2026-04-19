package cmd

import "github.com/spf13/cobra"

var workspaceTopicCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Workspace layout and requirements",
	Long: `A workspace is a directory whose subdirectories are git repository roots
(the main repo and its worktrees). Grove operates on this layout.

Example layout:

  ~/Code/org-name/repo-name/    workspace root (matches the remote URL)
  ├── main/                     the only full git repository
  ├── wt-add-auth/              worktree for a feature branch
  ├── wt-fix-bug-123/           worktree for a bug-fix branch
  └── pr-456/                   worktree checked out from a pull request

Requirements:

  1. The workspace directory name must match the tail of the git remote
     origin URL (e.g., "repo-name" for "git@github.com:org-name/repo-name").
  2. You must be at or below the workspace root for grove to detect it.
  3. The workspace only contains git repository or worktree roots as
     subdirectories.`,
}

func init() {
	rootCmd.AddCommand(workspaceTopicCmd)
}
