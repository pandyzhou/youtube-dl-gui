package formats

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// InputFilters converts the input filter settings to yt-dlp's options.
// Sizes must be positive, dates YYYY-MM-DD with a mode, filters non-empty.
func InputFilters(s config.InputFilterSettings) ytdlp.InputFilterOptions {
	var out ytdlp.InputFilterOptions
	out.MinFilesize = sizeArg(s.MinSize)
	out.MaxFilesize = sizeArg(s.MaxSize)
	if date := dateArg(s.DateFilter.Value); date != "" {
		switch s.DateFilter.Mode {
		case "exact":
			out.Date = date
		case "before":
			out.DateBefore = date
		case "after":
			out.DateAfter = date
		}
	}
	out.MatchFilters = strings.TrimSpace(s.MatchFilters)
	out.BreakMatchFilters = strings.TrimSpace(s.BreakMatchFilters)
	return out
}

// InputFiltersActive reports whether any input filter is on.
func InputFiltersActive(s config.InputFilterSettings) bool {
	f := InputFilters(s)
	return f.MinFilesize != "" || f.MaxFilesize != "" || f.Date != "" || f.DateBefore != "" ||
		f.DateAfter != "" || f.MatchFilters != "" || f.BreakMatchFilters != ""
}

var unitSuffix = map[string]string{"B": "B", "KB": "K", "MB": "M", "GB": "G", "TB": "T"}

func sizeArg(f config.SizeFilter) string {
	if f.Value == nil || math.IsNaN(*f.Value) || math.IsInf(*f.Value, 0) || *f.Value <= 0 {
		return ""
	}
	suffix, ok := unitSuffix[f.Unit]
	if !ok {
		suffix = "M"
	}
	rounded := math.Round(*f.Value*100) / 100
	return strconv.FormatFloat(rounded, 'f', -1, 64) + suffix
}

var dateValue = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func dateArg(v string) string {
	v = strings.TrimSpace(v)
	if !dateValue.MatchString(v) {
		return ""
	}
	return strings.ReplaceAll(v, "-", "")
}

// DatePreset returns a date filter for a preset ("today", "yesterday",
// "last7Days", "last30Days") relative to now's calendar date.
func DatePreset(preset string, now time.Time) config.DateFilter {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	f := func(d time.Time) string { return d.Format("2006-01-02") }
	switch preset {
	case "yesterday":
		return config.DateFilter{Mode: "exact", Value: f(day.AddDate(0, 0, -1))}
	case "last7Days":
		return config.DateFilter{Mode: "after", Value: f(day.AddDate(0, 0, -7))}
	case "last30Days":
		return config.DateFilter{Mode: "after", Value: f(day.AddDate(0, 0, -30))}
	}
	return config.DateFilter{Mode: "exact", Value: f(day)}
}

// SelectionRow is one row of a playlist selection: a single position or a
// range. Positions count from 1; negative ones count from the end; 0 is
// unset.
type SelectionRow struct {
	Range      bool
	Index      int // single
	Start, End int // range
}

func (r SelectionRow) empty() bool {
	if r.Range {
		return r.Start == 0 && r.End == 0
	}
	return r.Index == 0
}

// Valid reports whether a row is complete; an empty row is not an error.
func (r SelectionRow) Valid() bool {
	if r.empty() {
		return true
	}
	if r.Range {
		return r.Start != 0 && r.End != 0
	}
	return true
}

// PlaylistItemsSpec is yt-dlp's -I value for rows ("1,3:5,-2:-1"), or ""
// for all items.
func PlaylistItemsSpec(rows []SelectionRow) string {
	var specs []string
	for _, r := range rows {
		switch {
		case r.empty() || !r.Valid():
		case r.Range:
			specs = append(specs, strconv.Itoa(r.Start)+":"+strconv.Itoa(r.End))
		default:
			specs = append(specs, strconv.Itoa(r.Index))
		}
	}
	return strings.Join(specs, ",")
}

// SelectEntries applies rows to entries, in row order without repeats; no
// rows select everything. Ranges may run backwards.
func SelectEntries(entries []ytdlp.PlaylistEntry, rows []SelectionRow) []ytdlp.PlaylistEntry {
	active := false
	for _, r := range rows {
		active = active || (!r.empty() && r.Valid())
	}
	if !active {
		return append([]ytdlp.PlaylistEntry(nil), entries...)
	}
	position := func(v int) int {
		if v == 0 {
			return -1
		}
		p := v - 1
		if v < 0 {
			p = len(entries) + v
		}
		if p < 0 || p >= len(entries) {
			return -1
		}
		return p
	}
	seen := map[int]bool{}
	var out []ytdlp.PlaylistEntry
	add := func(p int) {
		if p >= 0 && !seen[entries[p].Index] {
			seen[entries[p].Index] = true
			out = append(out, entries[p])
		}
	}
	for _, r := range rows {
		if r.empty() || !r.Valid() {
			continue
		}
		if !r.Range {
			add(position(r.Index))
			continue
		}
		start, end := position(r.Start), position(r.End)
		if start < 0 || end < 0 {
			continue
		}
		step := 1
		if start > end {
			step = -1
		}
		for p := start; ; p += step {
			add(p)
			if p == end {
				break
			}
		}
	}
	return out
}

// PlaylistIndex is the 1-based number of a 0-based entry index, counted
// from the end when reverse is set and the count is known.
func PlaylistIndex(index, count int, reverse bool) int {
	n := index + 1
	if !reverse || count <= 0 || count < n {
		return n
	}
	return count - n + 1
}

// ParseURLs splits pasted text on spaces and commas into unique http(s)
// links, counting the other words as skipped.
func ParseURLs(text string) (urls []string, skipped int) {
	for _, token := range splitTokens(text) {
		if ValidURL(token) {
			urls = appendUnique(urls, token)
		} else {
			skipped++
		}
	}
	return urls, skipped
}

var urlInText = regexp.MustCompile(`(?i)https?://[^\s<>"'` + "`" + `]+`)

// ParseURLFile finds the http(s) links in a .txt or .csv file, trimming
// trailing punctuation, and counts link-like words that are not valid.
func ParseURLFile(text string) (urls []string, skipped int) {
	for _, m := range urlInText.FindAllString(text, -1) {
		if c := trimURL(m); ValidURL(c) {
			urls = appendUnique(urls, c)
		}
	}
	for _, token := range splitTokens(text) {
		if (strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://")) && !ValidURL(trimURL(token)) {
			skipped++
		}
	}
	return urls, skipped
}

// ValidURL reports whether s is an absolute http or https URL.
func ValidURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func trimURL(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "),.;!?")
}

func splitTokens(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
	})
}

func appendUnique(items []string, s string) []string {
	for _, item := range items {
		if item == s {
			return items
		}
	}
	return append(items, s)
}
