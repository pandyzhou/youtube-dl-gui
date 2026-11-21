package ytdlp

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
)

// Ported from src-tauri/src/runners/ytdlp_args/tests.rs and the tests in
// src-tauri/src/runners/ytdlp_runner.rs. Partial download and SponsorBlock
// tests are gone with those features.

func defaultOutput() config.OutputSettings {
	out := config.DefaultSettings("/downloads").Output
	out.Video.Container = "mp4" // the expectations below were written for MP4
	return out
}

func audio(abr int) FormatOptions { return FormatOptions{TrackType: TrackAudio, Abr: abr} }
func video(h, fps int) FormatOptions {
	return FormatOptions{TrackType: TrackVideo, Height: h, Fps: fps}
}
func both(h, fps int) FormatOptions { return FormatOptions{TrackType: TrackBoth, Height: h, Fps: fps} }

func expect(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args\n got %q\nwant %q", got, want)
	}
}

func TestInputFilterArgsIncludeSupportedFilters(t *testing.T) {
	expect(t, InputFilterArgs(InputFilterOptions{
		PlaylistItems: "1:3", MinFilesize: "10M", MaxFilesize: "250M", Date: "today-2weeks",
		DateBefore: "20250101", DateAfter: "20240101", MatchFilters: "!is_live",
		BreakMatchFilters: "duration >? 3600", PlaylistMode: PlaylistModeSingle,
	}),
		"-I", "1:3", "--min-filesize", "10M", "--max-filesize", "250M", "--date", "today-2weeks",
		"--datebefore", "20250101", "--dateafter", "20240101", "--match-filters", "!is_live",
		"--break-match-filters", "duration >? 3600", "--no-playlist")
}

func TestInputFilterArgsOmitEmptyValues(t *testing.T) {
	expect(t, InputFilterArgs(InputFilterOptions{PlaylistItems: "   ", MatchFilters: "   ", PlaylistMode: PlaylistModeAll}), "--yes-playlist")
}

func TestAudioFormatArgsIncludeTrackAndEncodingPreferences(t *testing.T) {
	f := audio(128)
	f.AudioTrack, f.AudioEncoding = "lang:en|channels:2", "aac"
	expect(t, FormatArgs(f, defaultOutput()), "-x", "-f", "ba[language=en]/ba/best", "-S", "lang:en,channels:2,abr~128,acodec:aac,aext:mp3")
}

func TestVideoFormatArgsIncludeTrackAndEncodingPreferences(t *testing.T) {
	f := video(1080, 60)
	f.VideoTrack, f.VideoEncoding = "lang:ja", "avc1"
	args := FormatArgs(f, defaultOutput())
	for _, part := range []string{"bv*[language=ja][height<=1080][fps<=60]", "b[language=ja][height<=1080][fps<=60]", "/bv/"} {
		if !strings.Contains(args[1], part) {
			t.Errorf("selector %q lacks %q", args[1], part)
		}
	}
	if !strings.HasSuffix(args[1], "/b") {
		t.Errorf("selector %q does not end with /b", args[1])
	}
	if args[3] != "lang:ja,height:1080,fps:60,vcodec:avc1,vext:mp4,vext:m4a" {
		t.Errorf("sort %q", args[3])
	}
}

func TestBothFormatArgsIncludeAudioAndVideoEncodingPreferences(t *testing.T) {
	f := both(720, 30)
	f.AudioTrack, f.VideoTrack, f.VideoEncoding, f.AudioEncoding = "lang:fr-FR", "lang:fr-FR", "vp9", "opus"
	args := FormatArgs(f, defaultOutput())
	for _, part := range []string{"b[language=fr-FR][height<=720][fps<=30]", "bv*[height<=720][fps<=30]+ba[language=fr-FR]", "b[language=fr][height<=720][fps<=30]", "bv*+ba/b"} {
		if !strings.Contains(args[1], part) {
			t.Errorf("selector %q lacks %q", args[1], part)
		}
	}
	if args[3] != "lang:fr-FR,height:720,fps:30,vcodec:vp9,acodec:opus,vext:mp4,vext:m4a" {
		t.Errorf("sort %q", args[3])
	}
}

