package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pandyzhou/lumadl/internal/config"
	"github.com/pandyzhou/lumadl/internal/proc"
)

// oneStepYtdlp answers the format selection and writes the parts of a
// one-step download.
type oneStepYtdlp struct {
	fakeYtdlp
	dir string
}

func (f *oneStepYtdlp) Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error) {
	switch {
	case contains(args, "--simulate"):
		f.mu.Lock()
		f.calls = append(f.calls, append([]string(nil), args...))
		f.mu.Unlock()
		onLine(proc.Line{Text: "137+251"})
		onLine(proc.Line{Text: filepath.Join(f.dir, "Video A.webm")})
		return 0, nil
	case strings.Contains(argAfter(args, "-f"), ","):
		out := argAfter(args, "-o")
		for _, id := range strings.Split(argAfter(args, "-f"), ",") {
			ext := map[string]string{"137": "mp4", "251": "webm"}[id]
			os.WriteFile(strings.NewReplacer("%(format_id)s", id, "%(ext)s", ext).Replace(out), []byte(id), 0o644)
		}
		parts := filepath.Dir(out)
		os.WriteFile(filepath.Join(parts, "sub.en.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"), 0o644)
		os.WriteFile(filepath.Join(parts, "cover.jpg"), []byte("jpg"), 0o644)
	}
	return f.fakeYtdlp.Run(ctx, args, onLine)
}

// fakeFFmpeg records the merge and writes its output.
type fakeFFmpeg struct {
	mu   sync.Mutex
	args []string
}

func (f *fakeFFmpeg) Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error) {
	f.mu.Lock()
	f.args = args
	f.mu.Unlock()
	onLine(proc.Line{Text: "out_time_us=5000000"})
	return 0, os.WriteFile(args[len(args)-1], []byte("merged"), 0o644)
}

func TestOneStepDownload(t *testing.T) {
	dir := t.TempDir()
	run := &oneStepYtdlp{fakeYtdlp: fakeYtdlp{infos: map[string]string{}, failInfo: map[string][]string{}, failDown: map[string][]string{}, block: map[string]bool{}}, dir: dir}
	run.infos["https://v/a"] = videoJSON("a", "Video A")
	ff := &fakeFFmpeg{}
	s := config.DefaultSettings(dir)
	s.Output.Video.Container = "mkv"
	s.Subtitles.Enabled, s.Subtitles.EmbedSubtitles = true, true
	s.Output.AddMetadata, s.Output.AddThumbnail = true, true
	var mu sync.Mutex
	var gotEarly any
	var gotVideo string
	e := New(Deps{
		Runner:   run,
		Settings: func() config.Settings { return s },
		BeforeDownload: func(ctx context.Context, st Started, r Reporter) any {
			return "translated"
		},
		FFmpeg: ff,
		MergeTracks: func(ctx context.Context, d Finished, tracks string, r Reporter) []Track {
			mu.Lock()
			gotEarly, gotVideo = d.Early, d.Destination
			mu.Unlock()
			ass := filepath.Join(tracks, "translated.ass")
			os.WriteFile(ass, []byte("ass"), 0o644)
			return []Track{{Path: ass, Title: "中英双语（描边）", Language: "chi", Default: true}}
		},
		OneStep: func() bool { return true },
	})
	t.Cleanup(e.Close)
	h := &harness{t: t, e: e, fake: &run.fakeYtdlp}
	id := e.Add("https://v/a", AddOptions{Start: true})
	j := h.waitStatus(id, StatusDone)

	dest := filepath.Join(dir, "Video A.mkv")
	if data, err := os.ReadFile(dest); err != nil || string(data) != "merged" || j.Items[0].Destination != dest {
		t.Fatalf("destination %q, file %q %v", j.Items[0].Destination, data, err)
	}
	mu.Lock()
	if gotEarly != "translated" || !strings.HasSuffix(gotVideo, "part.f137.mp4") {
		t.Errorf("merge tracks got early %v, video %q", gotEarly, gotVideo)
	}
	mu.Unlock()
	args := strings.Join(ff.args, " ")
	for _, want := range []string{"-map 0:v:0 -map 1:a:0", "-f ffmetadata", "-map_metadata 2", "title=中英双语（描边） -disposition:s:0 default",
		"-metadata:s:s:1 language=en", "-attach ", "-c copy"} {
		if !strings.Contains(args, want) {
			t.Errorf("ffmpeg args missing %q:\n%s", want, args)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, ".ovd-parts-*")); len(matches) != 0 {
		t.Errorf("parts left %q", matches)
	}
	for _, c := range run.downloads() {
		if !contains(c, "--simulate") && (contains(c, "--embed-subs") || contains(c, "--embed-thumbnail")) {
			t.Errorf("yt-dlp post-processed: %q", c)
		}
	}
}

func TestOneStepFallsBackForSingleFiles(t *testing.T) {
	h := newHarness(t, func(s *config.Settings) { s.Output.Video.Container = "mkv" })
	h.fake.infos["https://v/a"] = videoJSON("a", "Video A")
	ff := &fakeFFmpeg{}
	h.e.d.FFmpeg = ff
	h.e.d.MergeTracks = func(ctx context.Context, d Finished, dir string, r Reporter) []Track { return nil }
	h.e.d.OneStep = func() bool { return true }
	id := h.e.Add("https://v/a", AddOptions{Start: true})
	h.waitStatus(id, StatusDone) // the selection printed nothing: the usual download ran
	if ff.args != nil {
		t.Errorf("merged %q", ff.args)
	}
}
