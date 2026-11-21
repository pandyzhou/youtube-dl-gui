package translate

import (
	"context"
	"encoding/json"
	"fmt"
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
	if !reflect.DeepEqual(sizes, []int{120, 120, 120, 120, 120}) || b[1].start != 120 || len(b[1].context) != 5 {
		t.Errorf("sizes %v", sizes)
	}
	if len(b[0].after) != 3 || len(b[4].after) != 0 {
		t.Errorf("following context %d, %d", len(b[0].after), len(b[4].after))
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
	if got, ok := ParseTranslations("<think>Maybe [music] is a tag.</think>\n[\"好\"]"); !ok || !reflect.DeepEqual(got, []string{"好"}) {
		t.Errorf("think: %q %v", got, ok)
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

func TestPromptAndContext(t *testing.T) {
	if p := prompt("简体中文"); !strings.Contains(p, "into 简体中文") || !strings.Contains(p, "pronouns") || strings.Contains(p, "{") {
		t.Errorf("chinese prompt %q", p)
	}
	if p := prompt("English"); strings.Contains(p, "pronouns") || strings.Contains(p, "{style}") {
		t.Errorf("english prompt %q", p)
	}
	d := engine.Finished{Title: "Fallback", Video: &ytdlp.Video{Title: "Snow test", Uploader: "Cars", Description: "line one\n\nline   two"}}
	if got := videoContext(d); got != "Title: Snow test\nChannel: Cars\nDescription: line one line two\n" {
		t.Errorf("video context %q", got)
	}
	if got := videoContext(engine.Finished{Title: "Only"}); got != "Title: Only\n" {
		t.Errorf("title only %q", got)
	}
}

func TestHighQualityMode(t *testing.T) {
	var mu sync.Mutex
	var notesAsked bool
	var batchUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		system, user := body.Messages[0].Content, body.Messages[1].Content
		content := `{"topic": "测试视频", "terms": [{"source": "Hello", "target": "哈喽", "note": "greeting"}, {"source": "", "target": "x"}]}`
		mu.Lock()
		if strings.Contains(system, "You prepare") {
			notesAsked = strings.Contains(user, "Transcript:\nHello\nBye\n[music]\n")
		} else {
			batchUser = user
			content = `["哈喽", "再见", "[音乐]"]`
		}
		mu.Unlock()
		resp, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}, "finish_reason": "stop"}}})
		w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	e := env(t, srv, &subRunner{})
	e.Settings.Translation.HighQuality = true
	rec := &recorder{}
	AfterDownload(context.Background(), e, finished(t), rec.reporter())
	if len(rec.warnings) != 0 || !notesAsked {
		t.Fatalf("warnings %q, notes asked %v", rec.warnings, notesAsked)
	}
	if !strings.Contains(batchUser, "Topic: 测试视频\nGlossary, translate these terms this way:\n- Hello → 哈喽 (greeting)\n") {
		t.Errorf("batch request %q", batchUser)
	}
	if _, ok := ParseNotes(`{"topic": "", "terms": []}`); ok {
		t.Error("empty notes accepted")
	}
}

func TestSegmentAutomaticCaptions(t *testing.T) {
	// Rolling YouTube captions: words with offsets, newline appends, an
	// event running on under the next, a pause, a sound tag.
	data := []byte(`{"events": [
		{"tStartMs": 0, "dDurationMs": 6000, "wWinId": 1, "segs": [{"utf8": "so"}, {"utf8": " today", "tOffsetMs": 300}, {"utf8": " we're", "tOffsetMs": 700}, {"utf8": " going", "tOffsetMs": 900}]},
		{"tStartMs": 1200, "dDurationMs": 10, "aAppend": 1, "segs": [{"utf8": "\n"}]},
		{"tStartMs": 1200, "dDurationMs": 4000, "wWinId": 1, "segs": [{"utf8": "to"}, {"utf8": " look", "tOffsetMs": 200}, {"utf8": " at", "tOffsetMs": 400}, {"utf8": " this", "tOffsetMs": 500}]},
		{"tStartMs": 4000, "dDurationMs": 2000, "segs": [{"utf8": "[Music]"}]},
		{"tStartMs": 6500, "dDurationMs": 3000, "segs": [{"utf8": "okay"}, {"utf8": " thanks.", "tOffsetMs": 400}, {"utf8": " Bye", "tOffsetMs": 900}]}
	]}`)
	words, err := ParseJSON3(data)
	if err != nil || len(words) != 12 {
		t.Fatalf("words %d, %v", len(words), err)
	}
	if w := words[3]; w.Text != "going" || w.Start != 900 || w.End != 1200 {
		t.Errorf("going %+v", w)
	}
	if w := words[7]; w.Text != "this" || w.End != 1700+300+60*4 {
		t.Errorf("this %+v", w)
	}
	var got []string
	for _, c := range Segment(words) {
		got = append(got, c.Timing+" "+c.Text)
	}
	want := []string{
		"00:00:00,000 --> 00:00:02,740 so today we're going to look at this",
		"00:00:04,000 --> 00:00:05,220 [Music]",
		"00:00:06,500 --> 00:00:08,380 okay thanks. Bye",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cues\n%s", strings.Join(got, "\n"))
	}
}

