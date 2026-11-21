package ytdlp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pandyzhou/lumadl/internal/config"
)

// FormatArgs returns -f and -S (and -x for audio) for format, biased toward
// the output container or audio format so yt-dlp needs to convert less.
func FormatArgs(format FormatOptions, output config.OutputSettings) []string {
	audioPref := parseTrackPreference(format.AudioTrack)
	videoPref := parseTrackPreference(format.VideoTrack)
	if format.TrackType == TrackAudio {
		return []string{"-x", "-f", audioSelector(audioPref), "-S", audioSort(format, output, audioPref)}
	}
	filter := selectorFilter{heightLimit: format.Height, fpsLimit: format.Fps}
	var selector string
	if format.TrackType == TrackVideo {
		selector = videoOnlySelector(filter, videoPref)
	} else {
		selector = combinedSelector(filter, audioPref, videoPref)
	}
	return []string{"-f", selector, "-S", videoSort(format, output, audioPref, videoPref)}
}

type trackPreference struct {
	language string
	channels int
}

func parseTrackPreference(raw string) trackPreference {
	var pref trackPreference
	if strings.TrimSpace(raw) == "" || raw == "auto" {
		return pref
	}
	for _, part := range strings.Split(raw, "|") {
		if lang, ok := strings.CutPrefix(part, "lang:"); ok {
			if lang = strings.TrimSpace(lang); lang != "" {
				pref.language = lang
			}
		} else if ch, ok := strings.CutPrefix(part, "channels:"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(ch)); err == nil && n >= 0 {
				pref.channels = n
			}
		}
	}
	return pref
}

func audioSelector(pref trackPreference) string {
	if pref.language == "" {
		return "ba/best"
	}
	var parts []string
	for _, lang := range languageCandidates(pref.language) {
		parts = append(parts, "ba[language="+lang+"]")
	}
	return strings.Join(append(parts, "ba", "best"), "/")
}

func videoOnlySelector(filter selectorFilter, pref trackPreference) string {
	var parts []string
	if pref.language != "" {
		for _, lang := range languageCandidates(pref.language) {
			parts = append(parts, filter.boundedVariants("bv*[language="+lang+"]")...)
		}
		for _, lang := range languageCandidates(pref.language) {
			parts = append(parts, filter.boundedVariants("b[language="+lang+"]")...)
		}
	}
	parts = append(parts, filter.boundedVariants("bv")...)
	parts = append(parts, filter.boundedVariants("b")...)
	return strings.Join(parts, "/")
}

func combinedTail(f selectorFilter) []string {
	return []string{
		f.heightAndFps("bv*") + "+ba", f.heightAndFps("b"),
		f.height("bv*") + "+ba", f.height("b"),
		f.fps("bv*") + "+ba", f.fps("b"),
		"bv*+ba", "b",
	}
}

func combinedSelector(f selectorFilter, audioPref, videoPref trackPreference) string {
	var parts []string
	switch {
	case audioPref.language != "":
		for _, lang := range languageCandidates(audioPref.language) {
			ba := "+ba[language=" + lang + "]"
			b := "b[language=" + lang + "]"
			parts = append(parts,
				f.heightAndFps(b), f.heightAndFps("bv*")+ba, f.heightAndFps("bv")+ba,
				f.height(b), f.height("bv*")+ba, f.height("bv")+ba,
				f.fps(b), f.fps("bv*")+ba, f.fps("bv")+ba,
			)
		}
	case videoPref.language != "":
		for _, lang := range languageCandidates(videoPref.language) {
			bv := "bv*[language=" + lang + "]"
			b := "b[language=" + lang + "]"
			parts = append(parts,
				f.heightAndFps(bv)+"+ba", f.heightAndFps(b),
				f.height(bv)+"+ba", f.height(b),
				f.fps(bv)+"+ba", f.fps(b),
			)
		}
	}
	return strings.Join(append(parts, combinedTail(f)...), "/")
}

