package ui

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/netimage"

	"github.com/pandyzhou/youtube-dl-gui/internal/core"
	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// QueueSource is the engine as the queue page drives it.
type QueueSource interface {
	Jobs() []engine.JobView
	Add(url string, o engine.AddOptions) string
	Start(id string)
	StartAll() int
	Pause(id string)
	PauseAll()
	Retry(id string)
	Remove(id string)
	RemoveWhere(match func(engine.JobView) bool) int
	SetPreset(id, preset string)
	SetCustom(id string, f ytdlp.FormatOptions)
	SelectPlaylist(id string, rows []formats.SelectionRow, start bool) error
	Log(id string) []string
}

// Card geometry.
const (
	thumbWidth  = 128
	thumbHeight = 72
	cardPad     = 12
)

// queueState is the queue page's own state.
type queueState struct {
	link        string
	presetLabel string
	notice      string
	list        ui.ListState
	jobs        []engine.JobView

	selectJob  string // playlist whose selection dialog is open
	selectOpen bool
	selectAll  int // 0: all videos, 1: some
	selectText string
	selectErr  string

	detailJob string // job shown in place of the list
	detailTab int
}

func (a *App) addLinks(start bool) {
	urls, skipped := formats.ParseURLs(a.q.link)
	if len(urls) == 0 {
		a.q.notice = "没有找到有效的链接，链接需要以 http:// 或 https:// 开头。"
		return
	}
	preset := formats.PresetByLabel(a.q.presetLabel).ID
	for _, u := range urls {
		a.queue.Add(u, engine.AddOptions{Preset: preset, Start: start})
	}
	a.q.link = ""
	a.q.notice = ""
	if skipped > 0 {
		a.q.notice = fmt.Sprintf("已添加 %d 个链接，忽略了 %d 项无法识别的内容。", len(urls), skipped)
	}
}

// AddDropped adds links dropped on the window or caught from the
// clipboard, starting them when the settings say so.
func (a *App) AddDropped(text string) int {
	urls, _ := formats.ParseURLFile(text)
	if a.queue == nil {
		return 0
	}
	start := a.settings().Input.AutoDownload
	preset := formats.PresetByLabel(a.q.presetLabel).ID
	for _, u := range urls {
		a.queue.Add(u, engine.AddOptions{Preset: preset, Start: start})
	}
	return len(urls)
}

// addExisting queues subtitles for a video downloaded before and shows the
// queue.
func (a *App) addExisting(path string) {
	if a.deps.AddExisting == nil {
		return
	}
	name := filepath.Base(path)
	a.q.notice = "正在读取“" + name + "”…"
	a.router.Replace("/queue")
	a.deps.AddExisting(path, func(err error) {
		if err != nil {
			a.q.notice = "无法为“" + name + "”补字幕：" + err.Error()
			return
		}
		a.q.notice = "已加入补字幕：" + name
	})
}

// droppedFiles are the local files in a dropped URI list.
func droppedFiles(uriList string) []string {
	var files []string
	for _, line := range strings.Split(uriList, "\n") {
		u, err := url.Parse(strings.TrimSpace(line))
		if err != nil || u.Scheme != "file" || u.Path == "" {
			continue
		}
		p := u.Path
		if len(p) > 2 && p[0] == '/' && p[2] == ':' { // /C:/x
			p = p[1:]
		}
		files = append(files, filepath.FromSlash(p))
	}
	return files
}

// SetLink fills the link input, as clipboard auto-fill does.
func (a *App) SetLink(link string) { a.q.link = link }

