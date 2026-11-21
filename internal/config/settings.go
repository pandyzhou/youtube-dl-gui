// Package config holds the app's settings and preferences: their types and
// defaults, loading and saving them as JSON, and importing the Tauri app's
// files. JSON keys keep the old app's camelCase names so its files import
// field for field.
package config

import "runtime"

// Settings are the user's choices, edited on the settings page.
type Settings struct {
	Appearance    AppearanceSettings   `json:"appearance"`
	Auth          AuthSettings         `json:"auth"`
	Network       NetworkSettings      `json:"network"`
	Input         InputSettings        `json:"input"`
	InputFilters  InputFilterSettings  `json:"inputFilters"`
	Output        OutputSettings       `json:"output"`
	Performance   PerformanceSettings  `json:"performance"`
	Subtitles     SubtitleSettings     `json:"subtitles"`
	Translation   TranslationSettings  `json:"translation"`
	Update        UpdateSettings       `json:"update"`
	System        SystemSettings       `json:"system"`
	Notifications NotificationSettings `json:"notifications"`
}

// AppearanceSettings choose the theme and which extra selector cards show.
type AppearanceSettings struct {
	Theme           string `json:"theme"`           // system | light | dark
	ExpandedOptions string `json:"expandedOptions"` // none | encodings | tracks
}

// AuthSettings are the non-secret sign-in settings; passwords, tokens and
// headers live in the secrets store.
type AuthSettings struct {
	CookieFile    string `json:"cookieFile"`
	CookieBrowser string `json:"cookieBrowser"` // none | brave | chrome | … (lower case)
}

// NetworkSettings apply to yt-dlp and to the app's own HTTP requests.
type NetworkSettings struct {
	EnableProxy   bool   `json:"enableProxy"`
	Proxy         string `json:"proxy"`
	Impersonate   string `json:"impersonate"`   // none | any | a yt-dlp target
	ExtractorArgs string `json:"extractorArgs"` // one per line
}

// InputSettings shape how links get into the queue.
type InputSettings struct {
	AutoFillClipboard       bool `json:"autoFillClipboard"`
	PreferVideoInMixedLinks bool `json:"preferVideoInMixedLinks"`
	GlobalShortcuts         bool `json:"globalShortcuts"`
	// DefaultPreset is the quality preset new links get (see package
	// formats); new in the MyGo app.
	DefaultPreset string `json:"defaultPreset"`
	// AutoDownload starts a download as soon as a link's information is in,
	// with the default preset; new in the MyGo app.
	AutoDownload bool `json:"autoDownload"`
}

// Size units of the input filters.
const (
	UnitB  = "B"
	UnitKB = "KB"
	UnitMB = "MB"
	UnitGB = "GB"
	UnitTB = "TB"
)

// SizeFilter is a minimum or maximum file size; a nil Value is off.
type SizeFilter struct {
	Value *float64 `json:"value"`
	Unit  string   `json:"unit"`
}

// Date filter modes.
const (
	DateExact  = "exact"
	DateBefore = "before"
	DateAfter  = "after"
)

// DateFilter is an upload date filter; an empty Mode is off. Value is
// YYYY-MM-DD.
type DateFilter struct {
	Mode  string `json:"mode"`
	Value string `json:"value"`
}

// InputFilterSettings skip videos before they reach the queue.
type InputFilterSettings struct {
	MinSize           SizeFilter `json:"minSize"`
	MaxSize           SizeFilter `json:"maxSize"`
	DateFilter        DateFilter `json:"dateFilter"`
	MatchFilters      string     `json:"matchFilters"`
	BreakMatchFilters string     `json:"breakMatchFilters"`
}

// Transcode policies: "never" is "keep original streams" in the UI.
const (
	PolicyNever         = "never"
	PolicyAllowReencode = "allowReencode"
)

// VideoOutputSettings pick the container and post-processing of videos.
type VideoOutputSettings struct {
	Container             string `json:"container"`             // mp4 | mkv
	Policy                string `json:"policy"`                // never | allowReencode
	PostprocessPreset     string `json:"postprocessPreset"`     // none | fps30 | mp42 | custom
	CustomPostprocessMode string `json:"customPostprocessMode"` // remux | reencode
	PostprocessArgs       string `json:"postprocessArgs"`
}

