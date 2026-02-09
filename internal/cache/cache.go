package cache

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	clog "github.com/charmbracelet/log"
)

type envelope struct {
	CreatedAt time.Time `json:"created_at"`
	Key       string    `json:"key"`
	Payload   string    `json:"payload"`
}

// Cache is a file-based cache with a TTL.
// A nil *Cache is safe to use — Get always misses and Set is a no-op.
type Cache struct {
	dir string
	log *clog.Logger
	ttl time.Duration
}

// New creates a cache that stores entries in dir with the given TTL.
// A zero TTL disables caching (Get always misses, Set is a no-op).
func New(dir string, ttl time.Duration) *Cache {
	return &Cache{
		dir: dir,
		log: clog.Default().WithPrefix("cache"),
		ttl: ttl,
	}
}

// DefaultDir returns the default cache directory: os.UserCacheDir()/grove/v1.
func DefaultDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user cache dir: %w", err)
	}
	return filepath.Join(base, "grove", "v1"), nil
}

// BuildKey produces a deterministic hex key from a working directory and CLI args.
func BuildKey(workingDir string, args []string) string {
	parts := append([]string{workingDir}, args...)
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", h[:16])
}

// Get returns the cached payload for key, or ("", false) on miss.
func (c *Cache) Get(key string) (string, bool) {
	if c == nil || c.ttl == 0 {
		return "", false
	}

	path := c.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		c.log.Debug("cache: corrupt entry", "key", key, "error", err)
		return "", false
	}

	if env.Key != key {
		c.log.Debug("cache: key mismatch", "want", key, "got", env.Key)
		return "", false
	}

	if time.Since(env.CreatedAt) > c.ttl {
		c.log.Debug("cache: expired", "key", key, "age", time.Since(env.CreatedAt))
		return "", false
	}

	c.log.Debug("cache: hit", "key", key)
	return env.Payload, true
}

// Set stores payload under key. It is a no-op on a nil cache or zero TTL.
func (c *Cache) Set(key string, payload string) {
	if c == nil || c.ttl == 0 {
		return
	}

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		c.log.Debug("cache: mkdir failed", "error", err)
		return
	}

	env := envelope{
		CreatedAt: time.Now(),
		Key:       key,
		Payload:   payload,
	}
	data, err := json.Marshal(env)
	if err != nil {
		c.log.Debug("cache: marshal failed", "error", err)
		return
	}

	tmp, err := os.CreateTemp(c.dir, ".tmp-*")
	if err != nil {
		c.log.Debug("cache: temp file failed", "error", err)
		return
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		c.log.Debug("cache: write failed", "error", err)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		c.log.Debug("cache: close failed", "error", err)
		return
	}

	if err := os.Rename(tmpName, c.path(key)); err != nil {
		_ = os.Remove(tmpName)
		c.log.Debug("cache: rename failed", "error", err)
	}
}

// Clear removes all files in the cache directory.
func (c *Cache) Clear() error {
	if c == nil {
		return nil
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read cache dir: %w", err)
	}
	var errs []error
	for _, e := range entries {
		if err := os.Remove(filepath.Join(c.dir, e.Name())); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (c *Cache) path(key string) string {
	return filepath.Join(c.dir, key+".json")
}
