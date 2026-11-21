package ytdlp

import (
	"reflect"
	"testing"
)

// Ported from src-tauri/src/parsers: ytdlp_progress.rs, ytdlp_error.rs and
// ytdlp_single/*. Partial-download ffmpeg progress is gone with that feature.

func TestProgressFractionalEta(t *testing.T) {
	p := NewProgressParser(TrackVideo)
	ev := p.ParseLine("RAW|48.0| 48.0%|10066329.6|1532.8714285714|6276775936|NA|13076616533.333334|1869|3894")
	if ev.Progress == nil || ev.Progress.EtaSecs == nil || *ev.Progress.EtaSecs != 1533 {
		t.Fatalf("progress %+v", ev.Progress)
	}
	if pct := *ev.Progress.Percent; pct < 47.9 || pct > 48.1 {
		t.Errorf("percent %v", pct)
	}
	if *ev.Progress.SpeedBps != 10066329.6 || ev.Progress.Category != CategoryVideo {
		t.Errorf("progress %+v", ev.Progress)
	}
}

func TestProgressFallbacks(t *testing.T) {
	p := NewProgressParser(TrackAudio)
	if ev := p.ParseLine("RAW|NA|NA|NA|NA|50|200|NA|NA|NA"); ev.Progress == nil || *ev.Progress.Percent != 25 || ev.Progress.Category != CategoryAudio {
		t.Errorf("bytes fallback %+v", ev.Progress)
	}
	if ev := p.ParseLine("RAW|NA|NA|NA|NA|NA|NA|NA|3|4"); ev.Progress == nil || *ev.Progress.Percent != 75 {
		t.Errorf("fragment fallback %+v", ev.Progress)
	}
	if ev := p.ParseLine("RAW|150|"); *ev.Progress.Percent != 100 {
		t.Errorf("clamp %+v", ev.Progress)
	}
	if ev := p.ParseLine("RAW|NA|NA|NA|NA|NA|NA|NA|NA|NA"); ev.Progress != nil {
		t.Errorf("empty line gave %+v", ev.Progress)
	}
}

func TestProgressStages(t *testing.T) {
	p := NewProgressParser(TrackBoth)
	steps := []struct {
		line  string
		stage Stage
	}{
		{"[youtube] abc: Downloading webpage", ""},
		{`[download] Destination: C:\out\v.f137.mp4`, StageDownloading},
		{`[download] Destination: C:\out\v.f140.m4a`, ""},
		{`[Merger] Merging formats into "C:\out\v.mp4"`, StageMerging},
		{`[VideoRemuxer] Remuxing video from webm to mp4; Destination: C:\out\v.mp4`, StageRemuxing},
		{`[VideoConvertor] Converting video from mp4 to mkv; Destination: C:\out\v.mkv`, StageReencoding},
		{"[EmbedThumbnail] ffmpeg: Adding thumbnail", ""},
		{`Deleting original file C:\out\v.webp (pass -k to keep)`, StageFinalizing},
	}
	for _, s := range steps {
		if ev := p.ParseLine(s.line); ev.Stage != s.stage {
			t.Errorf("%q: stage %q, want %q", s.line, ev.Stage, s.stage)
		}
	}
}

func TestProgressCategoryFollowsDestination(t *testing.T) {
	p := NewProgressParser(TrackVideo)
	p.ParseLine(`[download] Destination: C:\out\v.en.vtt`)
	if ev := p.ParseLine("RAW|10|"); ev.Progress.Category != CategorySubtitles {
		t.Errorf("category %q", ev.Progress.Category)
	}
}

func TestDestinations(t *testing.T) {
	cases := []struct {
		line string
		want *Destination
	}{
		{`[download] Destination: C:\out\v.f137.mp4`, &Destination{Path: `C:\out\v.f137.mp4`, Confidence: 60}},
		{`[Merger] Merging formats into "C:\out\v.mp4"`, &Destination{Path: `C:\out\v.mp4`, Confidence: 80, Merged: true}},
		{`[VideoConvertor] Destination: "/tmp/out.mp4"`, &Destination{Path: "/tmp/out.mp4", Confidence: 95}},
		{`[ExtractAudio] Destination: C:\out\a.mp3`, &Destination{Path: `C:\out\a.mp3`, Confidence: 70}},
		{`[download] C:\out\v.mp4 has already been downloaded`, &Destination{Path: `C:\out\v.mp4`, Confidence: 65}},
		{`[VideoRemuxer] Not remuxing media file "C:\out\v.mp4"; already is in target format mp4`, &Destination{Path: `C:\out\v.mp4`, Confidence: 90}},
		{"[youtube] abc: Downloading webpage", nil},
	}
	for _, c := range cases {
		p := NewProgressParser(TrackVideo)
		if got := p.ParseLine(c.line).Destination; !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: %+v, want %+v", c.line, got, c.want)
		}
	}
}

