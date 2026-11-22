package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// One-step downloads. yt-dlp merges the video and audio, then embeds
// subtitles, the cover and the metadata one ffmpeg run at a time, each
// copying the whole file, and the translated subtitles took one more. When
// the subtitles go into an MKV anyway, the engine instead has yt-dlp only
// download the video, the audio, the subtitles and the cover as separate
// files, and merges them all, the translated subtitles included, in a
// single ffmpeg run.

// Track is a subtitle file to merge into a video.
type Track struct {
	Path     string
	Title    string
	Language string
	Default  bool
}

// oneStep reports whether an item downloads in one step.
func (e *Engine) oneStep(j *job, it *item, track ytdlp.TrackType, settings config.Settings) bool {
	if e.d.FFmpeg == nil || e.d.MergeTracks == nil || e.d.OneStep == nil || !e.d.OneStep() || it.video == nil {
		return false
	}
	v := settings.Output.Video
	return track == ytdlp.TrackBoth && choice(j, it.video).AudioFormat == "" && v.Container == "mkv" &&
		(v.PostprocessPreset == "" || v.PostprocessPreset == "none")
}

// downloadOneStep downloads an item in one step and reports whether it
// did; false leaves the item to the usual download, as for a video that
// comes as a single file.
func (e *Engine) downloadOneStep(ctx context.Context, j *job, it *item, args []string, track ytdlp.TrackType, settings config.Settings, early *earlyWork) bool {
	ids, filename, ok := e.selectFormats(ctx, args)
	if ctx.Err() != nil {
		return false // the usual path ends as paused
	}
	if !ok {
		return false
	}
	dest := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".mkv"
	parts := filepath.Join(filepath.Dir(dest), ".ovd-parts-"+safeName(it.video.ID+"-"+strings.Join(ids, "+")))
	if err := os.MkdirAll(parts, 0o755); err != nil {
		e.appendLog(j, "[one-step] "+err.Error())
		return false
	}
	e.appendLog(j, fmt.Sprintf("[one-step] Downloading %s and merging it with the subtitles in one pass", strings.Join(ids, "+")))

	auth := ytdlp.AuthArgs(settings.Auth, e.secrets())
	code, err := e.runYtdlp(ctx, j, it, partArgs(args[len(args)-1], ids, parts, dest, settings, it.video, auth), track)
	if err != nil || code != 0 || ctx.Err() != nil {
		if early != nil {
			early.cancel()
		}
		e.finishDownload(ctx, j, it, code, err)
		return true
	}
	video, audio := partFile(parts, ids[0]), partFile(parts, ids[1])
	if video == "" || audio == "" {
		if early != nil {
			early.cancel()
		}
		e.finishDownload(ctx, j, it, -1, fmt.Errorf("yt-dlp 没有写出视频或音频文件"))
		return true
	}

	r := e.reporter(j, it)
	result, ok := e.waitEarly(ctx, it, early, r)
	if !ok {
		e.finishDownload(ctx, j, it, -1, ctx.Err())
		return true
	}
	fin := e.finished(j, it, track)
	fin.Destination, fin.Early = video, result
	tracks := e.d.MergeTracks(ctx, fin, parts, r)
	if ctx.Err() != nil {
		e.finishDownload(ctx, j, it, -1, ctx.Err())
		return true
	}

	var subs []Track
	if settings.Subtitles.EmbedSubtitles {
		subs = subtitleFiles(parts)
	}
	tracks = append(tracks, subs...)
	cover := ""
	if c := filepath.Join(parts, "cover.jpg"); fileExists(c) {
		cover = c
	}
	meta := ""
	if settings.Output.AddMetadata {
		meta = filepath.Join(parts, "metadata.txt")
		if err := os.WriteFile(meta, []byte(ffMetadata(it.video, it.url)), 0o644); err != nil {
			meta = ""
		}
	}
	attach := cover
	if !settings.Output.AddThumbnail {
		attach = ""
	}
	tmp := strings.TrimSuffix(dest, ".mkv") + ".merging.mkv"
	r.Stage(ytdlp.StageMerging)
	if err := e.merge(ctx, j, mergeArgs(video, audio, meta, tracks, attach, tmp), duration(it.video), r); err != nil {
		os.Remove(tmp)
		e.finishDownload(ctx, j, it, -1, err)
		return true
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		e.finishDownload(ctx, j, it, -1, fmt.Errorf("无法保存视频：%w", err))
		return true
	}
	if settings.Output.SaveThumbnail && cover != "" {
		copyFile(cover, strings.TrimSuffix(dest, ".mkv")+".jpg")
	}
	os.RemoveAll(parts)
	e.appendLog(j, "[one-step] Wrote "+dest)
	e.mu.Lock()
	it.destination = ytdlp.Destination{Path: dest, Confidence: 100, Merged: true}
	e.mu.Unlock()
	e.finishDownload(ctx, j, it, 0, nil)
	return true
}

