package ui

import (
	"fmt"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// formatDuration formats seconds as "1:02:03" or "2:03".
func formatDuration(secs float64) string {
	if secs <= 0 {
		return ""
	}
	s := int(secs + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// formatSpeed formats bytes per second: "3.2 MB/s".
func formatSpeed(bps float64) string {
	if bps <= 0 {
		return ""
	}
	return formatBytes(int64(bps)) + "/s"
}

// formatEta formats the time left: "剩余 1:23".
func formatEta(secs int64) string {
	if secs < 0 {
		return ""
	}
	if secs == 0 {
		return "即将完成"
	}
	return "剩余 " + formatDuration(float64(secs))
}

var stageLabels = map[ytdlp.Stage]string{
	ytdlp.StageInitializing: "准备中",
	ytdlp.StageDownloading:  "下载中",
	ytdlp.StageMerging:      "合并音视频",
	ytdlp.StageRemuxing:     "转换封装",
	ytdlp.StageReencoding:   "重新编码",
	ytdlp.StageFinalizing:   "收尾处理",
	ytdlp.StageTranslating:  "翻译字幕",
	ytdlp.StageEmbedding:    "写入字幕",
}

var statusLabels = map[engine.Status]string{
	engine.StatusFetching:     "正在获取信息…",
	engine.StatusSelecting:    "选择要下载的视频",
	engine.StatusFetchingList: "正在获取视频信息…",
	engine.StatusReady:        "就绪",
	engine.StatusQueued:       "等待中",
	engine.StatusDownloading:  "下载中",
	engine.StatusPaused:       "已暂停",
	engine.StatusDone:         "已完成",
	engine.StatusSkipped:      "已跳过",
	engine.StatusError:        "出错",
}

// errorText explains a failed item in Chinese: the title of its last
// recognized diagnostic and its message, or the raw error.
func errorText(it engine.ItemView, fallback string) (title, detail string) {
	for i := len(it.Diagnostics) - 1; i >= 0; i-- {
		d := it.Diagnostics[i]
		if d.Level != ytdlp.LevelError {
			continue
		}
		if text, ok := diagnosticText[d.Code]; ok && d.Code != "unknown" {
			return text[0], text[1]
		}
		break
	}
	msg := it.Err
	if msg == "" {
		msg = fallback
	}
	return "出错", msg
}
