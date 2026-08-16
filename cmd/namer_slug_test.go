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

func TestExecuteNamerSlug(t *testing.T) {
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
			wantOutput: "add-user-auth",
		},
		{
			name:       "special characters",
			phrase:     "fix: handle 404!",
			wantOutput: "fix-handle-404",
		},
		{
			name:       "mixed casing",
			phrase:     "Add OAuth2 Google",
			wantOutput: "add-oauth2-google",
		},
		{
			name:       "raw slug is not capped",
			phrase:     "implement comprehensive user authentication and authorization system with role based access",
			wantOutput: "implement-comprehensive-user-authentication-and-authorization-system-with-role-based-access",
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
			wantErrContain: "empty result after slugification",
			wantErrIs:      naming.ErrEmptySlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := naming.SlugifyOptionsFromConfig(defaultTestConfig().Naming)

			var buf bytes.Buffer
			err := executeNamerSlug(&buf, opts, tt.phrase)

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

func TestNamerSlugHelpDocumentsNoNameCap(t *testing.T) {
	assert.Contains(t, newNamerSlugCmd().Long, "not capped by naming.max_length")
}
