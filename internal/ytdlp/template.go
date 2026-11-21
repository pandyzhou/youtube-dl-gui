package ytdlp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// TemplateValues fills output template fields yt-dlp cannot know, such as
// the position in a playlist the app expanded itself.
type TemplateValues map[string]string

var templateField = regexp.MustCompile(`%\(([^)]+)\)([0-9.]*[sd])`)

// Render substitutes the fields of tmpl that values knows, following
// yt-dlp's syntax: "a,b" falls back from a to b, "&text" replaces a present
// field, "|default" stands for a missing one, ">format" (dates) is ignored,
// "d" pads numbers. Unknown fields stay for yt-dlp; when only a fallback is
// known, the field becomes "%(primary|value)fmt". "%%" becomes "%".
func (values TemplateValues) Render(tmpl string) string {
	out := templateField.ReplaceAllStringFunc(tmpl, func(whole string) string {
		m := templateField.FindStringSubmatch(whole)
		expr, format := m[1], m[2]

		fieldPart, defaultPart, hasDefault := strings.Cut(expr, "|")
		fieldsPart, replacement, hasReplacement := strings.Cut(fieldPart, "&")

		var specs []string
		for _, s := range strings.Split(fieldsPart, ",") {
			if s = strings.TrimSpace(s); s != "" {
				specs = append(specs, s)
			}
		}
		if len(specs) == 0 {
			return whole
		}
		key := func(spec string) string {
			k, _, _ := strings.Cut(spec, ">")
			return strings.TrimSpace(k)
		}

		known := false
		resolved, resolvedIndex := "", -1
		for i, spec := range specs {
			if v, ok := values[key(spec)]; ok {
				known = true
				if v != "" && resolvedIndex < 0 {
					resolved, resolvedIndex = v, i
				}
			}
		}
		if !known {
			return whole
		}
		if resolvedIndex == 0 {
			value := sanitizeTemplateValue(resolved)
			if hasReplacement {
				value = replacement
			}
			return formatNumber(value, format)
		}
		if resolvedIndex > 0 {
			return fmt.Sprintf("%%(%s|%s)%s", key(specs[0]), sanitizeTemplateValue(resolved), format)
		}
		if hasDefault {
			return defaultPart
		}
		return ""
	})
	return strings.ReplaceAll(out, "%%", "%")
}

// formatNumber pads value as "%05d" or "%5d" would when the field's format
// is an integer with a width and value is a number.
func formatNumber(value, format string) string {
	if !strings.HasSuffix(format, "d") || len(format) < 2 {
		return value
	}
	width := format[:len(format)-1]
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return value
	}
	zero := strings.HasPrefix(width, "0")
	w, _ := strconv.Atoi(strings.TrimLeft(width, "0"))
	if w <= 0 {
		return value
	}
	if zero {
		return fmt.Sprintf("%0*d", w, n)
	}
	return fmt.Sprintf("%*d", w, n)
}

// sanitizeTemplateValue keeps a value from adding folders or control
// characters to a path.
func sanitizeTemplateValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '|' || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, s)
}
