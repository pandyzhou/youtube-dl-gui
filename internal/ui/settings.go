package ui

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/lumadl/internal/config"
	"github.com/pandyzhou/lumadl/internal/formats"
	"github.com/pandyzhou/lumadl/internal/secrets"
)

// Settings sections, in the order of the section list.
var settingSections = []string{"常用", "下载与文件", "字幕与翻译", "登录与网络", "筛选", "系统"}

// settingsState is the settings page's state: a draft of the settings,
// saved as soon as it differs from what was last saved.
type settingsState struct {
	loaded  bool
	draft   config.Settings
	saved   config.Settings
	prefs   config.Preferences
	savedP  config.Preferences
	secrets map[string]string
	query   string
	section int
	err     string

	// The translation API's models, once fetched.
	models     []string
	modelsBusy bool
	modelsErr  string

	// The model test's result.
	testBusy   bool
	testResult string
	testErr    string
}

// settingRow is one setting: where it is, what it is called and explained
// as (also what search looks through), and its control.
type settingRow struct {
	section int
	label   string
	hint    string
	keys    string
	control func()
	wide    bool // the control goes under the label
}

func (a *App) loadSettings() {
	a.s.draft = cloneSettings(a.settings()) // empty lists become nil on both sides
	a.s.saved = cloneSettings(a.s.draft)
	if a.deps.Preferences != nil {
		a.s.prefs = a.deps.Preferences()
	}
	a.s.savedP = a.s.prefs
	a.s.secrets = map[string]string{}
	if a.deps.GetSecret != nil {
		for _, k := range append(append([]string(nil), secrets.AuthKeys...), secrets.TranslationAPIKey) {
			a.s.secrets[k] = a.deps.GetSecret(k)
		}
	}
	a.s.loaded, a.s.err = true, ""
}

// cloneSettings copies the slices, so the draft and the saved copy do not
// share them.
func cloneSettings(s config.Settings) config.Settings {
	s.Subtitles.Languages = append([]string(nil), s.Subtitles.Languages...)
	s.Subtitles.FormatPreference = append([]string(nil), s.Subtitles.FormatPreference...)
	s.Notifications.DisabledNotifications = append([]string(nil), s.Notifications.DisabledNotifications...)
	return s
}

// saveSettings writes the draft when a control changed it.
func (a *App) saveSettings() {
	if !reflect.DeepEqual(a.s.draft, a.s.saved) && a.deps.UpdateSettings != nil {
		draft := cloneSettings(a.s.draft)
		if err := a.deps.UpdateSettings(func(s *config.Settings) { *s = draft }); err != nil {
			a.s.err = "保存设置失败：" + err.Error()
		} else {
			a.s.err = ""
		}
		a.s.saved = draft
	}
	if a.s.prefs.Paths != a.s.savedP.Paths && a.deps.UpdatePreferences != nil {
		paths := a.s.prefs.Paths
		if err := a.deps.UpdatePreferences(func(p *config.Preferences) { p.Paths = paths }); err != nil {
			a.s.err = "保存设置失败：" + err.Error()
		}
		a.s.savedP = a.s.prefs
	}
}

func (a *App) settingsPage(c *ui.Context, p palette) {
	if !a.s.loaded {
		a.loadSettings()
	}
	rows := a.settingRows(c, p)
	query := strings.ToLower(strings.TrimSpace(a.s.query))
	a.page(c, p, "设置", func() {
		ui.SearchField(c, &a.s.query).Placeholder("搜索设置").Width(240).Height(32)
	}, func() {
		if a.s.err != "" {
			ui.Text(c, a.s.err).FontSize(12).TextColor(c.Theme().Danger)
		}
		ui.Row(c).Grow(1).Gap(sectionGap).AlignItems(ui.Stretch).Children(func() {
			if query == "" {
				ui.Column(c).Width(132).Gap(2).AlignItems(ui.Stretch).Children(func() {
					for i, name := range settingSections {
						c.Key("section-" + name)
						row := ui.ButtonBase(c).Height(32).Justify(ui.Start).Padding(0, 10).Radius(6).Label(name)
						if i == a.s.section {
							row.Background(p.selected)
						} else if row.Hovered() {
							row.Background(c.Theme().SurfaceHover)
						}
						row.Children(func() {
							t := ui.Text(c, name).FontSize(13)
							if i == a.s.section {
								t.Bold()
							}
						})
						if row.Clicked() {
							a.s.section = i
						}
					}
				})
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(sectionGap).AlignItems(ui.Stretch).Padding(0, 4, 24, 0).Children(func() {
					a.settingCards(c, p, rows, query)
				})
			})
		})
	})
	a.saveSettings()
}

