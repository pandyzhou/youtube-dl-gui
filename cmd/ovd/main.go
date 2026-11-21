// Command ovd is Open Video Downloader, a yt-dlp front end with a native UI.
package main

import (
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/netimage"

	"github.com/pandyzhou/youtube-dl-gui/internal/binaries"
	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/core"
	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
	"github.com/pandyzhou/youtube-dl-gui/internal/httpx"
	"github.com/pandyzhou/youtube-dl-gui/internal/secrets"
	appui "github.com/pandyzhou/youtube-dl-gui/internal/ui"
)

// resolveDirs asks MyGo for the app's folders: %AppData%\<name> for data,
// its logs folder, and the user's Downloads folder.
func resolveDirs() core.Dirs {
	path := func(kind mygo.PathName) string {
		dir, err := mygo.App.Path(kind)
		if err != nil {
			slog.Error("resolve folder", "kind", kind, "err", err)
		}
		return dir
	}
	data := path(mygo.PathUserData)
	return core.Dirs{
		Data:      data,
		Bin:       filepath.Join(data, "bin"),
		Logs:      path(mygo.PathLogs),
		Downloads: path(mygo.PathDownloads),
		Legacy:    core.LegacyDir(),
	}
}

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	// The update window, drawn natively: checks a day apart when updates
	// are on (desktop.apply), and on demand from the about page.
	mygo.Use(native.Plugin)

	dirs := resolveDirs()
	if closer, err := core.SetupLogging(dirs.Logs); err == nil {
		defer closer.Close()
	}
	slog.Info("starting", "version", mygo.App.Version())
	c := core.Open(dirs, secrets.System{})

	// Background goroutines redraw the window through it once it exists.
	var window atomic.Pointer[mygo.Window]
	redraw := func() {
		if w := window.Load(); w != nil {
			w.Invalidate()
		}
	}
	tools := core.NewTools(c, redraw)
	if needsToolCheck(c, tools) {
		tools.Start()
	}

	var d *desktop // set before any event can come
	queue := core.NewEngine(c, tools, redraw, func(ev engine.Event) { d.event(ev) })
	defer queue.Close()
	netimage.SetClient(httpx.Client(c.Settings().Network, 30*time.Second))

	appui.Version = mygo.App.Version()
	app := appui.New(appui.Deps{
		Tools:       tools,
		Queue:       queue,
		Settings:    c.Settings,
		Preferences: c.Preferences,
		UpdateSettings: func(change func(*config.Settings)) error {
			err := c.UpdateSettings(change)
			s := c.Settings()
			queue.SetConcurrency(s.Performance.MaxConcurrency)
			netimage.SetClient(httpx.Client(s.Network, 30*time.Second))
			d.apply()
			redraw()
			return err
		},
		UpdatePreferences: c.UpdatePreferences,
		GetSecret: func(key string) string {
			v, err := c.Secrets.Get(key)
			if err != nil {
				slog.Warn("read secret", "key", key, "err", err)
			}
			return v
		},
		SetSecret: func(key, value string) error {
			if value == "" {
				return c.Secrets.Delete(key)
			}
			return c.Secrets.Set(key, value)
		},
		ShowInFolder: mygo.Shell.ShowItemInFolder,
		CheckUpdates: updater.CheckForUpdates,
		PickPath: func(title, current string, folder bool, done func(string)) {
			go func() {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
					Parent: window.Load(), Title: title, DefaultPath: current, Directory: folder,
				})
				path := ""
				if err == nil && len(paths) > 0 {
					path = paths[0]
				}
				mygo.RunOnMain(func() {
					done(path)
					redraw()
				})
			}()
		},
	})
	d = &desktop{core: c, queue: queue, app: app, window: window.Load, redraw: redraw}
	mygo.App.OnSecondInstance(func(args []string, workingDir string) { d.show() })
	mygo.App.WhenReady(func() {
		watchClipboard(c, app, redraw)
		// Started at login to sit in the tray: no window until asked.
		s := c.Settings()
		hidden := mygo.App.WasOpenedAtLogin() && s.System.AutoStartMinimised && s.System.TrayEnabled
		// Vibrancy puts Mica behind the sidebar on Windows 11 22H2+; opaque elsewhere.
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "Open Video Downloader",
			Width:     1000,
			Height:    760,
			MinWidth:  760,
			MinHeight: 560,
			StateKey:  "main",
			Vibrancy:  mygo.VibrancySidebar,
			Content:   ui.View(app.View),
			Hidden:    hidden,
		})
		window.Store(win)
		win.OnClose(d.onClose)
		d.apply()
		go d.watchProgress()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// needsToolCheck reports whether to fetch the tool manifest at startup: a
// tool is missing, or updates are on. Otherwise startup stays offline.
func needsToolCheck(c *core.Core, tools *core.Tools) bool {
	if c.Settings().Update.UpdateBinaries {
		return true
	}
	for _, name := range binaries.Tools {
		if !tools.Manager().Installed(name) {
			return true
		}
	}
	return false
}

// watchClipboard fills the link input with a link copied elsewhere, when
// the setting is on. It runs on the main thread, where the view's state
// lives.
func watchClipboard(c *core.Core, app *appui.App, redraw func()) {
	last := ""
	mygo.Clipboard.OnChanged(func() {
		if !c.Settings().Input.AutoFillClipboard {
			return
		}
		text := strings.TrimSpace(mygo.Clipboard.ReadText())
		if text == last || !formats.ValidURL(text) {
			return
		}
		last = text
		app.SetLink(text)
		redraw()
	})
}
