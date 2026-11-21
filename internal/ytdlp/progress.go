package ytdlp

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

// Category is what part of a download progress is about.
type Category string

// Categories.
const (
	CategoryVideo     Category = "video"
	CategoryAudio     Category = "audio"
	CategorySubtitles Category = "subtitles"
	CategoryThumbnail Category = "thumbnail"
	CategoryMetadata  Category = "metadata"
	CategoryOther     Category = "other"
)

// Stage is the phase a download is in.
type Stage string

// Stages, in the order a download usually goes through them.
const (
	StageInitializing Stage = "initializing"
	StageDownloading  Stage = "downloading"
	StageMerging      Stage = "merging"
	StageRemuxing     Stage = "remuxing"
	StageReencoding   Stage = "reencoding"
	StageFinalizing   Stage = "finalizing"
	StageTranslating  Stage = "translating" // the app's AI subtitle step
)

// Progress is one progress update. Nil fields are unknown.
type Progress struct {
	Category Category
	Percent  *float64 // 0–100
	SpeedBps *float64
	EtaSecs  *int64
}

// Destination is a file yt-dlp writes. Higher confidence wins when picking
// the final file: a converted or merged file over a downloaded part.
type Destination struct {
	Path       string
	Confidence int
	Merged     bool
}

// ProgressEvent is what one output line tells: any of a destination, a new
// stage and a progress update.
type ProgressEvent struct {
	Destination *Destination
	Stage       Stage // "" when unchanged
	Progress    *Progress
}

// ProgressParser follows a download's output line by line.
type ProgressParser struct {
	category Category
	stage    Stage
}

// NewProgressParser starts in the initializing stage, with the category the
// track type downloads first.
func NewProgressParser(trackType TrackType) *ProgressParser {
	category := CategoryVideo
	if trackType == TrackAudio {
		category = CategoryAudio
	}
	return &ProgressParser{category: category, stage: StageInitializing}
}

// Stage returns the current stage.
func (p *ProgressParser) Stage() Stage { return p.stage }

// ParseLine reads one line of yt-dlp output.
func (p *ProgressParser) ParseLine(line string) ProgressEvent {
	var ev ProgressEvent
	ev.Destination = parseDestination(line)

	switch {
	case strings.HasPrefix(line, "[VideoRemuxer]"):
		ev.Stage = p.enter(StageRemuxing)
		return ev
	case strings.HasPrefix(line, "[VideoConvertor]"):
		ev.Stage = p.enter(StageReencoding)
		return ev
	}

	if strings.Contains(line, "[download] Destination:") {
		_, path, _ := strings.Cut(strings.TrimSpace(line), "Destination:")
		p.category = categoryForExtension(strings.TrimPrefix(filepath.Ext(unquote(strings.TrimSpace(path))), "."))
		if stage := p.enter(StageDownloading); stage != "" {
			ev.Stage = stage
			return ev
		}
	}

	if progress := p.parseRaw(line); progress != nil {
		ev.Progress = progress
		return ev
	}

	if rest, ok := strings.CutPrefix(line, "[Merger] Merging formats into"); ok {
		ev.Destination = &Destination{Path: unquote(strings.TrimSpace(rest)), Confidence: 80, Merged: true}
	}
	if strings.HasPrefix(line, "[Merger]") {
		if stage := p.enter(StageMerging); stage != "" {
			ev.Stage = stage
			return ev
		}
	}
	for _, trigger := range []string{"[ffmpeg]", "[Fixup]", "Deleting original file"} {
		if strings.Contains(line, trigger) {
			ev.Stage = p.enter(StageFinalizing)
			break
		}
	}
	return ev
}

// enter moves to stage and returns it, or "" if already there.
func (p *ProgressParser) enter(stage Stage) Stage {
	if p.stage == stage {
		return ""
	}
	p.stage = stage
	return stage
}

