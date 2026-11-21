package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// Ported from src-tauri/src/translation/{srt,mod}.rs.

func TestParseSRT(t *testing.T) {
	content := strings.Join([]string{"\ufeff1", "00:00:01,000 --> 00:00:02,000", "Hello", "there", "", "2", "00:00:03,000 --> 00:00:04,000", "[music]", ""}, "\r\n")
	cues := ParseSRT(content)
	if len(cues) != 2 || cues[0].Timing != "00:00:01,000 --> 00:00:02,000" || cues[0].Text != "Hello\nthere" || cues[1].Text != "[music]" {
		t.Fatalf("cues %+v", cues)
	}
	if cues := ParseSRT("1\n00:00:01,000 --> 00:00:02,000\n\n2\nnot a cue\n"); len(cues) != 0 {
		t.Errorf("blocks without timing or text: %+v", cues)
	}
}

func TestWriteBilingual(t *testing.T) {
	cues := []Cue{{"00:00:01,000 --> 00:00:02,000", "Hello"}, {"00:00:03,000 --> 00:00:04,000", "Bye"}}
	want := strings.Join([]string{"\ufeff1", "00:00:01,000 --> 00:00:02,000", "你好", "Hello", "", "2", "00:00:03,000 --> 00:00:04,000", "Bye", "", ""}, "\n")
	if got := WriteBilingual(cues, []string{"你好", ""}); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func inv(manual, auto []string) ytdlp.SubtitleInventory {
	return ytdlp.SubtitleInventory{ManualLanguages: manual, AutoLanguages: auto}
}

func TestPickSource(t *testing.T) {
	if s, ok := PickSource(inv(nil, []string{"en", "en-orig", "zh-hans"})); !ok || s != (Source{"en-orig", true}) {
		t.Errorf("original captions: %+v %v", s, ok)
	}
	if s, ok := PickSource(inv([]string{"en-us"}, []string{"en-orig", "zh-hans"})); !ok || s != (Source{"en-us", false}) {
		t.Errorf("creator subtitles: %+v %v", s, ok)
	}
	if _, ok := PickSource(inv(nil, []string{"zh-hans-orig", "en"})); ok {
		t.Error("Chinese video picked")
	}
}

func TestChatCompletionsURL(t *testing.T) {
	if u, ok := ChatCompletionsURL(" https://api.deepseek.com/ "); !ok || u != "https://api.deepseek.com/chat/completions" {
		t.Errorf("base: %q", u)
	}
	full := "https://api.openai.com/v1/chat/completions"
	if u, _ := ChatCompletionsURL(full); u != full {
		t.Errorf("full: %q", u)
	}
	if _, ok := ChatCompletionsURL("  "); ok {
		t.Error("empty accepted")
	}
}

func TestMakeBatches(t *testing.T) {
	short := make([]string, 120)
	for i := range short {
		short[i] = "hello there"
	}
	if b := makeBatches(short); len(b) != 1 || len(b[0].lines) != 120 || len(b[0].context) != 0 {
		t.Errorf("short video: %d batches", len(b))
	}
	many := make([]string, 600)
	for i := range many {
		many[i] = "x"
	}
	b := makeBatches(many)
	var sizes []int
	for _, x := range b {
		sizes = append(sizes, len(x.lines))
	}
	if !reflect.DeepEqual(sizes, []int{250, 250, 100}) || b[1].start != 250 || len(b[1].context) != 5 {
		t.Errorf("sizes %v", sizes)
	}
	long := []string{strings.Repeat("y", 3000), strings.Repeat("y", 3000), strings.Repeat("y", 3000)}
	if b := makeBatches(long); len(b) != 3 {
		t.Errorf("character budget: %d batches", len(b))
	}
}

func TestParseTranslations(t *testing.T) {
	if got, ok := ParseTranslations("```json\n[\"你好\", \"[音乐]\"]\n```"); !ok || !reflect.DeepEqual(got, []string{"你好", "[音乐]"}) {
		t.Errorf("fenced: %q %v", got, ok)
	}
	if _, ok := ParseTranslations("sorry"); ok {
		t.Error("prose accepted")
	}
}

// subRunner plays yt-dlp writing source.en.srt.
type subRunner struct{ args []string }

func (r *subRunner) Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error) {
	r.args = args
	var out string
	for i, a := range args {
		if a == "-o" {
			out = args[i+1]
		}
	}
	srt := "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n2\n00:00:03,000 --> 00:00:04,000\nBye\n\n3\n00:00:05,000 --> 00:00:06,000\n[music]\n"
	return 0, os.WriteFile(strings.Replace(out, "%(ext)s", "en.srt", 1), []byte(srt), 0o644)
}

type recorder struct {
	mu       sync.Mutex
	stages   []ytdlp.Stage
	warnings []string
	progress []float64
}

