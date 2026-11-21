package main

import (
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/core"
	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
	appui "github.com/pandyzhou/youtube-dl-gui/internal/ui"
)

// desktop ties the app to Windows: the tray icon, global shortcuts,
// notifications, start at login, close to tray and the taskbar progress.
// Its methods run on the main thread unless they say otherwise.
type desktop struct {
	core   *core.Core
	queue  *engine.Engine
	app    *appui.App
	window func() *mygo.Window
	redraw func()

	tray      *mygo.Tray
	shortcuts bool
	quitting  bool
}

// Global shortcuts.
const (
	shortcutAdd      = "Alt+Shift+V"
	shortcutDownload = "Alt+Shift+D"
	shortcutStartAll = "Alt+Shift+Enter"
)

// apply brings the tray, shortcuts and start at login in line with the
// settings; it runs at startup and after every settings change.
func (d *desktop) apply() {
	s := d.core.Settings()
	switch {
	case s.System.TrayEnabled && d.tray == nil:
		tray, err := mygo.NewTray(mygo.TrayOptions{Icon: appui.AppIconPNG(), ToolTip: "流光下载", Menu: d.trayMenu()})
		if err != nil {
			slog.Warn("tray", "err", err)
		} else {
			d.tray = tray // its menu opens on click
		}
	case !s.System.TrayEnabled && d.tray != nil:
		d.tray.Destroy()
		d.tray = nil
	}

	switch {
	case s.Input.GlobalShortcuts && !d.shortcuts:
		for acc, fn := range map[string]func(){
			shortcutAdd:      func() { d.addClipboard(false) },
			shortcutDownload: func() { d.addClipboard(true) },
			shortcutStartAll: func() { d.queue.StartAll() },
		} {
			if err := mygo.GlobalShortcut.Register(acc, fn); err != nil {
				slog.Warn("global shortcut", "keys", acc, "err", err)
			}
		}
		d.shortcuts = true
	case !s.Input.GlobalShortcuts && d.shortcuts:
		for _, acc := range []string{shortcutAdd, shortcutDownload, shortcutStartAll} {
			mygo.GlobalShortcut.Unregister(acc)
		}
		d.shortcuts = false
	}

	if s.Update.UpdateApp != updater.AutomaticChecks() {
		updater.SetAutomaticChecks(s.Update.UpdateApp)
	}

	if s.System.AutoStartEnabled != mygo.App.OpenAtLogin() {
		if err := mygo.App.SetOpenAtLogin(s.System.AutoStartEnabled); err != nil {
			slog.Warn("start at login", "err", err)
		}
	}
}

func (d *desktop) trayMenu() *mygo.Menu {
	item := func(label string, fn func()) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) { fn() }}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		item("显示窗口", d.show),
		mygo.Separator(),
		item("加入复制的链接", func() { d.addClipboard(false) }),
		item("加入并下载复制的链接", func() { d.addClipboard(true) }),
		item("全部开始", func() { d.queue.StartAll() }),
		item("全部暂停", d.queue.PauseAll),
		mygo.Separator(),
		item("退出", d.quit),
	})
}

func (d *desktop) show() {
	if w := d.window(); w != nil {
		w.Show()
		w.Restore()
		w.Focus()
	}
}

func (d *desktop) quit() {
	d.quitting = true
	mygo.App.Quit()
}

// addClipboard adds the links on the clipboard.
func (d *desktop) addClipboard(start bool) {
	urls, _ := formats.ParseURLFile(mygo.Clipboard.ReadText())
	if len(urls) == 0 {
		d.notify("剪贴板里没有链接", "复制视频链接后再试。")
		return
	}
	mixed := d.core.Settings().Input.MixedLinks
	for _, u := range urls {
		d.queue.Add(u, engine.AddOptions{Start: start, Playlist: formats.MixedMode(u, mixed)})
	}
	d.redraw()
}

// onClose hides the window to the tray when the settings say so.
func (d *desktop) onClose(e *mygo.CloseEvent) {
	s := d.core.Settings()
	if d.quitting || s.System.CloseBehavior != config.CloseHide || d.tray == nil {
		return
	}
	e.PreventDefault()
	if w := d.window(); w != nil {
		w.Hide()
	}
}

// notice kinds of engine events, as the notification settings name them.
func noticeKind(ev engine.Event) string {
	switch ev.Kind {
	case engine.EventVideoReady:
		return config.NoticeVideoReady
	case engine.EventPlaylistReady:
		return config.NoticePlaylistReady
	case engine.EventDownloadDone:
		if ev.Count > 1 {
			return config.NoticePlaylistFinished
		}
		return config.NoticeVideoFinished
	case engine.EventDownloadError:
		return config.NoticeDownloadFailed
	case engine.EventQueueDone:
		return config.NoticeQueueFinished
	}
	return ""
}

func noticeText(ev engine.Event) (string, string) {
	switch ev.Kind {
	case engine.EventVideoReady:
		return "视频已就绪", ev.Title
	case engine.EventPlaylistReady:
		return "播放列表已就绪", ev.Title
	case engine.EventDownloadDone:
		return "下载完成", ev.Title
	case engine.EventDownloadError:
		return "下载失败", ev.Title
	case engine.EventQueueDone:
		return "队列已完成", "所有下载都已结束。"
	}
	return "", ""
}

// event shows a notification for an engine event; it is called from the
// engine's goroutines.
func (d *desktop) event(ev engine.Event) {
	mygo.RunOnMain(func() {
		s := d.core.Settings().Notifications
		kind := noticeKind(ev)
		if kind == "" || s.NotificationBehavior == config.NotifyNever || slices.Contains(s.DisabledNotifications, kind) {
			return
		}
		if s.NotificationBehavior == config.NotifyOnBackground && d.foreground() {
			return
		}
		title, body := noticeText(ev)
		d.notify(title, body)
	})
}

func (d *desktop) foreground() bool {
	w := d.window()
	return w != nil && w.IsVisible() && !w.IsMinimized() && mygo.App.IsActive()
}

func (d *desktop) notify(title, body string) {
	if strings.TrimSpace(body) == "" {
		body = title
	}
	n := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body})
	n.OnClick(d.show)
	if err := n.Show(); err != nil {
		slog.Warn("notification", "err", err)
	}
}

// watchProgress shows the overall download progress on the taskbar button
// every second; it runs on its own goroutine.
func (d *desktop) watchProgress() {
	last := -1.0
	for range time.Tick(time.Second) {
		var sum float64
		n := 0
		for _, j := range d.queue.Jobs() {
			if j.Status == engine.StatusDownloading || j.Status == engine.StatusQueued {
				sum += j.Percent
				n++
			}
		}
		value := 0.0
		if n > 0 {
			value = max(sum/float64(n)/100, 0.01)
		}
		if value == last {
			continue
		}
		last = value
		mygo.RunOnMain(func() {
			if w := d.window(); w != nil {
				w.SetProgressBar(mygo.ProgressBar{Value: value})
			}
		})
	}
}
