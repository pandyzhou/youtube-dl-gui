package ytdlp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
)

// Errors for output settings that cannot be combined. The UI shows them
// in Chinese (see the diagnostics catalog); the text here is for logs.
var (
	ErrFps30NeedsReencode  = errors.New("The 30 fps preset requires re-encoding. Disable Keep original streams to use it.")
	ErrMp42NeedsMp4        = errors.New("The mp42 preset requires mp4 output.")
	ErrCustomNeedsReencode = errors.New("Custom video post-processing in re-encode mode requires re-encoding. Disable Keep original streams to use it.")
	ErrInvalidCustomArgs   = errors.New("Invalid custom ffmpeg arguments. Check your quotes and spacing.")
)

// OutputArgs returns the conversion, thumbnail and metadata options for a
// download of format with output's settings.
func OutputArgs(format FormatOptions, output config.OutputSettings) ([]string, error) {
	args := []string{"--output-na-placeholder", ""}
	if format.TrackType == TrackAudio {
		args = appendAudioOutput(args, format, output)
		custom, err := audioPostprocessArgs(output.Audio)
		if err != nil {
			return nil, err
		}
		args = append(args, custom...)
	} else {
		plan, err := resolveVideoPlan(output.Video)
		if err != nil {
			return nil, err
		}
		switch plan.operation {
		case opRemux:
			args = append(args, "--merge-output-format", plan.container, "--remux-video", plan.container)
		case opReencode:
			args = append(args, "--recode-video", plan.container)
		}
		if output.AddThumbnail {
			args = append(args, "--embed-thumbnail")
		}
		if plan.postprocessorArgs != "" {
			args = append(args, "--postprocessor-args", plan.postprocessorArgs)
		}
	}
	if output.AddMetadata {
		args = append(args, "--add-metadata")
	}
	if output.SaveThumbnail {
		args = append(args, "--write-thumbnail")
	}
	if output.RestrictFilenames {
		args = append(args, "--restrict-filenames")
	}
	return args, nil
}

func appendAudioOutput(args []string, format FormatOptions, output config.OutputSettings) []string {
	f := output.Audio.Format
	if output.Audio.Policy != config.PolicyNever {
		args = append(args, "--audio-format", audioFormatArg(f))
		if audioQualitySelectable(f) {
			quality := "0"
			if format.Abr > 0 {
				quality = fmt.Sprintf("%dk", format.Abr)
			}
			args = append(args, "--audio-quality", quality)
		}
	}
	if output.AddThumbnail && audioEmbedsThumbnail(f) {
		args = append(args, "--embed-thumbnail")
	}
	return args
}

// audioFormatArg maps a format to yt-dlp's --audio-format value; yt-dlp
// has no "ogg", "vorbis" is Vorbis in an .ogg file.
func audioFormatArg(f string) string {
	if f == "ogg" {
		return "vorbis"
	}
	return f
}

func audioEmbedsThumbnail(f string) bool {
	switch f {
	case "mp3", "m4a", "opus", "ogg", "flac":
		return true
	}
	return false
}

func audioQualitySelectable(f string) bool {
	switch f {
	case "mp3", "m4a", "aac", "ogg", "opus":
		return true
	}
	return false
}

type videoOperation int

const (
	opNone videoOperation = iota
	opRemux
	opReencode
)

type videoPlan struct {
	operation         videoOperation
	container         string
	postprocessorArgs string
}

func resolveVideoPlan(video config.VideoOutputSettings) (videoPlan, error) {
	container := video.Container
	if container != "mkv" {
		container = "mp4"
	}
	keepOriginal := video.Policy == config.PolicyNever
	base := videoPlan{operation: opRemux, container: container}
	if keepOriginal {
		base = videoPlan{operation: opNone}
	}
	switch video.PostprocessPreset {
	case "fps30":
		if keepOriginal {
			return videoPlan{}, ErrFps30NeedsReencode
		}
		return videoPlan{opReencode, container, "VideoConvertor+ffmpeg:-vf fps=30"}, nil
	case "mp42":
		if container != "mp4" {
			return videoPlan{}, ErrMp42NeedsMp4
		}
		return videoPlan{opRemux, container, "VideoRemuxer+ffmpeg:-brand mp42"}, nil
	case "custom":
		custom, err := customPostprocessArgs(video.PostprocessArgs)
		if err != nil || custom == "" {
			return base, err
		}
		if video.CustomPostprocessMode == "reencode" {
			if keepOriginal {
				return videoPlan{}, ErrCustomNeedsReencode
			}
			return videoPlan{opReencode, container, "VideoConvertor+ffmpeg:" + custom}, nil
		}
		return videoPlan{opRemux, container, "VideoRemuxer+ffmpeg:" + custom}, nil
	}
	return base, nil
}

func audioPostprocessArgs(audio config.AudioOutputSettings) ([]string, error) {
	if audio.PostprocessPreset != "custom" {
		return nil, nil
	}
	custom, err := customPostprocessArgs(audio.PostprocessArgs)
	if err != nil || custom == "" {
		return nil, err
	}
	return []string{"--postprocessor-args", "ffmpeg:" + custom}, nil
}

// customPostprocessArgs trims raw and checks that it splits as shell words
// (balanced quotes, no dangling escape); yt-dlp gets it as one string.
func customPostprocessArgs(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if !shellSplittable(trimmed) {
		return "", ErrInvalidCustomArgs
	}
	return trimmed, nil
}

// shellSplittable reports whether s splits into POSIX shell words.
func shellSplittable(s string) bool {
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case r == '\\':
			escaped = true
		case quote == '"':
			if r == '"' {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		}
	}
	return quote == 0 && !escaped
}

// InputFilterArgs returns the selection and filter options of filters.
func InputFilterArgs(filters InputFilterOptions) []string {
	var args []string
	push := func(flag, value string) {
		if value = strings.TrimSpace(value); value != "" {
			args = append(args, flag, value)
		}
	}
	push("-I", filters.PlaylistItems)
	push("--min-filesize", filters.MinFilesize)
	push("--max-filesize", filters.MaxFilesize)
	push("--date", filters.Date)
	push("--datebefore", filters.DateBefore)
	push("--dateafter", filters.DateAfter)
	push("--match-filters", filters.MatchFilters)
	push("--break-match-filters", filters.BreakMatchFilters)
	switch filters.PlaylistMode {
	case PlaylistModeSingle:
		args = append(args, "--no-playlist")
	case PlaylistModeAll:
		args = append(args, "--yes-playlist")
	}
	return args
}

// InputArgs chooses between the video and the playlist of a link that has
// both: a forced mode wins, then the "prefer the video" setting.
func InputArgs(mode PlaylistMode, input config.InputSettings) []string {
	switch {
	case mode == PlaylistModeSingle:
		return []string{"--no-playlist"}
	case mode == PlaylistModeAll:
		return []string{"--yes-playlist"}
	case input.PreferVideoInMixedLinks:
		return []string{"--no-playlist"}
	}
	return []string{"--yes-playlist"}
}