func (a *App) queuePage(c *ui.Context, p palette) {
	if a.queue == nil {
		a.page(c, p, "下载队列", nil, func() {
			ui.Text(c, "下载引擎未启动。").TextColor(p.muted)
		})
		return
	}
	a.q.jobs = a.queue.Jobs()
	if a.q.detailJob != "" {
		if job, ok := a.findJob(a.q.detailJob); ok {
			a.detailView(c, p, job)
			return
		}
		a.q.detailJob = ""
	}
	count := fmt.Sprintf("%d 个项目", len(a.q.jobs))
	root := ui.Column(c).Fill().AlignItems(ui.Stretch)
	opts := transfer.DropOptions{Formats: []transfer.Format{transfer.URIList, transfer.Text}, Operations: transfer.Copy}
	if _, over := ui.DataDragOver(root, opts); over {
		root.Border(2, c.Theme().Accent)
	}
	if drop, ok := ui.DropData(root, opts); ok {
		var videos []string
		if uris, err := drop.Data.Read(transfer.URIList); err == nil {
			for _, f := range droppedFiles(string(uris)) {
				if core.IsVideoFile(f) {
					videos = append(videos, f)
				}
			}
		}
		if len(videos) > 0 {
			for _, v := range videos {
				a.addExisting(v)
			}
		} else {
			text, err := drop.Data.Read(transfer.Text)
			if err != nil {
				text, _ = drop.Data.Read(transfer.URIList)
			}
			if n := a.AddDropped(string(text)); n == 0 {
				a.q.notice = "拖入的内容里没有链接。拖入下载过的视频文件可以补字幕。"
			}
		}
	}
	root.Children(func() {
		a.page(c, p, "下载队列", func() {
			ui.Text(c, count).FontSize(12).TextColor(p.muted)
		}, func() {
			a.linkBar(c, p)
			if a.q.notice != "" {
				ui.Text(c, a.q.notice).FontSize(12).TextColor(c.Theme().Warning)
			}
			if len(a.q.jobs) > 0 {
				a.queueToolbar(c, p)
			}
			a.jobList(c, p)
		})
	})
	a.selectionDialog(c, p)
}

func (a *App) findJob(id string) (engine.JobView, bool) {
	for _, j := range a.q.jobs {
		if j.ID == id {
			return j, true
		}
	}
	return engine.JobView{}, false
}

func (a *App) linkBar(c *ui.Context, p palette) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		input := ui.TextInput(c, &a.q.link).Placeholder("粘贴视频或播放列表链接，可一次粘贴多个").Label("链接").
			Height(controlHeight).Grow(1)
		if input.Submitted() {
			a.addLinks(true)
		}
		ui.Select(c, &a.q.presetLabel, formats.PresetLabels()).Width(150).Height(controlHeight).Label("画质")
		if ui.PrimaryButton(c, "").Height(controlHeight).Padding(0, 16).Label("下载").Children(func() {
			ui.Icon(c, iconDownload)
			ui.Text(c, "下载").SingleLine()
		}).Clicked() {
			a.addLinks(true)
		}
		if ui.Button(c, "加入队列").Height(controlHeight).Padding(0, 16).Clicked() {
			a.addLinks(false)
		}
	})
}

func (a *App) queueToolbar(c *ui.Context, p palette) {
	var startable, active, finished, failed int
	for _, j := range a.q.jobs {
		switch j.Status {
		case engine.StatusReady, engine.StatusPaused:
			startable++
		case engine.StatusQueued, engine.StatusDownloading:
			active++
		case engine.StatusDone, engine.StatusSkipped:
			finished++
		case engine.StatusError:
			failed++
		}
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, fmt.Sprintf("全部开始 (%d)", startable)).Disabled(startable == 0).Height(30).Clicked() {
			a.queue.StartAll()
		}
		if ui.Button(c, "全部暂停").Disabled(active == 0).Height(30).Clicked() {
			a.queue.PauseAll()
		}
		ui.Spacer(c)
		ui.MenuButton(c, "清理", func(m *ui.Menu) {
			if m.Item(fmt.Sprintf("清除已完成 (%d)", finished)).Disabled(finished == 0).Chosen() {
				a.queue.RemoveWhere(func(j engine.JobView) bool {
					return j.Status == engine.StatusDone || j.Status == engine.StatusSkipped
				})
			}
			if m.Item(fmt.Sprintf("清除出错的 (%d)", failed)).Disabled(failed == 0).Chosen() {
				a.queue.RemoveWhere(func(j engine.JobView) bool { return j.Status == engine.StatusError })
			}
			m.Separator()
			if m.Item("清除全部").Chosen() {
				a.queue.RemoveWhere(func(engine.JobView) bool { return true })
			}
		}).Height(30)
	})
}

