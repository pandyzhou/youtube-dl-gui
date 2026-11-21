// Package engine is the download queue: it fetches link information, runs
// yt-dlp downloads a few at a time, follows their progress and keeps the
// state the queue page shows. It knows nothing of the UI; the window reads
// snapshots and calls its methods.
package engine

import (
	"context"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// Status of a job or an item.
type Status string

// Statuses.
const (
	StatusFetching     Status = "fetching"     // getting the link's information
	StatusSelecting    Status = "selecting"    // playlist: choose the videos
	StatusFetchingList Status = "fetchingList" // playlist: getting each video
	StatusReady        Status = "ready"        // waiting to be started
	StatusQueued       Status = "queued"       // started, waiting for a slot
	StatusDownloading  Status = "downloading"
	StatusPaused       Status = "paused"
	StatusDone         Status = "done"
	StatusSkipped      Status = "skipped" // an input filter skipped it
	StatusError        Status = "error"
)

// Active reports whether a status is queued or downloading.
func (s Status) Active() bool { return s == StatusQueued || s == StatusDownloading }

// Runner runs yt-dlp with args, passing each output line to onLine, and
// returns its exit code.
type Runner interface {
	Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error)
}

// YtdlpRunner runs the installed yt-dlp with the tools folder on PATH.
type YtdlpRunner struct {
	Path   string
	BinDir string
}

// Run implements Runner.
func (r YtdlpRunner) Run(ctx context.Context, args []string, onLine func(proc.Line)) (int, error) {
	return proc.Run(ctx, proc.Spec{Path: r.Path, Args: args, PathPrepend: r.BinDir}, onLine)
}

// EventKind names something worth a notification.
type EventKind string

// Events.
const (
	EventVideoReady    EventKind = "videoReady"
	EventPlaylistReady EventKind = "playlistReady"
	EventDownloadDone  EventKind = "downloadDone"
	EventDownloadError EventKind = "downloadError"
	EventQueueDone     EventKind = "queueDone"
)

// Event tells the app about a finished step.
type Event struct {
	Kind  EventKind
	JobID string
	Title string
	Count int
}

// Deps are what the engine needs from the app.
type Deps struct {
	Runner      Runner
	Settings    func() config.Settings
	Preferences func() config.Preferences
	Secrets     func() ytdlp.AuthSecrets
	// FallbackDir is the download folder when the settings name none.
	FallbackDir string
	// WaitTools blocks a download while the tools are being installed or
	// updated; nil never waits.
	WaitTools func(ctx context.Context) error
	// AfterDownload runs after a successful download, as AI subtitle
	// translation does. Its problems are warnings on the item; they never
	// fail the download. Nil does nothing.
	AfterDownload func(ctx context.Context, d Finished, r Reporter)
	OnChange      func() // state changed; called at most every Throttle
	OnEvent       func(Event)
	Throttle      time.Duration
}

// Finished describes a completed download for AfterDownload.
type Finished struct {
	JobID       string
	ItemID      string
	URL         string
	Title       string
	Destination string
	TrackType   ytdlp.TrackType
	Video       *ytdlp.Video
}

// Reporter lets AfterDownload show what it does on the item.
type Reporter struct {
	Stage    func(ytdlp.Stage)
	Progress func(percent float64)
	Log      func(line string)
	Warn     func(code, message string)
}

// AddOptions are how a link is added.
type AddOptions struct {
	Preset string // "" for the default preset
	// Start downloads as soon as the information is in; a playlist then
	// takes all its videos.
	Start bool
}

// ItemView is a snapshot of one video.
type ItemView struct {
	ID            string
	URL           string
	Title         string
	Video         *ytdlp.Video // nil until fetched
	PlaylistIndex int          // 0-based position in the playlist, -1 if none
	Status        Status
	Stage         ytdlp.Stage
	Percent       float64 // 0–100
	SpeedBps      float64
	EtaSecs       int64 // -1 when unknown
	Destination   string
	Diagnostics   []ytdlp.Diagnostic
	Err           string
	Choice        string // resolved quality, as "1080p60"
	Size          int64  // estimated bytes, 0 when unknown
}

// JobView is a snapshot of one queue card.
type JobView struct {
	ID        string
	URL       string
	Status    Status
	Title     string
	Thumbnail string
	Uploader  string
	Duration  float64
	Playlist  *ytdlp.Playlist // set for playlists and videos split from one
	Combined  bool            // a playlist downloaded as one card
	Items     []ItemView
	Preset    string
	Custom    *ytdlp.FormatOptions
	Percent   float64
	Done      int // items done or skipped
	Failed    int
	Size      int64
	Err       string
	Added     time.Time
}