func (r *recorder) reporter() engine.Reporter {
	return engine.Reporter{
		Stage: func(s ytdlp.Stage) {
			r.mu.Lock()
			r.stages = append(r.stages, s)
			r.mu.Unlock()
		},
		Progress: func(p float64) {
			r.mu.Lock()
			r.progress = append(r.progress, p)
			r.mu.Unlock()
		},
		Log: func(string) {},
		Warn: func(code, msg string) {
			r.mu.Lock()
			r.warnings = append(r.warnings, code)
			r.mu.Unlock()
		},
	}
}

func chatServer(t *testing.T, reply func(lines []string) (int, string)) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		user := body.Messages[1].Content
		var lines []string
		json.Unmarshal([]byte(user[strings.LastIndex(user, "\n")+1:]), &lines)
		status, content := reply(lines)
		if status != http.StatusOK {
			http.Error(w, content, status)
			return
		}
		resp, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}, "finish_reason": "stop"}}})
		w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func env(t *testing.T, srv *httptest.Server, runner engine.Runner) Env {
	s := config.DefaultSettings(t.TempDir())
	s.Translation = config.TranslationSettings{Enabled: true, BaseURL: srv.URL + "/v1", Model: "m"}
	return Env{Runner: runner, Settings: s, APIKey: "sk-test", Client: srv.Client(), Backoff: func(int) time.Duration { return time.Millisecond }}
}

func finished(t *testing.T) engine.Finished {
	dir := t.TempDir()
	video := filepath.Join(dir, "My Video.mp4")
	os.WriteFile(video, []byte("video"), 0o644)
	return engine.Finished{
		URL: "https://v/a", Destination: video, TrackType: ytdlp.TrackBoth,
		Video: &ytdlp.Video{SubtitleInventory: inv(nil, []string{"en-orig"})},
	}
}

func TestAfterDownloadWritesBilingualSubtitles(t *testing.T) {
	srv := chatServer(t, func(lines []string) (int, string) {
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = map[string]string{"Hello": "你好", "Bye": "再见", "[music]": "[音乐]"}[l]
		}
		b, _ := json.Marshal(out)
		return http.StatusOK, "```json\n" + string(b) + "\n```"
	})
	runner := &subRunner{}
	rec := &recorder{}
	d := finished(t)
	AfterDownload(context.Background(), env(t, srv, runner), d, rec.reporter())
	got, err := os.ReadFile(strings.TrimSuffix(d.Destination, ".mp4") + ".srt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "你好\nHello") || !strings.Contains(string(got), "[音乐]\n[music]") {
		t.Errorf("srt %q", got)
	}
	if len(rec.warnings) != 0 || rec.stages[0] != ytdlp.StageTranslating || rec.progress[len(rec.progress)-1] != 100 {
		t.Errorf("recorder %+v", rec)
	}
	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "--write-auto-subs --sub-langs (?i)en-orig") || !strings.Contains(joined, "--skip-download") {
		t.Errorf("subtitle args %q", runner.args)
	}
}

func TestAfterDownloadWarnings(t *testing.T) {
	srv := chatServer(t, func(lines []string) (int, string) { return http.StatusUnauthorized, "bad key" })
	cases := []struct {
		name   string
		change func(*Env, *engine.Finished)
		want   string
	}{
		{"no key", func(e *Env, _ *engine.Finished) { e.APIKey = "" }, CodeNotConfigured},
		{"no model", func(e *Env, _ *engine.Finished) { e.Settings.Translation.Model = "" }, CodeNotConfigured},
		{"no source", func(_ *Env, d *engine.Finished) { d.Video = &ytdlp.Video{} }, CodeSourceMissing},
		{"rejected", func(e *Env, _ *engine.Finished) { e.APIKey = "sk-wrong" }, CodeFailed},
	}
	for _, c := range cases {
		e, d, rec := env(t, srv, &subRunner{}), finished(t), &recorder{}
		c.change(&e, &d)
		AfterDownload(context.Background(), e, d, rec.reporter())
		if len(rec.warnings) != 1 || rec.warnings[0] != c.want {
			t.Errorf("%s: warnings %q", c.name, rec.warnings)
		}
	}

	// Off, or audio: nothing happens.
	e, d, rec := env(t, srv, &subRunner{}), finished(t), &recorder{}
	d.TrackType = ytdlp.TrackAudio
	AfterDownload(context.Background(), e, d, rec.reporter())
	if len(rec.stages) != 0 {
		t.Error("audio translated")
	}
}

func TestPartialTranslation(t *testing.T) {
	srv := chatServer(t, func(lines []string) (int, string) {
		if len(lines) > 1 {
			return http.StatusOK, `["only one"]` // wrong count: the batch splits
		}
		if lines[0] == "Bye" {
			return http.StatusInternalServerError, "busy"
		}
		return http.StatusOK, `["译文"]`
	})
	rec := &recorder{}
	d := finished(t)
	AfterDownload(context.Background(), env(t, srv, &subRunner{}), d, rec.reporter())
	if len(rec.warnings) != 1 || rec.warnings[0] != CodePartial {
		t.Fatalf("warnings %q", rec.warnings)
	}
}