// AudioOutputSettings pick the format and post-processing of audio.
type AudioOutputSettings struct {
	Format            string `json:"format"`            // mp3 | m4a | opus | aac | ogg | flac | wav
	Policy            string `json:"policy"`            // never | allowReencode
	PostprocessPreset string `json:"postprocessPreset"` // none | custom
	PostprocessArgs   string `json:"postprocessArgs"`
}

// OutputSettings decide what files a download produces and where.
type OutputSettings struct {
	Video                    VideoOutputSettings `json:"video"`
	Audio                    AudioOutputSettings `json:"audio"`
	AddMetadata              bool                `json:"addMetadata"`
	AddThumbnail             bool                `json:"addThumbnail"`
	SaveThumbnail            bool                `json:"saveThumbnail"`
	ReversePlaylistNumbering bool                `json:"reversePlaylistNumbering"`
	DownloadDir              string              `json:"downloadDir"`
	FileNameTemplate         string              `json:"fileNameTemplate"`
	AudioFileNameTemplate    string              `json:"audioFileNameTemplate"`
	RestrictFilenames        bool                `json:"restrictFilenames"`
}

// PerformanceSettings limit how much runs at once.
type PerformanceSettings struct {
	MaxConcurrency         int  `json:"maxConcurrency"`
	SplitPlaylistThreshold int  `json:"splitPlaylistThreshold"`
	AutoLoadSize           bool `json:"autoLoadSize"`
	ConcurrentFragments    int  `json:"concurrentFragments"`
}

// SubtitleSettings choose the subtitles downloaded with videos.
type SubtitleSettings struct {
	Enabled              bool     `json:"enabled"`
	IncludeAutoGenerated bool     `json:"includeAutoGenerated"`
	Languages            []string `json:"languages"` // ["all"] for every language
	FormatPreference     []string `json:"formatPreference"`
	EmbedSubtitles       bool     `json:"embedSubtitles"`
}

