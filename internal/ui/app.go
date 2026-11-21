// Package ui is the native interface of Open Video Downloader.
package ui

import (
	"github.com/egoist/mygo/ui"
)

// Version is shown in the sidebar; main sets it from mygo.App.Version().
var Version = "dev"

// Quality presets offered next to the link input. The engine maps each to
// the closest format a video has (milestone M4).
var qualityPresets = []string{
	"最佳画质", "2160p", "1440p", "1080p", "720p", "480p", "仅音频（最佳）", "仅音频 MP3", "自定义…",
}

// App is the state of the main window.
type App struct {
	router *ui.Router

	link   string
	preset string
}

// New returns the app on its queue page.
func New() *App {
	return &App{router: ui.NewRouter("/queue"), preset: qualityPresets[0]}
}

// View builds the main window.
func (a *App) View(c *ui.Context) {
	p := applyTheme(c)
	// Over the window material (Mica on Windows 11) the root stays clear so
	// the sidebar shows it; the content draws its own opaque background.
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		a.sidebar(c, p)
		ui.Column(c).Grow(1).AlignItems(ui.Stretch).Background(p.content).Children(func() {
			a.router.View(c, func(r *ui.Route) {
				switch {
				case r.Match("/queue"):
					r.Title("下载队列")
					a.queuePage(c, p)
				case r.Match("/settings"):
					r.Title("设置")
					a.placeholderPage(c, p, "设置", "设置页面将在后续版本中完成。")
				case r.Match("/about"):
					r.Title("关于")
					a.aboutPage(c, p)
				default:
					a.placeholderPage(c, p, "未找到", r.Path())
				}
			})
		})
	})
}

func (a *App) sidebar(c *ui.Context, p palette) {
	col := ui.Column(c).Width(220).AlignItems(ui.Stretch).Padding(16, 12).Gap(12)
	if !c.Vibrancy() {
		col.Background(p.sidebar)
	}
	col.Children(func() {
		ui.Text(c, "Open Video Downloader").Bold().Padding(0, 8)
		page := a.router.Path()
		if ui.Sidebar(c, &page, func() {
			ui.SidebarItem(c, "/queue", iconQueue, "下载队列")
			ui.SidebarItem(c, "/settings", iconSettings, "设置")
			ui.SidebarItem(c, "/about", iconInfo, "关于")
		}).Background(ui.Transparent).Grow(1).Changed() {
			a.router.Push(page)
		}
		ui.Text(c, "v"+Version).FontSize(12).TextColor(p.muted).Padding(0, 8)
	})
}

func (a *App) pageHeader(c *ui.Context, p palette, title, subtitle string) {
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, title).FontSize(20).Bold()
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(13).TextColor(p.muted)
		}
	})
}

func (a *App) queuePage(c *ui.Context, p palette) {
	ui.Column(c).Fill().AlignItems(ui.Stretch).Padding(20, 24).Gap(16).Children(func() {
		a.pageHeader(c, p, "下载队列", "粘贴链接，选好画质，点“下载”即可开始。")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.link).Placeholder("粘贴视频或播放列表链接").Label("链接").Grow(1)
			ui.Select(c, &a.preset, qualityPresets).Width(150)
			ui.PrimaryButton(c, "").Children(func() {
				ui.Icon(c, iconDownload)
				ui.Text(c, "下载").SingleLine()
			})
			ui.Button(c, "加入队列")
		})
		ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
			ui.Icon(c, iconQueue).FontSize(36).TextColor(p.muted)
			ui.Text(c, "队列为空").FontSize(16).Bold()
			ui.Text(c, "粘贴链接，或把链接拖到窗口里。").TextColor(p.muted)
		})
	})
}

func (a *App) placeholderPage(c *ui.Context, p palette, title, body string) {
	ui.Column(c).Fill().AlignItems(ui.Stretch).Padding(20, 24).Gap(16).Children(func() {
		a.pageHeader(c, p, title, "")
		ui.Text(c, body).TextColor(p.muted)
	})
}

func (a *App) aboutPage(c *ui.Context, p palette) {
	ui.Column(c).Fill().AlignItems(ui.Stretch).Padding(20, 24).Gap(16).Children(func() {
		a.pageHeader(c, p, "关于", "")
		ui.Column(c).Gap(6).Padding(16).Radius(10).Background(p.surface).Border(1, p.border).Children(func() {
			ui.Text(c, "Open Video Downloader").Bold()
			ui.Text(c, "版本 "+Version).TextColor(p.muted)
			ui.Text(c, "基于 yt-dlp 的视频下载工具，界面由 MyGo 原生绘制。").TextColor(p.muted)
		})
	})
}
