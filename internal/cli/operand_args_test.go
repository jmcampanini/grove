package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhraseArgs(t *testing.T) {
	validate := phraseArgs("phrase")
	tests := []struct {
		args    []string
		name    string
		wantErr string
	}{
		{name: "phrase", args: []string{"add user auth"}},
		{name: "empty", args: []string{""}, wantErr: "phrase cannot be empty"},
		{name: "blank", args: []string{" \t"}, wantErr: "phrase cannot be empty"},
		{name: "no operand", args: nil, wantErr: "accepts 1 arg(s), received 0"},
		{name: "two operands", args: []string{"a", "b"}, wantErr: "accepts 1 arg(s), received 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(nil, tt.args)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestNumberArgs(t *testing.T) {
	validate := numberArgs("issue")
	tests := []struct {
		args    []string
		name    string
		wantErr string
	}{
		{name: "positive integer", args: []string{"17"}},
		{name: "zero", args: []string{"0"}, wantErr: `invalid issue number "0"`},
		{name: "negative", args: []string{"-3"}, wantErr: `invalid issue number "-3"`},
		{name: "not a number", args: []string{"abc"}, wantErr: `invalid issue number "abc"`},
		{name: "no operand", args: nil, wantErr: "accepts 1 arg(s), received 0"},
		{name: "two operands", args: []string{"1", "2"}, wantErr: "accepts 1 arg(s), received 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(nil, tt.args)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
