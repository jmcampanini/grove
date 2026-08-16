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

func TestExecuteNamerBranch(t *testing.T) {
	tests := []struct {
		name           string
		phrase         string
		wantErr        bool
		wantErrContain string
		wantErrIs      error
		wantOutput     string
	}{
		{
			name:       "whole branch name is capped",
			phrase:     "add user authentication",
			wantOutput: "feature/add-user-authenticatio",
		},
		{
			name:       "special characters",
			phrase:     "fix: handle 404!",
			wantOutput: "feature/fix-handle-404",
		},
		{
			name:       "mixed casing",
			phrase:     "Add OAuth2 Google",
			wantOutput: "feature/add-oauth2-google",
		},
		{
			name:       "long whole name is truncated",
			phrase:     "implement comprehensive user authentication and authorization system with role based access",
			wantOutput: "feature/implement-comprehensiv",
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
			err = executeNamerBranch(&buf, ctx, tt.phrase)

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
