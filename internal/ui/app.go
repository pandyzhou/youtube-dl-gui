// Package ui is the native interface of Open Video Downloader.
package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/core"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
)

// Version is shown in the sidebar; main sets it from mygo.App.Version().
var Version = "dev"

// Layout grid shared by the sidebar and the pages, in DIPs, so their edges
// and top lines meet.
const (
	gridTop       = 16  // top of the first row in the sidebar and the pages
	headerHeight  = 40  // the brand row and the page title row
	sidebarWidth  = 240 // sidebar, including its padding
	sidebarPadX   = 12  // sidebar side padding
	rowInset      = 10  // left inset of every sidebar row's content
	navRowHeight  = 36  // sidebar item height
	pagePadX      = 28  // page side padding
	controlHeight = 36  // inputs, selects and buttons in the pages
	sectionGap    = 16  // between blocks on a page
)

type navItem struct {
	path  string
	icon  *ui.SVG
	label string
}

var (
	mainNav   = []navItem{{"/queue", iconQueue, "下载队列"}}
	footerNav = []navItem{{"/settings", iconSettings, "设置"}, {"/about", iconInfo, "关于"}}
)

// Deps are what the window shows and drives. Nil fields are left out, as
// in tests of a single page.
type Deps struct {
	Tools             ToolsSource
	Queue             QueueSource
	Settings          func() config.Settings
	Preferences       func() config.Preferences
	UpdateSettings    func(change func(*config.Settings)) error
	UpdatePreferences func(change func(*config.Preferences)) error
	GetSecret         func(key string) string
	SetSecret         func(key, value string) error
	// ShowInFolder reveals a downloaded file in Explorer.
	ShowInFolder func(path string)
	// CheckUpdates opens the update window and checks now.
	CheckUpdates func()
	// PickPath asks for a folder (or a file) and calls done on the main
	// thread with it, or "" when canceled.
	PickPath func(title, current string, folder bool, done func(path string))
	// ListModels asks the translation API for its models and calls done on
	// the main thread.
	ListModels func(baseURL, apiKey string, done func(models []string, err error))
}

// App is the state of the main window.
type App struct {
	router *ui.Router
	deps   Deps
	tools  ToolsSource
	queue  QueueSource
	q      queueState
	s      settingsState

	// skipInstall shows the app over a failed tool install, so the proxy
	// can be set; downloads stay unavailable until the tools exist.
	skipInstall bool
}

// New returns the app on its queue page.
func New(d Deps) *App {
	a := &App{router: ui.NewRouter("/queue"), deps: d, tools: d.Tools, queue: d.Queue}
	a.q.presetLabel = formats.PresetByID(a.settings().Input.DefaultPreset).Label
	return a
}

func (a *App) settings() config.Settings {
	if a.deps.Settings == nil {
		return config.DefaultSettings("")
	}
	return a.deps.Settings()
}

func (a *App) toolsState() core.ToolsState {
	if a.tools == nil {
		return core.ToolsState{Phase: core.ToolsReady}
	}
	return a.tools.State()
}

// View builds the main window.
func (a *App) View(c *ui.Context) {
	p, forced := applyTheme(c, a.settings().Appearance.Theme)
	// Over the window material (Mica on Windows 11) the root stays clear so
	// the sidebar shows it; the content draws its own opaque background.
	tools := a.toolsState()
	if len(tools.MissingRequired) > 0 && !a.skipInstall {
		a.installView(c, p, tools)
		return
	}
	vibrancy := c.Vibrancy() && !forced
	if vibrancy {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		a.sidebar(c, p, tools, vibrancy)
		ui.Column(c).Grow(1).AlignItems(ui.Stretch).Background(p.content).Children(func() {
			a.router.View(c, func(r *ui.Route) {
				if !r.Match("/settings") {
					a.s.loaded = false // read the settings again when it opens
				}
				switch {
				case r.Match("/queue"):
					r.Title("下载队列")
					a.queuePage(c, p)
				case r.Match("/settings"):
					r.Title("设置")
					a.settingsPage(c, p)
				case r.Match("/about"):
					r.Title("关于")
					a.aboutPage(c, p)
				default:
					a.page(c, p, "未找到", nil, func() { ui.Text(c, r.Path()).TextColor(p.muted) })
				}
			})
		})
	})
}

