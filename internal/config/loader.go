package config

import (
	"fmt"

	"github.com/jmcampanini/go-config-loader/configloader"
)

// LoadFiles loads and merges TOML config files in low-to-high priority order.
// Missing files and directory paths are ignored by go-config-loader. The
// returned report is the library's native provenance/load report.
func LoadFiles(paths []string) (Config, configloader.LoadReport, error) {
	fileLoader, err := configloader.NewMergeAllFilesLoader[Config](paths)
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}

	cfg, report, err := configloader.Load(DefaultConfig(), fileLoader)
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, configloader.LoadReport{}, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, report, nil
}
