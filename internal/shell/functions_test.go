package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFunctionGenerator_GenerateFish(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateFish()

	// Verify output contains the function definition
	assert.Contains(t, output, "function grc")
	assert.Contains(t, output, "grove create")
	assert.Contains(t, output, "command -q z") // zoxide check
	assert.Contains(t, output, "cd $output")   // fallback to cd
}

func TestFunctionGenerator_GenerateBash(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateBash()

	// Verify output contains the function definition
	assert.Contains(t, output, "grc()")
	assert.Contains(t, output, "grove create")
	assert.Contains(t, output, "command -v z") // zoxide check
	assert.Contains(t, output, `cd "$output"`) // fallback to cd
}

func TestFunctionGenerator_GenerateZsh(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateZsh()

	// Verify output contains the function definition
	assert.Contains(t, output, "grc()")
	assert.Contains(t, output, "grove create")
	assert.Contains(t, output, "command -v z") // zoxide check
	assert.Contains(t, output, `cd "$output"`) // fallback to cd
}

func TestFunctionGenerator_FishSyntax(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateFish()

	// Fish-specific syntax checks
	assert.Contains(t, output, "set -l output")
	assert.Contains(t, output, "$status")
	assert.Contains(t, output, "end")
}

func TestFunctionGenerator_BashZshSyntax(t *testing.T) {
	gen := NewFunctionGenerator()

	for _, shellName := range []string{"bash", "zsh"} {
		t.Run(shellName, func(t *testing.T) {
			var output string
			if shellName == "bash" {
				output = gen.GenerateBash()
			} else {
				output = gen.GenerateZsh()
			}

			// Bash/zsh-specific syntax checks
			assert.Contains(t, output, "local output")
			assert.Contains(t, output, "$?")
			assert.Contains(t, output, "fi")
		})
	}
}

func TestFunctionGenerator_NoEmptyOutput(t *testing.T) {
	gen := NewFunctionGenerator()

	tests := []struct {
		name     string
		generate func() string
	}{
		{"fish", gen.GenerateFish},
		{"bash", gen.GenerateBash},
		{"zsh", gen.GenerateZsh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.generate()
			assert.NotEmpty(t, strings.TrimSpace(output))
		})
	}
}

func TestFunctionGenerator_GrpStylePassthrough(t *testing.T) {
	gen := NewFunctionGenerator()

	tests := []struct {
		generate func() string
		name     string
		contains []string
	}{
		{
			name:     "bash",
			generate: gen.GenerateBash,
			contains: []string{
				`style="${1:-review}"`,
				"--color always",
				"--style $style",
				"grove pr preview",
			},
		},
		{
			name:     "zsh",
			generate: gen.GenerateZsh,
			contains: []string{
				`style="${1:-review}"`,
				"--color always",
				"--style $style",
				"grove pr preview",
			},
		},
		{
			name:     "fish",
			generate: gen.GenerateFish,
			contains: []string{
				"set -l style review",
				"count $argv",
				"set style $argv[1]",
				"--color always",
				"--style $style",
				"grove pr preview",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.generate()
			for _, want := range tt.contains {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestNewFunctionGenerator(t *testing.T) {
	gen := NewFunctionGenerator()
	assert.NotNil(t, gen)
}
