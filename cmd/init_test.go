package cmd

import (
	"bytes"
	"testing"

	"github.com/jmcampanini/grove-cli/internal/shell"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetInitFlags() {
	initBashFlag = false
	initFishFlag = false
	initZshFlag = false
	initCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
}

func TestRunInit_ShellFlags(t *testing.T) {
	gen := shell.NewFunctionGenerator()

	tests := []struct {
		flag string
		name string
		want string
	}{
		{flag: "--bash", name: "bash", want: gen.GenerateBash()},
		{flag: "--fish", name: "fish", want: gen.GenerateFish()},
		{flag: "--zsh", name: "zsh", want: gen.GenerateZsh()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInitFlags()

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)
			rootCmd.SetArgs([]string{"init", tt.flag})

			err := rootCmd.Execute()
			require.NoError(t, err)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

func TestRunInit_Errors(t *testing.T) {
	tests := []struct {
		args           []string
		name           string
		wantErrContain string
		wantOutput     []string
	}{
		{
			args:           []string{"init"},
			name:           "no flags shows help and errors",
			wantErrContain: "specify a shell flag",
			wantOutput:     []string{"Examples", "--bash", "--fish", "--zsh"},
		},
		{
			args:           []string{"init", "--zsh", "--bash"},
			name:           "mutually exclusive flags",
			wantErrContain: "if any flags in the group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInitFlags()

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)
			rootCmd.SetArgs(tt.args)

			err := rootCmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContain)

			for _, want := range tt.wantOutput {
				assert.Contains(t, buf.String(), want, "output should contain %q", want)
			}
		})
	}
}