// parseRaw reads a ProgressTemplate line: percent, percent string, speed,
// eta, downloaded, total, total estimate, fragment index, fragment count.
func (p *ProgressParser) parseRaw(line string) *Progress {
	raw, ok := strings.CutPrefix(strings.TrimRight(line, " \t"), "RAW|")
	if !ok {
		return nil
	}
	parts := strings.Split(raw, "|")
	for len(parts) < 9 {
		parts = append(parts, "")
	}
	percent := firstNonNil(parsePercent(parts[0]), parsePercent(parts[1]))
	speed := parseFloat(parts[2])
	eta := parseCount(parts[3])
	downloaded, total, estimate := parseCount(parts[4]), parseCount(parts[5]), parseCount(parts[6])
	fragIndex, fragCount := parseCount(parts[7]), parseCount(parts[8])

	if percent == nil {
		if t := firstNonNilInt(total, estimate); t != nil && downloaded != nil && *t > 0 {
			v := float64(*downloaded) / float64(*t) * 100
			percent = &v
		}
	}
	if percent == nil && fragIndex != nil && fragCount != nil && *fragCount > 0 {
		v := float64(*fragIndex) / float64(*fragCount) * 100
		percent = &v
	}
	if percent != nil {
		v := *percent
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = 0
		}
		v = math.Max(0, math.Min(100, v))
		percent = &v
	}
	if percent == nil && speed == nil && eta == nil {
		return nil
	}
	return &Progress{Category: p.category, Percent: percent, SpeedBps: speed, EtaSecs: eta}
}

func parseDestination(line string) *Destination {
	if rest, ok := strings.CutPrefix(line, "[download] "); ok {
		if path, _, found := strings.Cut(rest, " has already been downloaded"); found {
			if path = strings.TrimSpace(path); path != "" {
				return &Destination{Path: unquote(path), Confidence: 65}
			}
		}
	}
	if rest, ok := strings.CutPrefix(line, "[VideoRemuxer] Not remuxing media file "); ok {
		if start := strings.IndexByte(rest, '"'); start >= 0 {
			if end := strings.IndexByte(rest[start+1:], '"'); end >= 0 {
				if path := strings.TrimSpace(rest[start+1 : start+1+end]); path != "" {
					return &Destination{Path: path, Confidence: 90}
				}
			}
		}
	}
	_, rest, ok := strings.Cut(line, "Destination:")
	if !ok {
		return nil
	}
	tag, _, _ := strings.Cut(line, "]")
	tag = strings.TrimLeft(tag, "[")
	confidence := 50
	switch tag {
	case "VideoConvertor":
		confidence = 95
	case "VideoRemuxer":
		confidence = 90
	case "Merger", "ExtractAudio":
		confidence = 70
	case "download":
		confidence = 60
	}
	return &Destination{Path: unquote(strings.TrimSpace(rest)), Confidence: confidence}
}

func categoryForExtension(ext string) Category {
	switch strings.ToLower(ext) {
	case "mp4", "mkv", "webm", "flv", "mov", "avi", "m4v", "ts", "m2ts", "3gp":
		return CategoryVideo
	case "mp3", "m4a", "wav", "flac", "ogg", "opus", "mka", "aiff", "wma", "alac", "aac":
		return CategoryAudio
	case "vtt", "srt", "ass", "lrc", "ttml", "srv1", "srv2", "srv3":
		return CategorySubtitles
	case "jpg", "jpeg", "png", "webp", "bmp":
		return CategoryThumbnail
	case "json":
		return CategoryMetadata
	}
	return CategoryOther
}

// unquote removes one pair of surrounding double quotes.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func missing(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || strings.EqualFold(s, "na")
}

func parsePercent(s string) *float64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if missing(s) {
		return nil
	}
	return parseFloat(s)
}

func parseFloat(s string) *float64 {
	if missing(s) {
		return nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseCount reads a non-negative integer; yt-dlp prints some counters
// (eta, byte estimates of fragmented downloads) as floats, which round.
func parseCount(s string) *int64 {
	f := parseFloat(s)
	if f == nil || math.IsNaN(*f) || math.IsInf(*f, 0) || *f < 0 {
		return nil
	}
	v := int64(math.Round(*f))
	return &v
}

func firstNonNil(values ...*float64) *float64 {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func firstNonNilInt(values ...*int64) *int64 {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}
