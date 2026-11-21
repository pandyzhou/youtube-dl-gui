package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

type fakeQueue struct {
	jobs     []engine.JobView
	added    []string
	starts   []bool
	calls    []string
	selected [][]formats.SelectionRow
	modes    []ytdlp.PlaylistMode
}

func (f *fakeQueue) Jobs() []engine.JobView {
	return f.jobs
}

func (f *fakeQueue) Add(url string, o engine.AddOptions) string {
	f.added = append(f.added, url)
	f.starts = append(f.starts, o.Start)
	f.modes = append(f.modes, o.Playlist)
	f.jobs = append([]engine.JobView{{ID: url, URL: url, Title: url, Status: engine.StatusFetching}}, f.jobs...)
	return url
}

func (f *fakeQueue) record(call string) {
	f.calls = append(f.calls, call)
}

func (f *fakeQueue) Start(id string) {
	f.record("start " + id)
}

func (f *fakeQueue) StartAll() int {
	f.record("startAll")
	return 0
}

func (f *fakeQueue) Pause(id string) {
	f.record("pause " + id)
}

func (f *fakeQueue) PauseAll() {
	f.record("pauseAll")
}

func (f *fakeQueue) Retry(id string) {
	f.record("retry " + id)
}

func (f *fakeQueue) Remove(id string) {
	f.record("remove " + id)
}

func (f *fakeQueue) SetPreset(id, preset string) {
	f.record("preset " + id + " " + preset)
}

func (f *fakeQueue) SetCustom(id string, format ytdlp.FormatOptions) {
	f.record(fmt.Sprintf("custom %s %s %d %d", id, format.TrackType, format.Height, format.Abr))
	for i := range f.jobs {
		if f.jobs[i].ID == id {
			f.jobs[i].Custom, f.jobs[i].Preset = &format, formats.PresetCustom
		}
	}
}

func (f *fakeQueue) Log(id string) []string {
	return []string{"[youtube] Extracting URL", "[download] Destination: a.mp4"}
}

func (f *fakeQueue) RemoveWhere(match func(engine.JobView) bool) int {
	return 0
}

func (f *fakeQueue) SelectPlaylist(id string, rows []formats.SelectionRow, start bool) error {
	f.selected = append(f.selected, rows)
	f.record("select " + id)
	return nil
}

func ptrF(v float64) *float64 { return &v }

func sampleJobs() []engine.JobView {
	video := func(title string) *ytdlp.Video {
		return &ytdlp.Video{Title: title, Uploader: "频道名称", Duration: ptrF(754)}
	}
	item := func(status engine.Status, pct float64) engine.ItemView {
		return engine.ItemView{ID: "i", Status: status, Percent: pct, Video: video("v"), Choice: "1080p60", Size: 312 << 20, EtaSecs: -1, PlaylistIndex: -1}
	}
	downloading := item(engine.StatusDownloading, 46)
	downloading.SpeedBps, downloading.EtaSecs = 3.4*(1<<20), 83
	merging := item(engine.StatusDownloading, 100)
	merging.Stage = ytdlp.StageMerging
	failed := item(engine.StatusError, 0)
	failed.Err = "unable to download video data: HTTP Error 403: Forbidden"
	failed.Diagnostics = []ytdlp.Diagnostic{{Level: ytdlp.LevelError, Code: "accessForbidden403", Message: failed.Err, Raw: "ERROR: " + failed.Err}}
	done := item(engine.StatusDone, 100)
	done.Destination = `C:\Users\me\Downloads\a.mp4`
	playlist := &ytdlp.Playlist{Title: "一个很长很长的播放列表标题", Entries: make([]ytdlp.PlaylistEntry, 24)}
	list := []engine.ItemView{item(engine.StatusDone, 100), item(engine.StatusDone, 100), downloading, item(engine.StatusQueued, 0)}
	return []engine.JobView{
		{ID: "fetch", URL: "https://www.youtube.com/watch?v=abc", Title: "https://www.youtube.com/watch?v=abc", Status: engine.StatusFetching},
		{ID: "select", Title: playlist.Title, Status: engine.StatusSelecting, Playlist: playlist, Uploader: "频道名称"},
		{ID: "ready", Title: "准备好的视频：一段很长的标题会在这里被截断显示省略号", Status: engine.StatusReady, Preset: "1080p", Items: []engine.ItemView{item(engine.StatusReady, 0)}, Uploader: "频道名称", Duration: 754, Size: 312 << 20},
		{ID: "down", Title: "正在下载的视频", Status: engine.StatusDownloading, Items: []engine.ItemView{downloading}, Percent: 46, Uploader: "频道名称", Duration: 754, Size: 312 << 20},
		{ID: "merge", Title: "正在合并的视频", Status: engine.StatusDownloading, Items: []engine.ItemView{merging}, Percent: 100, Uploader: "频道名称"},
		{ID: "list", Title: "合集：播放列表一起下载", Status: engine.StatusDownloading, Combined: true, Playlist: playlist, Items: list, Percent: 61.5, Done: 2, Uploader: "频道名称", Duration: 3016, Size: 1248 << 20},
		{ID: "paused", Title: "暂停的视频", Status: engine.StatusPaused, Items: []engine.ItemView{item(engine.StatusPaused, 30)}, Percent: 30, Uploader: "频道名称"},
		{ID: "error", Title: "出错的视频", Status: engine.StatusError, Items: []engine.ItemView{failed}, Failed: 1, Uploader: "频道名称"},
		{ID: "done", Title: "下载完成的视频", Status: engine.StatusDone, Items: []engine.ItemView{done}, Percent: 100, Done: 1, Uploader: "频道名称", Size: 312 << 20},
	}
}