func (a *App) settingCards(c *ui.Context, p palette, rows []settingRow, query string) {
	shown := 0
	for section, name := range settingSections {
		if query == "" && section != a.s.section {
			continue
		}
		var matched []settingRow
		for _, r := range rows {
			if r.section != section {
				continue
			}
			if query == "" || strings.Contains(strings.ToLower(r.label+" "+r.hint+" "+r.keys), query) {
				matched = append(matched, r)
			}
		}
		if len(matched) == 0 {
			continue
		}
		shown += len(matched)
		c.Key("card-" + name)
		if query != "" {
			ui.Text(c, name).FontSize(12).Bold().TextColor(p.muted)
		}
		ui.Column(c).AlignItems(ui.Stretch).Radius(10).Background(p.surface).Border(1, p.border).Children(func() {
			for i, r := range matched {
				c.Key(r.label)
				if i > 0 {
					ui.Box(c).Height(1).Background(p.border).Margin(0, 16)
				}
				settingRowView(c, p, r)
			}
		})
	}
	if shown == 0 {
		ui.Text(c, "没有找到“"+a.s.query+"”相关的设置。").FontSize(13).TextColor(p.muted).Padding(24, 0)
	}
}

func settingRowView(c *ui.Context, p palette, r settingRow) {
	label := func() {
		ui.Column(c).Gap(3).Grow(1).Shrink(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			ui.Text(c, r.label).FontSize(13)
			if r.hint != "" {
				ui.Text(c, r.hint).FontSize(12).TextColor(p.muted).Wrap()
			}
		})
	}
	if r.wide {
		ui.Column(c).Gap(10).Padding(12, 16).AlignItems(ui.Stretch).Children(func() {
			label()
			r.control()
		})
		return
	}
	ui.Row(c).Gap(16).Padding(12, 16).AlignItems(ui.Center).MinHeight(56).Children(func() {
		label()
		r.control()
	})
}

// Controls bound to a draft field.

func toggle(c *ui.Context, label string, on *bool) func() {
	return func() { ui.Switch(c, on).Label(label) }
}

// choice is a select over labels standing for values.
func choice(c *ui.Context, label string, value *string, values, labels []string, width float32) func() {
	return func() {
		current := labels[0]
		for i, v := range values {
			if v == *value {
				current = labels[i]
			}
		}
		if ui.Select(c, &current, labels).Width(width).Height(32).Label(label).Changed() {
			for i, l := range labels {
				if l == current {
					*value = values[i]
				}
			}
		}
	}
}

func segmented(c *ui.Context, value *string, values, labels []string) func() {
	return func() {
		selected := 0
		for i, v := range values {
			if v == *value {
				selected = i
			}
		}
		if ui.Segmented(c, &selected, labels...).Changed() {
			*value = values[selected]
		}
	}
}

func textField(c *ui.Context, label string, value *string, placeholder string, width float32) func() {
	return func() {
		in := ui.TextInput(c, value).Label(label).Placeholder(placeholder).Height(32)
		if width > 0 {
			in.Width(width)
		}
	}
}

func number(c *ui.Context, label string, value *int, lo, hi int) func() {
	return func() {
		v := float64(*value)
		if ui.NumberInput(c, &v, float64(lo), float64(hi), 1).Width(120).Height(32).Label(label).Changed() {
			*value = int(v)
		}
	}
}