func TestAudioFormatArgsWithoutAbr(t *testing.T) {
	expect(t, FormatArgs(audio(0), defaultOutput()), "-x", "-f", "ba/best", "-S", "lang,aext:mp3")
}

func TestAudioFormatArgsWithAbr(t *testing.T) {
	expect(t, FormatArgs(audio(44), defaultOutput()), "-x", "-f", "ba/best", "-S", "lang,abr~44,aext:mp3")
}

const video720Selector = "bv[height<=720][fps<=60]/bv[height<=720]/bv[fps<=60]/bv/b[height<=720][fps<=60]/b[height<=720]/b[fps<=60]/b"

func TestVideoFormatArgsWithMp4Bias(t *testing.T) {
	expect(t, FormatArgs(video(720, 60), defaultOutput()), "-f", video720Selector, "-S", "lang,height:720,fps:60,vcodec:avc1,vext:mp4,vext:m4a")
}

func TestVideoFormatArgsWithMkvNoMp4Bias(t *testing.T) {
	out := defaultOutput()
	out.Video.Container = "mkv"
	expect(t, FormatArgs(video(720, 60), out), "-f", video720Selector, "-S", "lang,height:720,fps:60,vext")
}

func TestBothFormatArgsUseBothSelectorAndMp4Bias(t *testing.T) {
	expect(t, FormatArgs(both(1080, 30), defaultOutput()),
		"-f", "bv*[height<=1080][fps<=30]+ba/b[height<=1080][fps<=30]/bv*[height<=1080]+ba/b[height<=1080]/bv*[fps<=30]+ba/b[fps<=30]/bv*+ba/b",
		"-S", "lang,height:1080,fps:30,vcodec:avc1,acodec:aac,vext:mp4,vext:m4a")
}

func TestBothFormatArgsWithExplicitAudioEncodingSkipsAacBias(t *testing.T) {
	f := both(720, 30)
	f.AudioEncoding = "opus"
	expect(t, FormatArgs(f, defaultOutput()),
		"-f", "bv*[height<=720][fps<=30]+ba/b[height<=720][fps<=30]/bv*[height<=720]+ba/b[height<=720]/bv*[fps<=30]+ba/b[fps<=30]/bv*+ba/b",
		"-S", "lang,height:720,fps:30,vcodec:avc1,acodec:opus,vext:mp4,vext:m4a")
}

func TestBothFormatArgsBounded4K(t *testing.T) {
	expect(t, FormatArgs(both(2160, 30), defaultOutput()),
		"-f", "bv*[height<=2160][fps<=30]+ba/b[height<=2160][fps<=30]/bv*[height<=2160]+ba/b[height<=2160]/bv*[fps<=30]+ba/b[fps<=30]/bv*+ba/b",
		"-S", "lang,height:2160,fps:30,vcodec:avc1,acodec:aac,vext:mp4,vext:m4a")
}

func TestBestVideoWithoutLimits(t *testing.T) {
	expect(t, FormatArgs(FormatOptions{TrackType: TrackBoth}, defaultOutput()),
		"-f", "bv*+ba/b/bv*+ba/b/bv*+ba/b/bv*+ba/b",
		"-S", "lang,vcodec:avc1,acodec:aac,vext:mp4,vext:m4a")
}

func outputArgs(t *testing.T, f FormatOptions, out config.OutputSettings) []string {
	t.Helper()
	args, err := OutputArgs(f, out)
	if err != nil {
		t.Fatal(err)
	}
	return args
}

func TestAudioOutputNeverPolicy(t *testing.T) {
	out := defaultOutput()
	out.Audio.Policy, out.Audio.Format = config.PolicyNever, "mp3"
	expect(t, outputArgs(t, audio(0), out), "--output-na-placeholder", "", "--embed-thumbnail", "--add-metadata")
}

