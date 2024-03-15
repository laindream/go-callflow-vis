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
	MatchTypeEqual   MatchType = "equal"
	MatchTypeRegexp  MatchType = "regexp"
)

type Mode struct {
	OR    bool   `toml:"or" json:"or"`
	AND   bool   `toml:"and" json:"and"`
	Items []Item `toml:"items" json:"items"`
}

func (m *Mode) Match(s string) bool {
	if len(m.Items) == 0 {
		return false
	}
	if len(m.Items) == 1 {
		return m.Items[0].Match(s)
	}
	if m.OR == m.AND {
		return false
	}
	if m.OR {
		for _, item := range m.Items {
			if item.Match(s) {
				return true
			}
		}
		return false
	}
	for _, item := range m.Items {
		if !item.Match(s) {
			return false
		}
	}
	return true
}

type Item struct {
	Exclude bool      `toml:"exclude" json:"exclude"`
	Type    MatchType `toml:"type" json:"type"`
	Content string    `toml:"content" json:"content"`
}

func (m *Item) Match(s string) (result bool) {
	defer func() {
		if m.Exclude {
			result = !result
		}
	}()
	if m.Content == "" {
		return false
	}
	if m.Type == "" {
		m.Type = MatchTypeEqual
	}
	switch m.Type {
	case MatchTypePrefix:
		return strings.HasPrefix(s, m.Content)
	case MatchTypeSuffix:
		return strings.HasSuffix(s, m.Content)
	case MatchTypeContain:
		return strings.Contains(s, m.Content)
	case MatchTypeEqual:
		return s == m.Content
	case MatchTypeRegexp:
		r, err := regexp.Compile(m.Content)
		if err != nil {
			return false
		}
		return r.MatchString(s)
	}
	return false
}

type Set []*Mode

func (ms Set) Match(s string) bool {
	for _, m := range ms {
		if m.Match(s) {
			return true
		}
	}
	return false
}
