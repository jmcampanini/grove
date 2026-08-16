package naming

import (
	"regexp"
	"strings"

	"github.com/jmcampanini/grove/internal/config"
)

// SlugifyOptions configures Slugify.
type SlugifyOptions struct {
	Lowercase bool
}

func SlugifyOptionsFromConfig(cfg config.NamingConfig) SlugifyOptions {
	return SlugifyOptions{Lowercase: cfg.Lowercase}
}

var nonAlphaNumRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// Slugify converts runs outside ASCII alphanumeric characters to dashes.
func Slugify(input string, opts SlugifyOptions) string {
	result := nonAlphaNumRegex.ReplaceAllString(input, "-")
	result = strings.Trim(result, "-")
	if opts.Lowercase {
		result = strings.ToLower(result)
	}
	return result
}

// TruncateName caps a name by runes and removes dashes exposed at the cut.
func TruncateName(name string, maxLength int) string {
	if maxLength <= 0 {
		return name
	}

	runes := []rune(name)
	if len(runes) <= maxLength {
		return name
	}
	return strings.TrimRight(string(runes[:maxLength]), "-")
}
