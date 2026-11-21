// Package translate writes bilingual subtitles: after a download, yt-dlp
// fetches the source subtitles as SRT, an OpenAI-compatible chat API
// translates them, and a bilingual .srt is written next to the video. It
// is a port of the Tauri app's translation module.
package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// Tuning, as in the old app.
const (
	// batchChars bounds the source characters per request, so the reply
	// stays under the output limit many providers apply.
	batchChars = 5000
	// batchMaxLines bounds the lines per request, so the model reliably
	// returns one translation per line.
	batchMaxLines = 250
	// contextLines go along with a batch for context only.
	contextLines = 5
	concurrency  = 6
	maxAttempts  = 3
	// DefaultTargetLanguage is used when the setting is empty.
	DefaultTargetLanguage = "简体中文"
)

const systemPrompt = "You translate video subtitles. Translate every line of the JSON " +
	"array the user asks you to translate into {target}. The lines are consecutive subtitles from " +
	"one video, often produced by automatic speech recognition, so they can contain recognition " +
	"errors and sentences split across lines; use the neighbouring lines to work out what is " +
	"meant. The user may also send a few earlier lines for context only: never translate or return " +
	"those. Keep each translation short enough to read as a subtitle, and translate sound tags such " +
	"as [music] too, keeping the brackets. Reply with only a JSON array of strings: exactly one " +
	"translation per line to translate, in the same order, with nothing before or after it."

// Diagnostic codes of translation problems (explained in the UI).
const (
	CodeNotConfigured = "translationNotConfigured"
	CodeSourceMissing = "translationSourceMissing"
	CodeFailed        = "translationFailed"
	CodePartial       = "translationPartial"
)

// Env is what a translation needs from the app.
type Env struct {
	Runner   engine.Runner
	Settings config.Settings
	Secrets  ytdlp.AuthSecrets
	APIKey   string
	Client   *http.Client
	// Backoff is the delay before retry n (1, 2): 2^n seconds by default.
	Backoff func(attempt int) time.Duration
}

type failure struct {
	code, detail string
}

func (f *failure) Error() string { return f.detail }

// AfterDownload writes the bilingual subtitles of a finished video when
// translation is on; problems become warnings on the item.
func AfterDownload(ctx context.Context, env Env, d engine.Finished, r engine.Reporter) {
	if !env.Settings.Translation.Enabled || d.TrackType == ytdlp.TrackAudio {
		return
	}
	r.Stage(ytdlp.StageTranslating)
	r.Log("[translate] Creating bilingual subtitles")
	path, untranslated, err := run(ctx, env, d, r)
	var f *failure
	switch {
	case ctx.Err() != nil:
	case errors.As(err, &f):
		r.Log("[translate] " + f.detail)
		r.Warn(f.code, f.detail)
	case err != nil:
		r.Log("[translate] " + err.Error())
		r.Warn(CodeFailed, err.Error())
	default:
		r.Log("[translate] Wrote " + path)
		if untranslated > 0 {
			r.Warn(CodePartial, fmt.Sprintf("%d 行字幕没有翻译成功。", untranslated))
		}
	}
}

func run(ctx context.Context, env Env, d engine.Finished, r engine.Reporter) (string, int, error) {
	chat, err := chatConfig(env)
	if err != nil {
		return "", 0, err
	}
	if d.Destination == "" {
		return "", 0, &failure{CodeFailed, "找不到下载的文件。"}
	}
	var inventory ytdlp.SubtitleInventory
	if d.Video != nil {
		inventory = d.Video.SubtitleInventory
	}
	source, ok := PickSource(inventory)
	if !ok {
		return "", 0, &failure{CodeSourceMissing, "这个视频没有可翻译的字幕（或已经是中文）。"}
	}
	dir, err := os.MkdirTemp("", "ovd-translate-")
	if err != nil {
		return "", 0, &failure{CodeFailed, "无法创建临时文件夹：" + err.Error()}
	}
	defer os.RemoveAll(dir)
	srtPath, err := fetchSource(ctx, env, d.URL, source, dir)
	if err != nil {
		return "", 0, err
	}
	content, err := os.ReadFile(srtPath)
	if err != nil {
		return "", 0, &failure{CodeFailed, "无法读取字幕：" + err.Error()}
	}
	cues := ParseSRT(string(content))
	if len(cues) == 0 {
		return "", 0, &failure{CodeSourceMissing, "字幕是空的。"}
	}
	translations, err := translateCues(ctx, env, chat, cues, r)
	if err != nil {
		return "", 0, err
	}
	output := strings.TrimSuffix(d.Destination, filepath.Ext(d.Destination)) + ".srt"
	if err := os.WriteFile(output, []byte(WriteBilingual(cues, translations)), 0o644); err != nil {
		return "", 0, &failure{CodeFailed, "无法保存字幕：" + err.Error()}
	}
	untranslated := 0
	for _, t := range translations {
		if t == "" {
			untranslated++
		}
	}
	return output, untranslated, nil
}