func (a *App) sidebar(c *ui.Context, p palette, tools core.ToolsState, vibrancy bool) {
	col := ui.Column(c).Width(sidebarWidth).AlignItems(ui.Stretch).
		Padding(gridTop, sidebarPadX, 14, sidebarPadX).Gap(2)
	if !vibrancy {
		col.Background(p.sidebar)
	}
	col.Children(func() {
		ui.Row(c).Height(headerHeight).AlignItems(ui.Center).Gap(10).Padding(0, rowInset).Children(func() {
			ui.Image(c, appIcon).Size(22, 22).Fit(ui.Contain)
			ui.Text(c, "Open Video Downloader").FontSize(13).Bold().SingleLine()
		})
		ui.Box(c).Height(10)
		for _, item := range mainNav {
			a.navRow(c, p, item)
		}
		ui.Spacer(c)
		for _, item := range footerNav {
			a.navRow(c, p, item)
		}
		if note := toolsNote(tools); note != "" {
			ui.Text(c, note).FontSize(11).TextColor(p.muted).Padding(8, rowInset, 0, rowInset)
		}
		ui.Text(c, "v"+Version).FontSize(11).TextColor(p.muted).Padding(8, rowInset, 0, rowInset)
	})
}

// toolsNote tells, under the navigation, about a background tool update.
func toolsNote(s core.ToolsState) string {
	switch {
	case len(s.MissingRequired) > 0:
		return "下载工具未安装"
	case s.Phase == core.ToolsInstalling:
		return "正在更新下载工具…"
	case s.Phase == core.ToolsFailed && len(s.Tools) > 0:
		return "下载工具更新失败"
	}
	return ""
}

// navRow is one sidebar item: icon and label at the shared inset, a light
// accent tint when chosen, a faint fill under the pointer.
func (a *App) navRow(c *ui.Context, p palette, item navItem) {
	t := c.Theme()
	chosen := a.router.Path() == item.path
	c.Key(item.path)
	row := ui.ButtonBase(c).Height(navRowHeight).Justify(ui.Start).AlignItems(ui.Center).Gap(10).
		Padding(0, rowInset).Radius(6).Label(item.label)
	switch {
	case chosen:
		row.Background(p.selected)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		icon := ui.Icon(c, item.icon).FontSize(16)
		if chosen {
			icon.TextColor(t.Accent)
		} else {
			icon.TextColor(p.muted)
		}
		label := ui.Text(c, item.label).FontSize(13).SingleLine()
		if chosen {
			label.Bold()
		}
	})
	if row.Clicked() && !chosen {
		a.router.Push(item.path)
	}
}

// page lays out a page: the title row on the shared grid, an optional
// trailing part of the title row, then the content.
func (a *App) page(c *ui.Context, p palette, title string, trailing func(), content func()) {
	ui.Column(c).Fill().AlignItems(ui.Stretch).Padding(gridTop, pagePadX, 24, pagePadX).Gap(sectionGap).Children(func() {
		ui.Row(c).Height(headerHeight).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(20).Bold().SingleLine()
			ui.Spacer(c)
			if trailing != nil {
				trailing()
			}
		})
		content()
	})
}

func (a *App) aboutPage(c *ui.Context, p palette) {
	a.page(c, p, "关于", nil, func() {
		a.card(c, p, func() {
			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Image(c, appIcon).Size(44, 44).Fit(ui.Contain)
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Open Video Downloader").FontSize(15).Bold()
					ui.Text(c, "版本 "+Version).FontSize(13).TextColor(p.muted)
				})
			})
			ui.Text(c, "基于 yt-dlp 的视频下载工具，界面由 MyGo 原生绘制。").FontSize(13).TextColor(p.muted)
			if a.deps.CheckUpdates != nil {
				ui.Row(c).Children(func() {
					if ui.Button(c, "检查更新").Height(controlHeight).Padding(0, 16).Clicked() {
						a.deps.CheckUpdates()
					}
				})
			}
		})
	})
}

// card is a panel on the page: the surface color, a hairline, 10px corners.
func (a *App) card(c *ui.Context, p palette, content func()) {
	ui.Column(c).AlignItems(ui.Stretch).Gap(12).Padding(16).Radius(10).
		Background(p.surface).Border(1, p.border).Children(content)
}