func (a *App) jobList(c *ui.Context, p palette) {
	jobs := a.q.jobs
	a.q.list.Key = func(row int) any {
		if row < len(a.q.jobs) {
			return a.q.jobs[row].ID
		}
		return row
	}
	ui.List(c, &a.q.list, len(jobs), func(i int) {
		a.jobCard(c, p, jobs[i])
	}).Grow(1).Gap(10).Children(func() {
		if len(jobs) == 0 {
			ui.Column(c).Grow(1).Center().Gap(8).Padding(48, 0).Children(func() {
				ui.Icon(c, iconQueue).FontSize(32).TextColor(p.muted)
				ui.Text(c, "队列为空").FontSize(15).Bold()
				ui.Text(c, "粘贴链接后点“下载”，或把链接拖到窗口里。").FontSize(13).TextColor(p.muted)
			})
		}
	})
}

func (a *App) jobCard(c *ui.Context, p palette, j engine.JobView) {
	card := ui.Column(c).AlignItems(ui.Stretch).Padding(cardPad).Radius(10).Background(p.surface)
	card.Border(1, p.border).Children(func() {
		a.cardRow(c, p, j)
		if j.Custom != nil && j.Status == engine.StatusReady {
			a.customPanel(c, p, j)
		}
	})
}

func (a *App) cardRow(c *ui.Context, p palette, j engine.JobView) {
	ui.Row(c).Gap(cardPad).AlignItems(ui.Center).Children(func() {
		thumbnail(c, p, j.Thumbnail)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).AlignItems(ui.Stretch).Children(func() {
			title := ui.ButtonBase(c).Justify(ui.Start).Label("详情：" + j.Title).Children(func() {
				ui.Text(c, j.Title).FontSize(14).Bold().SingleLine()
			})
			if title.Clicked() {
				a.q.detailJob, a.q.detailTab = j.ID, 0
			}
			ui.Text(c, jobMeta(j)).FontSize(12).TextColor(p.muted).SingleLine()
			a.jobStatus(c, p, j)
		})
		a.jobActions(c, p, j)
	})
}

func thumbnail(c *ui.Context, p palette, src string) {
	if src == "" {
		ui.Column(c).Size(thumbWidth, thumbHeight).Radius(6).Background(p.control).Center().Children(func() {
			ui.Icon(c, iconFileVideo).FontSize(22).TextColor(p.muted)
		})
		return
	}
	netimage.Image(c, src, netimage.Options{MaxSide: 256}).Size(thumbWidth, thumbHeight).Radius(6).Background(p.control)
}

// jobMeta is the card's second line: playlist size, uploader, duration,
// quality and size.
func jobMeta(j engine.JobView) string {
	var parts []string
	if j.Combined || (j.Playlist != nil && len(j.Items) != 1) {
		n := len(j.Items)
		if n == 0 && j.Playlist != nil {
			n = len(j.Playlist.Entries)
		}
		parts = append(parts, fmt.Sprintf("播放列表 · %d 个视频", n))
	} else if j.Playlist != nil && len(j.Items) == 1 && j.Items[0].PlaylistIndex >= 0 {
		parts = append(parts, fmt.Sprintf("%s 第 %d 个", j.Playlist.Title, j.Items[0].PlaylistIndex+1))
	}
	if j.Uploader != "" {
		parts = append(parts, j.Uploader)
	}
	if d := formatDuration(j.Duration); d != "" {
		parts = append(parts, d)
	}
	if len(j.Items) == 1 && j.Items[0].Choice != "" {
		parts = append(parts, j.Items[0].Choice)
	}
	if j.Size > 0 {
		parts = append(parts, "约 "+formatBytes(j.Size))
	}
	if len(parts) == 0 {
		if u, err := url.Parse(j.URL); err == nil && u.Host != "" {
			return u.Host // the title shows the whole link
		}
		return ""
	}
	return strings.Join(parts, " · ")
}