type chat struct {
	endpoint, model, apiKey, target string
}

func chatConfig(env Env) (chat, error) {
	s := env.Settings.Translation
	endpoint, ok := ChatCompletionsURL(s.BaseURL)
	if !ok {
		return chat{}, &failure{CodeNotConfigured, "没有填写接口地址。"}
	}
	model := strings.TrimSpace(s.Model)
	if model == "" {
		return chat{}, &failure{CodeNotConfigured, "没有填写模型。"}
	}
	if strings.TrimSpace(env.APIKey) == "" {
		return chat{}, &failure{CodeNotConfigured, "没有填写 API Key。"}
	}
	target := strings.TrimSpace(s.TargetLanguage)
	if target == "" {
		target = DefaultTargetLanguage
	}
	return chat{endpoint, model, strings.TrimSpace(env.APIKey), target}, nil
}

// ChatCompletionsURL is {base}/chat/completions, unless the address
// already ends with that path.
func ChatCompletionsURL(base string) (string, bool) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	switch {
	case base == "":
		return "", false
	case strings.HasSuffix(base, "/chat/completions"):
		return base, true
	}
	return base + "/chat/completions", true
}

// Source is the subtitle track to translate.
type Source struct {
	Language  string
	Automatic bool
}

// PickSource picks the creator's subtitles in the spoken language when
// there are any, otherwise the automatic captions in the original
// language. Videos already in Chinese are skipped.
func PickSource(inv ytdlp.SubtitleInventory) (Source, bool) {
	manualFor := func(base string) (string, bool) {
		for _, l := range inv.ManualLanguages {
			if languageBase(l) == base {
				return l, true
			}
		}
		return "", false
	}
	for _, l := range inv.AutoLanguages {
		if !strings.HasSuffix(l, "-orig") {
			continue
		}
		base := languageBase(l)
		if base == "zh" {
			return Source{}, false
		}
		if manual, ok := manualFor(base); ok {
			return Source{Language: manual}, true
		}
		return Source{Language: l, Automatic: true}, true
	}
	for _, l := range inv.ManualLanguages {
		if languageBase(l) != "zh" {
			return Source{Language: l}, true
		}
	}
	for _, l := range inv.AutoLanguages {
		if l == "en" {
			return Source{Language: l, Automatic: true}, true
		}
	}
	return Source{}, false
}

func languageBase(language string) string {
	base, _, _ := strings.Cut(strings.ReplaceAll(language, "_", "-"), "-")
	return base
}