func TestQueueAddLinks(t *testing.T) {
	q := &fakeQueue{}
	a := New(Deps{Queue: q})
	tt := ui.NewTester(a.View, 1000, 760)
	if err := tt.Click("链接"); err != nil {
		t.Fatal(err)
	}
	tt.Type("看这个 https://v/1, https://v/2")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(q.added) != 2 || !q.starts[0] || a.q.link != "" {
		t.Fatalf("added %q starts %v link %q", q.added, q.starts, a.q.link)
	}
	if !tt.HasText("已添加 2 个链接，忽略了 1 项无法识别的内容。") {
		t.Errorf("notice missing, texts %q", tt.Texts())
	}
	tt.Type("nothing here")
	if err := tt.Click("加入队列"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(q.added) != 2 || !tt.HasText("没有找到有效的链接，链接需要以 http:// 或 https:// 开头。") {
		t.Errorf("invalid input accepted: %q", q.added)
	}
}

func TestQueueCardActions(t *testing.T) {
	q := &fakeQueue{jobs: sampleJobs()}
	var shown string
	a := New(Deps{Queue: q, ShowInFolder: func(p string) { shown = p }})
	tt := ui.NewTester(a.View, 1000, 1400)
	for _, want := range []string{"正在获取信息…", "选择视频…", "46% · 3.4 MB/s · 剩余 1:23", "100% · 合并音视频", "62% · 2/4 · 3.4 MB/s", "30% · 已暂停", "拒绝访问：您没有权限访问此资源（HTTP 403）。", "已完成", "9 个项目"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing", want)
		}
	}
	for _, label := range []string{"开始下载", "暂停", "继续", "重试", "在文件夹中显示"} {
		if err := tt.Click(label); err != nil {
			t.Errorf("click %s: %v", label, err)
		}
	}
	want := []string{"start ready", "pause down", "start paused", "retry error"}
	for i, w := range want {
		if i >= len(q.calls) || q.calls[i] != w {
			t.Fatalf("calls %q, want prefix %q", q.calls, want)
		}
	}
	if shown != `C:\Users\me\Downloads\a.mp4` {
		t.Errorf("shown %q", shown)
	}
}

