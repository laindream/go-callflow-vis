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
	Rules []Rule `toml:"rules" json:"rules"`
}

func (m *Mode) Match(s string) bool {
	if s == "" {
		return false
	}
	if m == nil {
		return true
	}
	if len(m.Rules) == 0 {
		return false
	}
	if len(m.Rules) == 1 {
		return m.Rules[0].Match(s)
	}
	if m.OR == m.AND {
		return false
	}
	if m.OR {
		for _, item := range m.Rules {
			if item.Match(s) {
				return true
			}
		}
		return false
	}
	for _, item := range m.Rules {
		if !item.Match(s) {
			return false
		}
	}
	return true
}

type Rule struct {
	Exclude            bool      `toml:"exclude" json:"exclude"`
	Type               MatchType `toml:"type" json:"type"`
	Content            string    `toml:"content" json:"content"`
	regexp             *regexp.Regexp
	regexpCompileError error
}

func (m *Rule) Match(s string) (result bool) {
	if s == "" {
		return false
	}
	if m == nil {
		return true
	}
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
		if m.regexp == nil && m.regexpCompileError == nil {
			m.regexp, m.regexpCompileError = regexp.Compile(m.Content)
		}
		if m.regexpCompileError != nil {
			return m.regexp.MatchString(s)
		}
	}
	return false
}

type Set []*Mode

func (ms Set) Match(s string) bool {
	if s == "" {
		return false
	}
	for _, m := range ms {
		if m.Match(s) {
			return true
		}
	}
	return false
}
