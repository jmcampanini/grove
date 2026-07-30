package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMutuallyExclusiveFlagHelpText(t *testing.T) {
	tests := []struct {
		help string
		name string
		want []string
	}{
		{
			help: createCmd.Long,
			name: "create",
			want: []string{
				"Creation mode flags are mutually exclusive",
				"--from <ref>",
				"--from-remote-primary",
				"--reuse",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.want {
				assert.Contains(t, tt.help, want)
			}
		})
	}
}
