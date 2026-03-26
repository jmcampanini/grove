package naming

import (
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// SlugifyOptions configures the behavior of the Slugify function.
type SlugifyOptions struct {
	// MaxLength limits the output length. 0 means no limit.
	// When the result exceeds this length, it will be truncated and a hash suffix added.
	MaxLength int

	// HashLength is the length of the hash suffix used when truncating.
	// Only used when MaxLength > 0 and the result exceeds MaxLength.
	HashLength int

	// Lowercase converts the entire string to lowercase.
	Lowercase bool

	// ReplaceNonAlphaNum replaces all non-alphanumeric characters with dashes.
	ReplaceNonAlphaNum bool

	// CollapseDashes collapses consecutive dashes into a single dash.
	CollapseDashes bool

	// TrimDashes removes leading and trailing dashes from the result.
	TrimDashes bool
}

func SlugifyOptionsFromConfig(cfg config.SlugifyConfig) SlugifyOptions {
	return SlugifyOptions{
		CollapseDashes:     cfg.CollapseDashes,
		HashLength:         cfg.HashLength,
		Lowercase:          cfg.Lowercase,
		MaxLength:          cfg.MaxLength,
		ReplaceNonAlphaNum: cfg.ReplaceNonAlphanum,
		TrimDashes:         cfg.TrimDashes,
	}
}

var (
	nonAlphaNumRegex     = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	consecutiveDashRegex = regexp.MustCompile(`-{2,}`)
)

// Slugify transforms a string into a slug based on the provided options.
func Slugify(input string, opts SlugifyOptions) string {
	if input == "" {
		return ""
	}

	result := input

	if opts.Lowercase {
		result = strings.ToLower(result)
	}

	if opts.ReplaceNonAlphaNum {
		result = nonAlphaNumRegex.ReplaceAllString(result, "-")
	}

	if opts.CollapseDashes {
		result = consecutiveDashRegex.ReplaceAllString(result, "-")
	}

	if opts.TrimDashes {
		result = strings.Trim(result, "-")
	}

	if result == "" {
		return ""
	}

	if opts.MaxLength > 0 && len(result) > opts.MaxLength {
		hash := computeHash(input, opts.HashLength)
		result = truncateWithHash(result, hash, opts.MaxLength, opts.HashLength)
	}

	return result
}

// computeHash generates a base36 hash of the input string.
func computeHash(input string, length int) string {
	if length <= 0 {
		return ""
	}

	h := fnv.New64a()
	h.Write([]byte(input))
	hashValue := h.Sum64()

	const base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	var result strings.Builder
	result.Grow(length)

	for i := 0; i < length; i++ {
		result.WriteByte(base36Chars[hashValue%36])
		hashValue /= 36
	}

	return result.String()
}

// truncateWithHash truncates the result and appends a hash suffix.
func truncateWithHash(result, hash string, maxLength, hashLength int) string {
	prefixLength := maxLength - hashLength - 1 // include dash separator
	if prefixLength < 1 {
		if len(hash) > maxLength {
			return hash[:maxLength]
		}
		return hash
	}

	prefix := result
	if len(prefix) > prefixLength {
		prefix = prefix[:prefixLength]
	}

	prefix = strings.TrimRight(prefix, "-")
	if prefix == "" {
		if len(hash) > maxLength {
			return hash[:maxLength]
		}
		return hash
	}

	return prefix + "-" + hash
}