func (a *App) jobStatus(c *ui.Context, p palette, j engine.JobView) {
	t := c.Theme()
	line := func(text string, color ui.Color) {
		ui.Text(c, text).FontSize(12).TextColor(color).SingleLine()
	}
	switch j.Status {
	case engine.StatusFetching:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Size(14, 14).Label("正在获取信息")
			line("正在获取信息…", p.muted)
		})
	case engine.StatusFetchingList:
		got := 0
		for _, it := range j.Items {
			if it.Status != engine.StatusFetching {
				got++
			}
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Progress(c, float64(got)/float64(max(len(j.Items), 1))).Width(160).Label("获取视频信息")
			line(fmt.Sprintf("正在获取视频信息 %d/%d", got, len(j.Items)), p.muted)
		})
	case engine.StatusSelecting:
		line("选择要下载的视频，或全部下载", p.muted)
	case engine.StatusQueued, engine.StatusDownloading, engine.StatusPaused:
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			bar := ui.Progress(c, j.Percent/100).Grow(1).Label("下载进度")
			if j.Status == engine.StatusPaused {
				bar.Opacity(0.5)
			}
			line(progressText(j), p.muted)
		})
	case engine.StatusDone:
		text := "已完成"
		if j.Combined {
			text = fmt.Sprintf("已完成 %d 个视频", j.Done)
			if j.Failed > 0 {
				text += fmt.Sprintf("，%d 个出错", j.Failed)
			}
		}
		line(text, t.Success)
	case engine.StatusSkipped:
		line("已跳过：不符合输入筛选条件", p.muted)
	case engine.StatusError:
		title, detail := jobError(j)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconAlert).FontSize(13).TextColor(t.Danger)
			line(title+"："+detail, t.Danger)
		})
	default:
		line("点击“下载”开始", p.muted)
	}
}

func jobError(j engine.JobView) (string, string) {
	for _, it := range j.Items {
		if it.Status == engine.StatusError {
			title, detail := errorText(it, j.Err)
			if j.Combined && j.Failed > 1 {
				title = fmt.Sprintf("%d 个视频出错", j.Failed)
			}
			return title, detail
		}
	}
	return "出错", j.Err
}

// progressText is "45% · 3.2 MB/s · 剩余 1:23", or the stage when it says
// more, or the item count of a playlist card.
func progressText(j engine.JobView) string {
	if j.Status == engine.StatusQueued {
		return "等待中"
	}
	parts := []string{fmt.Sprintf("%.0f%%", j.Percent)}
	if j.Status == engine.StatusPaused {
		return parts[0] + " · 已暂停"
	}
	var speed float64
	eta := int64(-1)
	var stage string
	for _, it := range j.Items {
		if it.Status == engine.StatusDownloading {
			speed += it.SpeedBps
			if it.EtaSecs > eta {
				eta = it.EtaSecs
			}
			if it.Stage != "" && stage == "" {
				stage = stageLabels[it.Stage]
			}
		}
	}
	if j.Combined {
		parts = append(parts, fmt.Sprintf("%d/%d", j.Done, len(j.Items)))
	}
	if stage != "" && stage != stageLabels["downloading"] && stage != stageLabels["initializing"] {
		return strings.Join(append(parts, stage), " · ")
	}
	if s := formatSpeed(speed); s != "" {
		parts = append(parts, s)
	}
	if e := formatEta(eta); e != "" && !j.Combined {
		parts = append(parts, e)
	}
	return strings.Join(parts, " · ")
}

func iconButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	return ui.Button(c, "").Size(32, 32).Padding(0).Label(label).Tooltip(label).Children(func() {
		ui.Icon(c, icon).FontSize(16)
	})
}

