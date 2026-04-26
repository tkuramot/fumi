package store

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		src         string
		wantFound   bool
		wantID      string
		wantMatches []string
		wantExclud  []string
		wantErr     string // substring; empty means no error
	}{
		{
			name:      "no frontmatter when code starts immediately",
			src:       "console.log('x');\n// ==Fumi Action==\n// @id ignored\n// ==/Fumi Action==\n",
			wantFound: false,
		},
		{
			name: "happy path",
			src: `// ==Fumi Action==
// @id my-action
// @match https://example.com/*
// @match https://*.example.org/*
// @exclude https://example.com/admin/*
// ==/Fumi Action==
console.log("hi");
`,
			wantFound:   true,
			wantID:      "my-action",
			wantMatches: []string{"https://example.com/*", "https://*.example.org/*"},
			wantExclud:  []string{"https://example.com/admin/*"},
		},
		{
			name: "blank and comment lines before block ok",
			src: `// preamble comment

// another comment
// ==Fumi Action==
// @id only-id
// ==/Fumi Action==
`,
			wantFound: true,
			wantID:    "only-id",
		},
		{
			name: "blank lines inside block ok",
			src: `// ==Fumi Action==
// @id x
//
// @match https://a/
// ==/Fumi Action==
`,
			wantFound:   true,
			wantID:      "x",
			wantMatches: []string{"https://a/"},
		},
		{
			name: "duplicate id rejected",
			src: `// ==Fumi Action==
// @id one
// @id two
// ==/Fumi Action==
`,
			wantErr: "duplicate @id",
		},
		{
			name: "unknown key rejected",
			src: `// ==Fumi Action==
// @runat start
// ==/Fumi Action==
`,
			wantErr: "unknown frontmatter key",
		},
		{
			name: "non-comment line inside block is malformed",
			src: `// ==Fumi Action==
not a comment
// ==/Fumi Action==
`,
			wantErr: "malformed frontmatter line",
		},
		{
			name: "unterminated block",
			src: `// ==Fumi Action==
// @id x
`,
			wantErr: "unterminated frontmatter block",
		},
		{
			name:      "empty source",
			src:       "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fm, err := ParseFrontmatter(tt.src)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if fm.Found != tt.wantFound {
				t.Errorf("found = %v, want %v", fm.Found, tt.wantFound)
			}
			if fm.ID != tt.wantID {
				t.Errorf("id = %q, want %q", fm.ID, tt.wantID)
			}
			if !equalSlice(fm.Matches, tt.wantMatches) {
				t.Errorf("matches = %v, want %v", fm.Matches, tt.wantMatches)
			}
			if !equalSlice(fm.Excludes, tt.wantExclud) {
				t.Errorf("excludes = %v, want %v", fm.Excludes, tt.wantExclud)
			}
		})
	}
}

func TestDeriveIDFromFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"hello.js", "hello"},
		{"My Action.js", "my-action"},
		{"Foo_Bar.Baz.js", "foo-bar-baz"},
		{"--leading-and-trailing--.js", "leading-and-trailing"},
		{"123abc.js", "123abc"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := deriveIDFromFilename(tt.in); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func equalSlice(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
