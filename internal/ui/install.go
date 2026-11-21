package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/youtube-dl-gui/internal/core"
)

// ToolsSource is the tool controller as the UI sees it.
type ToolsSource interface {
	State() core.ToolsState
	Start()
}

var toolLabels = map[string]string{
	"yt-dlp":        "yt-dlp（下载核心）",
	"ffmpeg":        "ffmpeg（合并与转换）",
	"ffprobe":       "ffprobe（媒体信息）",
	"deno":          "Deno（YouTube 解析）",
	"AtomicParsley": "AtomicParsley（写入封面）",
}

func toolLabel(name string) string {
	if l, ok := toolLabels[name]; ok {
		return l
	}
	return name
}

// installView replaces the window's content while required tools are
// missing: what is being downloaded, its progress, and a retry on failure.
func (a *App) installView(c *ui.Context, p palette, s core.ToolsState) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Background(p.content).Padding(32).Children(func() {
		ui.Column(c).Width(520).AlignItems(ui.Stretch).Gap(sectionGap).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Image(c, appIcon).Size(36, 36).Fit(ui.Contain)
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "正在准备下载工具").FontSize(18).Bold()
					ui.Text(c, "首次使用需要下载 yt-dlp、ffmpeg 等工具，约 150 MB，只需一次。").FontSize(12).TextColor(p.muted)
				})
			})
			a.card(c, p, func() {
				switch {
				case s.Phase == core.ToolsChecking || (s.Phase == core.ToolsIdle && len(s.Tools) == 0):
					ui.Row(c).Gap(10).AlignItems(ui.Center).Height(controlHeight).Children(func() {
						ui.Spinner(c).Label("正在获取工具清单")
						ui.Text(c, "正在获取工具清单…").FontSize(13).TextColor(p.muted)
					})
				default:
					for _, tool := range s.Tools {
						c.Key("tool-" + tool.Name)
						toolRow(c, p, tool)
					}
					if len(s.Tools) == 0 && s.Phase == core.ToolsFailed {
						ui.Text(c, "无法获取工具清单。").FontSize(13)
					}
				}
			})
			if s.Phase == core.ToolsFailed {
				ui.Column(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
					ui.Text(c, "出错了："+s.Err).FontSize(12).TextColor(t.Danger).Selectable()
					ui.Text(c, "请检查网络。如需代理，可在设置的“登录与网络”里开启后重试。").FontSize(12).TextColor(p.muted)
					ui.Row(c).Gap(8).Children(func() {
						if ui.PrimaryButton(c, "重试").Height(controlHeight).Padding(0, 20).Clicked() {
							a.tools.Start()
						}
						if ui.Button(c, "打开设置").Height(controlHeight).Padding(0, 16).Clicked() {
							a.skipInstall = true
							a.router.Push("/settings")
						}
					})
				})
			}
		})
	})
}

func toolRow(c *ui.Context, p palette, tool core.ToolStatus) {
	t := c.Theme()
	ui.Column(c).Gap(6).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, toolLabel(tool.Name)).FontSize(13).SingleLine()
			ui.Text(c, tool.Version).FontSize(12).TextColor(p.muted).SingleLine()
			ui.Spacer(c)
			status := ui.Text(c, toolStatusText(tool)).FontSize(12).SingleLine()
			switch {
			case tool.Err != "":
				status.TextColor(t.Danger)
			case tool.Done:
				status.TextColor(t.Success)
			default:
				status.TextColor(p.muted)
			}
		})
		bar := ui.Progress(c, tool.Fraction()).Label(tool.Name)
		if tool.Err != "" {
			bar.TextColor(t.Danger)
		}
		if !tool.Done && tool.Received == 0 && tool.Total == 0 {
			bar.Opacity(0.5) // waiting for its turn
		}
	})
}

func toolStatusText(tool core.ToolStatus) string {
	switch {
	case tool.Err != "":
		return "失败"
	case tool.Done:
		return "完成"
	case tool.Received == 0 && tool.Total == 0:
		return "等待中"
	case tool.Total > 0:
		return fmt.Sprintf("%s / %s", formatBytes(tool.Received), formatBytes(tool.Total))
	}
	return formatBytes(tool.Received)
}

// formatBytes formats a size in binary units with one decimal: "12.3 MB".
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value, i := float64(n)/unit, 0
	for value >= unit && i < 3 {
		value /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", value, []string{"KB", "MB", "GB", "TB"}[i])
}
