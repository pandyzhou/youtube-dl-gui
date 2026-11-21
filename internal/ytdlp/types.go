// Package ytdlp builds yt-dlp command lines and parses what yt-dlp prints:
// media information, progress, destinations and diagnostics. It is a port
// of the Tauri app's Rust runners and parsers; the tests pin the exact
// arguments yt-dlp gets.
package ytdlp

// TrackType is what a download keeps.
type TrackType string

// Track types.
const (
	TrackBoth  TrackType = "both"
	TrackAudio TrackType = "audio"
	TrackVideo TrackType = "video"
)

// FormatOptions choose the format of a download. Zero numbers are unset.
type FormatOptions struct {
	TrackType     TrackType
	Abr           int // audio bitrate in kbps
	Height        int
	Fps           int
	AudioEncoding string // codec id, e.g. "opus"
	VideoEncoding string
	AudioTrack    string // "lang:en|channels:2", "auto" or ""
	VideoTrack    string
}

// PlaylistMode forces yt-dlp to take a playlist or a single video.
type PlaylistMode string

// Playlist modes.
const (
	PlaylistModeNone   PlaylistMode = ""
	PlaylistModeSingle PlaylistMode = "singleVideo"
	PlaylistModeAll    PlaylistMode = "playlist"
)

// InputFilterOptions are yt-dlp's selection and filter options, built from
// the input filter settings when a link is added and from a playlist
// selection.
type InputFilterOptions struct {
	PlaylistItems     string // -I, e.g. "1:3,7"
	MinFilesize       string // e.g. "50M"
	MaxFilesize       string
	Date              string // YYYYMMDD
	DateBefore        string
	DateAfter         string
	MatchFilters      string
	BreakMatchFilters string
	PlaylistMode      PlaylistMode
}

// SubtitleInventory lists the subtitle languages a video has.
type SubtitleInventory struct {
	ManualLanguages []string `json:"manualLanguages"`
	AutoLanguages   []string `json:"autoLanguages"`
}

// AuthSecrets are the sign-in secrets passed to yt-dlp.
type AuthSecrets struct {
	Username      string
	Password      string
	VideoPassword string
	BearerToken   string
	Headers       string // one "Name: value" per line
}