func TestSegmentSpeakers(t *testing.T) {
	data := []byte(`{"events": [
		{"tStartMs": 0, "dDurationMs": 4000, "segs": [{"utf8": "Right."}, {"utf8": " Right.", "tOffsetMs": 240}, {"utf8": " Okay.", "tOffsetMs": 480}]},
		{"tStartMs": 1000, "dDurationMs": 4000, "segs": [{"utf8": ">> Hold"}, {"utf8": " on.", "tOffsetMs": 160}]},
		{"tStartMs": 1500, "dDurationMs": 4000, "segs": [{"utf8": ">>"}, {"utf8": " Nice.", "tOffsetMs": 100}]}
	]}`)
	words, err := ParseJSON3(data)
	if err != nil || len(words) != 6 || words[3].Text != "Hold" || !words[3].Turn || words[4].Turn || !words[5].Turn {
		t.Fatalf("words %+v, %v", words, err)
	}
	var got []string
	for _, c := range Segment(words) {
		got = append(got, c.Text)
	}
	if !reflect.DeepEqual(got, []string{"Right. Right. Okay.", "Hold on.", "Nice."}) {
		t.Errorf("lines %q", got)
	}
}

func TestSegmentLongAndCJK(t *testing.T) {
	var words []Word
	for i := range 40 {
		words = append(words, Word{Start: i * 300, End: i*300 + 300, Text: "word"})
	}
	cues := Segment(words)
	if len(cues) != 3 || len(strings.Fields(cues[0].Text)) != maxWeight {
		t.Errorf("long run: %d cues, first %q", len(cues), cues[0].Text)
	}
	cjk := []Word{{Start: 0, End: 500, Text: "今天"}, {Start: 500, End: 1000, Text: "我们"}, {Start: 1000, End: 1500, Text: "来看"}}
	if c := Segment(cjk); len(c) != 1 || c[0].Text != "今天我们来看" {
		t.Errorf("cjk %+v", c)
	}
	if n := wordWeight("今天我们"); n != 2 {
		t.Errorf("cjk weight %d", n)
	}
}

// json3Runner plays yt-dlp writing json3 automatic captions.
type json3Runner struct{ calls int }

func (r *json3Runner) Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error) {
	r.calls++
	var out string
	for i, a := range args {
		if a == "-o" {
			out = args[i+1]
		}
	}
	data := `{"events": [{"tStartMs": 0, "dDurationMs": 2000, "segs": [{"utf8": "Hello."}]}, {"tStartMs": 3000, "dDurationMs": 2000, "segs": [{"utf8": "Bye."}]}]}`
	return 0, os.WriteFile(strings.Replace(out, "%(ext)s", "en-orig.json3", 1), []byte(data), 0o644)
}

func TestAutomaticCaptionsUseJSON3(t *testing.T) {
	srv := chatServer(t, func(lines []string) (int, string) {
		b, _ := json.Marshal(lines)
		return http.StatusOK, strings.NewReplacer("Hello.", "你好", "Bye.", "再见").Replace(string(b))
	})
	runner, rec, d := &json3Runner{}, &recorder{}, finished(t)
	AfterDownload(context.Background(), env(t, srv, runner), d, rec.reporter())
	got, err := os.ReadFile(strings.TrimSuffix(d.Destination, ".mp4") + ".srt")
	if err != nil || len(rec.warnings) != 0 || runner.calls != 1 {
		t.Fatalf("err %v, warnings %q, calls %d", err, rec.warnings, runner.calls)
	}
	if !strings.Contains(string(got), "00:00:00,000 --> 00:00:01,160\n你好\nHello.") {
		t.Errorf("srt %q", got)
	}
}

