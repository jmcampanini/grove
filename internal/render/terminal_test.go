package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectDarkBackgroundFromEnv(t *testing.T) {
	tests := []struct {
		env      string
		name     string
		wantDark bool
		wantOK   bool
	}{
		{name: "unset", env: "", wantDark: false, wantOK: false},
		{name: "no semicolon", env: "15", wantDark: false, wantOK: false},
		{name: "black bg is dark", env: "15;0", wantDark: true, wantOK: true},
		{name: "white bg is light", env: "0;15", wantDark: false, wantOK: true},
		{name: "color 7 is light", env: "0;7", wantDark: false, wantOK: true},
		{name: "color 8 is dark", env: "0;8", wantDark: true, wantOK: true},
		{name: "color 6 is dark", env: "0;6", wantDark: true, wantOK: true},
		{name: "color 9 is light", env: "0;9", wantDark: false, wantOK: true},
		{name: "three parts uses last", env: "37;0;15", wantDark: false, wantOK: true},
		{name: "non-numeric bg", env: "0;abc", wantDark: false, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COLORFGBG", tt.env)
			dark, ok := DetectDarkBackgroundFromEnv()
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantDark, dark)
			}
		})
	}
}