// csvField edits a list as "a, b, c".
func csvField(c *ui.Context, label string, list *[]string, placeholder string) func() {
	return func() {
		text := strings.Join(*list, ", ")
		if ui.TextInput(c, &text).Label(label).Placeholder(placeholder).Width(260).Height(32).Changed() {
			var out []string
			for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == '，' || r == ' ' }) {
				out = append(out, part)
			}
			*list = out
		}
	}
}

// pathField shows a folder or file with a button to choose another.
func (a *App) pathField(c *ui.Context, label string, value *string, folder bool, placeholder string) func() {
	return func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, value).Label(label).Placeholder(placeholder).Width(300).Height(32)
			if a.deps.PickPath != nil && ui.Button(c, "选择…").Height(32).Clicked() {
				target := value
				a.deps.PickPath(label, *value, folder, func(path string) {
					if path != "" {
						*target = path
						a.saveSettings()
					}
				})
			}
		})
	}
}

// existingButton picks a downloaded video to make subtitles for.
func (a *App) existingButton(c *ui.Context) func() {
	return func() {
		if a.deps.PickPath != nil && a.deps.AddExisting != nil && ui.Button(c, "选择视频…").Height(32).Clicked() {
			a.deps.PickPath("选择要补字幕的视频", "", false, func(path string) {
				if path != "" {
					a.addExisting(path)
				}
			})
		}
	}
}

// modelField edits the translation model, with a button that fetches the
// API's models into a list to choose from.
func (a *App) modelField(c *ui.Context, p palette) func() {
	return func() {
		model := &a.s.draft.Translation.Model
		ui.Column(c).Gap(6).AlignItems(ui.Start).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.TextInput(c, model).Label("模型").Placeholder("gpt-4o-mini").Width(220).Height(32)
				label := "获取模型"
				if a.s.modelsBusy {
					label = "正在获取…"
				}
				if a.deps.ListModels != nil && ui.Button(c, label).Height(32).Disabled(a.s.modelsBusy).Clicked() {
					a.s.modelsBusy, a.s.modelsErr = true, ""
					a.deps.ListModels(a.s.draft.Translation.BaseURL, a.s.secrets[secrets.TranslationAPIKey], func(models []string, err error) {
						a.s.modelsBusy = false
						if err != nil {
							a.s.models, a.s.modelsErr = nil, "获取模型失败："+err.Error()
							return
						}
						a.s.models = models
					})
				}
				if len(a.s.models) > 0 {
					labels := append([]string{"选择模型…"}, a.s.models...)
					current := labels[0]
					if slices.Contains(a.s.models, *model) {
						current = *model
					}
					if ui.Select(c, &current, labels).Width(260).Height(32).Label("可用模型").Changed() && current != labels[0] {
						*model = current
					}
				}
				test := "测试"
				if a.s.testBusy {
					test = "正在测试…"
				}
				if a.deps.TestModel != nil && ui.Button(c, test).Height(32).Disabled(a.s.testBusy).Clicked() {
					a.s.testBusy, a.s.testResult, a.s.testErr = true, "", ""
					a.deps.TestModel(a.s.draft.Translation, a.s.secrets[secrets.TranslationAPIKey], func(result string, elapsed time.Duration, err error) {
						a.s.testBusy = false
						if err != nil {
							a.s.testErr = "测试失败：" + err.Error()
							return
						}
						a.s.testResult = fmt.Sprintf("测试成功，用时 %.1f 秒：%s", elapsed.Seconds(), result)
					})
				}
			})
			switch {
			case a.s.modelsErr != "":
				ui.Text(c, a.s.modelsErr).FontSize(12).TextColor(c.Theme().Danger)
			case len(a.s.models) > 0:
				ui.Text(c, "接口提供 "+strconv.Itoa(len(a.s.models))+" 个模型。").FontSize(12).TextColor(p.muted)
			}
			switch {
			case a.s.testErr != "":
				ui.Text(c, a.s.testErr).FontSize(12).TextColor(c.Theme().Danger)
			case a.s.testResult != "":
				ui.Text(c, a.s.testResult).FontSize(12).TextColor(p.muted)
			}
		})
	}
}

