package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"charm.land/huh/v2"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/spf13/cobra"
)

func newSyncCmd(services commandServices) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the current branch to match its remote tracking branch",
		Long: `Fetch from the remote and hard-reset the current branch to its remote
tracking branch, the worktree equivalent of:

  git fetch --prune && git reset --hard origin/<branch>

The reset discards every uncommitted change and every local commit the
tracking branch does not have. When the worktree has uncommitted changes,
the git status is printed and a confirmation prompt is shown; declining
prints "Sync aborted." on stdout and exits 0. The prompt reads stdin and
draws on stderr and needs a terminal on both (with TERM=dumb it becomes a
line-based prompt on stdin and stdout instead), so a script passes
--force, which discards the changes without asking. A branch without a
tracking branch is reported on stdout and left alone.

` + helpFailure,
		Args:    cobra.NoArgs,
		GroupID: "git",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return services.runSync(cmd, force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Discard uncommitted changes without prompting")
	return cmd
}

type syncContext struct {
	colorMode render.ColorMode
	gitClient git.Git
}

func (services commandServices) runSync(cmd *cobra.Command, force bool) error {
	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &syncContext{
		colorMode: colorMode(cmd),
		gitClient: rt.GitClient,
	}

	return executeSync(cmd.OutOrStdout(), ctx, force)
}

func executeSync(w io.Writer, ctx *syncContext, force bool) error {
	currentBranch, err := ctx.gitClient.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	if currentBranch == "HEAD" {
		return errors.New("cannot sync: detached HEAD state")
	}

	upstream, err := findUpstreamForBranch(ctx.gitClient, currentBranch)
	if err != nil {
		return err
	}
	if upstream == "" {
		_, err := fmt.Fprintf(w, "Branch %q has no remote tracking branch.\n", currentBranch)
		return err
	}

	remotes, err := ctx.gitClient.ListRemotes()
	if err != nil {
		return fmt.Errorf("failed to list remotes: %w", err)
	}

	remoteName, err := findRemoteForUpstream(upstream, remotes)
	if err != nil {
		return err
	}

	if _, err := ctx.gitClient.FetchRemote(remoteName); err != nil {
		return fmt.Errorf("failed to fetch remote %q: %w", remoteName, err)
	}

	exists, err := ctx.gitClient.RefExists(upstream)
	if err != nil {
		return fmt.Errorf("failed to verify ref %q: %w", upstream, err)
	}
	if !exists {
		remoteBranch := strings.TrimPrefix(upstream, remoteName+"/")
		return fmt.Errorf(
			"remote tracking branch %q does not exist on the remote (deleted after merge, or never pushed); push it with git push -u %s %s:%s or remove the worktree with grove prune",
			upstream, remoteName, currentBranch, remoteBranch,
		)
	}

	if err := checkDirtyAndConfirm(w, ctx, force); err != nil {
		if errors.Is(err, errSyncAborted) {
			_, err = fmt.Fprintln(w, "Sync aborted.")
			return err
		}
		return err
	}

	if err := ctx.gitClient.ResetHard(upstream); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	_, err = fmt.Fprintf(w, "Synced %s to %s\n", currentBranch, upstream)
	return err
}

// errSyncAborted is a sentinel indicating the user declined to proceed.
var errSyncAborted = errors.New("sync aborted")

func checkDirtyAndConfirm(w io.Writer, ctx *syncContext, force bool) error {
	gitClient := ctx.gitClient
	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return fmt.Errorf("failed to get worktree root: %w", err)
	}

	dirty, err := gitClient.IsWorktreeDirty(worktreeRoot)
	if err != nil {
		return fmt.Errorf("failed to check worktree state: %w", err)
	}

	if !dirty || force {
		return nil
	}

	status, err := gitClient.GetStatus(worktreeRoot)
	if err != nil {
		return fmt.Errorf("failed to get worktree status: %w", err)
	}
	if _, err := fmt.Fprintln(w, status); err != nil {
		return err
	}

	confirmed, err := promptSyncConfirm(ctx.colorMode)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errSyncAborted
		}
		return err
	}
	if !confirmed {
		return errSyncAborted
	}
	return nil
}

func findUpstreamForBranch(gitClient git.Git, branchName string) (string, error) {
	branches, err := gitClient.ListLocalBranches()
	if err != nil {
		return "", fmt.Errorf("failed to list local branches: %w", err)
	}

	for _, b := range branches {
		if b.Name == branchName {
			return b.UpstreamName, nil
		}
	}
	return "", nil
}

func promptSyncConfirm(mode render.ColorMode) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sync will discard all uncommitted changes. Continue?").
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	)
	if err := runPrompt(form, mode, "pass --force to discard the uncommitted changes, or run grove sync in a terminal"); err != nil {
		return false, err
	}
	return confirmed, nil
}

func findRemoteForUpstream(upstream string, remotes []string) (string, error) {
	sort.Slice(remotes, func(i, j int) bool {
		return len(remotes[i]) > len(remotes[j])
	})
	for _, remote := range remotes {
		if strings.HasPrefix(upstream, remote+"/") {
			return remote, nil
		}
	}
	return "", fmt.Errorf("could not determine remote from upstream %q", upstream)
}
