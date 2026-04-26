package store

import (
	"strings"
	"testing"
)

func TestParseFrontmatter_valid(t *testing.T) {
	src := `// ==Fumi Action==
// @id my-action
// @match https://example.com/*
// @match https://other.com/*
// @exclude https://example.com/skip
// ==/Fumi Action==
console.log("hello");
`
	fm, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fm.Found {
		t.Error("Expected Found=true")
	}
	if fm.ID != "my-action" {
		t.Errorf("ID = %q, want %q", fm.ID, "my-action")
	}
	if len(fm.Matches) != 2 {
		t.Errorf("Matches length = %d, want 2", len(fm.Matches))
	}
	if fm.Matches[0] != "https://example.com/*" {
		t.Errorf("Matches[0] = %q, want https://example.com/*", fm.Matches[0])
	}
	if len(fm.Excludes) != 1 {
		t.Errorf("Excludes length = %d, want 1", len(fm.Excludes))
	}
	if fm.Excludes[0] != "https://example.com/skip" {
		t.Errorf("Excludes[0] = %q", fm.Excludes[0])
	}
}

func TestParseFrontmatter_noFrontmatter(t *testing.T) {
	// Non-comment code before the marker => no frontmatter, no error.
	src := `console.log("hello");
// ==Fumi Action==
// @id my-action
// ==/Fumi Action==
`
	fm, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Found {
		t.Error("Expected Found=false when non-comment code precedes marker")
	}
}

func TestParseFrontmatter_blankBeforeStart(t *testing.T) {
	// Blank lines and comments before the marker are allowed.
	src := `
// A leading comment
// ==Fumi Action==
// @id my-action
// @match https://example.com/*
// ==/Fumi Action==
`
	fm, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fm.Found {
		t.Error("Expected Found=true")
	}
	if fm.ID != "my-action" {
		t.Errorf("ID = %q, want my-action", fm.ID)
	}
}

func TestParseFrontmatter_emptySource(t *testing.T) {
	fm, err := ParseFrontmatter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Found {
		t.Error("Expected Found=false for empty source")
	}
}

func TestParseFrontmatter_noID(t *testing.T) {
	src := `// ==Fumi Action==
// @match https://example.com/*
// ==/Fumi Action==
`
	fm, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fm.Found {
		t.Error("Expected Found=true")
	}
	if fm.ID != "" {
		t.Errorf("Expected empty ID, got %q", fm.ID)
	}
}

func TestParseFrontmatter_duplicateID(t *testing.T) {
	src := `// ==Fumi Action==
// @id first
// @id second
// ==/Fumi Action==
`
	_, err := ParseFrontmatter(src)
	if err == nil {
		t.Error("Expected error for duplicate @id")
	}
	if !strings.Contains(err.Error(), "duplicate @id") {
		t.Errorf("error message should mention duplicate @id: %v", err)
	}
}

func TestParseFrontmatter_unknownKey(t *testing.T) {
	src := `// ==Fumi Action==
// @id my-action
// @unknown something
// ==/Fumi Action==
`
	_, err := ParseFrontmatter(src)
	if err == nil {
		t.Error("Expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown frontmatter key") {
		t.Errorf("error message should mention unknown key: %v", err)
	}
}

func TestParseFrontmatter_malformedLine(t *testing.T) {
	src := `// ==Fumi Action==
this is not a comment line
// ==/Fumi Action==
`
	_, err := ParseFrontmatter(src)
	if err == nil {
		t.Error("Expected error for malformed line")
	}
	if !strings.Contains(err.Error(), "malformed frontmatter line") {
		t.Errorf("error message should mention malformed line: %v", err)
	}
}

func TestParseFrontmatter_unterminatedBlock(t *testing.T) {
	src := `// ==Fumi Action==
// @id my-action
`
	_, err := ParseFrontmatter(src)
	if err == nil {
		t.Error("Expected error for unterminated block")
	}
	if !strings.Contains(err.Error(), "unterminated frontmatter block") {
		t.Errorf("error message should mention unterminated block: %v", err)
	}
}

func TestParseFrontmatter_blankCommentInsideBlock(t *testing.T) {
	// Blank and plain comment lines inside the block are allowed.
	src := `// ==Fumi Action==
// @id my-action
//
// a plain comment
// @match https://example.com/*
// ==/Fumi Action==
`
	fm, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fm.Found {
		t.Error("Expected Found=true")
	}
	if fm.ID != "my-action" {
		t.Errorf("ID = %q, want my-action", fm.ID)
	}
}

func TestDeriveIDFromFilename(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"my-action.js", "my-action"},
		{"My Action.js", "my-action"},
		{"save_note.js", "save-note"},
		{"foo bar baz.js", "foo-bar-baz"},
		{"123test.js", "123test"},
		{"---action---.js", "action"},
		{"noextension", "noextension"},
		{"UPPER_CASE.js", "upper-case"},
	}
	for _, tc := range cases {
		got := deriveIDFromFilename(tc.input)
		if got != tc.want {
			t.Errorf("deriveIDFromFilename(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