func audioSort(format FormatOptions, output config.OutputSettings, pref trackPreference) string {
	var sort formatSort
	sort.preferLanguage(pref.language)
	if pref.channels > 0 {
		sort.push(fmt.Sprintf("channels:%d", pref.channels))
	}
	if format.Abr > 0 {
		sort.push(fmt.Sprintf("abr~%d", format.Abr))
	}
	sort.pushPrefixed("acodec", format.AudioEncoding)
	switch output.Audio.Format {
	case "m4a", "aac":
		sort.push("aext:m4a", "acodec:aac")
	case "opus":
		sort.push("acodec:opus", "aext:webm")
	case "ogg":
		sort.push("aext:ogg", "acodec:vorbis")
	case "flac", "wav":
		sort.push("acodec:opus")
	default: // mp3
		sort.push("aext:mp3")
	}
	return sort.render()
}

func videoSort(format FormatOptions, output config.OutputSettings, audioPref, videoPref trackPreference) string {
	var sort formatSort
	lang := videoPref.language
	if lang == "" {
		lang = audioPref.language
	}
	sort.preferLanguage(lang)
	if format.Height > 0 {
		sort.push(fmt.Sprintf("height:%d", format.Height))
	}
	if format.Fps > 0 {
		sort.push(fmt.Sprintf("fps:%d", format.Fps))
	}
	mp4 := output.Video.Container == "mp4"
	if v := strings.TrimSpace(format.VideoEncoding); v != "" {
		sort.push("vcodec:" + v)
	} else if mp4 {
		sort.push("vcodec:avc1")
	}
	if format.TrackType == TrackBoth {
		if a := strings.TrimSpace(format.AudioEncoding); a != "" {
			sort.push("acodec:" + a)
		} else if mp4 {
			sort.push("acodec:aac")
		}
	}
	if mp4 {
		sort.push("vext:mp4", "vext:m4a")
	} else {
		sort.push("vext")
	}
	return sort.render()
}

// selectorFilter bounds a format selector by height and frame rate.
type selectorFilter struct{ heightLimit, fpsLimit int }

func (f selectorFilter) height(s string) string {
	if f.heightLimit > 0 {
		s += fmt.Sprintf("[height<=%d]", f.heightLimit)
	}
	return s
}

func (f selectorFilter) fps(s string) string {
	if f.fpsLimit > 0 {
		s += fmt.Sprintf("[fps<=%d]", f.fpsLimit)
	}
	return s
}

func (f selectorFilter) heightAndFps(s string) string { return f.fps(f.height(s)) }

// boundedVariants returns s bounded by height and fps, then by each alone
// (when both are set), then unbounded, without consecutive duplicates.
func (f selectorFilter) boundedVariants(s string) []string {
	var out []string
	if f.heightLimit > 0 || f.fpsLimit > 0 {
		out = append(out, f.heightAndFps(s))
	}
	if f.heightLimit > 0 && f.fpsLimit > 0 {
		out = append(out, f.height(s), f.fps(s))
	}
	out = append(out, s)
	dedup := out[:1]
	for _, v := range out[1:] {
		if v != dedup[len(dedup)-1] {
			dedup = append(dedup, v)
		}
	}
	return dedup
}

type formatSort struct{ fields []string }

func (s *formatSort) push(fields ...string) { s.fields = append(s.fields, fields...) }

func (s *formatSort) preferLanguage(lang string) {
	if lang != "" {
		s.push("lang:" + lang)
	} else {
		s.push("lang")
	}
}

func (s *formatSort) pushPrefixed(prefix, value string) {
	if value = strings.TrimSpace(value); value != "" {
		s.push(prefix + ":" + value)
	}
}

func (s *formatSort) render() string { return strings.Join(s.fields, ",") }

// languageCandidates returns a language and its base ("fr-FR", "fr").
func languageCandidates(language string) []string {
	lang := strings.TrimSpace(language)
	if lang == "" {
		return nil
	}
	out := []string{lang}
	base, _, _ := strings.Cut(strings.ReplaceAll(lang, "_", "-"), "-")
	base = strings.TrimSpace(base)
	if base != "" && !strings.EqualFold(base, lang) {
		out = append(out, base)
	}
	return out
}
