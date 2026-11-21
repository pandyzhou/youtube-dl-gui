package core

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
)

// videoExts are the files "为已有视频补字幕" takes.
var videoExts = []string{".mp4", ".mkv", ".webm", ".mov", ".m4v", ".avi", ".flv"}

// IsVideoFile reports whether path looks like a video.
func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range videoExts {
		if ext == e {
			return true
		}
	}
	return false
}

// ErrNoSourceURL is returned for a video that does not tell where it came
// from.
var ErrNoSourceURL = errors.New("视频里没有记录原链接，文件名里也没有视频 ID，无法补字幕")

// SourceURL finds where a downloaded video came from, reading its
// metadata with ffprobe.
func SourceURL(ctx context.Context, ffprobe, file string) (string, error) {
	var out strings.Builder
	_, err := proc.Run(ctx, proc.Spec{
		Path: ffprobe,
		Args: []string{"-v", "error", "-show_entries", "format_tags", "-of", "json", file},
	}, func(l proc.Line) {
		if !l.Stderr {
			out.WriteString(l.Text + "\n")
		}
	})
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if u, ok := URLFromProbe([]byte(out.String()), file); ok {
		return u, nil
	}
	if err != nil {
		return "", err
	}
	return "", ErrNoSourceURL
}

var bracketID = regexp.MustCompile(`\[([A-Za-z0-9_-]{11})\]`)

// URLFromProbe picks the address from ffprobe's JSON: the purl or comment
// tag yt-dlp writes with --embed-metadata, else a YouTube ID in brackets
// in the file name, as yt-dlp names files by default.
func URLFromProbe(probe []byte, file string) (string, bool) {
	var doc struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if json.Unmarshal(probe, &doc) == nil {
		for _, want := range []string{"purl", "comment", "url"} {
			for k, v := range doc.Format.Tags {
				v = strings.TrimSpace(v)
				if strings.EqualFold(k, want) && (strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://")) && !strings.ContainsAny(v, " \n") {
					return v, true
				}
			}
		}
	}
	if m := bracketID.FindStringSubmatch(filepath.Base(file)); m != nil {
		return "https://www.youtube.com/watch?v=" + m[1], true
	}
	return "", false
}
