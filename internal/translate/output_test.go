package translate

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/pandyzhou/lumadl/internal/engine"
	"github.com/pandyzhou/lumadl/internal/ytdlp"
)

func TestWriteASS(t *testing.T) {
	cues := []Cue{
		{Timing: "00:00:01,000 --> 00:00:02,500", Text: "Hello {there}"},
		{Timing: "01:02:03,450 --> 01:02:04,000", Text: "[music]"},
		{Timing: "broken", Text: "skipped"},
	}
	got := WriteASS(cues, []string{"你好", ""}, 1920, 1080)
	for _, want := range []string{
		"PlayResX: 2560\nPlayResY: 1440\n",
		"Style: ZH,Microsoft YaHei,76,",
		",2,120,120,70,1\n",
		"Dialogue: 0,0:00:01.00,0:00:02.50,ZH,,0,0,0,,你好\\N{\\rEN}Hello (there)\n",
		"Dialogue: 0,1:02:03.45,1:02:04.00,EN,,0,0,0,,[music]\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "skipped") {
		t.Error("cue without a timing written")
	}
	if vertical := WriteASS(nil, nil, 1080, 1920); !strings.Contains(vertical, "PlayResX: 810\n") {
		t.Errorf("portrait video: %s", vertical[:80])
	}
}

func TestParseProbe(t *testing.T) {
	p := parseProbe([]byte(`{"streams": [
		{"index": 0, "codec_type": "video", "width": 2560, "height": 1440},
		{"index": 1, "codec_type": "audio"},
		{"index": 2, "codec_type": "subtitle", "tags": {"title": "中英双语（描边）"}},
		{"index": 3, "codec_type": "subtitle", "tags": {"language": "eng"}},
		{"index": 4, "codec_type": "attachment"}
	], "format": {"duration": "30280.232000"}}`))
	if p.Width != 2560 || p.Height != 1440 || p.Duration != 30280.232 || len(p.Subtitles) != 2 || p.Subtitles[0].Title != trackStyled {
		t.Errorf("probe %+v", p)
	}
	if empty := parseProbe([]byte("nope")); empty.Width != 0 || empty.Subtitles != nil {
		t.Errorf("bad probe %+v", empty)
	}
}

func TestEmbedArgs(t *testing.T) {
	probe := videoProbe{Subtitles: []probeStream{{Index: 2, Title: trackStyled}, {Index: 3, Title: "English"}, {Index: 4, Title: trackPlain}}}
	mkv := strings.Join(embedArgs("v.mkv", "s.ass", "s.srt", "out.mkv", probe, true), " ")
	for _, want := range []string{
		"-i v.mkv -i s.ass -i s.srt -map 0 -map -0:2 -map -0:4 -map 1 -map 2 -c copy",
		"-disposition:s:0 0",
		"-metadata:s:s:1 title=中英双语（描边） -disposition:s:1 default",
		"-metadata:s:s:2 title=中英双语（纯文本） -disposition:s:2 0",
	} {
		if !strings.Contains(mkv, want) {
			t.Errorf("mkv args missing %q:\n%s", want, mkv)
		}
	}
	if !strings.HasSuffix(mkv, " out.mkv") || !strings.Contains(mkv, "-progress pipe:1") {
		t.Errorf("mkv args %s", mkv)
	}
	mp4 := strings.Join(embedArgs("v.mp4", "s.ass", "s.srt", "out.mp4", videoProbe{}, false), " ")
	if !strings.Contains(mp4, "-i v.mp4 -i s.srt -map 0 -map 1 -c copy -c:s:0 mov_text") ||
		!strings.Contains(mp4, "title=中英双语（纯文本） -disposition:s:0 default") || strings.Contains(mp4, "s.ass") {
		t.Errorf("mp4 args %s", mp4)
	}
}

func TestTracksForOneStep(t *testing.T) {
	cues := []Cue{{Timing: "00:00:01,000 --> 00:00:02,000", Text: "Hello"}}
	e := Env{}
	e.Settings.Translation.Enabled = true
	d := engine.Finished{Early: &early{p: &prepared{cues: cues, translations: []string{"你好"}}}, TrackType: ytdlp.TrackBoth}
	dir := t.TempDir()
	tracks := Tracks(context.Background(), e, d, dir, engine.Reporter{
		Stage: func(ytdlp.Stage) {}, Progress: func(float64) {}, Log: func(string) {},
		Warn: func(code, msg string) { t.Errorf("warning %s: %s", code, msg) },
	})
	if len(tracks) != 2 || !tracks[0].Default || tracks[0].Title != trackStyled || tracks[1].Title != trackPlain {
		t.Fatalf("tracks %+v", tracks)
	}
	ass, err := os.ReadFile(tracks[0].Path)
	if err != nil || !strings.Contains(string(ass), `你好\N{\rEN}Hello`) {
		t.Errorf("ass %q, %v", ass, err)
	}
	e.Settings.Translation.Enabled = false
	if Tracks(context.Background(), e, d, dir, engine.Reporter{}) != nil {
		t.Error("tracks with translation off")
	}
}
