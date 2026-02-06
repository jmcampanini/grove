package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print current configuration in TOML format",
	Long: `Print the current effective configuration in TOML format.

This outputs the merged configuration (defaults with any user overrides applied).
The output can be redirected to a file to create a new configuration:

  grove config > grove.toml`,
	Args: cobra.NoArgs,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return fmt.Errorf("grove must be run inside a git repository")
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return fmt.Errorf("failed to get main worktree path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loader := config.NewDefaultLoader()
	loadResult, err := loader.Load(configPaths)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(loadResult.Config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), buf.String())
	return err
}
