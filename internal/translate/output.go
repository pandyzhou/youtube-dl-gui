package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// Where the bilingual subtitles go (settings.translation.output).
const (
	// OutputEmbed puts them into the video as subtitle tracks.
	OutputEmbed = "embed"
	// OutputFiles writes .ass and .srt files beside the video.
	OutputFiles = "files"
)

// Titles of the tracks the app embeds; a later run replaces them.
const (
	trackStyled = "中英双语（描边）"
	trackPlain  = "中英双语（纯文本）"
)

var ourTracks = []string{trackStyled, trackPlain}

// CodeEmbedFailed warns that the subtitles went to files instead.
const CodeEmbedFailed = "translationEmbedFailed"

// deliver puts the finished subtitles where the settings say and returns
// where they went. Embedding that cannot be done falls back to files.
func deliver(ctx context.Context, env Env, video string, cues []Cue, translations []string, r engine.Reporter) (string, error) {
	ext := strings.ToLower(filepath.Ext(video))
	styled := ext == ".mkv"
	plainOnly := ext == ".mp4" || ext == ".m4v" || ext == ".mov"
	probe := probeVideo(ctx, env.FFprobe, video)
	if env.Settings.Translation.Output != OutputFiles {
		switch {
		case env.FFmpeg == "":
			r.Log("[translate] ffmpeg is missing, writing subtitle files instead")
		case !styled && !plainOnly:
			r.Log("[translate] " + ext + " files cannot hold these subtitles, writing subtitle files instead")
		default:
			err := embed(ctx, env, video, probe, styled, cues, translations, r)
			if err == nil {
				return video, nil
			}
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			r.Log("[translate] Could not put the subtitles into the video: " + err.Error())
			r.Warn(CodeEmbedFailed, "没能把字幕放进视频（"+truncate(err.Error(), 120)+"），已在视频旁边保存为字幕文件。")
		}
	}
	base := strings.TrimSuffix(video, filepath.Ext(video))
	if err := os.WriteFile(base+".srt", []byte(WriteBilingual(cues, translations)), 0o644); err != nil {
		return "", &failure{CodeFailed, "无法保存字幕：" + err.Error()}
	}
	if err := os.WriteFile(base+".ass", []byte(WriteASS(cues, translations, probe.Width, probe.Height)), 0o644); err != nil {
		return "", &failure{CodeFailed, "无法保存字幕：" + err.Error()}
	}
	return base + ".ass", nil
}

