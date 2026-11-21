// Command ovd is Open Video Downloader, a yt-dlp front end with a native UI.
package main

import (
	"log"
	"log/slog"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

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
	_ = core.Open(dirs, secrets.System{}) // the engine (M4) and settings page (M6) use it

	appui.Version = mygo.App.Version()
	app := appui.New()
	var win *mygo.Window
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		if win != nil {
			win.Restore()
			win.Focus()
		}
	})
	mygo.App.WhenReady(func() {
		// Vibrancy puts Mica behind the sidebar on Windows 11 22H2+; opaque elsewhere.
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Open Video Downloader",
			Width:     1000,
			Height:    760,
			MinWidth:  760,
			MinHeight: 560,
			StateKey:  "main",
			Vibrancy:  mygo.VibrancySidebar,
			Content:   ui.View(app.View),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
