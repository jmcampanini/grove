package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteNamerBranch(t *testing.T) {
	tests := []struct {
		name           string
		phrase         string
		wantErr        bool
		wantErrContain string
		wantOutput     string
	}{
		{
			name:       "simple phrase",
			phrase:     "add user auth",
			wantOutput: "feature/add-user-auth",
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
			name:       "long phrase triggers hash truncation",
			phrase:     "implement comprehensive user authentication and authorization system with role based access",
			wantOutput: "feature/implement-comprehensive-user-authentication-a-nquu",
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
			wantErrContain: "empty name after slugification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultTestConfig()
			ctx := &namerContext{
				namer: naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Slugify),
			}

			var buf bytes.Buffer
			err := executeNamerBranch(&buf, ctx, tt.phrase)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, strings.TrimSpace(buf.String()))
		})
	}
}