// selectFormats asks yt-dlp which formats it would download and under
// what name, without downloading: two formats to merge, or not ok.
func (e *Engine) selectFormats(ctx context.Context, args []string) ([]string, string, bool) {
	url := args[len(args)-1]
	sel := append(append([]string(nil), args[:len(args)-1]...),
		"--simulate", "--no-warnings", "--print", "%(format_id)s", "--print", "filename", url)
	var lines []string
	code, err := e.d.Runner.Run(ctx, sel, func(l proc.Line) {
		if t := strings.TrimSpace(l.Text); !l.Stderr && t != "" {
			lines = append(lines, t)
		}
	})
	if err != nil || code != 0 || len(lines) < 2 {
		return nil, "", false
	}
	ids := strings.Split(lines[len(lines)-2], "+")
	filename := lines[len(lines)-1]
	if len(ids) != 2 || ids[0] == "" || ids[1] == "" || !filepath.IsAbs(filename) {
		return nil, "", false
	}
	return ids, filename, true
}

// partArgs download the formats, the subtitles and the cover as separate
// files, without any of yt-dlp's post-processing.
func partArgs(url string, ids []string, parts, dest string, settings config.Settings, video *ytdlp.Video, auth []string) []string {
	a := []string{"--encoding", "utf-8"}
	a = append(a, ytdlp.ProgressArgs()...)
	a = append(a, ytdlp.PerformanceArgs(settings.Performance.ConcurrentFragments)...)
	a = append(a, ytdlp.DownloadNetworkArgs(settings.Network, settings.Performance.ConcurrentFragments)...)
	a = append(a, auth...)
	subs := settings.Subtitles
	subs.EmbedSubtitles = false
	a = append(a, ytdlp.SubtitleArgs(subs, &video.SubtitleInventory)...)
	if settings.Output.AddThumbnail || settings.Output.SaveThumbnail {
		a = append(a, "--write-thumbnail", "--convert-thumbnails", "jpg")
	}
	subtitleTemplate := filepath.Join(parts, "sub.%(ext)s")
	if !settings.Subtitles.EmbedSubtitles {
		subtitleTemplate = strings.TrimSuffix(dest, ".mkv") + ".%(ext)s"
	}
	a = append(a, "-f", strings.Join(ids, ","),
		"-o", filepath.Join(parts, "part.f%(format_id)s.%(ext)s"),
		"-o", "subtitle:"+subtitleTemplate,
		"-o", "thumbnail:"+filepath.Join(parts, "cover.%(ext)s"),
		"--no-playlist", url)
	return a
}

// partFile finds the downloaded file of a format.
func partFile(parts, id string) string {
	matches, _ := filepath.Glob(filepath.Join(parts, "part.f"+id+".*"))
	for _, m := range matches {
		if !strings.HasSuffix(m, ".part") && !strings.HasSuffix(m, ".ytdl") {
			return m
		}
	}
	return ""
}

var subtitleName = regexp.MustCompile(`^sub\.(.+)\.(srt|vtt|ass|ssa)$`)

