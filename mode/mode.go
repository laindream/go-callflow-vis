package mode

import (
	"regexp"
	"strings"
)

type MatchType string

var (
	MatchTypePrefix  MatchType = "prefix"
	MatchTypeSuffix  MatchType = "suffix"
	MatchTypeContain MatchType = "contain"
	MatchTypeFull    MatchType = "full"
	MatchTypeRegex   MatchType = "regex"
)

type Mode struct {
	Exclude bool      `toml:"exclude" json:"exclude"`
	Type    MatchType `toml:"type" json:"type"`
	Content string    `toml:"content" json:"content"`
}

func (m *Mode) Match(s string) (result bool) {
	defer func() {
		if m.Exclude {
			result = !result
		}
	}()
	switch m.Type {
	case MatchTypePrefix:
		return strings.HasPrefix(s, m.Content)
	case MatchTypeSuffix:
		return strings.HasSuffix(s, m.Content)
	case MatchTypeContain:
		return strings.Contains(s, m.Content)
	case MatchTypeFull:
		return s == m.Content
	case MatchTypeRegex:
		r, err := regexp.Compile(m.Content)
		if err != nil {
			return false
		}
		return r.MatchString(s)
	}
	return false
}

type ModeSet []*Mode

func (ms ModeSet) Match(s string) bool {
	for _, m := range ms {
		if m.Match(s) {
			return true
		}
	}
	return false
}
