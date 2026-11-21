package formats

import (
	"reflect"
	"testing"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

func sample() []ytdlp.Format {
	return []ytdlp.Format{
		{ID: "401", Height: 2160, Fps: 60, Filesize: 900 << 20},
		{ID: "137", Height: 1080, Fps: 30, Filesize: 200 << 20},
		{ID: "299", Height: 1080, Fps: 60, Filesize: 300 << 20},
		{ID: "136", Height: 720, Fps: 30, Filesize: 100 << 20},
		{ID: "140", Abr: 129, Filesize: 10 << 20},
		{ID: "251", Abr: 160, Filesize: 12 << 20},
	}
}

func TestResolvePresets(t *testing.T) {
	cases := []struct {
		preset string
		height int
		fps    int
		label  string
		size   int64
	}{
		{PresetBest, 0, 0, "2160p60", (900 + 12) << 20},
		{"1440p", 1080, 60, "1080p60", (300 + 12) << 20},
		{"1080p", 1080, 60, "1080p60", (300 + 12) << 20},
		{"720p", 720, 30, "720p", (100 + 12) << 20},
		{"480p", 720, 30, "720p", (100 + 12) << 20}, // nothing lower: the lowest
	}
	for _, c := range cases {
		got := Resolve(PresetByID(c.preset), sample())
		if got.Format.TrackType != ytdlp.TrackBoth || got.Format.Height != c.height || got.Format.Fps != c.fps || got.Label() != c.label || got.Size != c.size {
			t.Errorf("%s: %+v label %q size %d", c.preset, got.Format, got.Label(), got.Size)
		}
	}
}

func TestResolveAudioAndUnknown(t *testing.T) {
	got := Resolve(PresetByID(PresetAudioMP3), sample())
	if got.Format.TrackType != ytdlp.TrackAudio || got.Label() != "音频 MP3" || got.Size != 12<<20 {
		t.Errorf("mp3: %+v %q", got, got.Label())
	}
	out := got.ApplyAudioFormat(config.OutputSettings{Audio: config.AudioOutputSettings{Format: "opus", Policy: config.PolicyNever}})
	if out.Audio.Format != "mp3" || out.Audio.Policy != config.PolicyAllowReencode {
		t.Errorf("audio output %+v", out.Audio)
	}
	if got := Resolve(PresetByID(PresetAudio), sample()); got.Label() != "最佳音频" || got.AudioFormat != "" {
		t.Errorf("best audio %+v", got)
	}
	if got := Resolve(PresetByID("1080p"), nil); got.Matched != nil || got.Format.Height != 0 || got.Label() != "最佳画质" {
		t.Errorf("no formats %+v", got)
	}
	if PresetByID("nope").ID != PresetBest || PresetByLabel("720p").MaxHeight != 720 || len(PresetLabels()) != len(Presets) {
		t.Error("preset lookup")
	}
}

func TestInputFilters(t *testing.T) {
	fifty := 50.0
	s := config.InputFilterSettings{
		MinSize:    config.SizeFilter{Value: &fifty, Unit: "MB"},
		DateFilter: config.DateFilter{Mode: "after", Value: "2026-06-01"},
	}
	if got := InputFilters(s); !reflect.DeepEqual(got, ytdlp.InputFilterOptions{MinFilesize: "50M", DateAfter: "20260601"}) {
		t.Errorf("filters %+v", got)
	}
	if !InputFiltersActive(s) || InputFiltersActive(config.InputFilterSettings{}) {
		t.Error("active state")
	}
	odd := 1.255
	zero := 0.0
	got := InputFilters(config.InputFilterSettings{
		MinSize: config.SizeFilter{Value: &odd, Unit: "GB"}, MaxSize: config.SizeFilter{Value: &zero, Unit: "GB"},
		DateFilter: config.DateFilter{Mode: "before", Value: "20260620"}, MatchFilters: "  !is_live ",
	})
	if got.MinFilesize != "1.25G" && got.MinFilesize != "1.26G" || got.MaxFilesize != "" || got.DateBefore != "" || got.MatchFilters != "!is_live" {
		t.Errorf("normalization %+v", got)
	}
}

func TestDatePresets(t *testing.T) {
	now := time.Date(2026, 6, 21, 13, 45, 0, 0, time.UTC)
	if got := DatePreset("today", now); got != (config.DateFilter{Mode: "exact", Value: "2026-06-21"}) {
		t.Errorf("today %+v", got)
	}
	if got := DatePreset("last7Days", now); got != (config.DateFilter{Mode: "after", Value: "2026-06-14"}) {
		t.Errorf("last7Days %+v", got)
	}
	if got := DatePreset("yesterday", now); got.Value != "2026-06-20" {
		t.Errorf("yesterday %+v", got)
	}
}

func entries(n int) []ytdlp.PlaylistEntry {
	out := make([]ytdlp.PlaylistEntry, n)
	for i := range out {
		out[i] = ytdlp.PlaylistEntry{URL: "https://example.com/watch?v=" + string(rune('a'+i)), Index: i + 1}
	}
	return out
}

func indexes(es []ytdlp.PlaylistEntry) []int {
	var out []int
	for _, e := range es {
		out = append(out, e.Index)
	}
	return out
}

func TestPlaylistSelection(t *testing.T) {
	rows := []SelectionRow{{Index: 7}, {Range: true, Start: 1, End: 3}, {Range: true, Start: -4, End: -2}}
	if got := PlaylistItemsSpec(rows); got != "7,1:3,-4:-2" {
		t.Errorf("spec %q", got)
	}
	if got := indexes(SelectEntries(entries(12), rows[:2])); !reflect.DeepEqual(got, []int{7, 1, 2, 3}) {
		t.Errorf("single and range %v", got)
	}
	reverse := []SelectionRow{{Index: -1}, {Range: true, Start: -2, End: -4}}
	if got := indexes(SelectEntries(entries(12), reverse)); !reflect.DeepEqual(got, []int{12, 11, 10, 9}) {
		t.Errorf("negative and reverse %v", got)
	}
	if got := SelectEntries(entries(12), nil); len(got) != 12 {
		t.Errorf("no rows selected %d", len(got))
	}
	incomplete := []SelectionRow{{Range: true, Start: 2}, {}, {Index: 40}}
	if got := PlaylistItemsSpec(incomplete); got != "40" {
		t.Errorf("incomplete spec %q", got)
	}
	if got := SelectEntries(entries(12), incomplete); len(got) != 0 {
		t.Errorf("out of range selected %v", indexes(got))
	}
}

func TestPlaylistIndex(t *testing.T) {
	cases := []struct{ index, count, want int }{{0, 10, 1}, {9, 10, 10}}
	for _, c := range cases {
		if got := PlaylistIndex(c.index, c.count, false); got != c.want {
			t.Errorf("PlaylistIndex(%d) = %d", c.index, got)
		}
	}
	reversed := []struct{ index, count, want int }{{0, 10, 10}, {9, 10, 1}, {4, 10, 6}, {2, 0, 3}}
	for _, c := range reversed {
		if got := PlaylistIndex(c.index, c.count, true); got != c.want {
			t.Errorf("reversed PlaylistIndex(%d, %d) = %d", c.index, c.count, got)
		}
	}
}

func TestParseURLs(t *testing.T) {
	urls, skipped := ParseURLs("https://example.com/a, https://example.com/b\nhttps://example.com/c\tinvalid https://example.com/a")
	if !reflect.DeepEqual(urls, []string{"https://example.com/a", "https://example.com/b", "https://example.com/c"}) || skipped != 1 {
		t.Errorf("urls %q skipped %d", urls, skipped)
	}
	urls, _ = ParseURLFile("title,url\nExample,https://example.com/watch?v=1\nTwo,https://example.com/watch?v=2).")
	if !reflect.DeepEqual(urls, []string{"https://example.com/watch?v=1", "https://example.com/watch?v=2"}) {
		t.Errorf("file urls %q", urls)
	}
	if ValidURL("ftp://x/y") || ValidURL("example.com") || !ValidURL("http://a.b/c") {
		t.Error("ValidURL")
	}
}

func TestParseSelection(t *testing.T) {
	rows, err := ParseSelection("1-10, 15，-3:-1  4:2")
	want := []SelectionRow{{Range: true, Start: 1, End: 10}, {Index: 15}, {Range: true, Start: -3, End: -1}, {Range: true, Start: 4, End: 2}}
	if err != nil || !reflect.DeepEqual(rows, want) {
		t.Errorf("rows %+v err %v", rows, err)
	}
	if rows, err := ParseSelection("-2"); err != nil || !reflect.DeepEqual(rows, []SelectionRow{{Index: -2}}) {
		t.Errorf("negative single %+v %v", rows, err)
	}
	for _, bad := range []string{"a", "0", "1-", "3:x"} {
		if _, err := ParseSelection(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if rows, err := ParseSelection("  "); err != nil || len(rows) != 0 {
		t.Errorf("blank %+v %v", rows, err)
	}
}

func TestResolveCustom(t *testing.T) {
	c := ResolveCustom(ytdlp.FormatOptions{TrackType: ytdlp.TrackBoth, Height: 1080, Fps: 60}, sample())
	if c.Label() != "1080p60" || c.Size != (300+12)<<20 {
		t.Errorf("video %q %d", c.Label(), c.Size)
	}
	c = ResolveCustom(ytdlp.FormatOptions{TrackType: ytdlp.TrackVideo, Height: 720}, sample())
	if c.Label() != "720p 无声" || c.Size != 100<<20 {
		t.Errorf("video only %q %d", c.Label(), c.Size)
	}
	c = ResolveCustom(ytdlp.FormatOptions{TrackType: ytdlp.TrackAudio, Abr: 129}, sample())
	if c.Label() != "音频 129kbps" || c.Size != 10<<20 {
		t.Errorf("audio %q %d", c.Label(), c.Size)
	}
	if c := ResolveCustom(ytdlp.FormatOptions{TrackType: ytdlp.TrackBoth}, sample()); c.Label() != "2160p60" {
		t.Errorf("best %q", c.Label())
	}
}
