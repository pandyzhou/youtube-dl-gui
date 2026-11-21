// Command ovd is Open Video Downloader, a yt-dlp front end with a native UI.
package main

import (
	"log"
	"log/slog"
	"path/filepath"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/youtube-dl-gui/internal/binaries"
	"github.com/pandyzhou/youtube-dl-gui/internal/core"
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

	appui.Version = mygo.App.Version()
	app := appui.New(appui.Deps{Tools: tools})
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		if w := window.Load(); w != nil {
			w.Restore()
			w.Focus()
		}
	})
	mygo.App.WhenReady(func() {
		// Vibrancy puts Mica behind the sidebar on Windows 11 22H2+; opaque elsewhere.
		window.Store(mygo.NewWindow(mygo.WindowOptions{
			Title:     "Open Video Downloader",
			Width:     1000,
			Height:    760,
			MinWidth:  760,
			MinHeight: 560,
			StateKey:  "main",
			Vibrancy:  mygo.VibrancySidebar,
			Content:   ui.View(app.View),
		}))
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