// embed adds the subtitles to the video as tracks, without re-encoding:
// MKV gets a styled ASS track (shown by default) and a plain SRT one, MP4
// a plain text track. Tracks from an earlier run are replaced. The video
// is copied to a temporary file beside it that then takes its place.
func embed(ctx context.Context, env Env, video string, probe videoProbe, styled bool, cues []Cue, translations []string, r engine.Reporter) error {
	dir, err := os.MkdirTemp("", "ovd-subs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	ass, srt := filepath.Join(dir, "subs.ass"), filepath.Join(dir, "subs.srt")
	if err := os.WriteFile(ass, []byte(WriteASS(cues, translations, probe.Width, probe.Height)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(srt, []byte(WriteBilingual(cues, translations)), 0o644); err != nil {
		return err
	}
	ext := filepath.Ext(video)
	out := strings.TrimSuffix(video, ext) + ".subtitled" + ext
	r.Stage(ytdlp.StageEmbedding)
	r.Log("[translate] Putting the subtitles into the video")
	var lastErr string
	code, err := proc.Run(ctx, proc.Spec{Path: env.FFmpeg, Args: embedArgs(video, ass, srt, out, probe, styled)}, func(l proc.Line) {
		if l.Stderr {
			if t := strings.TrimSpace(l.Text); t != "" {
				lastErr = t
			}
			return
		}
		if us, ok := strings.CutPrefix(l.Text, "out_time_us="); ok && probe.Duration > 0 {
			if n, err := strconv.ParseFloat(strings.TrimSpace(us), 64); err == nil && n >= 0 {
				r.Progress(min(n/1e6/probe.Duration*100, 100))
			}
		}
	})
	if err == nil && code != 0 {
		err = fmt.Errorf("ffmpeg 退出代码 %d：%s", code, lastErr)
	}
	if err != nil {
		os.Remove(out)
		return err
	}
	if err := os.Rename(out, video); err != nil {
		os.Remove(out)
		return fmt.Errorf("无法替换原视频（是否正在播放？）：%w", err)
	}
	r.Log("[translate] The subtitles are in the video")
	return nil
}

// embedArgs are ffmpeg's arguments for embed.
func embedArgs(video, ass, srt, out string, probe videoProbe, styled bool) []string {
	args := []string{"-hide_banner", "-nostdin", "-y", "-v", "error", "-progress", "pipe:1", "-nostats", "-i", video}
	if styled {
		args = append(args, "-i", ass, "-i", srt)
	} else {
		args = append(args, "-i", srt)
	}
	args = append(args, "-map", "0")
	kept := 0
	for _, s := range probe.Subtitles {
		if isOurTrack(s.Title) {
			args = append(args, "-map", fmt.Sprintf("-0:%d", s.Index))
		} else {
			kept++
		}
	}
	args = append(args, "-map", "1")
	if styled {
		args = append(args, "-map", "2")
	}
	args = append(args, "-c", "copy")
	for i := range kept {
		args = append(args, fmt.Sprintf("-disposition:s:%d", i), "0")
	}
	track := func(i int, title, disposition string) {
		s := fmt.Sprintf(":s:s:%d", i)
		args = append(args, "-metadata"+s, "language=chi", "-metadata"+s, "title="+title,
			fmt.Sprintf("-disposition:s:%d", i), disposition)
	}
	if styled {
		track(kept, trackStyled, "default")
		track(kept+1, trackPlain, "0")
	} else {
		args = append(args, fmt.Sprintf("-c:s:%d", kept), "mov_text")
		track(kept, trackPlain, "default")
	}
	// With the default limit ffmpeg 6.1 wrote nearly all of the new
	// subtitles at the end of the file, where players do not find them
	// while playing; without a limit they sit beside the video they go with.
	return append(args, "-max_interleave_delta", "0", out)
}

func isOurTrack(title string) bool {
	for _, t := range ourTracks {
		if title == t {
			return true
		}
	}
	return title == "中英双语"
}

// videoProbe is what embedding needs to know about a video.
type videoProbe struct {
	Width, Height int
	Duration      float64
	Subtitles     []probeStream
}

type probeStream struct {
	Index int
	Title string
}

// probeVideo reads a video's size, length and subtitle tracks; what it
// cannot read stays zero.
func probeVideo(ctx context.Context, ffprobe, video string) videoProbe {
	if ffprobe == "" {
		return videoProbe{}
	}
	var out strings.Builder
	_, _ = proc.Run(ctx, proc.Spec{Path: ffprobe, Args: []string{
		"-v", "error", "-show_entries", "stream=index,codec_type,width,height:stream_tags=title:format=duration",
		"-of", "json", video,
	}}, func(l proc.Line) {
		if !l.Stderr {
			out.WriteString(l.Text + "\n")
		}
	})
	return parseProbe([]byte(out.String()))
}

func parseProbe(data []byte) videoProbe {
	var doc struct {
		Streams []struct {
			Index  int               `json:"index"`
			Type   string            `json:"codec_type"`
			Width  int               `json:"width"`
			Height int               `json:"height"`
			Tags   map[string]string `json:"tags"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	var p videoProbe
	if json.Unmarshal(data, &doc) != nil {
		return p
	}
	p.Duration, _ = strconv.ParseFloat(doc.Format.Duration, 64)
	for _, s := range doc.Streams {
		switch {
		case s.Type == "video" && s.Width > 0 && p.Width == 0:
			p.Width, p.Height = s.Width, s.Height
		case s.Type == "subtitle":
			title := s.Tags["title"]
			if title == "" {
				title = s.Tags["TITLE"]
			}
			p.Subtitles = append(p.Subtitles, probeStream{Index: s.Index, Title: title})
		}
	}
	return p
}

// The ASS styles: sizes are for a 1440-pixel-high script and scale with
// the video, so subtitles look the same at 720p and 4K. The Box styles
// are the same text, invisible, on a dark see-through box (border style
// 3), drawn under the outlined text so it reads over text in the video.
const assStyles = `Style: ZH,Microsoft YaHei,76,&H00FFFFFF,&H000000FF,&H00000000,&H96000000,-1,0,0,0,100,100,1,0,1,4.5,2,2,%[1]d,%[1]d,70,1
Style: EN,Arial,50,&H00D8F0FF,&H000000FF,&H00000000,&H96000000,0,0,0,0,100,100,0,0,1,3.5,1.5,2,%[1]d,%[1]d,70,1
Style: ZHBox,Microsoft YaHei,76,&HFF000000,&HFF000000,&H40000000,&HFF000000,-1,0,0,0,100,100,1,0,3,10,0,2,%[1]d,%[1]d,70,1
Style: ENBox,Arial,50,&HFF000000,&HFF000000,&H40000000,&HFF000000,0,0,0,0,100,100,0,0,3,10,0,2,%[1]d,%[1]d,70,1`

// WriteASS writes cues as styled ASS: the translation in large white
// letters with a black outline, the original smaller under it, both on a
// dark box. Each cue is two events: the box on layer 0 and the text on
// layer 1, which lay out alike and so sit on each other.
func WriteASS(cues []Cue, translations []string, width, height int) string {
	playX := 2560
	if width > 0 && height > 0 {
		playX = int(float64(width)/float64(height)*1440 + 0.5)
	}
	var b strings.Builder
	b.WriteString("\ufeff[Script Info]\nScriptType: v4.00+\n")
	fmt.Fprintf(&b, "PlayResX: %d\nPlayResY: 1440\nWrapStyle: 0\nScaledBorderAndShadow: yes\nYCbCr Matrix: TV.709\n\n", playX)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, " +
		"Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, " +
		"MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, assStyles+"\n\n", playX*120/2560)
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for i, cue := range cues {
		start, end, ok := assTimes(cue.Timing)
		if !ok {
			continue
		}
		original := assText(cue.Text)
		text, box, style := original, original, "EN"
		if i < len(translations) && strings.TrimSpace(translations[i]) != "" {
			translation := assText(translations[i])
			text, box, style = translation+`\N{\rEN}`+original, translation+`\N{\rENBox}`+original, "ZH"
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,%sBox,,0,0,0,,%s\n", start, end, style, box)
		fmt.Fprintf(&b, "Dialogue: 1,%s,%s,%s,,0,0,0,,%s\n", start, end, style, text)
	}
	return b.String()
}

var srtTiming = regexp.MustCompile(`(\d+):(\d+):(\d+)[,.](\d+)\s*-->\s*(\d+):(\d+):(\d+)[,.](\d+)`)

// assTimes turns an SRT timing line into ASS start and end times.
func assTimes(timing string) (string, string, bool) {
	m := srtTiming.FindStringSubmatch(timing)
	if m == nil {
		return "", "", false
	}
	format := func(h, mm, s, ms string) string {
		hours, _ := strconv.Atoi(h)
		millis, _ := strconv.Atoi(ms)
		return fmt.Sprintf("%d:%s:%s.%02d", hours, mm, s, millis/10)
	}
	return format(m[1], m[2], m[3], m[4]), format(m[5], m[6], m[7], m[8]), true
}

// assText keeps text from being read as override tags.
func assText(text string) string {
	text = strings.NewReplacer("{", "(", "}", ")", "\r", "").Replace(strings.TrimSpace(text))
	return strings.ReplaceAll(text, "\n", `\N`)
}
