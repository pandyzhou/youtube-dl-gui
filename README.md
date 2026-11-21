> **本仓库已停止更新。** 应用已改名为 **流光下载 LumaDL**，并迁移到独立仓库 https://github.com/pandyzhou/lumadl ，请到那里下载新版本。

# Open Video Downloader（简体中文 · Windows）

基于 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 的视频下载器，用 [MyGo](https://github.com/egoist/mygo) 原生界面重写，只支持 Windows x64，界面为简体中文。

本项目 fork 自 [jely2002/youtube-dl-gui](https://github.com/jely2002/youtube-dl-gui)。3.x 的 Tauri 版本保留在标签 `tauri-final`。

## 功能

- **一步下载**：粘贴链接后，按画质预设（最佳、2160p … 480p、仅音频、自定义）直接开始。卡片上会显示实际匹配到的分辨率和大约的文件大小。
- **播放列表**：可以选择范围，也可以拆分成单个视频或合并成一张卡片。
- **AI 双语字幕**：调用 OpenAI 兼容接口，把字幕翻译成中英双语。
  - YouTube 自动字幕会按词级时间重新分句。
  - 高质量模式会先通读全文，整理出主题和术语表。
  - 请求是流式的，中断后可以从断点继续。
  - 字幕默认封装进视频：MKV 是带描边的 ASS，MP4 是纯文本。
- **为已有视频补字幕**：在设置里选择视频，或把视频文件拖进下载队列。
- **系统集成**：托盘、全局快捷键（Alt+Shift+V / D / Enter）、通知、开机启动、任务栏进度、自动更新。

## 安装

在 [Releases](https://github.com/pandyzhou/youtube-dl-gui/releases) 下载安装包或免安装版 exe。第一次启动时会下载 yt-dlp、ffmpeg 等工具（约 150 MB），之后自动更新。

3.x（Tauri 版）不会自动更新到 4.x，需要手动安装一次。新版第一次启动时会导入旧版设置，但登录信息和 API Key 要重新填写。

## 开发

- Go 1.27，界面用 MyGo v0.10（`go tool mygo`）。
- 代码在 `cmd/ovd`（入口）和 `internal/`（`ytdlp` 参数与解析、`engine` 队列、`translate` 翻译、`ui` 界面等）。
- 构建和测试都在 GitHub Actions 上运行：
  - `Go CI`：gofmt、vet、单元测试、界面测试和截图；
  - `MyGo build (Windows)`：分支上生成预览包，main 上发布正式版本。

## 许可证

[AGPL-3.0](LICENSE)
