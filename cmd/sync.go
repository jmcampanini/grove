package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/spf13/cobra"
)

var syncForceFlag bool

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync the current branch to match its remote tracking branch",
	Long: `Sync fetches from the remote and hard-resets the current branch to match
its remote tracking branch exactly. This is the worktree equivalent of:

  git fetch --prune && git reset --hard origin/<branch>

If the worktree has uncommitted changes, you will be prompted before
they are discarded. Use --force to skip the prompt.`,
	Args: cobra.NoArgs,
	RunE: runSync,
}

func init() {
	syncCmd.GroupID = "git"
	syncCmd.Flags().BoolVar(&syncForceFlag, "force", false, "Skip the dirty-state prompt and discard changes")
	rootCmd.AddCommand(syncCmd)
}

type syncContext struct {
	gitClient git.Git
}

func runSync(cmd *cobra.Command, _ []string) error {
	originalCwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	if rt.cwd != originalCwd {
		return errors.New("sync must be run from inside a worktree, not from the workspace root")
	}

	ctx := &syncContext{
		gitClient: rt.gitClient,
	}

	return executeSync(cmd.OutOrStdout(), ctx, syncForceFlag)
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

	if err := checkDirtyAndConfirm(w, ctx.gitClient, force); err != nil {
		if errors.Is(err, errSyncAborted) {
			return nil
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

func checkDirtyAndConfirm(w io.Writer, gitClient git.Git, force bool) error {
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

	confirmed, err := promptSyncConfirm()
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

func promptSyncConfirm() (bool, error) {
	var confirmed bool
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sync will discard all uncommitted changes. Continue?").
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	).Run()
	if err != nil {
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