// fetchSource downloads the source subtitles as SRT into dir with a
// separate yt-dlp run.
func fetchSource(ctx context.Context, env Env, url string, s Source, dir string) (string, error) {
	write := "--write-subs"
	if s.Automatic {
		write = "--write-auto-subs"
	}
	args := []string{"--encoding", "utf-8"}
	args = append(args, ytdlp.NetworkArgs(env.Settings.Network)...)
	args = append(args, ytdlp.AuthArgs(env.Settings.Auth, env.Secrets)...)
	args = append(args, "--skip-download", "--no-playlist", write,
		"--sub-langs", "(?i)"+s.Language, "--sub-format", "srt/vtt/best", "--convert-subs", "srt",
		"-o", filepath.Join(dir, "source.%(ext)s"), url)
	var last string
	code, err := env.Runner.Run(ctx, args, func(l proc.Line) {
		if l.Stderr && strings.TrimSpace(l.Text) != "" {
			last = strings.TrimSpace(l.Text)
		}
	})
	if err != nil {
		return "", &failure{CodeFailed, err.Error()}
	}
	if code != 0 {
		if last == "" {
			last = "字幕下载失败。"
		}
		return "", &failure{CodeSourceMissing, last}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.srt"))
	if len(matches) == 0 {
		return "", &failure{CodeSourceMissing, "yt-dlp 没有写出字幕文件。"}
	}
	return matches[0], nil
}

// batch is a run of consecutive cues sent in one request.
type batch struct {
	start   int
	context []string
	lines   []string
}

// makeBatches packs as many lines as fit in batchChars (at most
// batchMaxLines) per request, each with the lines before it as context.
func makeBatches(texts []string) []batch {
	var out []batch
	for start := 0; start < len(texts); {
		end, chars := start, 0
		for end < len(texts) && end-start < batchMaxLines {
			n := len([]rune(texts[end]))
			if end > start && chars+n > batchChars {
				break
			}
			chars += n
			end++
		}
		out = append(out, batch{start: start, context: texts[max(start-contextLines, 0):start], lines: texts[start:end]})
		start = end
	}
	return out
}

// errFatal stops the translation: the API rejected the request outright
// (bad key, unknown model).
type errFatal struct{ msg string }

func (e errFatal) Error() string { return e.msg }

var errTooLong = errors.New("reply cut off")

// translateCues translates every cue, a few batches at a time; lines that
// keep failing stay "".
func translateCues(ctx context.Context, env Env, c chat, cues []Cue, r engine.Reporter) ([]string, error) {
	texts := make([]string, len(cues))
	for i, cue := range cues {
		texts[i] = cue.Text
	}
	batches := makeBatches(texts)
	out := make([]string, len(cues))
	var (
		mu    sync.Mutex
		done  int
		fatal error
		wg    sync.WaitGroup
	)
	sem := make(chan struct{}, concurrency)
	for _, b := range batches {
		wg.Add(1)
		sem <- struct{}{}
		go func(b batch) {
			defer func() {
				<-sem
				wg.Done()
			}()
			lines, err := translateBatch(ctx, env, c, b.context, b.lines)
			mu.Lock()
			defer mu.Unlock()
			var f errFatal
			switch {
			case errors.As(err, &f):
				if fatal == nil {
					fatal = &failure{CodeFailed, f.msg}
				}
			case err != nil:
				r.Log(fmt.Sprintf("[translate] Lines from %d failed: %v", b.start+1, err))
			default:
				copy(out[b.start:], lines)
			}
			done++
			r.Progress(float64(done) / float64(len(batches)) * 100)
		}(b)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if fatal != nil {
		return nil, fatal
	}
	return out, nil
}

// translateBatch translates one batch; when it keeps failing (usually a
// reply cut off or missing a line), its halves go separately.
func translateBatch(ctx context.Context, env Env, c chat, prior, lines []string) ([]string, error) {
	out, err := translateWithRetries(ctx, env, c, prior, lines)
	var f errFatal
	if err == nil || errors.As(err, &f) || len(lines) < 2 {
		return out, err
	}
	half := len(lines) / 2
	first, err := translateWithRetries(ctx, env, c, prior, lines[:half])
	if err != nil {
		return nil, err
	}
	second, err := translateWithRetries(ctx, env, c, lines[max(half-contextLines, 0):half], lines[half:])
	if err != nil {
		return nil, err
	}
	return append(first, second...), nil
}

func translateWithRetries(ctx context.Context, env Env, c chat, prior, lines []string) ([]string, error) {
	var last error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<attempt) * time.Second
			if env.Backoff != nil {
				delay = env.Backoff(attempt)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		out, err := request(ctx, env, c, prior, lines)
		var f errFatal
		switch {
		case err == nil && len(out) == len(lines):
			return out, nil
		case err == nil:
			last = fmt.Errorf("expected %d lines, got %d", len(lines), len(out))
		case errors.As(err, &f), errors.Is(err, errTooLong), ctx.Err() != nil:
			return nil, err
		default:
			last = err
		}
	}
	return nil, last
}

func jsonArray(lines []string) string {
	b, _ := json.Marshal(lines)
	return string(b)
}

func request(ctx context.Context, env Env, c chat, prior, lines []string) ([]string, error) {
	var user strings.Builder
	if len(prior) > 0 {
		user.WriteString("Earlier lines for context only, do not translate:\n" + jsonArray(prior) + "\n\n")
	}
	fmt.Fprintf(&user, "Translate these %d lines:\n%s", len(lines), jsonArray(lines))
	body, _ := json.Marshal(map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.ReplaceAll(systemPrompt, "{target}", c.target)},
			{"role": "user", "content": user.String()},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errFatal{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return nil, errors.New(msg)
		}
		return nil, errFatal{msg}
	}
	var reply struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return nil, err
	}
	if len(reply.Choices) == 0 {
		return nil, errors.New("the reply had no choices")
	}
	if reply.Choices[0].FinishReason == "length" {
		return nil, errTooLong
	}
	out, ok := ParseTranslations(reply.Choices[0].Message.Content)
	if !ok {
		return nil, errors.New("reply was not a JSON array")
	}
	return out, nil
}

// ParseTranslations pulls the JSON array of strings out of a model reply,
// tolerating code fences around it.
func ParseTranslations(content string) ([]string, bool) {
	start, end := strings.Index(content, "["), strings.LastIndex(content, "]")
	if start < 0 || end <= start {
		return nil, false
	}
	var out []string
	if err := json.Unmarshal([]byte(content[start:end+1]), &out); err != nil {
		return nil, false
	}
	return out, true
}

func truncate(text string, limit int) string {
	text = strings.TrimSpace(text)
	if r := []rune(text); len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return text
}