// secretField edits a secret, saved to the system's credential store.
func (a *App) secretField(c *ui.Context, label, key string, multiline bool, placeholder string) func() {
	return func() {
		value := a.s.secrets[key]
		var in ui.Element
		if multiline {
			in = ui.TextArea(c, &value).Label(label).Placeholder(placeholder).Lines(3, 6)
		} else {
			in = ui.TextInput(c, &value).Label(label).Placeholder(placeholder).Password().Width(260).Height(32)
		}
		if in.Changed() {
			a.s.secrets[key] = value
			if a.deps.SetSecret != nil {
				if err := a.deps.SetSecret(key, value); err != nil {
					a.s.err = "保存“" + label + "”失败：" + err.Error()
				}
			}
		}
	}
}

func (a *App) settingRows(c *ui.Context, p palette) []settingRow {
	s := &a.s.draft
	paths := &a.s.prefs.Paths
	var presetIDs, presetLabels []string
	for _, pr := range formats.Presets {
		if !pr.Custom {
			presetIDs, presetLabels = append(presetIDs, pr.ID), append(presetLabels, pr.Label)
		}
	}
	const (
		general = iota
		files
		subtitles
		network
		filters
		system
	)
	rows := []settingRow{
		{general, "下载位置", "下载的视频和音频保存到这里。", "文件夹 目录 路径", a.pathField(c, "下载位置", &s.Output.DownloadDir, true, ""), true},
		{general, "默认画质", "新加入的链接使用的画质，可在每张卡片上单独修改。", "清晰度 分辨率 预设", choice(c, "默认画质", &s.Input.DefaultPreset, presetIDs, presetLabels, 160), false},
		{general, "加入后自动开始下载", "拖入、剪贴板和快捷键加入的链接获取信息后立即下载。", "自动", toggle(c, "加入后自动开始下载", &s.Input.AutoDownload), false},
		{general, "自动填入复制的链接", "复制视频链接后自动填到链接输入框。", "剪贴板", toggle(c, "自动填入复制的链接", &s.Input.AutoFillClipboard), false},
		{general, "下载字幕", "按字幕设置下载字幕。", "字幕", toggle(c, "下载字幕", &s.Subtitles.Enabled), false},
		{general, "AI 双语字幕", "下载后用 AI 把字幕翻译成双语，需在“字幕与翻译”里填写接口。", "翻译 双语", toggle(c, "AI 双语字幕", &s.Translation.Enabled), false},
		{general, "外观", "", "主题 深色 浅色 暗色", segmented(c, &s.Appearance.Theme, []string{"system", "light", "dark"}, []string{"跟随系统", "浅色", "深色"}), false},

		{files, "视频格式", "合并后的视频文件格式。", "容器 mp4 mkv", segmented(c, &s.Output.Video.Container, []string{"mp4", "mkv"}, []string{"MP4", "MKV"}), false},
		{files, "保留原始视频流", "不转换格式，更快，但文件可能不是所选格式。", "转码 封装 policy", a.policyToggle(c, "保留原始视频流", &s.Output.Video.Policy), false},
		{files, "视频后处理", "", "ffmpeg 30帧 mp42 后处理", choice(c, "视频后处理", &s.Output.Video.PostprocessPreset, []string{"none", "fps30", "mp42", "custom"}, []string{"无", "转为 30 帧", "MP4 兼容 (mp42)", "自定义 ffmpeg 参数"}, 180), false},
	}
	if s.Output.Video.PostprocessPreset == "custom" {
		rows = append(rows,
			settingRow{files, "自定义视频处理方式", "", "ffmpeg 封装 重新编码", segmented(c, &s.Output.Video.CustomPostprocessMode, []string{"remux", "reencode"}, []string{"只封装", "重新编码"}), false},
			settingRow{files, "自定义 ffmpeg 参数（视频）", "例如 -movflags +faststart", "ffmpeg 参数", textField(c, "自定义 ffmpeg 参数（视频）", &s.Output.Video.PostprocessArgs, "", 260), false},
		)
	}
	rows = append(rows, []settingRow{
		{files, "音频格式", "“仅音频”下载转换成的格式。", "mp3 m4a opus flac", choice(c, "音频格式", &s.Output.Audio.Format, []string{"mp3", "m4a", "opus", "aac", "ogg", "flac", "wav"}, []string{"MP3", "M4A", "Opus", "AAC", "OGG", "FLAC", "WAV"}, 120), false},
		{files, "保留原始音频流", "不转换音频格式。", "转码 policy", a.policyToggle(c, "保留原始音频流", &s.Output.Audio.Policy), false},
		{files, "写入元数据", "把标题、作者等信息写入文件。", "metadata 标签", toggle(c, "写入元数据", &s.Output.AddMetadata), false},
		{files, "嵌入封面", "把视频封面写入文件。", "缩略图 thumbnail", toggle(c, "嵌入封面", &s.Output.AddThumbnail), false},
		{files, "另存封面图片", "在视频旁边保存一份封面图片。", "缩略图 thumbnail", toggle(c, "另存封面图片", &s.Output.SaveThumbnail), false},
		{files, "视频文件名", "yt-dlp 输出模板，如 %(title)s.%(ext)s。", "模板 文件名 template", textField(c, "视频文件名", &s.Output.FileNameTemplate, config.DefaultFileNameTemplate, 0), true},
		{files, "音频文件名", "", "模板 文件名 template", textField(c, "音频文件名", &s.Output.AudioFileNameTemplate, config.DefaultAudioFileNameTemplate, 0), true},
		{files, "文件名只用英文字符", "去掉文件名里的中文、空格和特殊字符。", "ascii restrict", toggle(c, "文件名只用英文字符", &s.Output.RestrictFilenames), false},
		{files, "播放列表倒序编号", "让最早的视频编号为 1。", "序号 playlist_index", toggle(c, "播放列表倒序编号", &s.Output.ReversePlaylistNumbering), false},
		{files, "视频单独保存到", "留空则使用下载位置。", "文件夹 目录", a.pathField(c, "视频单独保存到", &paths.VideoDownloadDir, true, "使用下载位置"), true},
		{files, "音频单独保存到", "留空则使用下载位置。", "文件夹 目录", a.pathField(c, "音频单独保存到", &paths.AudioDownloadDir, true, "使用下载位置"), true},
		{files, "视频子文件夹", "例如 %(uploader)s，按作者分文件夹。", "模板 文件夹", textField(c, "视频子文件夹", &paths.VideoDirectoryTemplate, "不使用子文件夹", 260), false},
		{files, "音频子文件夹", "", "模板 文件夹", textField(c, "音频子文件夹", &paths.AudioDirectoryTemplate, "不使用子文件夹", 260), false},
		{files, "链接同时含视频和播放列表时", "例如在播放列表或合辑里打开的视频。快捷键和托盘加入的链接在“每次询问”时只下载视频。", "播放列表 合辑 mix mixed 列表", choice(c, "链接同时含视频和播放列表时", &s.Input.MixedLinks, []string{"ask", "video", "list"}, []string{"每次询问", "只下载这个视频", "下载整个列表"}, 160), false},
		{files, "同时下载数量", "同时获取信息和下载的项目数。", "并发 速度", number(c, "同时下载数量", &s.Performance.MaxConcurrency, 1, 10), false},
		{files, "分片并发数", "分片视频（HLS/DASH）同时下载的分片数，1 为不并发。", "并发 速度 fragments", number(c, "分片并发数", &s.Performance.ConcurrentFragments, 1, 16), false},
		{files, "播放列表拆分", "视频数少于这个数的播放列表拆成单独的卡片。", "合并 拆分", number(c, "播放列表拆分", &s.Performance.SplitPlaylistThreshold, 1, 500), false},

		{subtitles, "下载字幕", "", "字幕", toggle(c, "下载字幕 ", &s.Subtitles.Enabled), false},
		{subtitles, "字幕语言", "语言代码，逗号分隔，如 zh-Hans, en；all 为全部。", "语言", csvField(c, "字幕语言", &s.Subtitles.Languages, "en"), false},
		{subtitles, "包含自动生成的字幕", "没有人工字幕时使用自动生成的字幕。", "自动字幕", toggle(c, "包含自动生成的字幕", &s.Subtitles.IncludeAutoGenerated), false},
		{subtitles, "把字幕嵌入视频", "关闭则保存为单独的字幕文件。", "内嵌", toggle(c, "把字幕嵌入视频", &s.Subtitles.EmbedSubtitles), false},
		{subtitles, "字幕格式优先级", "按顺序选第一个可用的格式。", "srt vtt ass", csvField(c, "字幕格式优先级", &s.Subtitles.FormatPreference, "srt, vtt"), false},
		{subtitles, "AI 双语字幕", "", "翻译 双语", toggle(c, "AI 双语字幕 ", &s.Translation.Enabled), false},
		{subtitles, "接口地址", "OpenAI 兼容接口的地址。", "翻译 api url", textField(c, "接口地址", &s.Translation.BaseURL, "https://api.openai.com/v1", 300), false},
		{subtitles, "模型", "可以手动填写，或从接口获取可用的模型；“测试”会用当前设置翻译一句话。", "翻译 model 模型 获取", a.modelField(c, p), true},
		{subtitles, "双语字幕放在哪里", "封装进视频：不重新编码，但要把视频复制一遍；MKV 带描边样式，MP4 只能放纯文本字幕。单独文件：在视频旁边保存 .ass 和 .srt。", "字幕 封装 内嵌 嵌入 ass srt 文件", segmented(c, &s.Translation.Output, []string{"embed", "files"}, []string{"封装进视频", "单独的字幕文件"}), false},
		{subtitles, "同时翻译的请求数", "模型较慢时调大可以加快长视频的翻译；接口限制并发时调小。", "翻译 并发 速度 请求", number(c, "同时翻译的请求数", &s.Translation.Concurrency, 1, 32), false},
		{subtitles, "思考强度", "推理模型思考得越深，翻译越慢、费用越高。模型不支持时请选“默认”。", "翻译 思考 推理 reasoning effort", choice(c, "思考强度", &s.Translation.ReasoningEffort, []string{"", "none", "minimal", "low", "medium", "high"}, []string{"默认", "关闭思考", "最低", "低", "中", "高"}, 150), false},
		{subtitles, "目标语言", "翻译成的语言。", "翻译 language", textField(c, "目标语言", &s.Translation.TargetLanguage, "简体中文", 200), false},
		{subtitles, "高质量模式", "翻译前先让 AI 通读全部字幕，整理出主题和术语表，人名和专有名词更准、前后一致。每个视频多一次请求。", "翻译 高质量 术语 质量", toggle(c, "高质量模式", &s.Translation.HighQuality), false},
		{subtitles, "为已有视频补字幕", "选择下载过的视频，按上面的设置生成双语字幕，保存在视频旁边；也可以把视频文件拖进下载队列。视频里要记录了原链接（下载时写入元数据即可）。", "补字幕 翻译 已有 本地 视频 文件", a.existingButton(c), false},
		{subtitles, "API Key", "保存在系统凭据中。", "翻译 密钥 key", a.secretField(c, "API Key", secrets.TranslationAPIKey, false, "sk-…"), false},

		{network, "从浏览器读取 Cookie", "用浏览器里的登录状态下载需要登录的视频。", "cookie 登录 浏览器", choice(c, "从浏览器读取 Cookie", &s.Auth.CookieBrowser, []string{"none", "chrome", "edge", "firefox", "brave", "opera", "vivaldi", "chromium"}, []string{"不使用", "Chrome", "Edge", "Firefox", "Brave", "Opera", "Vivaldi", "Chromium"}, 150), false},
		{network, "Cookie 文件", "Netscape 格式的 cookies.txt。", "cookie 登录", a.pathField(c, "Cookie 文件", &s.Auth.CookieFile, false, "不使用"), true},
		{network, "用户名", "", "账号 登录", a.secretField(c, "用户名", secrets.Username, false, ""), false},
		{network, "密码", "", "登录", a.secretField(c, "密码", secrets.Password, false, ""), false},
		{network, "视频密码", "有密码保护的视频。", "vimeo", a.secretField(c, "视频密码", secrets.VideoPassword, false, ""), false},
		{network, "Bearer 令牌", "", "token 登录", a.secretField(c, "Bearer 令牌", secrets.BearerToken, false, ""), false},
		{network, "自定义请求头", "每行一个，如 Referer: https://example.com", "header", a.secretField(c, "自定义请求头", secrets.Headers, true, ""), true},
		{network, "使用代理", "", "proxy 翻墙", toggle(c, "使用代理", &s.Network.EnableProxy), false},
		{network, "代理地址", "如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080", "proxy", textField(c, "代理地址", &s.Network.Proxy, "http://127.0.0.1:7890", 260), false},
		{network, "浏览器伪装", "部分网站需要伪装成浏览器访问。", "impersonate", choice(c, "浏览器伪装", &s.Network.Impersonate, []string{"none", "any", "chrome", "edge", "safari", "firefox"}, []string{"不伪装", "自动", "Chrome", "Edge", "Safari", "Firefox"}, 140), false},
		{network, "提取器参数", "每行一个，如 youtube:player_client=web", "extractor args", func() {
			ui.TextArea(c, &s.Network.ExtractorArgs).Label("提取器参数").Lines(2, 6)
		}, true},

		{filters, "最小文件大小", "小于这个大小的视频跳过。", "大小 筛选", a.sizeField(c, "最小文件大小", &s.InputFilters.MinSize), false},
		{filters, "最大文件大小", "大于这个大小的视频跳过。", "大小 筛选", a.sizeField(c, "最大文件大小", &s.InputFilters.MaxSize), false},
		{filters, "上传日期", "只下载这个日期（之前/之后）上传的视频。", "日期 筛选", a.dateField(c, &s.InputFilters.DateFilter), true},
		{filters, "匹配条件", "yt-dlp --match-filters，如 !is_live & duration < 600", "match filter", textField(c, "匹配条件", &s.InputFilters.MatchFilters, "", 0), true},
		{filters, "遇到即停止", "yt-dlp --break-match-filters：遇到不符合的视频就停止。", "break filter", textField(c, "遇到即停止", &s.InputFilters.BreakMatchFilters, "", 0), true},

		{system, "关闭窗口时", "", "托盘 退出", segmented(c, &s.System.CloseBehavior, []string{config.CloseExit, config.CloseHide}, []string{"退出程序", "最小化到托盘"}), false},
		{system, "显示托盘图标", "", "托盘", toggle(c, "显示托盘图标", &s.System.TrayEnabled), false},
		{system, "开机启动", "", "自启", toggle(c, "开机启动", &s.System.AutoStartEnabled), false},
		{system, "启动时最小化", "开机启动时不显示窗口。", "自启 托盘", toggle(c, "启动时最小化", &s.System.AutoStartMinimised), false},
		{system, "全局快捷键", "Alt+Shift+V 加入复制的链接，Alt+Shift+D 加入并下载。", "快捷键 热键", toggle(c, "全局快捷键", &s.Input.GlobalShortcuts), false},
		{system, "通知", "", "提醒", segmented(c, &s.Notifications.NotificationBehavior, []string{config.NotifyAlways, config.NotifyOnBackground, config.NotifyNever}, []string{"总是", "窗口在后台时", "从不"}), false},
		{system, "通知哪些事", "取消勾选的事不再通知。", "提醒 通知 完成 失败", a.noticeKinds(c, &s.Notifications), true},
		{system, "自动更新下载工具", "启动时更新 yt-dlp 等工具。", "更新 yt-dlp", toggle(c, "自动更新下载工具", &s.Update.UpdateBinaries), false},
		{system, "自动检查应用更新", "", "更新 升级", toggle(c, "自动检查应用更新", &s.Update.UpdateApp), false},
	}...)
	return rows
}