func TestDiagnosticRulesLoad(t *testing.T) {
	m, err := loadMatcher(rulesJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.rules) != 33 || m.fallback != "unknown" {
		t.Fatalf("%d rules, fallback %q", len(m.rules), m.fallback)
	}
}

func TestDiagnosticSkipLines(t *testing.T) {
	cases := map[string]string{
		"[download] 2022-01-05 upload date is not in range 2026-06-18 to 9999-12-31": "Skipped by upload date filter",
		"[download] Video does not pass filter (!is_live), skipping ..":              "Skipped by advanced input filter",
		"[download] File is larger than max-filesize (123 > 100), skipping":          "Skipped by maximum filesize filter",
		"Some title does not pass filter (duration <? 1), skipping ..":               "Skipped by advanced input filter",
	}
	for line, want := range cases {
		d := ParseDiagnostic(line)
		if d == nil || d.Level != LevelWarning || d.Code != CodeInputFilterSkipped || d.Message != want || d.Component != "download" {
			t.Errorf("%q: %+v", line, d)
		}
	}
	if d := ParseDiagnostic("[download] Destination: /tmp/video.mp4"); d != nil {
		t.Errorf("destination line gave %+v", d)
	}
}

func TestDiagnosticCodes(t *testing.T) {
	cases := []struct {
		line, code, component, message string
		level                          Level
	}{
		{"ERROR: [youtube] dQw4w9WgXcQ: Sign in to confirm your age. This video may be inappropriate", "signInRequired", "youtube", "Sign in to confirm your age. This video may be inappropriate", LevelError},
		{"ERROR: [youtube] dQw4w9WgXcQ: Sign in to confirm you're not a bot", "signInRequiredForBotDetection", "youtube", "Sign in to confirm you're not a bot", LevelError},
		{"ERROR: unable to download video data: HTTP Error 403: Forbidden", "accessForbidden403", "", "unable to download video data: HTTP Error 403: Forbidden", LevelError},
		{"ERROR: something odd", "unknown", "", "something odd", LevelError},
		{"WARNING: [youtube] Falling back to generic n function search", "unknown", "youtube", "Falling back to generic n function search", LevelWarning},
	}
	for _, c := range cases {
		d := ParseDiagnostic(c.line)
		if d == nil || d.Code != c.code || d.Component != c.component || d.Message != c.message || d.Level != c.level {
			t.Errorf("%q: %+v", c.line, d)
		}
	}
	if d := ParseDiagnostic("[info] nothing wrong"); d != nil {
		t.Errorf("info line gave %+v", d)
	}
}

func ptr[T any](v T) *T { return &v }

func groupFormat(id, lang string, pref int64, height int64, fps, abr float64, vcodec, acodec, ext string) InfoFormat {
	f := InfoFormat{FormatID: id, Language: lang, LanguagePreference: &pref, AudioChannels: ptr(2.0), Vcodec: vcodec, Acodec: acodec, Ext: ext}
	if height > 0 {
		f.Height = &height
	}
	if fps > 0 {
		f.Fps = &fps
	}
	if abr > 0 {
		f.Abr = &abr
	}
	return f
}

func single(t *testing.T, formats ...InfoFormat) *Video {
	t.Helper()
	m := ParseInfoValue(&Info{ID: "v", Type: "video", Formats: formats})
	if m.Kind != KindSingle {
		t.Fatalf("kind %q", m.Kind)
	}
	return m.Video
}

func findFormat(t *testing.T, v *Video, match func(Format) bool) Format {
	t.Helper()
	for _, f := range v.Formats {
		if match(f) {
			return f
		}
	}
	t.Fatalf("format missing from %+v", v.Formats)
	return Format{}
}

