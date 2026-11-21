// Package formats turns what the user picks (a quality preset, a playlist
// range, filter settings, pasted text) into what the engine and yt-dlp
// need. It holds the logic of the Tauri app's front-end helpers.
package formats

import (
	"fmt"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// Preset is a quality choice offered beside the link input.
type Preset struct {
	ID    string
	Label string
	// MaxHeight bounds video presets; 0 is the best available.
	MaxHeight int
	Audio     bool
	// AudioFormat forces the output audio format ("mp3"); "" uses the
	// setting.
	AudioFormat string
	Custom      bool
}

// Preset IDs, stored in settings.input.defaultPreset.
const (
	PresetBest      = "best"
	PresetAudio     = "audio"
	PresetAudioMP3  = "audio-mp3"
	PresetCustom    = "custom"
	presetHeightFmt = "%dp"
)

// Presets in menu order.
var Presets = []Preset{
	{ID: PresetBest, Label: "最佳画质"},
	{ID: "2160p", Label: "2160p (4K)", MaxHeight: 2160},
	{ID: "1440p", Label: "1440p (2K)", MaxHeight: 1440},
	{ID: "1080p", Label: "1080p", MaxHeight: 1080},
	{ID: "720p", Label: "720p", MaxHeight: 720},
	{ID: "480p", Label: "480p", MaxHeight: 480},
	{ID: PresetAudio, Label: "仅音频（最佳）", Audio: true},
	{ID: PresetAudioMP3, Label: "仅音频 MP3", Audio: true, AudioFormat: "mp3"},
	{ID: PresetCustom, Label: "自定义…", Custom: true},
}

// PresetByID returns a preset, or the best quality one for an unknown ID.
func PresetByID(id string) Preset {
	for _, p := range Presets {
		if p.ID == id {
			return p
		}
	}
	return Presets[0]
}

// PresetLabels are the labels in menu order, for a select.
func PresetLabels() []string {
	out := make([]string, len(Presets))
	for i, p := range Presets {
		out[i] = p.Label
	}
	return out
}

// PresetByLabel returns the preset with a label, or the best quality one.
func PresetByLabel(label string) Preset {
	for _, p := range Presets {
		if p.Label == label {
			return p
		}
	}
	return Presets[0]
}

// Choice is a resolved quality for one video: what yt-dlp gets, the format
// it matched (for the card) and an estimated size.
type Choice struct {
	Format  ytdlp.FormatOptions
	Matched *ytdlp.Format // nil for "best audio" or when nothing matched
	Size    int64         // estimated bytes, 0 when unknown
	// AudioFormat overrides the output audio format, from the preset.
	AudioFormat string
}

// Resolve matches a preset against a video's formats. Video presets take
// the highest format at or below their height (frame rate as high as
// possible), or the lowest format when all are higher; best quality takes
// the highest. yt-dlp gets the matched height and frame rate as upper
// bounds, so it still picks the best streams within them.
func Resolve(p Preset, formats []ytdlp.Format) Choice {
	if p.Audio {
		c := Choice{Format: ytdlp.FormatOptions{TrackType: ytdlp.TrackAudio}, AudioFormat: p.AudioFormat}
		if best := bestAudio(formats); best != nil {
			c.Size = best.Filesize
		}
		return c
	}
	c := Choice{Format: ytdlp.FormatOptions{TrackType: ytdlp.TrackBoth}}
	matched := matchVideo(formats, p.MaxHeight)
	if matched == nil {
		return c
	}
	c.Matched = matched
	if p.MaxHeight > 0 {
		c.Format.Height, c.Format.Fps = matched.Height, matched.Fps
	}
	if matched.Filesize > 0 {
		c.Size = matched.Filesize
		if a := bestAudio(formats); a != nil {
			c.Size += a.Filesize
		}
	}
	return c
}

// ResolveCustom describes an explicit format: the format it names (for the
// card's label) and an estimated size. A video format with no height is
// the best available.
func ResolveCustom(f ytdlp.FormatOptions, formats []ytdlp.Format) Choice {
	c := Choice{Format: f}
	audio := bestAudio(formats)
	switch {
	case f.TrackType == ytdlp.TrackAudio:
		for i := range formats {
			if formats[i].Height == 0 && formats[i].Abr == f.Abr && f.Abr > 0 {
				c.Matched = &formats[i]
			}
		}
		if c.Matched == nil {
			c.Matched = audio
		}
		if c.Matched != nil {
			c.Size = c.Matched.Filesize
		}
		return c
	case f.Height > 0:
		for i := range formats {
			if formats[i].Height == f.Height && (f.Fps == 0 || formats[i].Fps == f.Fps) {
				c.Matched = &formats[i]
				break
			}
		}
	default:
		c.Matched = matchVideo(formats, 0)
	}
	if c.Matched != nil && c.Matched.Filesize > 0 {
		c.Size = c.Matched.Filesize
		if audio != nil && f.TrackType == ytdlp.TrackBoth {
			c.Size += audio.Filesize
		}
	}
	return c
}

func matchVideo(formats []ytdlp.Format, maxHeight int) *ytdlp.Format {
	var best, lowest *ytdlp.Format
	for i := range formats {
		f := &formats[i]
		if f.Height <= 0 {
			continue
		}
		if lowest == nil || f.Height < lowest.Height || (f.Height == lowest.Height && f.Fps > lowest.Fps) {
			lowest = f
		}
		if maxHeight > 0 && f.Height > maxHeight {
			continue
		}
		if best == nil || f.Height > best.Height || (f.Height == best.Height && f.Fps > best.Fps) {
			best = f
		}
	}
	if best == nil {
		return lowest
	}
	return best
}

func bestAudio(formats []ytdlp.Format) *ytdlp.Format {
	var best *ytdlp.Format
	for i := range formats {
		f := &formats[i]
		if f.Height == 0 && f.Abr > 0 && (best == nil || f.Abr > best.Abr) {
			best = f
		}
	}
	return best
}

// Label describes a resolved choice for a card: "1080p60", "最佳音频".
func (c Choice) Label() string {
	switch {
	case c.Format.TrackType == ytdlp.TrackAudio && c.AudioFormat != "":
		return "音频 " + upper(c.AudioFormat)
	case c.Format.TrackType == ytdlp.TrackAudio && c.Matched != nil && c.Format.Abr > 0:
		return "音频 " + ResolutionLabel(*c.Matched)
	case c.Format.TrackType == ytdlp.TrackAudio:
		return "最佳音频"
	case c.Matched == nil:
		return "最佳画质"
	case c.Format.TrackType == ytdlp.TrackVideo:
		return ResolutionLabel(*c.Matched) + " 无声"
	}
	return ResolutionLabel(*c.Matched)
}

// ResolutionLabel names a format: "1080p60", "720p", "128kbps".
func ResolutionLabel(f ytdlp.Format) string {
	switch {
	case f.Height > 0 && f.Fps > 0 && f.Fps != 30 && f.Fps != 25 && f.Fps != 24:
		return fmt.Sprintf("%dp%d", f.Height, f.Fps)
	case f.Height > 0:
		return fmt.Sprintf(presetHeightFmt, f.Height)
	case f.Abr > 0:
		return fmt.Sprintf("%dkbps", f.Abr)
	}
	return f.ID
}

// ApplyAudioFormat returns output with the preset's audio format, which
// also needs re-encoding allowed.
func (c Choice) ApplyAudioFormat(output config.OutputSettings) config.OutputSettings {
	if c.AudioFormat != "" {
		output.Audio.Format = c.AudioFormat
		output.Audio.Policy = config.PolicyAllowReencode
	}
	return output
}

func upper(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'a' && ch <= 'z' {
			b[i] = ch - 'a' + 'A'
		}
	}
	return string(b)
}
