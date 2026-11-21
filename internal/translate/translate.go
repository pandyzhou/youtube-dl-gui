// Package translate writes bilingual subtitles: after a download, yt-dlp
// fetches the source subtitles as SRT, an OpenAI-compatible chat API
// translates them, and a bilingual .srt is written next to the video. It
// is a port of the Tauri app's translation module.
package translate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// Tuning.
const (
	// batchChars bounds the source characters per request, so the reply
	// stays under the output limit many providers apply and comes back in
	// a minute or two.
	batchChars = 1500
	// batchMaxLines bounds the lines per request: smaller batches keep the
	// model returning exactly one translation per line.
	batchMaxLines = 60
	// contextLines before a batch and followLines after it go along for
	// context only, so sentences cut by the batch edge read right.
	contextLines = 5
	followLines  = 3
	// descriptionChars bounds the video description sent as context.
	descriptionChars = 300
	// transcriptChars bounds the transcript read in high quality mode.
	transcriptChars = 30000
	maxTerms        = 20
	concurrency     = 6
	maxConcurrency  = 32
	maxAttempts     = 3
	// DefaultTargetLanguage is used when the setting is empty.
	DefaultTargetLanguage = "简体中文"
)

const systemPrompt = `You translate video subtitles into {target}.

The user sends a JSON array of consecutive subtitle lines from one video. Reply with only a JSON array of strings: exactly one translation per line, in the same order, with nothing before or after it.

- The lines often come from automatic speech recognition: punctuation and capitals may be missing, words may be misheard, and one sentence is often split across several lines. Use the surrounding lines and the video's title and description to work out what was meant, and translate that, quietly fixing obvious recognition errors.
- When a sentence spans several lines, give each line the part of the translation that matches it. You may move words between lines of the same sentence so the translation reads naturally, but never merge lines and never leave a line empty.
- Keep each translation short enough to read as a subtitle. Write natural spoken language, not translationese.
- Translate sound tags such as [music] too, keeping the brackets.
- Keep names of people, channels, brands and products as they are unless they have a well-known translation.
- The user may also send the video's title and description and lines before and after the ones to translate. They are context only: never translate or return them.{style}

Example, into 简体中文:
["so today we're going to look at how the", "new tesla model why handles in the snow", "[music]"]
Reply:
["今天我们来看看", "新款特斯拉 Model Y 在雪地里表现如何", "[音乐]"]`

// notesPrompt asks for the topic and glossary of high quality mode.
const notesPrompt = `You prepare the translation of a video's subtitles into {target}. Read the transcript the user sends. It may come from automatic speech recognition, without punctuation and with misheard words. Reply with only a JSON object:
{"topic": "what the video is about, in at most two sentences, in {target}", "terms": [{"source": "a term as it appears in the transcript", "target": "how to write it in {target}, or the original when it should stay as it is", "note": "optional: what it is, or the correct word when it was misheard"}]}
List at most 20 terms: names of people, places, channels, brands and products, and technical terms whose translation must stay the same throughout. Leave out ordinary words.`

// chineseStyle is added to the prompt for Chinese targets.
const chineseStyle = "\n- Write Chinese the way people speak it: drop pronouns Chinese would leave out, use Chinese punctuation within a line and no full stop at the end of a line."

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
	// CacheDir keeps checkpoints of unfinished translations; "" keeps
	// none.
	CacheDir string
	// FFmpeg and FFprobe put the subtitles into the video; without them
	// they go to files.
	FFmpeg, FFprobe string
}

type failure struct {
	code, detail string
}

func (f *failure) Error() string { return f.detail }

