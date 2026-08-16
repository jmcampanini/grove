package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteNamerWorktree(t *testing.T) {
	tests := []struct {
		name           string
		phrase         string
		wantErr        bool
		wantErrContain string
		wantErrIs      error
		wantOutput     string
	}{
		{
			name:       "simple phrase",
			phrase:     "add user auth",
			wantOutput: "wt-add-user-auth",
		},
		{
			name:       "special characters",
			phrase:     "fix: handle 404!",
			wantOutput: "wt-fix-handle-404",
		},
		{
			name:       "mixed casing",
			phrase:     "Add OAuth2 Google",
			wantOutput: "wt-add-oauth2-google",
		},
		{
			name:       "generated branch cap is preserved",
			phrase:     "implement comprehensive user authentication and authorization system with role based access",
			wantOutput: "wt-implement-comprehensiv",
		},
		{
			name:           "empty phrase",
			phrase:         "   ",
			wantErr:        true,
			wantErrContain: "phrase cannot be empty",
		},
		{
			name:           "all special chars slugifies to empty",
			phrase:         "@#$%^&*",
			wantErr:        true,
			wantErrContain: "failed to generate branch name",
			wantErrIs:      naming.ErrEmptySlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultTestConfig()
			namer, err := naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Naming)
			require.NoError(t, err)
			ctx := &namerContext{namer: namer}

			var buf bytes.Buffer
			err = executeNamerWorktree(&buf, ctx, tt.phrase)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs))
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, strings.TrimSpace(buf.String()))
		})
	}
}
