package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/youtube-dl-gui/internal/core"
)

type fakeTools struct {
	state   core.ToolsState
	started int
}

func (f *fakeTools) State() core.ToolsState { return f.state }
func (f *fakeTools) Start() { f.started++ }

var installing = core.ToolsState{
	Phase: core.ToolsInstalling, MissingRequired: []string{"yt-dlp", "ffmpeg"},
	Tools: []core.ToolStatus{
		{Name: "yt-dlp", Version: "2026.10.26", Missing: true, Received: 18 << 20, Total: 18 << 20, Done: true},
		{Name: "ffmpeg", Version: "b6.1.1", Missing: true, Received: 31 << 20, Total: 84 << 20},
		{Name: "deno", Version: "v2.9.8", Missing: true},
	},
}

func TestInstallPage(t *testing.T) {
	tools := &fakeTools{state: installing}
	a := New(Deps{Tools: tools})
	tt := ui.NewTester(a.View, 1000, 760)
	for _, want := range []string{"正在准备下载工具", "完成", "31.0 MB / 84.0 MB", "等待中"} {
		if !tt.HasText(want) {
			t.Fatalf("%q missing, texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("队列为空") {
		t.Fatal("queue shown while tools are missing")
	}

	tools.state.Phase, tools.state.Err = core.ToolsFailed, "manifest: dial tcp: timeout"
	tools.state.Tools[1].Done, tools.state.Tools[1].Err = true, "download: EOF"
	tt.Frame()
	if !tt.HasText("失败") {
		t.Fatalf("failure not shown, texts %q", tt.Texts())
	}
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	if tools.started != 1 {
		t.Fatalf("retry started %d checks", tools.started)
	}
	if err := tt.Click("打开设置"); err != nil {
		t.Fatal(err)
	}
	if a.router.Path() != "/settings" || !tt.HasText("下载工具未安装") {
		t.Fatalf("settings not reachable: path %q, texts %q", a.router.Path(), tt.Texts())
	}

	tools.state = core.ToolsState{Phase: core.ToolsInstalling, Tools: installing.Tools}
	tt.Frame()
	if !tt.HasText("正在更新下载工具…") {
		t.Fatalf("background update not noted, texts %q", tt.Texts())
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 31 << 20: "31.0 MB", 3 << 30: "3.0 GB"}
	for n, want := range cases {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestNavigation(t *testing.T) {
	a := New(Deps{})
	tt := ui.NewTester(a.View, 1000, 760)
	if !tt.HasText("队列为空") {
		t.Fatalf("queue page not shown, texts %q", tt.Texts())
	}
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	if a.router.Path() != "/about" || !tt.HasText("版本 dev") {
		t.Fatalf("about page not shown: path %q, texts %q", a.router.Path(), tt.Texts())
	}
}

// TestScreenshots writes PNGs of the main pages to $SCREENSHOT_DIR, which CI
// uploads as an artifact so the layout can be reviewed without running the app.
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{true, false} {
		scheme := "light"
		if dark {
			scheme = "dark"
		}
		for _, page := range []string{"/queue", "/settings", "/about"} {
			a := New(Deps{})
			a.router.Replace(page)
			tt := ui.NewTesterWith(a.View, 1000, 760, ui.TesterOptions{Dark: dark})
			tt.Frame()
			name := filepath.Join(dir, scheme+"-"+filepath.Base(page)+".png")
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
		for name, state := range map[string]core.ToolsState{"install": installing, "install-checking": {MissingRequired: []string{"yt-dlp"}}} {
			a := New(Deps{Tools: &fakeTools{state: state}})
			tt := ui.NewTesterWith(a.View, 1000, 760, ui.TesterOptions{Dark: dark})
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