// AfterDownload writes the bilingual subtitles of a finished video when
// translation is on; problems become warnings on the item.
func AfterDownload(ctx context.Context, env Env, d engine.Finished, r engine.Reporter) {
	if (!env.Settings.Translation.Enabled && !d.SubtitlesOnly) || d.TrackType == ytdlp.TrackAudio {
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
	sourcePath, err := fetchSource(ctx, env, d.URL, source, dir)
	if err != nil {
		return "", 0, err
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", 0, &failure{CodeFailed, "无法读取字幕：" + err.Error()}
	}
	var cues []Cue
	if strings.HasSuffix(sourcePath, ".json3") {
		words, err := ParseJSON3(content)
		if err != nil {
			return "", 0, &failure{CodeFailed, "无法读取自动字幕：" + err.Error()}
		}
		cues = Segment(words)
		r.Log(fmt.Sprintf("[translate] Regrouped %d words of automatic captions into %d lines", len(words), len(cues)))
	} else {
		cues = ParseSRT(string(content))
	}
	if len(cues) == 0 {
		return "", 0, &failure{CodeSourceMissing, "字幕是空的。"}
	}
	chat.video = videoContext(d)
	chat.log = r.Log
	texts := make([]string, len(cues))
	for i, cue := range cues {
		texts[i] = cue.Text
	}
	cp := openCheckpoint(env.CacheDir, d.URL+"\n"+source.Language, texts, chat.target)
	if env.Settings.Translation.HighQuality {
		if notes := cp.notes(); notes != "" {
			chat.notes = notes
			r.Log("[translate] Using the glossary saved by the last run")
		} else {
			r.Log("[translate] Reading the whole transcript for a topic and glossary")
			notes, err := readTranscript(ctx, env, chat, cues)
			switch {
			case ctx.Err() != nil:
				return "", 0, ctx.Err()
			case err != nil:
				r.Log("[translate] Going on without a glossary: " + err.Error())
			default:
				chat.notes = notes
				cp.setNotes(notes)
				r.Log("[translate] " + strings.ReplaceAll(strings.TrimSpace(notes), "\n", " | "))
			}
		}
	}
	translations, err := translateCues(ctx, env, chat, texts, r, cp)
	if err != nil {
		return "", 0, err
	}
	output, err := deliver(ctx, env, d.Destination, cues, translations, r)
	if err != nil {
		return "", 0, err
	}
	untranslated := 0
	for _, t := range translations {
		if t == "" {
			untranslated++
		}
	}
	if untranslated == 0 {
		cp.remove()
	}
	return output, untranslated, nil
}

type chat struct {
	endpoint, model, apiKey, target string
	// effort is the reasoning effort, "" for the model's default.
	effort string
	// video describes the video (title, channel, description) as context.
	video string
	// notes are the topic and glossary of high quality mode.
	notes string
	// log writes to the item's log; nil for none.
	log func(string)
}

func (c chat) logf(format string, args ...any) {
	if c.log != nil {
		c.log("[translate] " + fmt.Sprintf(format, args...))
	}
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
	return chat{
		endpoint: endpoint,
		model:    model,
		apiKey:   strings.TrimSpace(env.APIKey),
		target:   target,
		effort:   strings.TrimSpace(s.ReasoningEffort),
	}, nil
}

// videoContext describes a video for the model: its title, channel and
// the start of its description.
func videoContext(d engine.Finished) string {
	title, channel, description := d.Title, "", ""
	if v := d.Video; v != nil {
		if v.Title != "" {
			title = v.Title
		}
		channel = v.Uploader
		description = truncate(strings.Join(strings.Fields(v.Description), " "), descriptionChars)
	}
	var b strings.Builder
	for _, f := range [][2]string{{"Title", title}, {"Channel", channel}, {"Description", description}} {
		if v := strings.TrimSpace(f[1]); v != "" {
			b.WriteString(f[0] + ": " + v + "\n")
		}
	}
	return b.String()
}

// prompt is the system prompt for a target language.
func prompt(target string) string {
	style := ""
	lower := strings.ToLower(target)
	if strings.Contains(target, "中文") || strings.Contains(target, "汉语") || strings.Contains(lower, "chinese") || strings.HasPrefix(lower, "zh") {
		style = chineseStyle
	}
	return strings.NewReplacer("{target}", target, "{style}", style).Replace(systemPrompt)
}

// ModelsURL is the model list of an OpenAI-compatible API: {base}/models,
// with a trailing /chat/completions dropped.
func ModelsURL(base string) (string, bool) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	base = strings.TrimSuffix(base, "/chat/completions")
	if base == "" {
		return "", false
	}
	return base + "/models", true
}

