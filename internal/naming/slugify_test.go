package naming

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts SlugifyOptions
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "lowercase", in: "Add User Auth", opts: SlugifyOptions{Lowercase: true}, want: "add-user-auth"},
		{name: "preserve case", in: "Add User Auth", want: "Add-User-Auth"},
		{name: "replace and collapse runs", in: "--one/// two...three__", want: "one-two-three"},
		{name: "trim surrounding runs", in: " *** value !!! ", want: "value"},
		{name: "ASCII alphanumeric only", in: "café 東京 123", opts: SlugifyOptions{Lowercase: true}, want: "caf-123"},
		{name: "all non alphanumeric", in: "🎉---_", want: ""},
		{name: "ASCII case preserved", in: "ABC123xyz", want: "ABC123xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.in, tt.opts); got != tt.want {
				t.Fatalf("Slugify(%q, %#v) = %q, want %q", tt.in, tt.opts, got, tt.want)
			}
		})
	}
}

func TestTruncateName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "zero unchanged", in: "any-name-", max: 0, want: "any-name-"},
		{name: "negative unchanged", in: "any-name-", max: -1, want: "any-name-"},
		{name: "under cap unchanged", in: "short-", max: 10, want: "short-"},
		{name: "at cap unchanged", in: "short-", max: 6, want: "short-"},
		{name: "cuts end", in: "abcdefgh", max: 5, want: "abcde"},
		{name: "rune safe", in: "éclair", max: 2, want: "éc"},
		{name: "trims one dash at cut", in: "abc-def", max: 4, want: "abc"},
		{name: "trims dash run at cut", in: "ab---cd", max: 5, want: "ab"},
		{name: "only dash at cut becomes empty", in: "-value", max: 1, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateName(tt.in, tt.max); got != tt.want {
				t.Fatalf("TruncateName(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}