// policyToggle shows "keep original streams" for a transcode policy.
func (a *App) policyToggle(c *ui.Context, label string, policy *string) func() {
	return func() {
		keep := *policy == config.PolicyNever
		if ui.Switch(c, &keep).Label(label).Changed() {
			if keep {
				*policy = config.PolicyNever
			} else {
				*policy = config.PolicyAllowReencode
			}
		}
	}
}

// sizeField edits a size filter: empty or 0 is off.
func (a *App) sizeField(c *ui.Context, label string, f *config.SizeFilter) func() {
	return func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			text := ""
			if f.Value != nil {
				text = strconv.FormatFloat(*f.Value, 'f', -1, 64)
			}
			if ui.TextInput(c, &text).Label(label).Placeholder("不限").Width(100).Height(32).Changed() {
				if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v > 0 {
					f.Value = &v
				} else {
					f.Value = nil
				}
			}
			if f.Unit == "" {
				f.Unit = config.UnitMB
			}
			units := []string{config.UnitKB, config.UnitMB, config.UnitGB}
			choice(c, label+"单位", &f.Unit, units, units, 80)()
		})
	}
}

// dateField edits the upload date filter, with presets.
func (a *App) dateField(c *ui.Context, f *config.DateFilter) func() {
	return func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			choice(c, "日期条件", &f.Mode, []string{"", config.DateExact, config.DateBefore, config.DateAfter}, []string{"不限", "当天", "之前", "之后"}, 100)()
			if f.Mode != "" {
				ui.TextInput(c, &f.Value).Label("日期").Placeholder("2026-01-31").Width(130).Height(32)
			}
			ui.Spacer(c)
			for _, preset := range []struct{ id, label string }{{"today", "今天"}, {"last7Days", "近 7 天"}, {"last30Days", "近 30 天"}} {
				c.Key("date-" + preset.id)
				if ui.Button(c, preset.label).Height(32).Clicked() {
					*f = formats.DatePreset(preset.id, time.Now())
				}
			}
		})
	}
}

