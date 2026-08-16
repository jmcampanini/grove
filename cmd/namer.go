package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/spf13/cobra"
)

func newNamerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "namer",
		Short:   "Generate branch and worktree names from a phrase",
		GroupID: "utility",
	}

	cmd.AddCommand(
		newNamerBranchCmd(),
		newNamerSlugCmd(),
		newNamerWorktreeCmd(),
	)

	return cmd
}

type namerContext struct {
	namer *naming.LocalBranchNamer
}

func loadNamingConfig(cmd *cobra.Command) (config.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	cfg, _, err := loadReportingConfig(cmd, cwd)
	return cfg, err
}

func generateBranchName(ctx *namerContext, phrase string) (string, error) {
	if strings.TrimSpace(phrase) == "" {
		return "", errors.New("phrase cannot be empty")
	}

	name, err := ctx.namer.GenerateBranchName(phrase)
	if err != nil {
		return "", fmt.Errorf("failed to generate branch name for phrase %q: %w", phrase, err)
	}

	return name, nil
}