// ListModels asks an OpenAI-compatible API for its models, sorted by ID.
func ListModels(ctx context.Context, client *http.Client, base, apiKey string) ([]string, error) {
	endpoint, ok := ModelsURL(base)
	if !ok {
		return nil, errors.New("没有填写接口地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 200))
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("返回的不是模型列表：%w", err)
	}
	var models []string
	for _, m := range list.Data {
		if id := strings.TrimSpace(m.ID); id != "" && !slices.Contains(models, id) {
			models = append(models, id)
		}
	}
	if len(models) == 0 {
		return nil, errors.New("接口没有返回任何模型")
	}
	sort.Strings(models)
	return models, nil
}

// TestLine is what TestModel translates.
const TestLine = "Hey everyone, welcome back to the channel. Today we're testing something new."

// TestModel translates TestLine with the translation settings, as the
// settings page's test.
func TestModel(ctx context.Context, client *http.Client, s config.TranslationSettings, apiKey string) (string, error) {
	env := Env{APIKey: apiKey, Client: client}
	env.Settings.Translation = s
	c, err := chatConfig(env)
	if err != nil {
		return "", err
	}
	out, err := request(ctx, env, c, batch{lines: []string{TestLine}})
	switch {
	case errors.Is(err, errTooLong):
		return "", errors.New("回复太长被截断了，可以调低思考强度")
	case err != nil:
		return "", err
	case len(out) != 1 || strings.TrimSpace(out[0]) == "":
		return "", fmt.Errorf("模型返回了 %d 行译文，应该是 1 行", len(out))
	}
	return strings.TrimSpace(out[0]), nil
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

// fetchSource downloads the source subtitles into dir with a separate
// yt-dlp run: automatic captions as json3, with word timings, when the
// site has them, anything else as SRT.
func fetchSource(ctx context.Context, env Env, url string, s Source, dir string) (string, error) {
	if s.Automatic {
		sub := filepath.Join(dir, "json3")
		if err := os.Mkdir(sub, 0o755); err == nil {
			path, err := downloadSubs(ctx, env, url, s, sub, "json3")
			if err == nil || ctx.Err() != nil {
				return path, err
			}
		}
	}
	return downloadSubs(ctx, env, url, s, dir, "srt")
}

// downloadSubs runs yt-dlp for subtitles in a format, json3 or srt
// (converted from whatever the site has).
func downloadSubs(ctx context.Context, env Env, url string, s Source, dir, format string) (string, error) {
	write := "--write-subs"
	if s.Automatic {
		write = "--write-auto-subs"
	}
	formatArgs := []string{"--sub-format", "json3"}
	if format == "srt" {
		formatArgs = []string{"--sub-format", "srt/vtt/best", "--convert-subs", "srt"}
	}
	args := []string{"--encoding", "utf-8"}
	args = append(args, ytdlp.NetworkArgs(env.Settings.Network)...)
	args = append(args, ytdlp.AuthArgs(env.Settings.Auth, env.Secrets)...)
	args = append(args, "--skip-download", "--no-playlist", write, "--sub-langs", "(?i)"+s.Language)
	args = append(args, formatArgs...)
	args = append(args, "-o", filepath.Join(dir, "source.%(ext)s"), url)
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
	matches, _ := filepath.Glob(filepath.Join(dir, "*."+format))
	if len(matches) == 0 {
		return "", &failure{CodeSourceMissing, "yt-dlp 没有写出字幕文件。"}
	}
	return matches[0], nil
}

// batch is a run of consecutive cues sent in one request, with the lines
// around it as context.
type batch struct {
	start   int
	context []string
	lines   []string
	after   []string
}

// makeBatches packs as many lines as fit in batchChars (at most
// batchMaxLines) per request, each with the lines around it as context.
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
		out = append(out, batch{
			start:   start,
			context: texts[max(start-contextLines, 0):start],
			lines:   texts[start:end],
			after:   texts[end:min(end+followLines, len(texts))],
		})
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
func translateCues(ctx context.Context, env Env, c chat, texts []string, r engine.Reporter, cp *checkpoint) ([]string, error) {
	batches := makeBatches(texts)
	out := make([]string, len(texts))
	var (
		mu    sync.Mutex
		done  int
		fatal error
		wg    sync.WaitGroup
	)
	for _, b := range batches {
		if lines, ok := cp.batch(b.start, len(b.lines)); ok {
			copy(out[b.start:], lines)
			done++
		}
	}
	if done > 0 {
		c.logf("Going on from the last run: %d of %d batches already translated", done, len(batches))
		r.Progress(float64(done) / float64(len(batches)) * 100)
	}
	n := env.Settings.Translation.Concurrency
	if n <= 0 {
		n = concurrency
	}
	sem := make(chan struct{}, min(n, maxConcurrency))
	for _, b := range batches {
		if _, ok := cp.batch(b.start, len(b.lines)); ok {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(b batch) {
			defer func() {
				<-sem
				wg.Done()
			}()
			began := time.Now()
			lines, err := translateBatch(ctx, env, c, b)
			if err == nil {
				cp.save(b.start, lines)
				c.logf("Lines %d-%d done in %.0fs", b.start+1, b.start+len(b.lines), time.Since(began).Seconds())
			}
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
func translateBatch(ctx context.Context, env Env, c chat, b batch) ([]string, error) {
	out, err := translateWithRetries(ctx, env, c, b)
	var f errFatal
	lines := b.lines
	if err == nil || errors.As(err, &f) || len(lines) < 2 {
		return out, err
	}
	c.logf("Lines %d-%d keep failing (%v), translating them in halves", b.start+1, b.start+len(lines), err)
	half := len(lines) / 2
	first, err := translateWithRetries(ctx, env, c, batch{
		start:   b.start,
		context: b.context,
		lines:   lines[:half],
		after:   lines[half:min(half+followLines, len(lines))],
	})
	if err != nil {
		return nil, err
	}
	second, err := translateWithRetries(ctx, env, c, batch{
		start:   b.start + half,
		context: lines[max(half-contextLines, 0):half],
		lines:   lines[half:],
		after:   b.after,
	})
	if err != nil {
		return nil, err
	}
	return append(first, second...), nil
}

func translateWithRetries(ctx context.Context, env Env, c chat, b batch) ([]string, error) {
	lines := b.lines
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
		began := time.Now()
		out, err := request(ctx, env, c, b)
		var f errFatal
		switch {
		case err == nil && len(out) == len(lines):
			return out, nil
		case err == nil:
			last = fmt.Errorf("expected %d lines, got %d", len(lines), len(out))
		case errors.As(err, &f), errors.Is(err, errTooLong), ctx.Err() != nil:
			if errors.Is(err, errTooLong) {
				c.logf("Lines %d-%d: the reply was cut off after %.0fs", b.start+1, b.start+len(lines), time.Since(began).Seconds())
			}
			return nil, err
		default:
			last = err
		}
		c.logf("Lines %d-%d, attempt %d of %d failed after %.0fs: %v", b.start+1, b.start+len(lines), attempt+1, maxAttempts, time.Since(began).Seconds(), last)
	}
	return nil, last
}

func jsonArray(lines []string) string {
	b, _ := json.Marshal(lines)
	return string(b)
}

// notes is the reply to notesPrompt.
type notes struct {
	Topic string `json:"topic"`
	Terms []struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Note   string `json:"note"`
	} `json:"terms"`
}

// readTranscript has the model read the transcript (its start, when it is
// long) and returns the topic and glossary as text for the batches.
func readTranscript(ctx context.Context, env Env, c chat, cues []Cue) (string, error) {
	var transcript strings.Builder
	chars := 0
	for _, cue := range cues {
		line := strings.Join(strings.Fields(cue.Text), " ")
		if chars += len([]rune(line)) + 1; chars > transcriptChars {
			transcript.WriteString("(the rest is left out)\n")
			break
		}
		transcript.WriteString(line + "\n")
	}
	user := "Transcript:\n" + transcript.String()
	if c.video != "" {
		user = "About the video:\n" + c.video + "\n" + user
	}
	system := strings.ReplaceAll(notesPrompt, "{target}", c.target)
	var last error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		content, err := complete(ctx, env, c, system, user)
		var f errFatal
		if errors.As(err, &f) || ctx.Err() != nil {
			return "", err
		}
		if err != nil {
			last = err
			continue
		}
		if text, ok := ParseNotes(content); ok {
			return text, nil
		}
		last = errors.New("reply was not the expected JSON object")
	}
	return "", last
}

// ParseNotes turns a reply to notesPrompt into text for the batches.
func ParseNotes(content string) (string, bool) {
	if _, after, ok := strings.Cut(content, "</think>"); ok {
		content = after
	}
	start, end := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return "", false
	}
	var n notes
	if err := json.Unmarshal([]byte(content[start:end+1]), &n); err != nil {
		return "", false
	}
	var b strings.Builder
	if t := strings.TrimSpace(n.Topic); t != "" {
		b.WriteString("Topic: " + t + "\n")
	}
	count := 0
	for _, t := range n.Terms {
		src, tgt := strings.TrimSpace(t.Source), strings.TrimSpace(t.Target)
		if src == "" || tgt == "" || count == maxTerms {
			continue
		}
		if count == 0 {
			b.WriteString("Glossary, translate these terms this way:\n")
		}
		count++
		b.WriteString("- " + src + " → " + tgt)
		if note := strings.TrimSpace(t.Note); note != "" {
			b.WriteString(" (" + note + ")")
		}
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "", false
	}
	return b.String(), true
}

// request sends one batch. The lines to translate come last, after the
// context.
func request(ctx context.Context, env Env, c chat, b batch) ([]string, error) {
	var user strings.Builder
	if c.video != "" {
		user.WriteString("About the video, for context only:\n" + c.video + "\n")
	}
	if c.notes != "" {
		user.WriteString("Notes from reading the whole transcript:\n" + c.notes + "\n")
	}
	if len(b.context) > 0 {
		user.WriteString("Lines just before, for context only, do not translate:\n" + jsonArray(b.context) + "\n\n")
	}
	if len(b.after) > 0 {
		user.WriteString("Lines just after, for context only, do not translate:\n" + jsonArray(b.after) + "\n\n")
	}
	fmt.Fprintf(&user, "Translate these %d lines:\n%s", len(b.lines), jsonArray(b.lines))
	content, err := complete(ctx, env, c, prompt(c.target), user.String())
	if err != nil {
		return nil, err
	}
	out, ok := ParseTranslations(content)
	if !ok {
		return nil, errors.New("reply was not a JSON array")
	}
	return out, nil
}

// complete sends one chat request and returns the reply's text. It asks
// for a stream: gateways such as Cloudflare cut a request off after about
// two minutes without a byte (HTTP 524), which a slow model answering a
// long batch reaches, while a stream keeps data flowing. Providers that
// ignore stream answer with plain JSON, which is read too.
func complete(ctx context.Context, env Env, c chat, system, user string) (string, error) {
	payload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream": true,
	}
	if c.effort != "" {
		payload["reasoning_effort"] = c.effort
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errFatal{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK && strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return readStream(resp.Body)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", errors.New(msg)
		}
		return "", errFatal{msg}
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
		return "", err
	}
	if len(reply.Choices) == 0 {
		return "", errors.New("the reply had no choices")
	}
	if reply.Choices[0].FinishReason == "length" {
		return "", errTooLong
	}
	return reply.Choices[0].Message.Content, nil
}

// readStream reads a chat completion stream (server-sent events) into the
// reply's text.
func readStream(body io.Reader) (string, error) {
	var text strings.Builder
	finish := ""
	scanner := bufio.NewScanner(io.LimitReader(body, 16<<20))
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		data = strings.TrimSpace(data)
		if !ok || data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return "", errors.New("the stream ended with an error: " + chunk.Error.Message)
		}
		for _, ch := range chunk.Choices {
			text.WriteString(ch.Delta.Content)
			if ch.FinishReason != "" {
				finish = ch.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("the stream broke off: %w", err)
	}
	if finish == "length" {
		return "", errTooLong
	}
	if text.Len() == 0 {
		return "", errors.New("the stream had no text")
	}
	return text.String(), nil
}

// ParseTranslations pulls the JSON array of strings out of a model reply,
// tolerating code fences around it and a reasoning model's <think> block
// before it.
func ParseTranslations(content string) ([]string, bool) {
	if _, after, ok := strings.Cut(content, "</think>"); ok {
		content = after
	}
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
