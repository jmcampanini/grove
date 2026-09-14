package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Remote
		wantErr string
	}{
		{name: "scp-like ssh", raw: "git@github.com:acme/app.git", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "scp-like without user or suffix", raw: "github.com:acme/app", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "ssh scheme", raw: "ssh://git@github.com/acme/app.git", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "ssh scheme with port", raw: "ssh://git@gitlab.example.com:2222/group/app.git", want: Remote{Host: "gitlab.example.com", Owner: "group", Repo: "app"}},
		{name: "https", raw: "https://github.com/acme/app.git", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "https with credentials and no suffix", raw: "https://user:token@GitHub.com/acme/app", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "git scheme", raw: "git://github.com/acme/app.git", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "nested groups join into owner", raw: "git@gitlab.com:group/sub/team/app.git", want: Remote{Host: "gitlab.com", Owner: "group/sub/team", Repo: "app"}},
		{name: "trailing slash", raw: "https://github.com/acme/app/", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "surrounding whitespace", raw: "  git@github.com:acme/app.git\n", want: Remote{Host: "github.com", Owner: "acme", Repo: "app"}},
		{name: "empty", raw: "", wantErr: "remote URL is empty"},
		{name: "local absolute path", raw: "/srv/git/app.git", wantErr: "has no host"},
		{name: "local relative path with colon later", raw: "./repos/app:1", wantErr: "has no host"},
		{name: "file scheme", raw: "file:///srv/git/app.git", wantErr: "unsupported scheme"},
		{name: "no owner", raw: "git@github.com:app.git", wantErr: "has no owner and repository path"},
		{name: "empty repo", raw: "https://github.com/acme/.git", wantErr: "has no owner and repository path"},
		{name: "dot segment", raw: "https://github.com/acme/../app.git", wantErr: "invalid path segment"},
		{name: "empty segment", raw: "git@github.com:acme//app.git", wantErr: "invalid path segment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRemoteURL(tt.raw)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
