// Command ovd is Open Video Downloader, a yt-dlp front end with a native UI.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	appui "github.com/pandyzhou/youtube-dl-gui/internal/ui"
)

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

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
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Open Video Downloader",
			Width:     1000,
			Height:    760,
			MinWidth:  760,
			MinHeight: 560,
			StateKey:  "main",
			Content:   ui.View(app.View),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
