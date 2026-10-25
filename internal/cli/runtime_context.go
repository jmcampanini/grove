package cli

import (
	"fmt"
	"os"

	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/repository"
	"github.com/spf13/cobra"
)

func loadCommandRuntime(cmd *cobra.Command) (*repository.Runtime, error) {
	logger := commandLogger(cmd)
	originalCwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	loaded, err := loadConfigAt(cmd, logger, originalCwd, requireRuntime)
	if err != nil {
		return nil, err
	}

	ctx := cmd.Context()
	return &repository.Runtime{
		Config:           loaded.cfg,
		ConfigReport:     loaded.report,
		Context:          ctx,
		Directory:        loaded.gitDir,
		GitClient:        git.New(ctx, false, loaded.gitDir, loaded.cfg.Git.Timeout, logger),
		Logger:           logger,
		MainWorktreePath: loaded.mainWorktreePath,
	}, nil
}