func TestAudioOutputAllowReencodeMp3Best(t *testing.T) {
	expect(t, outputArgs(t, audio(0), defaultOutput()),
		"--output-na-placeholder", "", "--audio-format", "mp3", "--audio-quality", "0", "--embed-thumbnail", "--add-metadata")
}

func TestAudioOutputAllowReencodeOgg(t *testing.T) {
	out := defaultOutput()
	out.Audio.Format = "ogg"
	expect(t, outputArgs(t, audio(160), out),
		"--output-na-placeholder", "", "--audio-format", "vorbis", "--audio-quality", "160k", "--embed-thumbnail", "--add-metadata")
}

func TestAudioOutputCustomPostprocess(t *testing.T) {
	out := defaultOutput()
	out.Audio.PostprocessPreset, out.Audio.PostprocessArgs = "custom", "-metadata artist=Example"
	out.AddThumbnail, out.AddMetadata = false, false
	expect(t, outputArgs(t, audio(0), out),
		"--output-na-placeholder", "", "--audio-format", "mp3", "--audio-quality", "0", "--postprocessor-args", "ffmpeg:-metadata artist=Example")
}

func TestAudioCustomPostprocessDoesNotLeakIntoVideo(t *testing.T) {
	out := defaultOutput()
	out.Audio.PostprocessPreset, out.Audio.PostprocessArgs = "custom", "-metadata artist=Example"
	out.AddThumbnail, out.AddMetadata = false, false
	expect(t, outputArgs(t, video(720, 60), out),
		"--output-na-placeholder", "", "--merge-output-format", "mp4", "--remux-video", "mp4")
}

func TestVideoOutputNeverPolicy(t *testing.T) {
	out := defaultOutput()
	out.Video.Policy = config.PolicyNever
	expect(t, outputArgs(t, video(720, 60), out), "--output-na-placeholder", "", "--embed-thumbnail", "--add-metadata")
}

func TestVideoOutputRemuxes(t *testing.T) {
	for _, container := range []string{"mp4", "mkv"} {
		out := defaultOutput()
		out.Video.Container = container
		expect(t, outputArgs(t, video(720, 60), out),
			"--output-na-placeholder", "", "--merge-output-format", container, "--remux-video", container, "--embed-thumbnail", "--add-metadata")
	}
}

func TestBothTrackTypeUsesVideoPolicy(t *testing.T) {
	expect(t, outputArgs(t, both(720, 60), defaultOutput()),
		"--output-na-placeholder", "", "--merge-output-format", "mp4", "--remux-video", "mp4", "--embed-thumbnail", "--add-metadata")
}

func bareVideoOutput(policy, preset string) config.OutputSettings {
	out := defaultOutput()
	out.Video.Policy, out.Video.PostprocessPreset = policy, preset
	out.AddThumbnail, out.AddMetadata = false, false
	return out
}

func TestVideoOutputNonePresetHasNoPostprocessorArgs(t *testing.T) {
	for _, arg := range outputArgs(t, video(720, 60), bareVideoOutput(config.PolicyNever, "none")) {
		if arg == "--postprocessor-args" {
			t.Fatal("postprocessor args with no preset")
		}
	}
}

func TestVideoOutputPresetErrors(t *testing.T) {
	cases := []struct {
		name string
		out  config.OutputSettings
		want error
	}{
		{"fps30 with keep original", bareVideoOutput(config.PolicyNever, "fps30"), ErrFps30NeedsReencode},
		{"mp42 with mkv", func() config.OutputSettings {
			o := bareVideoOutput(config.PolicyNever, "mp42")
			o.Video.Container = "mkv"
			return o
		}(), ErrMp42NeedsMp4},
		{"custom reencode with keep original", func() config.OutputSettings {
			o := bareVideoOutput(config.PolicyNever, "custom")
			o.Video.CustomPostprocessMode, o.Video.PostprocessArgs = "reencode", "-movflags +faststart"
			return o
		}(), ErrCustomNeedsReencode},
		{"unbalanced quotes", func() config.OutputSettings {
			o := bareVideoOutput(config.PolicyAllowReencode, "custom")
			o.Video.PostprocessArgs = `-metadata title="oops`
			return o
		}(), ErrInvalidCustomArgs},
	}
	for _, c := range cases {
		if _, err := OutputArgs(video(720, 60), c.out); !errors.Is(err, c.want) {
			t.Errorf("%s: err %v, want %v", c.name, err, c.want)
		}
	}
}