// TranslationSettings configure AI bilingual subtitles; the API key is a
// secret (SecretTranslationAPIKey).
type TranslationSettings struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"baseUrl"`
	Model          string `json:"model"`
	TargetLanguage string `json:"targetLanguage"`
	// HighQuality reads the whole transcript first for a topic and a
	// glossary that every batch gets.
	HighQuality bool `json:"highQuality"`
	// ReasoningEffort goes to the API as reasoning_effort (none, minimal,
	// low, medium, high); "" leaves it out.
	ReasoningEffort string `json:"reasoningEffort"`
	// Concurrency is how many requests run at once; 0 is the default.
	Concurrency int `json:"concurrency"`
}

// UpdateSettings turn the automatic updates of the tools and the app on.
type UpdateSettings struct {
	UpdateBinaries bool `json:"updateBinaries"`
	UpdateApp      bool `json:"updateApp"`
}

// Close behaviors of the main window.
const (
	CloseExit = "exit"
	CloseHide = "hide"
)

// SystemSettings integrate the app with the desktop.
type SystemSettings struct {
	TrayEnabled        bool   `json:"trayEnabled"`
	AutoStartEnabled   bool   `json:"autoStartEnabled"`
	AutoStartMinimised bool   `json:"autoStartMinimised"`
	CloseBehavior      string `json:"closeBehavior"` // exit | hide
}

// Notification behaviors.
const (
	NotifyAlways       = "always"
	NotifyOnBackground = "onBackground"
	NotifyNever        = "never"
)

// Notification kinds, which can be turned off one by one.
const (
	NoticeQueueAdded       = "queueAdded"
	NoticeQueueDownloading = "queueDownloading"
	NoticeQueueFinished    = "queueFinished"
	NoticeVideoFinished    = "videoFinished"
	NoticePlaylistFinished = "playlistFinished"
	NoticeVideoReady       = "videoReady"
	NoticePlaylistReady    = "playlistReady"
	NoticeDownloadFailed   = "downloadFailed"
)

// NotificationSettings decide when the app shows system notifications.
type NotificationSettings struct {
	NotificationBehavior  string   `json:"notificationBehavior"`
	DisabledNotifications []string `json:"disabledNotifications"`
}

// DefaultFileNameTemplate and DefaultAudioFileNameTemplate are yt-dlp
// output templates for videos and audio.
const (
	DefaultFileNameTemplate      = "%(title).200s-(%(height)sp%(fps).0d).%(ext)s"
	DefaultAudioFileNameTemplate = "%(title).200s-(%(abr)dk).%(ext)s"
)

// DefaultSubtitleFormats is the subtitle format order yt-dlp is asked for.
var DefaultSubtitleFormats = []string{"srt", "vtt", "ass", "ttml", "json3"}

// DefaultSettings returns the settings of a new install. downloadDir is the
// system's Downloads folder.
func DefaultSettings(downloadDir string) Settings {
	concurrency := (runtime.NumCPU() + 1) / 2
	if concurrency < 1 {
		concurrency = 1
	}
	return Settings{
		Appearance: AppearanceSettings{Theme: "system", ExpandedOptions: "none"},
		Auth:       AuthSettings{CookieBrowser: "none"},
		Network:    NetworkSettings{Impersonate: "none"},
		Input: InputSettings{
			AutoFillClipboard: true,
			GlobalShortcuts:   true,
			DefaultPreset:     "best",
			AutoDownload:      true,
		},
		InputFilters: InputFilterSettings{
			MinSize: SizeFilter{Unit: UnitMB},
			MaxSize: SizeFilter{Unit: UnitMB},
		},
		Output: OutputSettings{
			Video: VideoOutputSettings{
				Container:             "mp4",
				Policy:                PolicyAllowReencode,
				PostprocessPreset:     "none",
				CustomPostprocessMode: "remux",
			},
			Audio: AudioOutputSettings{
				Format:            "mp3",
				Policy:            PolicyAllowReencode,
				PostprocessPreset: "none",
			},
			AddMetadata:           true,
			AddThumbnail:          true,
			DownloadDir:           downloadDir,
			FileNameTemplate:      DefaultFileNameTemplate,
			AudioFileNameTemplate: DefaultAudioFileNameTemplate,
		},
		Performance: PerformanceSettings{
			MaxConcurrency:         concurrency,
			SplitPlaylistThreshold: 50,
			AutoLoadSize:           true,
			ConcurrentFragments:    8,
		},
		Subtitles: SubtitleSettings{
			Languages:        []string{"en"},
			FormatPreference: append([]string(nil), DefaultSubtitleFormats...),
			EmbedSubtitles:   true,
		},
		Translation:   TranslationSettings{TargetLanguage: "简体中文", Concurrency: 6},
		Update:        UpdateSettings{UpdateBinaries: true, UpdateApp: true},
		System:        SystemSettings{CloseBehavior: CloseExit},
		Notifications: NotificationSettings{NotificationBehavior: NotifyNever, DisabledNotifications: []string{}},
	}
}

// Preferences are remembered by the app rather than chosen on the settings
// page: per-kind download folders, folder templates, recent paths.
type Preferences struct {
	Paths   PathPreferences   `json:"paths"`
	Recents RecentPreferences `json:"recents"`
}

// PathPreferences override the download folder and add a folder template,
// separately for video and audio downloads.
type PathPreferences struct {
	AudioDownloadDir       string `json:"audioDownloadDir"`
	VideoDownloadDir       string `json:"videoDownloadDir"`
	VideoDirectoryTemplate string `json:"videoDirectoryTemplate"`
	AudioDirectoryTemplate string `json:"audioDirectoryTemplate"`
}

// RecentPreferences keep recently chosen paths by label (downloadDir,
// videoDir, audioDir, cookieFile), newest first.
type RecentPreferences struct {
	Recent map[string][]string `json:"recent"`
}

// MaxRecents is how many recent paths are kept per label.
const MaxRecents = 5

// DefaultPreferences returns the preferences of a new install.
func DefaultPreferences() Preferences {
	return Preferences{Recents: RecentPreferences{Recent: map[string][]string{}}}
}

// AddRecent puts path first in label's recent list, keeping MaxRecents.
func (p *Preferences) AddRecent(label, path string) {
	if path == "" {
		return
	}
	if p.Recents.Recent == nil {
		p.Recents.Recent = map[string][]string{}
	}
	list := []string{path}
	for _, existing := range p.Recents.Recent[label] {
		if existing != path && len(list) < MaxRecents {
			list = append(list, existing)
		}
	}
	p.Recents.Recent[label] = list
}