// subtitleFiles are the subtitles yt-dlp downloaded, by language.
func subtitleFiles(parts string) []Track {
	entries, _ := os.ReadDir(parts)
	var out []Track
	for _, e := range entries {
		if m := subtitleName.FindStringSubmatch(e.Name()); m != nil {
			lang := strings.TrimSuffix(m[1], "-orig")
			out = append(out, Track{Path: filepath.Join(parts, e.Name()), Title: lang, Language: lang})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Path < out[b].Path })
	return out
}

// mergeArgs are ffmpeg's arguments for the one-step merge.
func mergeArgs(video, audio, meta string, tracks []Track, cover, out string) []string {
	args := []string{"-hide_banner", "-nostdin", "-y", "-v", "error", "-progress", "pipe:1", "-nostats", "-i", video, "-i", audio}
	next := 2
	metaInput := -1
	if meta != "" {
		args = append(args, "-f", "ffmetadata", "-i", meta)
		metaInput, next = next, next+1
	}
	for _, t := range tracks {
		args = append(args, "-i", t.Path)
	}
	args = append(args, "-map", "0:v:0", "-map", "1:a:0")
	for i := range tracks {
		args = append(args, "-map", strconv.Itoa(next+i))
	}
	if metaInput >= 0 {
		args = append(args, "-map_metadata", strconv.Itoa(metaInput), "-map_chapters", strconv.Itoa(metaInput))
	}
	args = append(args, "-c", "copy")
	for i, t := range tracks {
		s := fmt.Sprintf(":s:s:%d", i)
		if t.Language != "" {
			args = append(args, "-metadata"+s, "language="+t.Language)
		}
		if t.Title != "" {
			args = append(args, "-metadata"+s, "title="+t.Title)
		}
		disposition := "0"
		if t.Default {
			disposition = "default"
		}
		args = append(args, fmt.Sprintf("-disposition:s:%d", i), disposition)
	}
	if cover != "" {
		args = append(args, "-attach", cover, "-metadata:s:t:0", "mimetype=image/jpeg", "-metadata:s:t:0", "filename=cover.jpg")
	}
	return append(args, out)
}

// ffMetadata is an ffmetadata file with what --add-metadata writes: the
// title, channel, description, address and chapters.
func ffMetadata(v *ytdlp.Video, url string) string {
	esc := strings.NewReplacer(`\`, `\\`, "=", `\=`, ";", `\;`, "#", `\#`, "\n", "\\\n")
	var b strings.Builder
	b.WriteString(";FFMETADATA1\n")
	for _, kv := range [][2]string{
		{"title", v.Title}, {"artist", v.Uploader}, {"description", v.Description},
		{"synopsis", v.Description}, {"comment", url}, {"purl", url},
	} {
		if kv[1] != "" {
			b.WriteString(kv[0] + "=" + esc.Replace(kv[1]) + "\n")
		}
	}
	for _, c := range v.Chapters {
		if c.EndTime <= c.StartTime {
			continue
		}
		fmt.Fprintf(&b, "[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=%s\n",
			int64(c.StartTime*1000), int64(c.EndTime*1000), esc.Replace(c.Title))
	}
	return b.String()
}

// merge runs ffmpeg, showing its progress on the item.
func (e *Engine) merge(ctx context.Context, j *job, args []string, length float64, r Reporter) error {
	var last string
	code, err := e.d.FFmpeg.Run(ctx, args, func(l proc.Line) {
		if l.Stderr {
			if t := strings.TrimSpace(l.Text); t != "" {
				last = t
				e.appendLog(j, "[merge] "+t)
			}
			return
		}
		if us, ok := strings.CutPrefix(l.Text, "out_time_us="); ok && length > 0 {
			if n, err := strconv.ParseFloat(strings.TrimSpace(us), 64); err == nil && n >= 0 {
				r.Progress(min(n/1e6/length*100, 100))
			}
		}
	})
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("合并失败（ffmpeg 退出代码 %d）：%s", code, last)
	}
	return nil
}

func duration(v *ytdlp.Video) float64 {
	if v != nil && v.Duration != nil {
		return *v.Duration
	}
	return 0
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_.+-]+`)

func safeName(s string) string { return unsafeName.ReplaceAllString(s, "_") }

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, in)
}