// noticeKindList are the notifications the app sends, with their names.
var noticeKindList = []struct{ kind, label string }{
	{config.NoticeVideoReady, "视频信息就绪"},
	{config.NoticePlaylistReady, "播放列表就绪"},
	{config.NoticeVideoFinished, "视频下载完成"},
	{config.NoticePlaylistFinished, "播放列表下载完成"},
	{config.NoticeDownloadFailed, "下载失败"},
	{config.NoticeQueueFinished, "队列全部完成"},
}

// noticeKinds turns each kind of notification on or off; off kinds are
// kept in DisabledNotifications.
func (a *App) noticeKinds(c *ui.Context, n *config.NotificationSettings) func() {
	return func() {
		off := n.NotificationBehavior == config.NotifyNever
		ui.Row(c).Gap(16).Wrap().Children(func() {
			for _, k := range noticeKindList {
				c.Key("notice-" + k.kind)
				on := !slices.Contains(n.DisabledNotifications, k.kind)
				if ui.Checkbox(c, &on, k.label).Disabled(off).Changed() {
					n.DisabledNotifications = slices.DeleteFunc(n.DisabledNotifications, func(s string) bool { return s == k.kind })
					if !on {
						n.DisabledNotifications = append(n.DisabledNotifications, k.kind)
					}
				}
			}
		})
	}
}
