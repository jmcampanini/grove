package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/jmcampanini/grove-cli/internal/pr"
	"github.com/spf13/cobra"
)

var prCreateCmd = &cobra.Command{
	Use:   "create [number]",
	Short: "Create worktree from pull request",
	Long: `Create a local worktree from a GitHub pull request.

Note: Only works with PRs from the same repository. Fork PRs are not yet supported.`,
	Args: cobra.ExactArgs(1),
	RunE: runPRCreate,
}

func init() {
	prCmd.AddCommand(prCreateCmd)
}

func runPRCreate(cmd *cobra.Command, args []string) error {
	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid PR number: %s", args[0])
	}

	ctx, err := initPRCreateContextFromEnv()
	if err != nil {
		return err
	}

	if err := ctx.ghClient.Validate(); err != nil {
		return err
	}

	prInfo, err := ctx.ghClient.GetPullRequest(prNum)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}

	if prInfo.IsCrossRepository {
		return fmt.Errorf("PR #%d is from a fork, which is not yet supported.\nTip: You can manually add the fork as a remote and create a worktree with 'git worktree add'", prInfo.Number)
	}

	if prInfo.State == github.PRStateMerged || prInfo.State == github.PRStateClosed {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Note: PR #%d is %s\n", prInfo.Number, strings.ToLower(string(prInfo.State)))
	}

	return createPRWorktree(cmd.OutOrStdout(), cmd.ErrOrStderr(), ctx, prInfo)
}

type prCreateContext struct {
	cfg       config.Config
	ghClient  github.GitHub
	gitClient git.Git
}

func initPRCreateContextFromEnv() (*prCreateContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return nil, fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return nil, fmt.Errorf("grove must be run inside a git repository")
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main worktree path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loader := config.NewDefaultLoader()
	loadResult, err := loader.Load(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &prCreateContext{
		cfg:       loadResult.Config,
		ghClient:  github.New(cwd, loadResult.Config.Git.Timeout),
		gitClient: git.New(false, cwd, loadResult.Config.Git.Timeout),
	}, nil
}

func createPRWorktree(stdout, stderr io.Writer, ctx *prCreateContext, prInfo github.PullRequest) error {
	namer, err := naming.NewPRWorktreeNamer(ctx.cfg.PR, ctx.cfg.Slugify)
	if err != nil {
		return fmt.Errorf("failed to create PR namer: %w", err)
	}

	prData := naming.PRTemplateData{
		BranchName: prInfo.BranchName,
		Number:     prInfo.Number,
	}
	localBranch, err := namer.GenerateBranchName(prData)
	if err != nil {
		return fmt.Errorf("failed to generate branch name: %w", err)
	}

	worktreeName := namer.GenerateWorktreeName(localBranch)
	if worktreeName == "" {
		return fmt.Errorf("failed to generate worktree name: empty result")
	}

	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := pr.NewMatcher(namer)
	existingWorktree := matcher.FindWorktreeForPR(prInfo, worktrees)

	if existingWorktree != nil {
		_, _ = fmt.Fprintf(stderr, "Worktree already exists\n")
		_, err := fmt.Fprintln(stdout, existingWorktree.AbsolutePath)
		return err
	}

	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	wtPath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree path %s already exists (not a PR worktree or different branch)", wtPath)
	}

	branchExists, err := ctx.gitClient.BranchExists(localBranch, false)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}

	if !branchExists {
		remote, err := ctx.gitClient.GetDefaultRemote("origin")
		if err != nil {
			return fmt.Errorf("failed to determine remote: %w", err)
		}
		if err := ctx.gitClient.FetchRemoteBranch(remote, prInfo.BranchName, localBranch); err != nil {
			return fmt.Errorf("failed to fetch remote branch: %w", err)
		}
	}

	if err := ctx.gitClient.CreateWorktreeForExistingBranch(localBranch, wtPath); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	_, err = fmt.Fprintln(stdout, wtPath)
	return err
}