func TestVideoOutputFps30Reencodes(t *testing.T) {
	expect(t, outputArgs(t, video(720, 60), bareVideoOutput(config.PolicyAllowReencode, "fps30")),
		"--output-na-placeholder", "", "--recode-video", "mp4", "--postprocessor-args", "VideoConvertor+ffmpeg:-vf fps=30")
}

func TestVideoOutputMp42AddsBrand(t *testing.T) {
	expect(t, outputArgs(t, video(720, 60), bareVideoOutput(config.PolicyNever, "mp42")),
		"--output-na-placeholder", "", "--merge-output-format", "mp4", "--remux-video", "mp4", "--postprocessor-args", "VideoRemuxer+ffmpeg:-brand mp42")
}

func TestVideoOutputCustomRemux(t *testing.T) {
	out := bareVideoOutput(config.PolicyNever, "custom")
	out.Video.CustomPostprocessMode, out.Video.PostprocessArgs = "remux", `-metadata synopsis="My Video"`
	expect(t, outputArgs(t, video(720, 60), out),
		"--output-na-placeholder", "", "--merge-output-format", "mp4", "--remux-video", "mp4", "--postprocessor-args", `VideoRemuxer+ffmpeg:-metadata synopsis="My Video"`)
}

func TestVideoOutputCustomReencode(t *testing.T) {
	out := bareVideoOutput(config.PolicyAllowReencode, "custom")
	out.Video.CustomPostprocessMode, out.Video.PostprocessArgs = "reencode", "-movflags +faststart"
	expect(t, outputArgs(t, video(720, 60), out),
		"--output-na-placeholder", "", "--recode-video", "mp4", "--postprocessor-args", "VideoConvertor+ffmpeg:-movflags +faststart")
}

func TestOutputFlags(t *testing.T) {
	out := defaultOutput()
	out.Video.Policy = config.PolicyNever
	out.RestrictFilenames, out.SaveThumbnail = true, true
	args := outputArgs(t, video(720, 60), out)
	for _, flag := range []string{"--restrict-filenames", "--write-thumbnail"} {
		if !contains(args, flag) {
			t.Errorf("%s missing from %q", flag, args)
		}
	}
	out.RestrictFilenames, out.SaveThumbnail = false, false
	args = outputArgs(t, video(720, 60), out)
	if contains(args, "--restrict-filenames") || contains(args, "--write-thumbnail") {
		t.Errorf("flags present when off: %q", args)
	}
}

func TestLocationArgsUseTrackSpecificPreferences(t *testing.T) {
	paths := config.PathPreferences{
		AudioDownloadDir: "/tmp/audio", VideoDownloadDir: "/tmp/video",
		VideoDirectoryTemplate: "%(playlist_title)s", AudioDirectoryTemplate: "%(uploader)s",
	}
	args := LocationArgs(TrackVideo, TemplateValues{}, defaultOutput(), paths, "/tmp/fallback")
	want := filepath.Join("/tmp/video", "%(playlist_title)s", config.DefaultFileNameTemplate)
	expect(t, args, "-o", want)

	args = LocationArgs(TrackAudio, TemplateValues{"uploader": "Some/One"}, defaultOutput(), paths, "/tmp/fallback")
	want = filepath.Join("/tmp/audio", "Some_One", config.DefaultAudioFileNameTemplate)
	expect(t, args, "-o", want)

	out := defaultOutput()
	out.DownloadDir = ""
	args = LocationArgs(TrackVideo, TemplateValues{}, out, config.PathPreferences{}, "/tmp/fallback")
	expect(t, args, "-o", filepath.Join("/tmp/fallback", config.DefaultFileNameTemplate))
}

