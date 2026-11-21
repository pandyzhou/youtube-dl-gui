package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
)

// detailView shows a job in place of the list: what it is, what went
// wrong, and yt-dlp's output.
func (a *App) detailView(c *ui.Context, p palette, j engine.JobView) {
	ui.Column(c).Fill().AlignItems(ui.Stretch).Padding(gridTop, pagePadX, 24, pagePadX).Gap(sectionGap).Children(func() {
		ui.Row(c).Height(headerHeight).AlignItems(ui.Center).Gap(10).Children(func() {
			if iconButton(c, iconBack, "返回队列").Clicked() {
				a.q.detailJob = ""
			}
			ui.Text(c, j.Title).FontSize(18).Bold().SingleLine().Shrink(1)
		})
		ui.Tabs(c, &a.q.detailTab, "信息", "日志")
		if a.q.detailTab == 1 {
			a.logView(c, p, j)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(sectionGap).AlignItems(ui.Stretch).Children(func() {
				a.card(c, p, func() {
					row := func(label, value string) {
						if value == "" {
							return
						}
						ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
							ui.Text(c, label).FontSize(13).TextColor(p.muted).Width(72)
							ui.Text(c, value).FontSize(13).Selectable().Wrap().Shrink(1)
						})
					}
					row("链接", j.URL)
					row("状态", statusLabels[j.Status])
					row("上传者", j.Uploader)
					row("时长", formatDuration(j.Duration))
					if j.Size > 0 {
						row("大小", "约 "+formatBytes(j.Size))
					}
					if len(j.Items) == 1 {
						it := j.Items[0]
						row("画质", it.Choice)
						row("文件", it.Destination)
						if v := it.Video; v != nil {
							row("网站", v.Extractor)
							if v.Views != nil {
								row("播放量", fmt.Sprint(*v.Views))
							}
							if len(v.SubtitleInventory.ManualLanguages) > 0 {
								row("字幕", strings.Join(v.SubtitleInventory.ManualLanguages, ", "))
							}
						}
					}
					if j.Playlist != nil {
						row("播放列表", j.Playlist.Title)
					}
				})
				a.problemList(c, p, j)
				if j.Combined {
					a.itemList(c, p, j)
				}
			})
		})
	})
}

// problemList lists the errors and warnings yt-dlp printed.
func (a *App) problemList(c *ui.Context, p palette, j engine.JobView) {
	type problem struct{ title, detail, raw string }
	var problems []problem
	for _, it := range j.Items {
		for _, d := range it.Diagnostics {
			title, detail := d.Code, d.Message
			if text, ok := diagnosticText[d.Code]; ok && d.Code != "unknown" {
				title, detail = text[0], text[1]
			} else if d.Level == "warning" {
				title = "警告"
			} else {
				title = "错误"
			}
			problems = append(problems, problem{title, detail, d.Raw})
		}
		if it.Status == engine.StatusError && len(it.Diagnostics) == 0 && it.Err != "" {
			problems = append(problems, problem{"错误", it.Err, ""})
		}
	}
	if len(problems) == 0 {
		return
	}
	t := c.Theme()
	a.card(c, p, func() {
		ui.Text(c, "问题").FontSize(14).Bold()
		for i, pr := range problems {
			c.Key(fmt.Sprint("problem", i))
			ui.Column(c).Gap(2).AlignItems(ui.Stretch).Children(func() {
				ui.Text(c, pr.title+"："+pr.detail).FontSize(13).TextColor(t.Danger).Wrap()
				if pr.raw != "" {
					ui.Text(c, pr.raw).FontSize(11).TextColor(p.muted).Selectable().Wrap()
				}
			})
		}
	})
}

// itemList lists the videos of a playlist card.
func (a *App) itemList(c *ui.Context, p palette, j engine.JobView) {
	a.card(c, p, func() {
		ui.Text(c, fmt.Sprintf("视频（%d）", len(j.Items))).FontSize(14).Bold()
		for _, it := range j.Items {
			c.Key(it.ID)
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprint(it.PlaylistIndex+1)).FontSize(12).TextColor(p.muted).Width(28)
				ui.Text(c, it.Title).FontSize(13).SingleLine().Grow(1).Shrink(1)
				status := statusLabels[it.Status]
				if it.Status == engine.StatusDownloading {
					status = fmt.Sprintf("%.0f%%", it.Percent)
				}
				ui.Text(c, status).FontSize(12).TextColor(p.muted).SingleLine()
			})
		}
	})
}

func (a *App) logView(c *ui.Context, p palette, j engine.JobView) {
	lines := a.queue.Log(j.ID)
	ui.List(c, nil, len(lines), func(i int) {
		ui.Text(c, lines[i]).FontSize(12).Font("Cascadia Mono, Consolas, monospace").Selectable().Wrap().Padding(1, 4)
	}).Grow(1).Padding(8).Radius(8).Background(p.surface).Border(1, p.border).Children(func() {
		if len(lines) == 0 {
			ui.Text(c, "还没有输出。").FontSize(13).TextColor(p.muted).Padding(8)
		}
	})
}
