package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// fakeYtdlp answers information fetches from infos and plays a scripted
// download, recording the arguments.
type fakeYtdlp struct {
	mu       sync.Mutex
	infos    map[string]string   // URL → JSON
	failInfo map[string][]string // URL → stderr lines, exit 1
	failDown map[string][]string
	block    map[string]bool // downloads that run until canceled
	calls    [][]string
	running  int
	peak     int
}

func (f *fakeYtdlp) Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string(nil), args...))
	url := args[len(args)-1]
	info := contains(args, "-J")
	failInfo, infoFails := f.failInfo[url]
	body, known := f.infos[url]
	failDown, downFails := f.failDown[url]
	block := f.block[url]
	f.mu.Unlock()
	if info {
		if lines := failInfo; infoFails {
			for _, l := range lines {
				onLine(proc.Line{Stderr: true, Text: l})
			}
			return 1, nil
		}
		onLine(proc.Line{Text: "[youtube] Extracting URL: " + url})
		if !known {
			onLine(proc.Line{Stderr: true, Text: "ERROR: Unsupported URL: " + url})
			return 1, nil
		}
		onLine(proc.Line{Text: body})
		return 0, nil
	}

	f.mu.Lock()
	f.running++
	f.peak = max(f.peak, f.running)
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.running--
		f.mu.Unlock()
	}()
	out := argAfter(args, "-o")
	onLine(proc.Line{Text: "[download] Destination: " + out + ".f137.mp4"})
	onLine(proc.Line{Text: "RAW|50.0| 50.0%|1048576|10|5|10|NA|NA|NA"})
	if block {
		<-ctx.Done()
		return -1, ctx.Err()
	}
	time.Sleep(5 * time.Millisecond)
	if lines := failDown; downFails {
		for _, l := range lines {
			onLine(proc.Line{Stderr: true, Text: l})
		}
		return 1, nil
	}
	onLine(proc.Line{Text: fmt.Sprintf(`[Merger] Merging formats into "%s.mp4"`, out)})
	return 0, nil
}

func (f *fakeYtdlp) downloads() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out [][]string
	for _, c := range f.calls {
		if !contains(c, "-J") {
			out = append(out, c)
		}
	}
	return out
}

func contains(items []string, s string) bool {
	for _, item := range items {
		if item == s {
			return true
		}
	}
	return false
}

func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func videoJSON(id, title string) string {
	b, _ := json.Marshal(map[string]any{
		"id": id, "title": title, "webpage_url": "https://v/" + id, "duration": 60,
		"formats": []map[string]any{
			{"format_id": "137", "height": 1080, "fps": 30, "vcodec": "avc1.640028", "acodec": "none", "ext": "mp4", "filesize": 100 << 20},
			{"format_id": "136", "height": 720, "fps": 30, "vcodec": "avc1.4d401f", "acodec": "none", "ext": "mp4", "filesize": 50 << 20},
			{"format_id": "140", "abr": 129, "vcodec": "none", "acodec": "mp4a.40.2", "ext": "m4a", "filesize": 5 << 20},
		},
	})
	return string(b)
}

func playlistJSON(id string, n int) string {
	var entries []map[string]any
	for i := 1; i <= n; i++ {
		entries = append(entries, map[string]any{"url": fmt.Sprintf("https://v/%s-%d", id, i)})
	}
	b, _ := json.Marshal(map[string]any{"id": id, "_type": "playlist", "title": "List " + id, "playlist_count": n, "entries": entries})
	return string(b)
}

type harness struct {
	t      *testing.T
	fake   *fakeYtdlp
	e      *Engine
	mu     sync.Mutex
	events []Event
	s      config.Settings
}

func newHarness(t *testing.T, change func(*config.Settings)) *harness {
	h := &harness{t: t, fake: &fakeYtdlp{infos: map[string]string{}, failInfo: map[string][]string{}, failDown: map[string][]string{}, block: map[string]bool{}}}
	h.s = config.DefaultSettings(filepath.Join("C:", "Downloads"))
	h.s.Performance.MaxConcurrency = 2
	h.s.Performance.SplitPlaylistThreshold = 3
	if change != nil {
		change(&h.s)
	}
	h.e = New(Deps{
		Runner:      h.fake,
		Settings:    func() config.Settings { return h.s },
		Preferences: func() config.Preferences { return config.Preferences{} },
		FallbackDir: "fallback",
		OnEvent: func(ev Event) {
			h.mu.Lock()
			h.events = append(h.events, ev)
			h.mu.Unlock()
		},
	})
	t.Cleanup(h.e.Close)
	return h
}