func (a *App) jobActions(c *ui.Context, p palette, j engine.JobView) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		switch j.Status {
		case engine.StatusReady:
			if j.Existing != "" {
				break
			}
			preset := formats.PresetByID(j.Preset).Label
			if j.Custom != nil {
				preset = formats.PresetByID(formats.PresetCustom).Label
			}
			if ui.Select(c, &preset, formats.PresetLabels()).Width(130).Height(32).Label("画质").Changed() {
				if chosen := formats.PresetByLabel(preset); chosen.Custom {
					a.queue.SetCustom(j.ID, customStart(j))
				} else {
					a.queue.SetPreset(j.ID, chosen.ID)
				}
			}
			if ui.PrimaryButton(c, "").Size(32, 32).Padding(0).Label("开始下载").Tooltip("开始下载").Children(func() {
				ui.Icon(c, iconPlay).FontSize(16)
			}).Clicked() {
				a.queue.Start(j.ID)
			}
		case engine.StatusSelecting:
			if ui.Button(c, "选择视频…").Height(32).Clicked() {
				a.openSelection(j.ID)
			}
			if ui.PrimaryButton(c, "全部下载").Height(32).Clicked() {
				_ = a.queue.SelectPlaylist(j.ID, nil, true)
			}
		case engine.StatusQueued, engine.StatusDownloading:
			if iconButton(c, iconPause, "暂停").Clicked() {
				a.queue.Pause(j.ID)
			}
		case engine.StatusPaused:
			if iconButton(c, iconPlay, "继续").Clicked() {
				a.queue.Start(j.ID)
			}
		case engine.StatusError:
			if iconButton(c, iconRetry, "重试").Clicked() {
				a.queue.Retry(j.ID)
			}
		case engine.StatusDone:
			if dest := firstDestination(j); dest != "" && a.deps.ShowInFolder != nil {
				if iconButton(c, iconFolderOpen, "在文件夹中显示").Clicked() {
					a.deps.ShowInFolder(dest)
				}
			}
		}
		if iconButton(c, iconClose, "移除").Clicked() {
			a.queue.Remove(j.ID)
		}
	})
}

func firstDestination(j engine.JobView) string {
	for _, it := range j.Items {
		if it.Destination != "" {
			return it.Destination
		}
	}
	return ""
}

func (a *App) openSelection(jobID string) {
	a.q.selectJob, a.q.selectOpen, a.q.selectAll, a.q.selectText, a.q.selectErr = jobID, true, 0, "", ""
}

// selectionDialog chooses the videos of a playlist: all, or positions and
// ranges typed as "1-10, 15, -3:-1".
func (a *App) selectionDialog(c *ui.Context, p palette) {
	if !a.q.selectOpen {
		return
	}
	j, ok := a.findJob(a.q.selectJob)
	if !ok || j.Status != engine.StatusSelecting {
		a.q.selectOpen = false
		return
	}
	total := len(j.Playlist.Entries)
	ui.Modal(c, &a.q.selectOpen, func() {
		ui.Column(c).Width(440).Gap(sectionGap).Padding(20).AlignItems(ui.Stretch).Children(func() {
			ui.Text(c, "选择要下载的视频").FontSize(16).Bold()
			ui.Text(c, fmt.Sprintf("“%s”共有 %d 个视频。", j.Title, total)).FontSize(13).TextColor(p.muted).Wrap()
			ui.Segmented(c, &a.q.selectAll, "全部视频", "部分视频")
			if a.q.selectAll == 1 {
				ui.TextInput(c, &a.q.selectText).Placeholder("例如 1-10, 15, -3:-1").Label("视频序号").Height(controlHeight)
				ui.Text(c, "用逗号分隔序号或范围，负数从末尾数起，-1 是最后一个。").FontSize(12).TextColor(p.muted).Wrap()
			}
			if a.q.selectErr != "" {
				ui.Text(c, a.q.selectErr).FontSize(12).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Height(controlHeight).Padding(0, 16).Clicked() {
					a.q.selectOpen = false
				}
				confirm := func(start bool) {
					var rows []formats.SelectionRow
					if a.q.selectAll == 1 {
						var err error
						if rows, err = formats.ParseSelection(a.q.selectText); err != nil {
							a.q.selectErr = err.Error()
							return
						}
						if len(rows) == 0 {
							a.q.selectErr = "请输入要下载的视频序号。"
							return
						}
					}
					if err := a.queue.SelectPlaylist(j.ID, rows, start); err != nil {
						a.q.selectErr = err.Error()
						return
					}
					a.q.selectOpen = false
				}
				if ui.Button(c, "加入队列").Height(controlHeight).Padding(0, 16).Clicked() {
					confirm(false)
				}
				if ui.PrimaryButton(c, "下载").Height(controlHeight).Padding(0, 16).Clicked() {
					confirm(true)
				}
			})
		})
	})
}
