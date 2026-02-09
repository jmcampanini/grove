package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name       string
		workingDir string
		args       []string
	}{
		{
			name:       "deterministic across calls",
			workingDir: "/some/path",
			args:       []string{"pr", "view", "11"},
		},
		{
			name:       "empty args",
			workingDir: "/some/path",
			args:       []string{},
		},
		{
			name:       "single arg",
			workingDir: "/some/path",
			args:       []string{"status"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k1 := BuildKey(tt.workingDir, tt.args)
			k2 := BuildKey(tt.workingDir, tt.args)
			assert.Equal(t, k1, k2, "same inputs must produce same key")
			assert.Len(t, k1, 32, "key should be 32 hex chars")
		})
	}

	t.Run("different inputs produce different keys", func(t *testing.T) {
		k1 := BuildKey("/a", []string{"x"})
		k2 := BuildKey("/b", []string{"x"})
		assert.NotEqual(t, k1, k2)
	})
}

func TestCache_GetSet(t *testing.T) {
	tests := []struct {
		name    string
		ttl     time.Duration
		setup   func(t *testing.T, dir string)
		key     string
		wantVal string
		wantHit bool
	}{
		{
			name:    "hit within TTL",
			ttl:     5 * time.Minute,
			key:     "test-key",
			wantVal: "hello world",
			wantHit: true,
			setup: func(t *testing.T, dir string) {
				writeEnvelope(t, dir, "test-key", "hello world", time.Now())
			},
		},
		{
			name:    "miss after TTL",
			ttl:     1 * time.Millisecond,
			key:     "test-key",
			wantVal: "",
			wantHit: false,
			setup: func(t *testing.T, dir string) {
				writeEnvelope(t, dir, "test-key", "stale", time.Now().Add(-1*time.Second))
			},
		},
		{
			name:    "miss on empty cache",
			ttl:     5 * time.Minute,
			key:     "nonexistent",
			wantVal: "",
			wantHit: false,
			setup:   func(t *testing.T, dir string) {},
		},
		{
			name:    "zero TTL disables caching",
			ttl:     0,
			key:     "test-key",
			wantVal: "",
			wantHit: false,
			setup: func(t *testing.T, dir string) {
				writeEnvelope(t, dir, "test-key", "should not be found", time.Now())
			},
		},
		{
			name:    "corrupt file is a miss",
			ttl:     5 * time.Minute,
			key:     "test-key",
			wantVal: "",
			wantHit: false,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "test-key.json"),
					[]byte("not json"),
					0o644,
				))
			},
		},
		{
			name:    "key mismatch is a miss",
			ttl:     5 * time.Minute,
			key:     "wanted-key",
			wantVal: "",
			wantHit: false,
			setup: func(t *testing.T, dir string) {
				writeEnvelope(t, dir, "other-key", "data", time.Now())
				require.NoError(t, os.Rename(
					filepath.Join(dir, "other-key.json"),
					filepath.Join(dir, "wanted-key.json"),
				))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)
			c := New(dir, tt.ttl)

			val, hit := c.Get(tt.key)
			assert.Equal(t, tt.wantHit, hit)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

func TestCache_Set_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	c := New(dir, 5*time.Minute)

	c.Set("k", "v")

	val, hit := c.Get("k")
	assert.True(t, hit)
	assert.Equal(t, "v", val)
}

func TestCache_Set_ZeroTTL_IsNoop(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 0)

	c.Set("k", "v")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "zero TTL should not write any files")
}

func TestCache_Clear(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 5*time.Minute)

	c.Set("a", "1")
	c.Set("b", "2")

	require.NoError(t, c.Clear())

	_, hit := c.Get("a")
	assert.False(t, hit, "cleared cache should miss")
}

func TestCache_Clear_EmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	c := New(dir, 5*time.Minute)

	assert.NoError(t, c.Clear())
}

func TestCache_Nil_Safe(t *testing.T) {
	var c *Cache
	val, hit := c.Get("k")
	assert.False(t, hit)
	assert.Equal(t, "", val)

	c.Set("k", "v") // should not panic

	assert.NoError(t, c.Clear())
}

func TestDefaultDir(t *testing.T) {
	dir, err := DefaultDir()
	require.NoError(t, err)
	assert.Contains(t, dir, "grove")
	assert.Contains(t, dir, "v1")
}

func writeEnvelope(t *testing.T, dir, key, payload string, createdAt time.Time) {
	t.Helper()
	env := envelope{
		CreatedAt: createdAt,
		Key:       key,
		Payload:   payload,
	}
	data, err := json.Marshal(env)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, key+".json"),
		data,
		0o644,
	))
}