// wait polls until cond holds for the job list.
func (h *harness) wait(what string, cond func([]JobView) bool) []JobView {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		jobs := h.e.Jobs()
		if cond(jobs) {
			return jobs
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s; jobs %+v", what, jobs)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (h *harness) waitStatus(id string, s Status) JobView {
	h.t.Helper()
	var view JobView
	h.wait(fmt.Sprintf("%s to be %s", id, s), func(jobs []JobView) bool {
		for _, j := range jobs {
			if j.ID == id {
				view = j
				return j.Status == s
			}
		}
		return false
	})
	return view
}

func (h *harness) hasEvent(kind EventKind) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ev := range h.events {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

func TestSingleVideoAddAndDownload(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/a"] = videoJSON("a", "Video A")
	id := h.e.Add("https://v/a", AddOptions{Preset: "720p", Start: true})
	j := h.waitStatus(id, StatusDone)
	if j.Title != "Video A" || j.Items[0].Choice != "720p" || j.Items[0].Size != 55<<20 || j.Percent != 100 {
		t.Errorf("job %+v", j)
	}
	if !strings.HasSuffix(j.Items[0].Destination, ".mp4") || strings.Contains(j.Items[0].Destination, ".f137") {
		t.Errorf("destination %q, want the merged file", j.Items[0].Destination)
	}
	args := h.fake.downloads()[0]
	if args[0] != "--encoding" || args[len(args)-1] != "https://v/a" || argAfter(args, "-f") == "" || !strings.Contains(argAfter(args, "-f"), "[height<=720]") {
		t.Errorf("download args %q", args)
	}
	if out := argAfter(args, "-o"); !strings.HasPrefix(out, filepath.Join("C:", "Downloads")) {
		t.Errorf("output %q", out)
	}
	if !h.hasEvent(EventVideoReady) || !h.hasEvent(EventDownloadDone) || !h.hasEvent(EventQueueDone) {
		t.Errorf("events %+v", h.events)
	}
	if log := h.e.Log(id); len(log) == 0 || strings.HasPrefix(log[len(log)-1], "RAW|") {
		t.Errorf("log %q", log)
	}
}

func TestAddWithoutStartWaits(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/a"] = videoJSON("a", "Video A")
	h.fake.infos["https://v/b"] = videoJSON("b", "Video B")
	a := h.e.Add("https://v/a", AddOptions{})
	b := h.e.Add("https://v/b", AddOptions{Preset: formats.PresetAudioMP3})
	h.waitStatus(a, StatusReady)
	h.waitStatus(b, StatusReady)
	if jobs := h.e.Jobs(); jobs[0].ID != b {
		t.Error("newest job not first")
	}
	if len(h.fake.downloads()) != 0 {
		t.Fatal("downloaded without being started")
	}
	if n := h.e.StartAll(); n != 2 {
		t.Fatalf("StartAll started %d", n)
	}
	h.waitStatus(a, StatusDone)
	jb := h.waitStatus(b, StatusDone)
	if jb.Items[0].Choice != "音频 MP3" {
		t.Errorf("choice %q", jb.Items[0].Choice)
	}
	var audioArgs []string
	for _, c := range h.fake.downloads() {
		if c[len(c)-1] == "https://v/b" {
			audioArgs = c
		}
	}
	if !contains(audioArgs, "-x") || argAfter(audioArgs, "--audio-format") != "mp3" {
		t.Errorf("audio args %q", audioArgs)
	}
}

func TestFetchErrorAndRetry(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.failInfo["https://v/x"] = []string{"ERROR: [youtube] dQw4w9WgXcQ: Sign in to confirm you're not a bot"}
	id := h.e.Add("https://v/x", AddOptions{Start: true})
	j := h.waitStatus(id, StatusError)
	if j.Err != "Sign in to confirm you're not a bot" || len(j.Items[0].Diagnostics) != 1 || j.Items[0].Diagnostics[0].Code != "signInRequiredForBotDetection" {
		t.Errorf("job %+v", j)
	}
	h.fake.mu.Lock()
	delete(h.fake.failInfo, "https://v/x")
	h.fake.infos["https://v/x"] = videoJSON("x", "Now works")
	h.fake.mu.Unlock()
	h.e.Retry(id)
	h.waitStatus(id, StatusDone) // auto start is kept
}

func TestDownloadErrorAndRetry(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/a"] = videoJSON("a", "A")
	h.fake.failDown["https://v/a"] = []string{"ERROR: unable to download video data: HTTP Error 403: Forbidden"}
	id := h.e.Add("https://v/a", AddOptions{Start: true})
	j := h.waitStatus(id, StatusError)
	if j.Items[0].Err != "unable to download video data: HTTP Error 403: Forbidden" || !h.hasEvent(EventDownloadError) {
		t.Errorf("item %+v", j.Items[0])
	}
	h.fake.mu.Lock()
	delete(h.fake.failDown, "https://v/a")
	h.fake.mu.Unlock()
	h.e.Retry(id)
	h.waitStatus(id, StatusDone)
}

func TestPauseAndResume(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/a"] = videoJSON("a", "A")
	h.fake.block["https://v/a"] = true
	id := h.e.Add("https://v/a", AddOptions{Start: true})
	j := h.waitStatus(id, StatusDownloading)
	h.wait("progress", func(jobs []JobView) bool { return jobs[0].Items[0].Percent == 50 })
	h.e.Pause(id)
	j = h.waitStatus(id, StatusPaused)
	if j.Items[0].SpeedBps != 0 {
		t.Errorf("paused item still has speed %+v", j.Items[0])
	}
	h.fake.mu.Lock()
	h.fake.block["https://v/a"] = false
	h.fake.mu.Unlock()
	h.e.Start(id)
	h.waitStatus(id, StatusDone)
	if n := len(h.fake.downloads()); n != 2 {
		t.Errorf("%d download runs, want 2", n)
	}
}

func TestPlaylistSelectionAndSplit(t *testing.T) {
	h := newHarness(t, nil) // split below 3 videos
	h.fake.infos["https://v/list"] = playlistJSON("PL", 5)
	for i := 1; i <= 5; i++ {
		u := fmt.Sprintf("https://v/PL-%d", i)
		h.fake.infos[u] = videoJSON(fmt.Sprintf("PL-%d", i), fmt.Sprintf("Entry %d", i))
	}
	older := h.e.Add("https://v/none", AddOptions{}) // fails; stays below the playlist
	id := h.e.Add("https://v/list", AddOptions{})
	j := h.waitStatus(id, StatusSelecting)
	if j.Title != "List PL" || len(j.Playlist.Entries) != 5 {
		t.Fatalf("playlist %+v", j)
	}
	if err := h.e.SelectPlaylist(id, []formats.SelectionRow{{Index: 9}}, false); err == nil {
		t.Error("empty selection accepted")
	}
	if err := h.e.SelectPlaylist(id, []formats.SelectionRow{{Index: 4}, {Index: 2}}, false); err != nil {
		t.Fatal(err)
	}
	jobs := h.wait("split", func(jobs []JobView) bool {
		return len(jobs) == 3 && jobs[0].Status == StatusReady && jobs[1].Status == StatusReady
	})
	if jobs[0].Title != "Entry 4" || jobs[1].Title != "Entry 2" || jobs[2].ID != older || jobs[0].Playlist == nil {
		t.Fatalf("split jobs %+v", jobs)
	}
	h.e.Start(jobs[1].ID)
	h.waitStatus(jobs[1].ID, StatusDone)
	args := h.fake.downloads()[0]
	if contains(args, "-I") || !contains(args, "--no-playlist") {
		t.Errorf("entry download args %q", args)
	}
}

func TestPlaylistCombinedTemplateValues(t *testing.T) {
	h := newHarness(t, func(s *config.Settings) {
		s.Output.FileNameTemplate = "%(playlist_index)02d-%(playlist_title)s-%(title)s.%(ext)s"
		s.Output.ReversePlaylistNumbering = true
	})
	h.fake.infos["https://v/list"] = playlistJSON("PL", 4)
	for i := 1; i <= 4; i++ {
		h.fake.infos[fmt.Sprintf("https://v/PL-%d", i)] = videoJSON(fmt.Sprintf("PL-%d", i), fmt.Sprintf("Entry %d", i))
	}
	id := h.e.Add("https://v/list", AddOptions{Start: true}) // 4 ≥ 3: one card, all videos
	j := h.waitStatus(id, StatusDone)
	if !j.Combined || len(j.Items) != 4 || j.Done != 4 || j.Duration != 240 {
		t.Fatalf("combined job %+v", j)
	}
	var outs []string
	for _, c := range h.fake.downloads() {
		outs = append(outs, filepath.Base(argAfter(c, "-o")))
	}
	want := map[string]bool{"04-List PL-%(title)s.%(ext)s": true, "01-List PL-%(title)s.%(ext)s": true}
	found := 0
	for _, o := range outs {
		if want[o] {
			found++
		}
	}
	if found != 2 {
		t.Errorf("outputs %q", outs)
	}
	if !h.hasEvent(EventPlaylistReady) {
		t.Error("no playlist ready event")
	}
}

func TestConcurrencyLimit(t *testing.T) {
	h := newHarness(t, nil)
	for i := 0; i < 6; i++ {
		u := fmt.Sprintf("https://v/%d", i)
		h.fake.infos[u] = videoJSON(fmt.Sprint(i), u)
		h.e.Add(u, AddOptions{Start: true})
	}
	h.wait("all done", func(jobs []JobView) bool {
		for _, j := range jobs {
			if j.Status != StatusDone {
				return false
			}
		}
		return len(jobs) == 6
	})
	if h.fake.peak > 2 {
		t.Errorf("%d downloads at once, limit 2", h.fake.peak)
	}
}

func TestRemoveCancelsAndClear(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/a"] = videoJSON("a", "A")
	h.fake.infos["https://v/b"] = videoJSON("b", "B")
	h.fake.block["https://v/a"] = true
	a := h.e.Add("https://v/a", AddOptions{Start: true})
	b := h.e.Add("https://v/b", AddOptions{Start: true})
	h.waitStatus(a, StatusDownloading)
	h.waitStatus(b, StatusDone)
	h.e.Remove(a)
	h.wait("a removed and stopped", func(jobs []JobView) bool {
		h.fake.mu.Lock()
		defer h.fake.mu.Unlock()
		return len(jobs) == 1 && h.fake.running == 0
	})
	if n := h.e.RemoveWhere(func(j JobView) bool { return j.Status == StatusDone }); n != 1 || len(h.e.Jobs()) != 0 {
		t.Errorf("cleared %d", n)
	}
}

func TestSkippedByFilter(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/a"] = videoJSON("a", "A")
	runner := &skipRunner{fakeYtdlp: h.fake}
	h.e.d.Runner = runner
	id := h.e.Add("https://v/a", AddOptions{Start: true})
	j := h.waitStatus(id, StatusSkipped)
	if j.Items[0].Diagnostics[0].Code != ytdlp.CodeInputFilterSkipped {
		t.Errorf("item %+v", j.Items[0])
	}
}

// skipRunner downloads nothing and reports a filter skip.
type skipRunner struct{ *fakeYtdlp }

func (s *skipRunner) Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error) {
	if contains(args, "-J") {
		return s.fakeYtdlp.Run(ctx, args, onLine)
	}
	onLine(proc.Line{Text: "[download] File is larger than max-filesize (123 > 100), skipping"})
	return 0, nil
}

func TestPoolTakesTurns(t *testing.T) {
	p := newPool(1)
	defer p.close()
	var mu sync.Mutex
	var order []string
	record := func(name string) {
		mu.Lock()
		order = append(order, name)
		mu.Unlock()
	}
	done := make(chan struct{})
	gate := make(chan struct{})
	ctx := context.Background()
	p.submit("block", ctx, func(context.Context) { <-gate })
	for _, name := range []string{"a1", "a2", "a3"} {
		name := name
		p.submit("a", ctx, func(context.Context) { record(name) })
	}
	p.submit("b", ctx, func(context.Context) { record("b1") })
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	p.submit("c", canceled, func(context.Context) { record("c1") })
	p.submit("z", ctx, func(context.Context) { close(done) })
	close(gate)
	<-done
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if got := strings.Join(order, ","); got != "a1,b1,a2,a3" {
		t.Errorf("order %s", got)
	}
}
