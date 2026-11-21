package formats

import (
	"net/url"
	"strings"

	"github.com/pandyzhou/lumadl/internal/ytdlp"
)

// What to do with a link that points at a video and a playlist at once
// (settings.input.mixedLinks).
const (
	MixedAsk   = "ask"
	MixedVideo = "video"
	MixedList  = "list"
)

// MixedLink tells whether a YouTube link names both a video and a
// playlist, and whether that playlist is a Mix (list=RD…), the endless
// radio YouTube generates from a video.
func MixedLink(link string) (mixed, mix bool) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return false, false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	q := u.Query()
	list := q.Get("list")
	var video bool
	switch host {
	case "youtube.com", "m.youtube.com", "music.youtube.com":
		video = q.Get("v") != ""
	case "youtu.be":
		video = strings.Trim(u.Path, "/") != ""
	}
	if !video || list == "" {
		return false, false
	}
	return true, strings.HasPrefix(list, "RD")
}

// MixedMode is the playlist mode for a link when no one is asked: a mixed
// link takes the video unless the setting says the whole list. Other links
// keep the default.
func MixedMode(link, setting string) ytdlp.PlaylistMode {
	if mixed, _ := MixedLink(link); !mixed {
		return ytdlp.PlaylistModeNone
	}
	if setting == MixedList {
		return ytdlp.PlaylistModeAll
	}
	return ytdlp.PlaylistModeSingle
}