func TestPerformanceArgs(t *testing.T) {
	expect(t, PerformanceArgs(0))
	expect(t, PerformanceArgs(1))
	expect(t, PerformanceArgs(8), "--concurrent-fragments", "8")
	expect(t, PerformanceArgs(64), "--concurrent-fragments", "16")
}

func TestNetworkArgs(t *testing.T) {
	expect(t, NetworkArgs(config.NetworkSettings{Impersonate: "none"}))
	expect(t, NetworkArgs(config.NetworkSettings{EnableProxy: false, Proxy: "socks5://x", Impersonate: "none"}))
	expect(t, NetworkArgs(config.NetworkSettings{
		EnableProxy: true, Proxy: " socks5://127.0.0.1:1080 ", Impersonate: "chrome-131:windows-10",
		ExtractorArgs: " youtube:player_js_variant=main \n\n youtube:skip=hls,dash \r\n generic:impersonate",
	}),
		"--proxy", "socks5://127.0.0.1:1080", "--impersonate", "chrome-131:windows-10",
		"--extractor-args", "youtube:player_js_variant=main",
		"--extractor-args", "youtube:skip=hls,dash",
		"--extractor-args", "generic:impersonate")
	expect(t, NetworkArgs(config.NetworkSettings{Impersonate: "any"}), "--impersonate", "")
}

func TestAuthArgs(t *testing.T) {
	expect(t, AuthArgs(config.AuthSettings{CookieBrowser: "none"}, AuthSecrets{}))
	expect(t, AuthArgs(
		config.AuthSettings{CookieBrowser: "firefox", CookieFile: `C:\cookies.txt`},
		AuthSecrets{Username: "u", Password: "-p", VideoPassword: "vp", BearerToken: "tok", Headers: "X-A: 1\nbad line\n\n X-B:2 "},
	),
		"--cookies-from-browser", "firefox", "--cookies", `C:\cookies.txt`,
		"--username", "u", "--password", "-p", "--video-password", "vp",
		"--add-header", "Authorization:Bearer tok", "--add-header", "X-A: 1", "--add-header", "X-B:2")
}

func TestProgressArgs(t *testing.T) {
	args := ProgressArgs()
	if args[4] != ProgressTemplate || !strings.HasPrefix(ProgressTemplate, "RAW|") || strings.Count(ProgressTemplate, "|") != 18 {
		t.Fatalf("progress template %q", args[4])
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize([]string{"--username", "my_secure_user", "--password", "-secret", "--proxy", "http://u:p@h:8080", "--cookies-from-browser", "chrome", "--no-playlist"})
	if !s.HasAuth || !s.HasProxy || !s.HasBrowserCookies || s.HasCookies || s.ArgCount != 9 {
		t.Errorf("summary %+v", s)
	}
	s = Summarize([]string{"--proxy=http://p:8080", "--cookies=/c.txt", "--add-header=Authorization:Bearer x"})
	if !s.HasProxy || !s.HasCookies || !s.HasAuth || s.HasBrowserCookies {
		t.Errorf("equals form %+v", s)
	}
	if s := Summarize([]string{"https://youtube.com/watch?v=x", "--no-playlist"}); s.HasAuth || s.HasProxy || s.HasCookies || s.HasBrowserCookies {
		t.Errorf("false positives %+v", s)
	}
}

func TestInputArgs(t *testing.T) {
	expect(t, InputArgs(PlaylistModeNone, config.InputSettings{}), "--yes-playlist")
	expect(t, InputArgs(PlaylistModeNone, config.InputSettings{PreferVideoInMixedLinks: true}), "--no-playlist")
	expect(t, InputArgs(PlaylistModeAll, config.InputSettings{PreferVideoInMixedLinks: true}), "--yes-playlist")
	expect(t, InputArgs(PlaylistModeSingle, config.InputSettings{}), "--no-playlist")
}
