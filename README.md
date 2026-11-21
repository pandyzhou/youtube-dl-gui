# 流光下载 LumaDL

基于 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 的 Windows 视频下载器，界面用 [MyGo](https://github.com/egoist/mygo) 原生绘制，只做简体中文。

## 功能

- **一步下载**：粘贴链接后，按画质预设（最佳、2160p … 480p、仅音频、自定义）直接开始。卡片上会显示实际匹配到的分辨率和大约的文件大小。YouTube 用分片并行下载。
- **播放列表**：可以选择范围，也可以拆分成单个视频或合并成一张卡片。在播放列表或合辑里打开的视频链接，会询问只下载这个视频还是整个列表。
- **AI 双语字幕**：调用 OpenAI 兼容接口，把字幕翻译成中英双语。
  - 翻译和下载同时进行，断了可以从断点继续。
  - YouTube 自动字幕会按词级时间重新分句。
  - 高质量模式会先通读全文，整理出主题和术语表。
  - MKV 里视频、音频和字幕一次合成，字幕是带描边的 ASS。
- **为已有视频补字幕**：在设置里选择视频，或把视频文件拖进下载队列。
- **下载队列会保存**：重启或更新后自动恢复。
- **系统集成**：托盘、全局快捷键（Alt+Shift+V / D / Enter）、通知、开机启动、任务栏进度、自动更新。

## 安装

在 [Releases](https://github.com/pandyzhou/lumadl/releases) 下载安装包 `lumadl-setup-*-amd64.exe`，或免安装版 `lumadl-*-portable.exe`。免安装版建议放在单独的文件夹里，自动更新会在它旁边写入几个文件。

第一次启动时会下载 yt-dlp、ffmpeg 等工具（约 150 MB），之后这些工具会自动更新。

用过 Open Video Downloader 4.x 的话，先退出旧版，再安装流光下载。第一次启动会自动搬过设置、下载队列、已下载的工具和 API Key，之后就可以卸载旧版了。

## 开发

- Go 1.27，界面用 MyGo v0.10（`go tool mygo`）。
- 入口在 `cmd/lumadl`，其他代码在 `internal/`：`ytdlp`（参数与解析）、`engine`（下载队列）、`translate`（翻译）、`ui`（界面）等。
- 构建和测试都在 GitHub Actions 上运行：每次推送到 `main` 都会发布一个新版本，其他分支只生成预览包。

## 来源与许可证

最初 fork 自 [jely2002/youtube-dl-gui](https://github.com/jely2002/youtube-dl-gui)（Open Video Downloader），后来用 Go 和 MyGo 完全重写。参数生成、进度解析等逻辑和测试是从原项目移植过来的，工具清单目前仍使用原项目发布的签名清单。

[AGPL-3.0](LICENSE)
