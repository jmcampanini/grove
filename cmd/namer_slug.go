package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

func newNamerSlugCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "slug <phrase>",
		Short: "Slugify a phrase using the configured slug settings",
		Args:  cobra.ExactArgs(1),
		RunE:  runNamerSlug,
	}
}

func runNamerSlug(cmd *cobra.Command, args []string) error {
	cfg, err := loadNamingConfig(cmd)
	if err != nil {
		return err
	}

	return executeNamerSlug(cmd.OutOrStdout(), naming.SlugifyOptionsFromConfig(cfg.Slugify), args[0])
}

func executeNamerSlug(w io.Writer, opts naming.SlugifyOptions, phrase string) error {
	if strings.TrimSpace(phrase) == "" {
		return errors.New("phrase cannot be empty")
	}

	result := naming.Slugify(phrase, opts)
	if result == "" {
		return fmt.Errorf("phrase %q produces an empty result after slugification", phrase)
	}

	if _, err := fmt.Fprintln(w, result); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
