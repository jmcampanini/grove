package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFunctionGenerator_GenerateFish(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateFish()

	assert.Contains(t, output, "function grc")
	assert.Contains(t, output, "grove create")
	assert.Contains(t, output, "command -q z")
	assert.Contains(t, output, "cd $output")
}

func TestFunctionGenerator_GenerateBash(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateBash()

	assert.Contains(t, output, "grc()")
	assert.Contains(t, output, "grove create")
	assert.Contains(t, output, "command -v z")
	assert.Contains(t, output, `cd "$output"`)
}

func TestFunctionGenerator_GenerateZsh(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateZsh()

	assert.Contains(t, output, "grc()")
	assert.Contains(t, output, "grove create")
	assert.Contains(t, output, "command -v z")
	assert.Contains(t, output, `cd "$output"`)
}

func TestFunctionGenerator_FishSyntax(t *testing.T) {
	gen := NewFunctionGenerator()
	output := gen.GenerateFish()

	assert.Contains(t, output, "set -l output")
	assert.Contains(t, output, "$status")
	assert.Contains(t, output, "end")
}

func TestFunctionGenerator_BashZshSyntax(t *testing.T) {
	gen := NewFunctionGenerator()

	tests := []struct {
		generate func() string
		name     string
	}{
		{name: "bash", generate: gen.GenerateBash},
		{name: "zsh", generate: gen.GenerateZsh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.generate()
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

func TestFunctionGenerator_GrpPreviewCommand(t *testing.T) {
	gen := NewFunctionGenerator()

	wantContains := []string{"--color always", "grove pr preview", "--fzf {1}"}
	tests := []struct {
		generate func() string
		name     string
	}{
		{name: "bash", generate: gen.GenerateBash},
		{name: "zsh", generate: gen.GenerateZsh},
		{name: "fish", generate: gen.GenerateFish},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.generate()
			for _, want := range wantContains {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestNewFunctionGenerator(t *testing.T) {
	gen := NewFunctionGenerator()
	assert.NotNil(t, gen)
}
