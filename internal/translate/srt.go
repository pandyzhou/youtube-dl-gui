package translate

import (
	"strconv"
	"strings"
)

// Cue is one subtitle: its timing line, kept verbatim, and its text.
type Cue struct {
	Timing string
	Text   string
}

// ParseSRT reads SubRip content. Cue numbers are ignored; blocks without
// a timing line or text are skipped.
func ParseSRT(content string) []Cue {
	content = strings.TrimPrefix(content, "﻿")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var cues []Cue
	var block []string
	flush := func() {
		for i, line := range block {
			if !strings.Contains(line, "-->") {
				continue
			}
			text := strings.Join(block[i+1:], "\n")
			if strings.TrimSpace(text) != "" {
				cues = append(cues, Cue{Timing: strings.TrimSpace(line), Text: text})
			}
			break
		}
		block = block[:0]
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
		} else {
			block = append(block, line)
		}
	}
	flush()
	return cues
}

// WriteBilingual writes cues as numbered SRT with a byte order mark; a
// non-empty translations[i] goes on the line above cue i's original text.
func WriteBilingual(cues []Cue, translations []string) string {
	var b strings.Builder
	b.WriteString("﻿")
	for i, cue := range cues {
		b.WriteString(strconv.Itoa(i+1) + "\n" + cue.Timing + "\n")
		if i < len(translations) {
			if t := strings.TrimSpace(translations[i]); t != "" {
				b.WriteString(t + "\n")
			}
		}
		b.WriteString(cue.Text + "\n\n")
	}
	return b.String()
}