func TestFormatsCollectPerGroupTrackIDs(t *testing.T) {
	v := single(t,
		groupFormat("1080p-main", "en", 10, 1080, 30, 0, "avc1", "aac", "mp4"),
		groupFormat("1080p-alt", "fr", 8, 1080, 30, 0, "avc1", "aac", "mp4"),
		groupFormat("720p-main", "en", 10, 720, 30, 0, "avc1", "aac", "mp4"),
	)
	f1080 := findFormat(t, v, func(f Format) bool { return f.Height == 1080 })
	both := []string{"lang:en|channels:2", "lang:fr|channels:2"}
	if !reflect.DeepEqual(f1080.AudioTrackIDs, both) || !reflect.DeepEqual(f1080.VideoTrackIDs, both) {
		t.Errorf("1080p tracks %+v", f1080)
	}
	f720 := findFormat(t, v, func(f Format) bool { return f.Height == 720 })
	en := []string{"lang:en|channels:2"}
	if !reflect.DeepEqual(f720.AudioTrackIDs, en) || !reflect.DeepEqual(f720.VideoTrackIDs, en) {
		t.Errorf("720p tracks %+v", f720)
	}
	if v.Formats[0].Height != 1080 || v.AudioTracks[0].ID != "lang:en|channels:2" {
		t.Errorf("order: formats %+v, tracks %+v", v.Formats, v.AudioTracks)
	}
}

func TestFormatsGroupAudioByAbr(t *testing.T) {
	v := single(t,
		groupFormat("128k-en", "en", 10, 0, 0, 128, "none", "aac", "m4a"),
		groupFormat("128k-fr", "fr", 8, 0, 0, 128, "none", "aac", "m4a"),
	)
	f := findFormat(t, v, func(f Format) bool { return f.Abr == 128 })
	if f.Height != 0 || len(f.AudioCodecs) != 1 || f.AudioCodecs[0].ID != "aac" || len(f.AudioTrackIDs) != 2 || len(f.VideoTrackIDs) != 0 {
		t.Errorf("format %+v", f)
	}
}

func TestFormatsDistinctAudioCodecsPerBitrate(t *testing.T) {
	v := single(t,
		groupFormat("49k-aac", "en", 10, 0, 0, 49, "none", "mp4a.40.5", "m4a"),
		groupFormat("50k-opus", "en", 10, 0, 0, 50, "none", "opus", "webm"),
		groupFormat("130k-aac", "en", 10, 0, 0, 130, "none", "mp4a.40.2", "m4a"),
		groupFormat("130k-opus", "en", 10, 0, 0, 130, "none", "opus", "webm"),
	)
	ids := func(cs []Codec) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.ID)
		}
		return out
	}
	if got := ids(findFormat(t, v, func(f Format) bool { return f.Abr == 49 }).AudioCodecs); !reflect.DeepEqual(got, []string{"mp4a.40.5"}) {
		t.Errorf("49k codecs %q", got)
	}
	if got := ids(findFormat(t, v, func(f Format) bool { return f.Abr == 130 }).AudioCodecs); !reflect.DeepEqual(got, []string{"mp4a.40.2", "opus"}) {
		t.Errorf("130k codecs %q", got)
	}
}

func TestFormatsWithoutRealTracks(t *testing.T) {
	v := single(t, InfoFormat{FormatID: "empty", Format: "1080p", FormatNote: "1080p", Height: ptr[int64](1080), Fps: ptr(30.0), Ext: "mp4", Vcodec: "none", Acodec: "none"})
	f := v.Formats[0]
	if len(f.AudioCodecs) != 0 || len(f.AudioTrackIDs) != 0 || len(f.VideoTrackIDs) != 0 || len(v.AudioTracks) != 1 || len(v.VideoTracks) != 1 {
		t.Errorf("video %+v", v)
	}
}

func TestFormatsAutoTracksAndStoryboards(t *testing.T) {
	v := single(t, InfoFormat{Vcodec: "none", Acodec: "aac"}, InfoFormat{Vcodec: "none", Acodec: "aac"})
	if len(v.AudioTracks) != 1 || v.AudioTracks[0].ID != "auto" || len(v.VideoTracks) != 1 || v.VideoTracks[0].ID != "auto" {
		t.Errorf("tracks %+v %+v", v.AudioTracks, v.VideoTracks)
	}
	v = single(t, InfoFormat{FormatID: "sb0", Format: "storyboard", FormatNote: "storyboard", Height: ptr[int64](1080), Fps: ptr(1.0), Ext: "mhtml", Vcodec: "none", Acodec: "none"})
	if len(v.Formats) != 0 || v.VideoTracks[0].ID != "auto" {
		t.Errorf("storyboard %+v", v)
	}
	v = single(t)
	if v.AudioTracks[0].ID != "auto" || v.VideoTracks[0].ID != "auto" {
		t.Errorf("no formats %+v", v)
	}
}

