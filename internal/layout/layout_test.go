package layout

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jmcampanini/grove/internal/config"
)

func envOf(values map[string]string) LookupEnv {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func worktreeConfig(root, layout, space string) config.WorktreeConfig {
	return config.WorktreeConfig{Layout: layout, Root: root, Space: space}
}

var acme = Remote{Host: "github.com", Owner: "acme", Repo: "app"}

func TestNew_DefaultConfigRendersFullPath(t *testing.T) {
	cfg := config.DefaultConfig().Worktree
	resolver, err := New(cfg, envOf(map[string]string{"CODE_DIR": "/home/dev/Code"}), "/home/dev")
	require.NoError(t, err)

	assert.Equal(t, "/home/dev/Code/.worktrees", resolver.Root())
	assert.True(t, resolver.NeedsRemote())

	path, err := resolver.Path(acme, cfg.Space, "wt-add-auth")
	require.NoError(t, err)
	assert.Equal(t, "/home/dev/Code/.worktrees/grove/github.com/acme/app/wt-add-auth", path)
}

func TestNew_RootExpansion(t *testing.T) {
	tests := []struct {
		env      map[string]string
		homeDir  string
		name     string
		root     string
		wantErr  string
		wantRoot string
	}{
		{name: "dollar var", root: "$CODE_DIR/.worktrees", env: map[string]string{"CODE_DIR": "/code"}, wantRoot: "/code/.worktrees"},
		{name: "braced var", root: "${CODE_DIR}/.worktrees", env: map[string]string{"CODE_DIR": "/code"}, wantRoot: "/code/.worktrees"},
		{name: "tilde", root: "~/src/.worktrees", homeDir: "/home/dev", wantRoot: "/home/dev/src/.worktrees"},
		{name: "bare tilde", root: "~", homeDir: "/home/dev", wantRoot: "/home/dev"},
		{name: "tilde not expanded mid-path", root: "/srv/~/x", wantRoot: "/srv/~/x"},
		{name: "plain absolute", root: "/srv/worktrees/", wantRoot: "/srv/worktrees"},
		{name: "cleaned", root: "/srv//worktrees/./x", wantRoot: "/srv/worktrees/x"},
		{name: "unset variable", root: "$CODE_DIR/.worktrees", env: map[string]string{}, wantErr: "references unset environment variable CODE_DIR"},
		{name: "empty variable", root: "$CODE_DIR/.worktrees", env: map[string]string{"CODE_DIR": ""}, wantErr: "references unset environment variable CODE_DIR"},
		{name: "several unset variables listed once each", root: "$A/$B/$A", env: map[string]string{}, wantErr: "unset environment variable A, B;"},
		{name: "unset variable is not silently empty", root: "$CODE_DIR", env: map[string]string{}, wantErr: "unset environment variable"},
		{name: "relative after expansion", root: "$CODE_DIR/.worktrees", env: map[string]string{"CODE_DIR": "Code"}, wantErr: "is not an absolute path"},
		{name: "relative literal", root: "worktrees", wantErr: "is not an absolute path"},
		{name: "tilde without home", root: "~/x", homeDir: "", wantErr: "home directory is unknown"},
		{name: "dollar without name is literal", root: "/srv/$/x", wantRoot: "/srv/$/x"},
		{name: "unterminated brace is literal", root: "/srv/${X/x", wantRoot: "/srv/${X/x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := New(worktreeConfig(tt.root, "{{.Name}}", "grove"), envOf(tt.env), tt.homeDir)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Contains(t, err.Error(), "invalid worktree.root")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRoot, resolver.Root())
		})
	}
}

func TestNew_LayoutValidation(t *testing.T) {
	tests := []struct {
		layout  string
		name    string
		wantErr string
	}{
		{name: "unknown field", layout: "{{.Space}}/{{.Branch}}", wantErr: "unavailable field .Branch"},
		{name: "nested field", layout: "{{.Space.X}}", wantErr: "nested field access"},
		{name: "parse error", layout: "{{.Space", wantErr: "invalid worktree.layout"},
		{name: "empty render", layout: "{{if false}}x{{end}}", wantErr: "rendered an empty path"},
		{name: "absolute render", layout: "/{{.Name}}", wantErr: "rendered an absolute path"},
		{name: "escapes root", layout: "../{{.Name}}", wantErr: "escapes worktree.root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(worktreeConfig("/root", tt.layout, "grove"), envOf(nil), "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNew_NeedsRemote(t *testing.T) {
	tests := []struct {
		layout string
		want   bool
	}{
		{layout: "{{.Space}}/{{.Name}}", want: false},
		{layout: "{{.Name}}", want: false},
		{layout: "{{.Host}}/{{.Name}}", want: true},
		{layout: "{{.Owner}}/{{.Name}}", want: true},
		{layout: "{{.Repo}}-{{.Name}}", want: true},
		{layout: "{{if .Host}}{{.Host}}/{{end}}{{.Name}}", want: true},
		{layout: "{{$.Repo}}/{{.Name}}", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.layout, func(t *testing.T) {
			resolver, err := New(worktreeConfig("/root", tt.layout, "grove"), envOf(nil), "")
			require.NoError(t, err)
			assert.Equal(t, tt.want, resolver.NeedsRemote())
		})
	}
}

func TestResolver_Path(t *testing.T) {
	tests := []struct {
		layout  string
		name    string
		remote  Remote
		space   string
		wantErr string
		want    string
	}{
		{name: "default layout with nested owner", layout: "{{.Space}}/{{.Host}}/{{.Owner}}/{{.Repo}}/{{.Name}}", remote: Remote{Host: "gitlab.com", Owner: "group/sub", Repo: "app"}, space: "claude", want: "/root/claude/gitlab.com/group/sub/app/wt-x"},
		{name: "flat layout ignores remote", layout: "{{.Space}}/{{.Name}}", space: "grove", want: "/root/grove/wt-x"},
		{name: "layout without space", layout: "{{.Repo}}/{{.Name}}", remote: acme, space: "grove", want: "/root/app/wt-x"},
		{name: "empty segments are collapsed", layout: "{{.Space}}//{{.Name}}", space: "grove", want: "/root/grove/wt-x"},
		{name: "remote required but missing", layout: "{{.Host}}/{{.Name}}", space: "grove", wantErr: "no remote is available"},
		{name: "invalid space", layout: "{{.Space}}/{{.Name}}", space: "a/b", wantErr: `invalid space "a/b": cannot contain '/'`},
		{name: "empty space", layout: "{{.Name}}", space: "", wantErr: "invalid space"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := New(worktreeConfig("/root", tt.layout, "grove"), envOf(nil), "")
			require.NoError(t, err)

			got, err := resolver.Path(tt.remote, tt.space, "wt-x")
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filepath.FromSlash(tt.want), got)
		})
	}
}

func TestResolver_PathRejectsEmptyName(t *testing.T) {
	resolver, err := New(worktreeConfig("/root", "{{.Name}}", "grove"), envOf(nil), "")
	require.NoError(t, err)

	_, err = resolver.Path(Remote{}, "grove", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree name is empty")
}
