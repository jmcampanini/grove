package config

import (
	"fmt"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

// RegisterFlags registers CLI flags for config fields tagged `config:"..."`.
// Values set via these flags override file config in LoadFilesWithFlags.
func RegisterFlags(flags *pflag.FlagSet) error {
	return pflagloader.Register[Config](flags)
}

// LoadFiles loads and merges TOML config files in low-to-high priority order.
// Missing files and directory paths are ignored by go-config-loader. The
// returned report is the library's native provenance/load report.
func LoadFiles(paths []string) (Config, configloader.LoadReport, error) {
	return LoadFilesWithFlags(paths, nil)
}

// LoadFilesWithFlags loads files like LoadFiles, then overlays values from
// CLI flags that were explicitly set (flags > files > defaults). The flag set
// must have been through RegisterFlags; nil skips the flag layer.
func LoadFilesWithFlags(paths []string, flags *pflag.FlagSet) (Config, configloader.LoadReport, error) {
	fileLoader, err := configloader.NewMergeAllFilesLoader[Config](paths)
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}

	loaders := []configloader.ConfigLoader[Config]{fileLoader}
	if flags != nil {
		flagLoader, err := pflagloader.NewLoader[Config](flags)
		if err != nil {
			return Config{}, configloader.LoadReport{}, err
		}
		loaders = append(loaders, flagLoader)
	}

	cfg, report, err := configloader.Load(DefaultConfig(), loaders...)
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, configloader.LoadReport{}, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, report, nil
}
