package translate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// YouTube's automatic captions come as json3 with a start time for every
// word. As SRT they lose those times and turn into rolling fragments that
// repeat the line before, so they are read as words here and regrouped
// into short sentences, each timed from its first word to its last.

// Word is one word of automatic captions, in milliseconds.
type Word struct {
	Start, End int
	Text       string
}

// Segmentation tuning; weights count words, or two CJK characters as one.
const (
	// pauseMs always ends a line.
	pauseMs = 1000
	// softPauseMs or a comma ends a line of at least softWeight.
	softPauseMs = 400
	softWeight  = 8
	// maxWeight ends a line however it goes on.
	maxWeight = 15
	// Lines of at most shortWeight join a neighbour up to mergeWeight.
	shortWeight = 2
	mergeWeight = 20
	// minCueMs is the shortest a line stays on screen when there is room.
	minCueMs = 800
	// lingerMs keeps a line up after its last word before a pause.
	lingerMs = 500
)

// ParseJSON3 reads the words of json3 captions in time order. A word ends
// when the next begins, at the latest when its event ends, and no later
// than it takes to say.
func ParseJSON3(data []byte) ([]Word, error) {
	var doc struct {
		Events []struct {
			Start    int `json:"tStartMs"`
			Duration int `json:"dDurationMs"`
			Append   int `json:"aAppend"`
			Segs     []struct {
				Text   string `json:"utf8"`
				Offset int    `json:"tOffsetMs"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var words []Word
	for _, e := range doc.Events {
		if e.Append != 0 {
			continue
		}
		for _, s := range e.Segs {
			if text := strings.Join(strings.Fields(s.Text), " "); text != "" {
				words = append(words, Word{Start: e.Start + s.Offset, End: e.Start + e.Duration, Text: text})
			}
		}
	}
	sort.SliceStable(words, func(a, b int) bool { return words[a].Start < words[b].Start })
	for i := range words {
		w := &words[i]
		w.End = min(w.End, w.Start+300+60*len([]rune(w.Text)))
		if i+1 < len(words) && words[i+1].Start > w.Start {
			w.End = min(w.End, words[i+1].Start)
		}
		w.End = max(w.End, w.Start+1)
	}
	return words, nil
}

// Segment groups words into subtitle lines: a line ends at the end of a
// sentence, at a pause, around a sound tag such as [Music] or when it gets
// long, and very short lines join a neighbour.
func Segment(words []Word) []Cue {
	var groups [][]Word
	var cur []Word
	weight := 0
	flush := func() {
		if len(cur) > 0 {
			groups = append(groups, cur)
		}
		cur, weight = nil, 0
	}
	for i, w := range words {
		if isTag(w.Text) {
			flush()
		}
		cur = append(cur, w)
		weight += wordWeight(w.Text)
		gap := pauseMs
		if i+1 < len(words) {
			gap = words[i+1].Start - w.End
			if isTag(words[i+1].Text) {
				gap = pauseMs
			}
		}
		if isTag(w.Text) || endsSentence(w.Text) || gap >= pauseMs || weight >= maxWeight ||
			weight >= softWeight && (gap >= softPauseMs || endsClause(w.Text)) {
			flush()
		}
	}
	flush()
	groups = mergeShort(groups)

	cues := make([]Cue, len(groups))
	for i, g := range groups {
		start, end := g[0].Start, g[len(g)-1].End
		next := -1
		if i+1 < len(groups) {
			next = groups[i+1][0].Start
		}
		switch {
		case next >= 0 && next-end < pauseMs:
			end = next
		case next >= 0:
			end = min(end+lingerMs, next)
		default:
			end += lingerMs
		}
		if end-start < minCueMs && (next < 0 || start+minCueMs <= next) {
			end = start + minCueMs
		}
		cues[i] = Cue{Timing: srtTime(start) + " --> " + srtTime(end), Text: joinWords(g)}
	}
	return cues
}

// mergeShort joins lines of a word or two to the line before, or else the
// line after, when they belong together.
func mergeShort(groups [][]Word) [][]Word {
	short := func(g []Word) bool { return groupWeight(g) <= shortWeight && !isTag(g[0].Text) }
	fits := func(a, b []Word) bool {
		last := a[len(a)-1]
		return !isTag(a[0].Text) && !isTag(b[0].Text) && !endsSentence(last.Text) &&
			b[0].Start-last.End < pauseMs && groupWeight(a)+groupWeight(b) <= mergeWeight
	}
	var out [][]Word
	for _, g := range groups {
		if n := len(out); n > 0 && short(g) && fits(out[n-1], g) {
			out[n-1] = append(out[n-1], g...)
			continue
		}
		out = append(out, g)
	}
	var merged [][]Word
	for i := 0; i < len(out); i++ {
		if g := out[i]; short(g) && i+1 < len(out) && fits(g, out[i+1]) {
			out[i+1] = append(append([]Word(nil), g...), out[i+1]...)
			continue
		}
		merged = append(merged, out[i])
	}
	return merged
}

func groupWeight(g []Word) int {
	n := 0
	for _, w := range g {
		n += wordWeight(w.Text)
	}
	return n
}

// wordWeight counts words, and two CJK characters as one word.
func wordWeight(text string) int {
	cjk, other := 0, 0
	for _, field := range strings.Fields(text) {
		latin := false
		for _, r := range field {
			if isCJK(r) {
				cjk++
			} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
				latin = true
			}
		}
		if latin {
			other++
		}
	}
	return max(other+(cjk+1)/2, 1)
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

func isTag(text string) bool {
	return strings.HasPrefix(text, "[") || strings.HasPrefix(text, "(") || strings.HasPrefix(text, "♪")
}

func lastRune(text string) rune {
	text = strings.TrimRight(text, `"'”’)]」』`)
	r := []rune(text)
	if len(r) == 0 {
		return 0
	}
	return r[len(r)-1]
}

func endsSentence(text string) bool { return strings.ContainsRune(".?!…。？！", lastRune(text)) }

func endsClause(text string) bool { return strings.ContainsRune(",;:，；：、", lastRune(text)) }

// joinWords writes words with spaces between them, except between CJK
// characters.
func joinWords(words []Word) string {
	var b strings.Builder
	for i, w := range words {
		if i > 0 {
			prev := []rune(words[i-1].Text)
			if !isCJK(prev[len(prev)-1]) || !isCJK([]rune(w.Text)[0]) {
				b.WriteByte(' ')
			}
		}
		b.WriteString(w.Text)
	}
	return b.String()
}

func srtTime(ms int) string {
	ms = max(ms, 0)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
