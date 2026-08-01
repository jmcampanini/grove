package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigCommandHasProvenanceFlags(t *testing.T) {
	assert.NotNil(t, configCmd.Flags().Lookup("provenance"))
	assert.NotNil(t, configCmd.Flags().Lookup("sources"))
}

func TestWriteConfigProvenance(t *testing.T) {
	var buf bytes.Buffer
	err := writeConfigProvenance(&buf, []string{"Path", "Source"}, [][]string{
		{"git.timeout", "<default>"},
		{"github.preview_cache_ttl", "/tmp/grove.toml"},
	})
	require.NoError(t, err)

	assert.Equal(t, "Path\tSource\ngit.timeout\t<default>\ngithub.preview_cache_ttl\t/tmp/grove.toml\n", buf.String())
}