func TestListModelsAndEffort(t *testing.T) {
	var effort any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/models" && r.Header.Get("Authorization") == "Bearer sk-test":
			w.Write([]byte(`{"data": [{"id": "m-2"}, {"id": "m-1"}, {"id": "m-2"}]}`))
		case r.URL.Path == "/v1/chat/completions":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			effort = body["reasoning_effort"]
			w.Write([]byte(`{"choices": [{"message": {"content": "[\"好\"]"}, "finish_reason": "stop"}]}`))
		default:
			http.Error(w, "no", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(srv.Close)
	models, err := ListModels(context.Background(), srv.Client(), srv.URL+"/v1/chat/completions", "sk-test")
	if err != nil || !reflect.DeepEqual(models, []string{"m-1", "m-2"}) {
		t.Fatalf("models %q, %v", models, err)
	}
	if _, err := ListModels(context.Background(), srv.Client(), srv.URL+"/v1", "wrong"); err == nil {
		t.Error("rejected key listed models")
	}

	e := env(t, srv, nil)
	c, err := chatConfig(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := request(context.Background(), e, c, batch{lines: []string{"hi"}}); err != nil || effort != nil {
		t.Errorf("default effort sent: %v, %v", effort, err)
	}
	e.Settings.Translation.ReasoningEffort = "low"
	c, _ = chatConfig(e)
	if _, err := request(context.Background(), e, c, batch{lines: []string{"hi"}}); err != nil || effort != "low" {
		t.Errorf("effort %v, %v", effort, err)
	}
}

func TestModelTest(t *testing.T) {
	srv := chatServer(t, func(lines []string) (int, string) {
		if len(lines) != 1 || lines[0] != TestLine {
			return http.StatusOK, `["a", "b"]`
		}
		return http.StatusOK, `["大家好"]`
	})
	s := config.TranslationSettings{BaseURL: srv.URL + "/v1", Model: "m"}
	if got, err := TestModel(context.Background(), srv.Client(), s, "sk-test"); err != nil || got != "大家好" {
		t.Errorf("test %q, %v", got, err)
	}
	if _, err := TestModel(context.Background(), srv.Client(), s, "sk-wrong"); err == nil {
		t.Error("wrong key passed")
	}
	s.Model = ""
	if _, err := TestModel(context.Background(), srv.Client(), s, "sk-test"); err == nil || !strings.Contains(err.Error(), "模型") {
		t.Errorf("no model: %v", err)
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

func TestCheckpointResumes(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	fail := true
	srv := chatServer(t, func(lines []string) (int, string) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if fail && strings.Contains(strings.Join(lines, "|"), "Bye") {
			return http.StatusBadRequest, "stop here"
		}
		out := make([]string, len(lines))
		for i := range lines {
			out[i] = "译" + lines[i]
		}
		b, _ := json.Marshal(out)
		return http.StatusOK, string(b)
	})
	e := env(t, srv, nil)
	e.CacheDir = t.TempDir()
	texts := make([]string, 0, 300)
	for i := range 300 {
		texts = append(texts, fmt.Sprintf("line %d", i))
	}
	texts[200] = "Bye" // the second batch fails outright the first time
	c, _ := chatConfig(e)
	rec := &recorder{}
	cp := openCheckpoint(e.CacheDir, "https://v/a\nen", texts, c.target)
	if _, err := translateCues(context.Background(), e, c, texts, rec.reporter(), cp); err == nil {
		t.Fatal("first run should stop on the rejected batch")
	}
	saved := cp.count()
	if saved == 0 || saved >= len(makeBatches(texts)) {
		t.Fatalf("saved %d batches", saved)
	}

	fail, calls = false, 0
	cp = openCheckpoint(e.CacheDir, "https://v/a\nen", texts, c.target)
	out, err := translateCues(context.Background(), e, c, texts, rec.reporter(), cp)
	if err != nil || out[0] != "译line 0" || out[200] != "译Bye" || out[299] != "译line 299" {
		t.Fatalf("second run: %v, %q %q", err, out[0], out[200])
	}
	if want := len(makeBatches(texts)) - saved; calls != want {
		t.Errorf("second run sent %d requests, want %d", calls, want)
	}

	// Other lines or another language start over.
	if openCheckpoint(e.CacheDir, "https://v/a\nen", texts[1:], c.target).count() != 0 ||
		openCheckpoint(e.CacheDir, "https://v/a\nen", texts, "English").count() != 0 {
		t.Error("checkpoint reused for different lines or language")
	}
}