func TestTrackIdentityIgnoresQualityNotes(t *testing.T) {
	a := toTrack(&InfoFormat{Language: "en-us", FormatNote: "1080p", Format: "1080p", AudioChannels: ptr(2.0)})
	b := toTrack(&InfoFormat{Language: "en-us", FormatNote: "720p", Format: "720p", AudioChannels: ptr(2.0)})
	if a.ID != "lang:en-us|channels:2" || a.ID != b.ID || a.Label != "en-us - 2ch" {
		t.Errorf("tracks %+v %+v", a, b)
	}
}

func TestCodecLabels(t *testing.T) {
	cases := map[string]string{
		"avc1.640028": "H.264 High, Level 4.0", "av01.0.08M.10": "AV1 Main, Level 2.0, 10-bit",
		"aac": "AAC", "mp4a.40.2": "AAC LC", "mp4a.20.5": "HE-AAC v1 (mp4a.20.5)", "avc1": "avc1", "vp09": "VP9",
	}
	for id, want := range cases {
		if got := CodecLabel(id); got != want {
			t.Errorf("CodecLabel(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestParseInfoJSON(t *testing.T) {
	m, err := ParseInfo([]byte(`{
		"id": "abc", "title": "T", "_type": "video", "webpage_url": "https://x/abc",
		"duration": 61.5, "view_count": 10, "filesize_approx": 1000,
		"formats": [
			{"format_id": "137", "height": 1080, "fps": 30, "vcodec": "avc1.640028", "acodec": "none", "ext": "mp4", "filesize": 900},
			{"format_id": "140", "abr": 129.5, "vcodec": "none", "acodec": "mp4a.40.2", "ext": "m4a", "audio_channels": 2}
		],
		"subtitles": {"en": [{"ext": "vtt"}], "de": [{"ext": "srt"}]},
		"automatic_captions": {"en-orig": [{"ext": "json3"}], "fr": [{"ext": "json3"}], "es": []},
		"chapters": [{"title": "Intro", "start_time": 0, "end_time": 25}, {"title": "", "start_time": 25, "end_time": 50}, {"title": "Main", "start_time": 25, "end_time": 120}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	v := m.Video
	if m.Kind != KindSingle || v.Title != "T" || *v.Duration != 61.5 || *v.Views != 10 || *v.Filesize != 1000 {
		t.Fatalf("video %+v", v)
	}
	if len(v.Formats) != 2 || v.Formats[0].Height != 1080 || v.Formats[0].Filesize != 900 || v.Formats[1].Abr != 130 {
		t.Errorf("formats %+v", v.Formats)
	}
	if !reflect.DeepEqual(v.SubtitleInventory.ManualLanguages, []string{"de", "en"}) || !reflect.DeepEqual(v.SubtitleInventory.AutoLanguages, []string{"en-orig", "fr"}) {
		t.Errorf("subtitles %+v", v.SubtitleInventory)
	}
	if len(v.Chapters) != 2 || v.Chapters[0].Title != "Intro" || v.Chapters[1].Title != "Main" {
		t.Errorf("chapters %+v", v.Chapters)
	}
}

func TestParsePlaylistAndLive(t *testing.T) {
	m, err := ParseInfo([]byte(`{"id": "PL1", "_type": "playlist", "title": "P", "playlist_count": 3,
		"thumbnails": [{"url": "s", "width": 10, "height": 10}, {"url": "l", "width": 100, "height": 50}],
		"entries": [{"url": "https://x/1"}, {"webpage_url": "https://x/2"}, {}]}`))
	if err != nil {
		t.Fatal(err)
	}
	p := m.Playlist
	want := []PlaylistEntry{{URL: "https://x/1", Index: 0}, {URL: "https://x/2", Index: 1}}
	if m.Kind != KindPlaylist || p.PlaylistID != "PL1" || *p.PlaylistCount != 3 || p.Thumbnail != "l" || !reflect.DeepEqual(p.Entries, want) {
		t.Errorf("playlist %+v", p)
	}
	m, _ = ParseInfo([]byte(`{"id": "L", "is_live": true, "title": "Live", "entries": [{"url": "u"}]}`))
	if m.Kind != KindLivestream || m.Video.Title != "Live" {
		t.Errorf("live %+v", m)
	}
	if _, err := ParseInfo([]byte(`not json`)); err == nil {
		t.Error("bad JSON accepted")
	}
}
