package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestHandlePreviewError(t *testing.T) {
	tests := []struct {
		err        error
		fzfMode    bool
		name       string
		wantErr    bool
		wantOutput string
	}{
		{
			err:        assert.AnError,
			fzfMode:    true,
			name:       "fzf mode prints error to stdout and returns nil",
			wantErr:    false,
			wantOutput: "Error: assert.AnError general error for testing\n",
		},
		{
			err:        assert.AnError,
			fzfMode:    false,
			name:       "normal mode returns error",
			wantErr:    true,
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			resultErr := handlePreviewError(cmd, tt.err, tt.fzfMode)

			if tt.wantErr {
				assert.Error(t, resultErr)
				assert.Equal(t, tt.err, resultErr)
			} else {
				assert.NoError(t, resultErr)
			}

			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}