func TestPlaylistSelectionDialog(t *testing.T) {
	q := &fakeQueue{jobs: sampleJobs()[1:2]}
	a := New(Deps{Queue: q})
	tt := ui.NewTester(a.View, 1000, 760)
	if err := tt.Click("选择视频…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("选择要下载的视频") {
		t.Fatalf("dialog not open, texts %q", tt.Texts())
	}
	if err := tt.Click("部分视频"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("视频序号"); err != nil {
		t.Fatal(err)
	}
	tt.Type("1-3, x")
	if err := tt.Click("下载"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("无法识别“x”") || len(q.selected) != 0 {
		t.Fatalf("bad selection accepted, texts %q", tt.Texts())
	}
	a.q.selectText = "1-3, -1"
	tt.Frame()
	if err := tt.Click("下载"); err != nil {
		t.Fatal(err)
	}
	if len(q.selected) != 1 || len(q.selected[0]) != 2 || a.q.selectOpen {
		t.Fatalf("selection %+v open %v", q.selected, a.q.selectOpen)
	}
}

func TestDetailView(t *testing.T) {
	q := &fakeQueue{jobs: sampleJobs()}
	a := New(Deps{Queue: q})
	tt := ui.NewTester(a.View, 1000, 1400)
	if err := tt.Click("详情：出错的视频"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("问题") || !tt.HasText("ERROR: unable to download video data: HTTP Error 403: Forbidden") {
		t.Fatalf("details missing, texts %q", tt.Texts())
	}
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("[download] Destination: a.mp4") {
		t.Fatalf("log missing, texts %q", tt.Texts())
	}
	if err := tt.Click("返回队列"); err != nil {
		t.Fatal(err)
	}
	if a.q.detailJob != "" {
		t.Fatal("still in details")
	}
}

func TestQueueScreenshots(t *testing.T) {
	dir := os.Getenv("SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{true, false} {
		scheme := map[bool]string{true: "dark", false: "light"}[dark]
		shots := []struct {
			name  string
			setup func(a *App)
		}{
			{"queue-cards", func(a *App) {}},
			{"queue-details", showDetails},
			{"queue-select", showSelection},
			{"queue-custom", showCustom},
		}
		for _, shot := range shots {
			name := shot.name
			a := New(Deps{Queue: &fakeQueue{jobs: sampleJobs()}})
			shot.setup(a)
			tt := ui.NewTesterWith(a.View, 1000, 1100, ui.TesterOptions{Dark: dark})
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, scheme+"-"+name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}

func showDetails(a *App) {
	a.q.detailJob = "error"
}

func showSelection(a *App) {
	a.openSelection("select")
	a.q.selectAll = 1
}

func customJob() engine.JobView {
	v := &ytdlp.Video{
		Title: "可选格式的视频",
		Formats: []ytdlp.Format{
			{ID: "137", Height: 1080, Fps: 30, Filesize: 100 << 20},
			{ID: "136", Height: 720, Fps: 30, Filesize: 50 << 20},
			{ID: "140", Abr: 129, Filesize: 5 << 20},
		},
		VideoCodecs: []ytdlp.Codec{{ID: "avc1.640028", Label: "H.264 High, Level 4.0"}, {ID: "vp9", Label: "VP9"}},
		AudioCodecs: []ytdlp.Codec{{ID: "mp4a.40.2", Label: "AAC LC"}},
		AudioTracks: []ytdlp.Track{{ID: "lang:en|channels:2", Label: "en - 2ch"}, {ID: "lang:ja|channels:2", Label: "ja - 2ch"}},
	}
	return engine.JobView{ID: "c", Title: v.Title, Status: engine.StatusReady, Preset: "720p",
		Items: []engine.ItemView{{ID: "i", Status: engine.StatusReady, Video: v, PlaylistIndex: -1}}}
}

func TestCustomFormatPanel(t *testing.T) {
	q := &fakeQueue{jobs: []engine.JobView{customJob()}}
	a := New(Deps{Queue: q})
	tt := ui.NewTester(a.View, 1000, 760)
	if tt.HasText("仅视频（无声）") {
		t.Fatal("panel shown for a preset")
	}
	// Choosing 自定义… starts from what the preset resolved to.
	if got := customStart(q.jobs[0]); got.TrackType != ytdlp.TrackBoth || got.Height != 720 {
		t.Fatalf("start %+v", got)
	}
	q.SetCustom("c", customStart(q.jobs[0]))
	tt.Frame()
	for _, want := range []string{"视频", "仅视频（无声）", "仅音频", "720p", "视频编码：自动", "音频编码：自动", "音轨：自动"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing, texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("仅音频"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if last := q.calls[len(q.calls)-1]; last != "custom c audio 0 0" {
		t.Fatalf("calls %q", q.calls)
	}
	if !tt.HasText("最佳音质") || tt.HasText("视频编码：自动") {
		t.Errorf("audio controls wrong, texts %q", tt.Texts())
	}
}

func showCustom(a *App) {
	q := a.queue.(*fakeQueue)
	q.jobs = append([]engine.JobView{customJob()}, q.jobs...)
	q.SetCustom("c", ytdlp.FormatOptions{TrackType: ytdlp.TrackBoth, Height: 1080, Fps: 30, VideoEncoding: "vp9"})
}

func TestMixedLinkDialog(t *testing.T) {
	q := &fakeQueue{}
	f := newFakeSettings()
	d := f.deps()
	d.Queue = q
	a := New(d)
	tt := ui.NewTester(a.View, 1000, 700)
	a.q.link = "https://www.youtube.com/watch?v=TAMyLwW9HW8&list=RDTAMyLwW9HW8&start_radio=1\nhttps://www.youtube.com/watch?v=plain"
	a.addLinks(true)
	tt.Frame()
	if len(q.added) != 1 || q.added[0] != "https://www.youtube.com/watch?v=plain" {
		t.Fatalf("added before the choice: %q", q.added)
	}
	if !tt.HasText("这个链接同时包含视频和播放列表") || !tt.HasText("这是 YouTube 根据视频自动生成的合辑，可以无限往下播，整个列表可能有几百个视频。") {
		t.Fatalf("no dialog, texts %q", tt.Texts())
	}
	if err := tt.Click("只下载这个视频"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(q.added) != 2 || q.modes[1] != ytdlp.PlaylistModeSingle || !q.starts[1] {
		t.Fatalf("after choosing: %q %q", q.added, q.modes)
	}
	if tt.HasText("这个链接同时包含视频和播放列表") {
		t.Error("dialog still open")
	}

	// Remembered: the next one adds the list without asking.
	a.q.link = "https://www.youtube.com/watch?v=a&list=PLx"
	a.addLinks(false)
	tt.Frame()
	tt.Click("记住我的选择，以后不再询问")
	tt.Frame()
	if err := tt.Click("下载整个列表"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if f.s.Input.MixedLinks != formats.MixedList || q.modes[len(q.modes)-1] != ytdlp.PlaylistModeAll {
		t.Fatalf("setting %q, modes %q", f.s.Input.MixedLinks, q.modes)
	}
	a.q.link = "https://youtu.be/b?list=PLy"
	a.addLinks(false)
	tt.Frame()
	if tt.HasText("这个链接同时包含视频和播放列表") || q.modes[len(q.modes)-1] != ytdlp.PlaylistModeAll {
		t.Errorf("asked again or wrong mode: %q", q.modes)
	}
}
