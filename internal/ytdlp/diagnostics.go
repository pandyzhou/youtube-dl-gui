package ytdlp

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Level of a diagnostic.
type Level string

// Levels.
const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
)

// CodeInputFilterSkipped marks a video an input filter skipped.
const CodeInputFilterSkipped = "input_filter_skipped"

// Diagnostic is an error or warning yt-dlp printed, classified by Code
// (see diagnostic_rules.json) so the UI can explain it and offer a fix.
type Diagnostic struct {
	Level     Level
	Code      string
	Component string // the "[youtube]" part, without brackets
	Message   string
	Raw       string
}

//go:embed diagnostic_rules.json
var rulesJSON []byte

type rulesFile struct {
	Rules []struct {
		Code      string `json:"code"`
		AppliesTo string `json:"appliesTo"` // error | warning | both
		Component string `json:"component"`
		Patterns  []struct {
			Kind  string `json:"kind"` // substr | regex
			Value string `json:"value"`
		} `json:"patterns"`
	} `json:"rules"`
	FallbackCode string `json:"fallbackCode"`
}

type pattern struct {
	substr string // lower case
	re     *regexp.Regexp
}

type rule struct {
	code      string
	appliesTo string
	component string // lower case
	patterns  []pattern
}

type matcher struct {
	rules    []rule
	fallback string
}

var (
	defaultMatcher     *matcher
	defaultMatcherErr  error
	defaultMatcherOnce sync.Once
)

func loadMatcher(data []byte) (*matcher, error) {
	var file rulesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("diagnostic rules: %w", err)
	}
	m := &matcher{fallback: file.FallbackCode}
	for _, r := range file.Rules {
		loaded := rule{code: r.Code, appliesTo: r.AppliesTo, component: strings.ToLower(r.Component)}
		for _, p := range r.Patterns {
			if p.Kind == "regex" {
				re, err := regexp.Compile(p.Value)
				if err != nil {
					return nil, fmt.Errorf("diagnostic rule %s: %w", r.Code, err)
				}
				loaded.patterns = append(loaded.patterns, pattern{re: re})
			} else {
				loaded.patterns = append(loaded.patterns, pattern{substr: strings.ToLower(p.Value)})
			}
		}
		m.rules = append(m.rules, loaded)
	}
	return m, nil
}

func (m *matcher) code(level Level, component, message string) string {
	lower := strings.ToLower(message)
	componentLower := strings.ToLower(component)
	for _, r := range m.rules {
		if (r.appliesTo == "error" && level != LevelError) || (r.appliesTo == "warning" && level != LevelWarning) {
			continue
		}
		if r.component != "" && r.component != componentLower {
			continue
		}
		for _, p := range r.patterns {
			if (p.re != nil && p.re.MatchString(message)) || (p.re == nil && strings.Contains(lower, p.substr)) {
				return r.code
			}
		}
	}
	return m.fallback
}

// ParseDiagnostic classifies an output line, or returns nil for a line that
// is neither an error, a warning nor an input filter skip.
func ParseDiagnostic(line string) *Diagnostic {
	defaultMatcherOnce.Do(func() { defaultMatcher, defaultMatcherErr = loadMatcher(rulesJSON) })
	if defaultMatcherErr != nil {
		panic(defaultMatcherErr) // the embedded rules are tested
	}
	return defaultMatcher.parse(line)
}

func (m *matcher) parse(line string) *Diagnostic {
	trimmed := strings.TrimRight(line, " \t")
	if message := skipMessage(trimmed); message != "" {
		return &Diagnostic{Level: LevelWarning, Code: CodeInputFilterSkipped, Component: "download", Message: message, Raw: trimmed}
	}
	var level Level
	var rest string
	if r, ok := strings.CutPrefix(trimmed, "ERROR:"); ok {
		level, rest = LevelError, strings.TrimSpace(r)
	} else if r, ok := strings.CutPrefix(trimmed, "WARNING:"); ok {
		level, rest = LevelWarning, strings.TrimSpace(r)
	} else {
		return nil
	}
	component, message := splitComponent(rest)
	message = stripVideoID(message)
	return &Diagnostic{Level: level, Code: m.code(level, component, message), Component: component, Message: message, Raw: trimmed}
}

// skipMessage names the filter that skipped a video, or returns "".
func skipMessage(line string) string {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.Contains(lower, "upload date is not in range"):
		return "Skipped by upload date filter"
	case strings.Contains(lower, "file is larger than max-filesize"):
		return "Skipped by maximum filesize filter"
	case strings.Contains(lower, "file is smaller than min-filesize"):
		return "Skipped by minimum filesize filter"
	case strings.Contains(lower, "does not pass filter"):
		if strings.Contains(lower, "break-match") {
			return "Skipped by break-match filter"
		}
		return "Skipped by advanced input filter"
	case strings.Contains(lower, "maximum number of downloads reached"), strings.Contains(lower, "max downloads reached"):
		return "Skipped by maximum downloads filter"
	case strings.Contains(lower, "skipping") && strings.Contains(lower, "age limit"), strings.Contains(lower, "age restricted"):
		return "Skipped by age limit filter"
	case strings.Contains(lower, "skipping") && strings.Contains(lower, "filter"), strings.Contains(lower, " is not in range "):
		return "Skipped by input filter"
	}
	return ""
}

func splitComponent(s string) (string, string) {
	if rest, ok := strings.CutPrefix(s, "["); ok {
		if end := strings.IndexByte(rest, ']'); end >= 0 {
			return rest[:end], strings.TrimSpace(rest[end+1:])
		}
	}
	return "", s
}

// stripVideoID drops a leading "dQw4w9WgXcQ: " video id.
func stripVideoID(s string) string {
	first, rest, ok := strings.Cut(s, ":")
	if ok && probableID(strings.TrimSpace(first)) {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(s)
}

func probableID(s string) bool {
	if len(s) < 10 || len(s) > 15 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
