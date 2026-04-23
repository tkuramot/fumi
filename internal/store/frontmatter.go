package store

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

var (
	startRe = regexp.MustCompile(`^\s*//\s*==Fumi Action==\s*$`)
	endRe   = regexp.MustCompile(`^\s*//\s*==/Fumi Action==\s*$`)
	lineRe  = regexp.MustCompile(`^\s*//\s*@(\w+)\s+(.+?)\s*$`)
	commentRe = regexp.MustCompile(`^\s*//`)
	blankRe   = regexp.MustCompile(`^\s*$`)
)

type Frontmatter struct {
	ID       string
	Matches  []string
	Excludes []string
	Found    bool
}

func ParseFrontmatter(src string) (*Frontmatter, error) {
	fm := &Frontmatter{}
	scanner := bufio.NewScanner(strings.NewReader(src))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	inBlock := false
	sawStart := false
	for scanner.Scan() {
		line := scanner.Text()
		if !sawStart {
			if startRe.MatchString(line) {
				sawStart = true
				inBlock = true
				fm.Found = true
				continue
			}
			// Before the start marker, only blank lines and comments are allowed.
			if blankRe.MatchString(line) || commentRe.MatchString(line) {
				continue
			}
			// Non-comment code encountered before the block: no frontmatter.
			return fm, nil
		}
		if inBlock {
			if endRe.MatchString(line) {
				return fm, nil
			}
			m := lineRe.FindStringSubmatch(line)
			if m == nil {
				// Blank comment line inside the block is allowed.
				if blankRe.MatchString(line) || commentRe.MatchString(line) {
					continue
				}
				return nil, fmt.Errorf("malformed frontmatter line: %q", line)
			}
			key, val := m[1], strings.TrimSpace(m[2])
			switch key {
			case "id":
				if fm.ID != "" {
					return nil, fmt.Errorf("duplicate @id")
				}
				fm.ID = val
			case "match":
				fm.Matches = append(fm.Matches, val)
			case "exclude":
				fm.Excludes = append(fm.Excludes, val)
			default:
				return nil, fmt.Errorf("unknown frontmatter key: @%s", key)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if sawStart && inBlock {
		return nil, fmt.Errorf("unterminated frontmatter block")
	}
	return fm, nil
}

var kebabRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func deriveIDFromFilename(name string) string {
	base := strings.TrimSuffix(name, ".js")
	s := kebabRe.ReplaceAllString(base, "-")
	s = strings.Trim(s, "-")
	return strings.ToLower(s)
}
