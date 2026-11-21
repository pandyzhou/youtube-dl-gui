package ytdlp

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Info is the part of yt-dlp's --dump-single-json output the app reads.
type Info struct {
	ID                string                    `json:"id"`
	Title             string                    `json:"title"`
	Uploader          string                    `json:"uploader"`
	UploaderID        string                    `json:"uploader_id"`
	Duration          *float64                  `json:"duration"`
	LikeCount         *int64                    `json:"like_count"`
	DislikeCount      *int64                    `json:"dislike_count"`
	AverageRating     *float64                  `json:"average_rating"`
	ViewCount         *int64                    `json:"view_count"`
	CommentCount      *int64                    `json:"comment_count"`
	Description       string                    `json:"description"`
	ExtractorKey      string                    `json:"extractor_key"`
	Thumbnail         string                    `json:"thumbnail"`
	Thumbnails        []InfoThumbnail           `json:"thumbnails"`
	Formats           []InfoFormat              `json:"formats"`
	Subtitles         map[string][]InfoSubtitle `json:"subtitles"`
	AutomaticCaptions map[string][]InfoSubtitle `json:"automatic_captions"`
	Type              string                    `json:"_type"`
	IsLive            bool                      `json:"is_live"`
	Entries           []InfoEntry               `json:"entries"`
	WebpageURL        string                    `json:"webpage_url"`
	Filesize          *int64                    `json:"filesize"`
	FilesizeApprox    *int64                    `json:"filesize_approx"`
	PlaylistCount     *int64                    `json:"playlist_count"`
	Chapters          []InfoChapter             `json:"chapters"`
}

// InfoThumbnail is a thumbnail candidate.
type InfoThumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// InfoEntry is a playlist entry of a flat playlist.
type InfoEntry struct {
	URL        string `json:"url"`
	WebpageURL string `json:"webpage_url"`
}

// InfoFormat is one format yt-dlp can download.
type InfoFormat struct {
	FormatID           string   `json:"format_id"`
	Format             string   `json:"format"`
	FormatNote         string   `json:"format_note"`
	Height             *int64   `json:"height"`
	Fps                *float64 `json:"fps"`
	Abr                *float64 `json:"abr"`
	AudioChannels      *float64 `json:"audio_channels"`
	Language           string   `json:"language"`
	LanguagePreference *int64   `json:"language_preference"`
	Ext                string   `json:"ext"`
	Vcodec             string   `json:"vcodec"`
	Acodec             string   `json:"acodec"`
	Filesize           *int64   `json:"filesize"`
	FilesizeApprox     *int64   `json:"filesize_approx"`
	Tbr                *float64 `json:"tbr"`
}

// InfoSubtitle is one subtitle file of a language.
type InfoSubtitle struct {
	Ext string `json:"ext"`
}

// InfoChapter is a chapter of a video.
type InfoChapter struct {
	Title     string   `json:"title"`
	StartTime *float64 `json:"start_time"`
	EndTime   *float64 `json:"end_time"`
}

// Kind says what a link turned out to be.
type Kind string

// Kinds.
const (
	KindSingle     Kind = "single"
	KindPlaylist   Kind = "playlist"
	KindLivestream Kind = "livestream"
)

// Media is the parsed result of fetching a link.
type Media struct {
	Kind     Kind
	Video    *Video    // KindSingle and KindLivestream (only title, uploader, URL)
	Playlist *Playlist // KindPlaylist
}

// Codec is a codec id with a readable label.
type Codec struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Format is a selectable quality: a height and frame rate for video, or a
// bitrate for audio, with the codecs and tracks available at it.
type Format struct {
	ID            string   `json:"id"`
	Abr           int      `json:"abr,omitempty"`
	Height        int      `json:"height,omitempty"`
	Fps           int      `json:"fps,omitempty"`
	AudioCodecs   []Codec  `json:"audioCodecs"`
	VideoCodecs   []Codec  `json:"videoCodecs"`
	AudioTrackIDs []string `json:"audioTrackIds"`
	VideoTrackIDs []string `json:"videoTrackIds"`
	Filesize      int64    `json:"filesize,omitempty"` // largest known size at this quality, 0 if unknown
}

// Track is an audio or video track: a language and channel layout. Its ID
// is the FormatOptions track preference ("lang:en|channels:2", "auto").
type Track struct {
	ID                 string `json:"id"`
	Label              string `json:"label"`
	Language           string `json:"language,omitempty"`
	LanguagePreference *int64 `json:"languagePreference,omitempty"`
	FormatNote         string `json:"formatNote,omitempty"`
	Format             string `json:"format,omitempty"`
	AudioChannels      int    `json:"audioChannels,omitempty"`
}

// Chapter of a video.
type Chapter struct {
	Title     string  `json:"title"`
	StartTime float64 `json:"startTime"`
	EndTime   float64 `json:"endTime"`
}

// Video is a parsed single video.
type Video struct {
	ID                string            `json:"id"`
	URL               string            `json:"url"`
	Title             string            `json:"title"`
	Thumbnail         string            `json:"thumbnail"`
	Description       string            `json:"description"`
	Uploader          string            `json:"uploader"`
	UploaderID        string            `json:"uploaderId"`
	Views             *int64            `json:"views,omitempty"`
	Comments          *int64            `json:"comments,omitempty"`
	Likes             *int64            `json:"likes,omitempty"`
	Dislikes          *int64            `json:"dislikes,omitempty"`
	Duration          *float64          `json:"duration,omitempty"`
	Rating            *float64          `json:"rating,omitempty"`
	Extractor         string            `json:"extractor"`
	VideoCodecs       []Codec           `json:"videoCodecs"`
	AudioCodecs       []Codec           `json:"audioCodecs"`
	VideoTracks       []Track           `json:"videoTracks"`
	AudioTracks       []Track           `json:"audioTracks"`
	Formats           []Format          `json:"formats"`
	SubtitleInventory SubtitleInventory `json:"subtitleInventory"`
	Chapters          []Chapter         `json:"chapters"`
	Filesize          *int64            `json:"filesize,omitempty"`
}

// PlaylistEntry is a video of a playlist; Index counts from 0 in the order
// yt-dlp listed them.
type PlaylistEntry struct {
	URL   string `json:"videoUrl"`
	Index int    `json:"index"`
}

// Playlist is a parsed playlist.
type Playlist struct {
	URL           string          `json:"url"`
	Title         string          `json:"title"`
	Thumbnail     string          `json:"thumbnail"`
	Uploader      string          `json:"uploader"`
	UploaderID    string          `json:"uploaderId"`
	PlaylistID    string          `json:"playlistId"`
	PlaylistCount *int64          `json:"playlistCount,omitempty"`
	Entries       []PlaylistEntry `json:"entries"`
}

// ParseInfo parses yt-dlp's JSON for a link.
func ParseInfo(data []byte) (*Media, error) {
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("yt-dlp JSON: %w", err)
	}
	return ParseInfoValue(&info), nil
}

// ParseInfoValue classifies and parses decoded info.
func ParseInfoValue(info *Info) *Media {
	switch {
	case info.IsLive:
		return &Media{Kind: KindLivestream, Video: &Video{ID: info.ID, URL: info.WebpageURL, Title: info.Title, Uploader: info.Uploader}}
	case info.Type == "playlist" || len(info.Entries) > 0:
		return &Media{Kind: KindPlaylist, Playlist: parsePlaylist(info)}
	}
	return &Media{Kind: KindSingle, Video: parseSingle(info)}
}

func parsePlaylist(info *Info) *Playlist {
	p := &Playlist{
		URL: info.WebpageURL, Title: info.Title, Uploader: info.Uploader, UploaderID: info.UploaderID,
		PlaylistID: info.ID, PlaylistCount: nonNegative(info.PlaylistCount),
	}
	for i, e := range info.Entries {
		url := e.URL
		if url == "" {
			url = e.WebpageURL
		}
		if url != "" {
			p.Entries = append(p.Entries, PlaylistEntry{URL: url, Index: i})
		}
	}
	best := -1
	for _, t := range info.Thumbnails {
		if area := t.Width * t.Height; t.URL != "" && area > best {
			best, p.Thumbnail = area, t.URL
		}
	}
	return p
}

func parseSingle(info *Info) *Video {
	v := &Video{
		ID: info.ID, URL: info.WebpageURL, Title: info.Title, Thumbnail: info.Thumbnail,
		Description: info.Description, Uploader: info.Uploader, UploaderID: info.UploaderID,
		Views: nonNegative(info.ViewCount), Comments: nonNegative(info.CommentCount),
		Likes: nonNegative(info.LikeCount), Dislikes: nonNegative(info.DislikeCount),
		Duration: info.Duration, Rating: info.AverageRating, Extractor: info.ExtractorKey,
		SubtitleInventory: SubtitleInventory{
			ManualLanguages: subtitleLanguages(info.Subtitles),
			AutoLanguages:   subtitleLanguages(info.AutomaticCaptions),
		},
		Chapters: parseChapters(info.Chapters),
	}
	if info.Filesize != nil {
		v.Filesize = nonNegative(info.Filesize)
	} else {
		v.Filesize = nonNegative(info.FilesizeApprox)
	}
	processFormats(v, info.Formats)
	return v
}

func nonNegative(v *int64) *int64 {
	if v == nil || *v < 0 {
		return nil
	}
	return v
}

func subtitleLanguages(subs map[string][]InfoSubtitle) []string {
	var out []string
	for lang, tracks := range subs {
		lang = strings.ToLower(strings.TrimSpace(lang))
		if lang == "" || contains(out, lang) {
			continue
		}
		for _, t := range tracks {
			if t.Ext != "" {
				out = append(out, lang)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func parseChapters(chapters []InfoChapter) []Chapter {
	var out []Chapter
	for _, c := range chapters {
		title := strings.TrimSpace(c.Title)
		if title == "" || c.StartTime == nil || c.EndTime == nil || *c.EndTime <= *c.StartTime {
			continue
		}
		out = append(out, Chapter{Title: title, StartTime: *c.StartTime, EndTime: *c.EndTime})
	}
	return out
}

type formatAggregate struct {
	format      Format
	audioCodecs map[string]Codec
	videoCodecs map[string]Codec
	audioTracks map[string]bool
	videoTracks map[string]bool
}

func processFormats(v *Video, formats []InfoFormat) {
	videoCodecs, audioCodecs := map[string]Codec{}, map[string]Codec{}
	seenAudio, seenVideo := map[string]bool{}, map[string]bool{}
	aggregates := map[string]*formatAggregate{}

	for i := range formats {
		f := &formats[i]
		if realVideoCodec(f.Vcodec) {
			insertCodec(videoCodecs, f.Vcodec)
		}
		if realAudioCodec(f.Acodec) {
			insertCodec(audioCodecs, f.Acodec)
		}
		var audioTrack, videoTrack *Track
		if realAudioCodec(f.Acodec) {
			t := toTrack(f)
			audioTrack = &t
			if !seenAudio[t.ID] {
				seenAudio[t.ID] = true
				v.AudioTracks = append(v.AudioTracks, t)
			}
		}
		if hasVideoStream(f) {
			t := toTrack(f)
			videoTrack = &t
			if !seenVideo[t.ID] {
				seenVideo[t.ID] = true
				v.VideoTracks = append(v.VideoTracks, t)
			}
		}
		key, group, ok := formatGroup(f)
		if !ok {
			continue
		}
		agg := aggregates[key]
		if agg == nil {
			agg = &formatAggregate{format: group, audioCodecs: map[string]Codec{}, videoCodecs: map[string]Codec{}, audioTracks: map[string]bool{}, videoTracks: map[string]bool{}}
			aggregates[key] = agg
		}
		if realAudioCodec(f.Acodec) {
			insertCodec(agg.audioCodecs, f.Acodec)
		}
		if realVideoCodec(f.Vcodec) {
			insertCodec(agg.videoCodecs, f.Vcodec)
		}
		if audioTrack != nil && audioTrack.ID != "auto" {
			agg.audioTracks[audioTrack.ID] = true
		}
		if videoTrack != nil && videoTrack.ID != "auto" {
			agg.videoTracks[videoTrack.ID] = true
		}
		if size := formatSize(f); size > agg.format.Filesize {
			agg.format.Filesize = size
		}
	}

	if len(v.AudioTracks) == 0 {
		v.AudioTracks = []Track{autoTrack()}
	}
	if len(v.VideoTracks) == 0 {
		v.VideoTracks = []Track{autoTrack()}
	}
	sortTracks(v.AudioTracks)
	sortTracks(v.VideoTracks)
	v.VideoCodecs = sortedCodecs(videoCodecs)
	v.AudioCodecs = sortedCodecs(audioCodecs)

	for _, agg := range aggregates {
		f := agg.format
		f.AudioCodecs = sortedCodecs(agg.audioCodecs)
		f.VideoCodecs = sortedCodecs(agg.videoCodecs)
		f.AudioTrackIDs = sortedKeys(agg.audioTracks)
		f.VideoTrackIDs = sortedKeys(agg.videoTracks)
		v.Formats = append(v.Formats, f)
	}
	sort.Slice(v.Formats, func(i, j int) bool { return formatLess(v.Formats[i], v.Formats[j]) })
}

// formatSize is the format's size, or its estimate, or 0.
func formatSize(f *InfoFormat) int64 {
	if f.Filesize != nil && *f.Filesize > 0 {
		return *f.Filesize
	}
	if f.FilesizeApprox != nil && *f.FilesizeApprox > 0 {
		return *f.FilesizeApprox
	}
	return 0
}

// formatGroup groups video formats by height and rounded fps, and audio-only
// formats by rounded bitrate. Storyboards and formats with both a height and
// a bitrate (muxed) are left out, as in the old app.
func formatGroup(f *InfoFormat) (string, Format, bool) {
	if f.Ext == "mhtml" || f.FormatID == "" {
		return "", Format{}, false
	}
	abr := f.Abr
	if abr != nil && *abr == 0 {
		abr = nil
	}
	fps := f.Fps
	if fps != nil && *fps == 0 {
		fps = nil
	}
	switch {
	case f.Height != nil && abr == nil:
		g := Format{ID: f.FormatID}
		if *f.Height >= 0 {
			g.Height = int(*f.Height)
		}
		fpsKey := "none"
		if fps != nil {
			g.Fps = int(math.Round(*fps))
			fpsKey = strconv.Itoa(g.Fps)
		}
		return fmt.Sprintf("%dx%s", *f.Height, fpsKey), g, true
	case f.Height == nil && abr != nil && fps == nil:
		g := Format{ID: f.FormatID, Abr: int(math.Round(*abr))}
		return strconv.Itoa(g.Abr), g, true
	}
	return "", Format{}, false
}

// formatLess puts video before audio, then higher height, fps or bitrate
// first, then by id.
func formatLess(a, b Format) bool {
	switch {
	case a.Height > 0 && b.Height > 0:
		if a.Height != b.Height {
			return a.Height > b.Height
		}
		if a.Fps != b.Fps {
			return a.Fps > b.Fps
		}
	case a.Height > 0:
		return true
	case b.Height > 0:
		return false
	case a.Abr != b.Abr:
		return a.Abr > b.Abr
	}
	return a.ID < b.ID
}

func realVideoCodec(c string) bool {
	c = strings.ToLower(strings.TrimSpace(c))
	return c != "" && c != "none" && c != "images"
}

func realAudioCodec(c string) bool { return c != "" && c != "none" }

func storyboard(f *InfoFormat) bool {
	return strings.Contains(strings.ToLower(f.FormatNote), "storyboard") || strings.Contains(strings.ToLower(f.Format), "storyboard")
}

func hasVideoStream(f *InfoFormat) bool {
	return !storyboard(f) && (realVideoCodec(f.Vcodec) || f.Height != nil)
}

func toTrack(f *InfoFormat) Track {
	t := Track{
		Language: strings.TrimSpace(f.Language), FormatNote: strings.TrimSpace(f.FormatNote),
		Format: strings.TrimSpace(f.Format), LanguagePreference: f.LanguagePreference,
	}
	if f.AudioChannels != nil {
		if ch := int(math.Round(*f.AudioChannels)); ch > 0 {
			t.AudioChannels = ch
		}
	}
	var ids, labels []string
	if t.Language != "" {
		ids, labels = append(ids, "lang:"+t.Language), append(labels, t.Language)
	}
	if t.AudioChannels > 0 {
		ids = append(ids, fmt.Sprintf("channels:%d", t.AudioChannels))
		labels = append(labels, fmt.Sprintf("%dch", t.AudioChannels))
	}
	t.ID, t.Label = "auto", "Auto"
	if len(ids) > 0 {
		t.ID, t.Label = strings.Join(ids, "|"), strings.Join(labels, " - ")
	}
	return t
}

func autoTrack() Track { return Track{ID: "auto", Label: "Auto"} }

// sortTracks puts preferred languages first and "auto" last.
func sortTracks(tracks []Track) {
	pref := func(t Track) int64 {
		if t.LanguagePreference == nil {
			return math.MinInt64
		}
		return *t.LanguagePreference
	}
	sort.SliceStable(tracks, func(i, j int) bool {
		a, b := tracks[i], tracks[j]
		if (a.ID == "auto") != (b.ID == "auto") {
			return b.ID == "auto"
		}
		if pref(a) != pref(b) {
			return pref(a) > pref(b)
		}
		return a.Label < b.Label
	})
}

func sortedKeys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func insertCodec(target map[string]Codec, codec string) {
	id := strings.TrimSpace(codec)
	if id == "" {
		return
	}
	key := strings.ToLower(id)
	if _, ok := target[key]; !ok {
		target[key] = Codec{ID: id, Label: CodecLabel(id)}
	}
}

func sortedCodecs(m map[string]Codec) []Codec {
	out := []Codec{}
	for _, c := range m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// CodecLabel names a codec id: "avc1.640028" is "H.264 High, Level 4.0".
func CodecLabel(codec string) string {
	raw := strings.TrimSpace(codec)
	lower := strings.ToLower(raw)
	var label string
	switch {
	case strings.HasPrefix(lower, "av01."):
		label = av1Label(raw)
	case strings.HasPrefix(lower, "avc1."):
		label = avc1Label(lower)
	case strings.HasPrefix(lower, "mp4a."):
		label = mp4aLabel(raw)
	}
	if label != "" {
		return label
	}
	switch lower {
	case "aac", "mp4a":
		return "AAC"
	case "flac":
		return "FLAC"
	case "mp3":
		return "MP3"
	case "opus":
		return "Opus"
	case "vorbis":
		return "Vorbis"
	case "vp9", "vp09":
		return "VP9"
	}
	return raw
}

func av1Label(codec string) string {
	parts := strings.Split(codec, ".")
	if len(parts) != 4 || len(parts[2]) < 3 {
		return ""
	}
	level, err := strconv.Atoi(parts[2][:2])
	if err != nil || level < 0 {
		return ""
	}
	profile := "Main"
	if parts[1] != "0" {
		profile = "Profile " + parts[1]
	}
	return fmt.Sprintf("AV1 %s, Level %.1f, %s-bit", profile, float64(level)/4, parts[3])
}

func avc1Label(lower string) string {
	rest := strings.TrimPrefix(lower, "avc1.")
	if len(rest) != 6 {
		return ""
	}
	profiles := map[string]string{"42": "Baseline", "4d": "Main", "58": "Extended", "64": "High"}
	profile, ok := profiles[rest[:2]]
	if !ok {
		return ""
	}
	level, err := strconv.ParseUint(rest[4:6], 16, 32)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("H.264 %s, Level %.1f", profile, float64(level)/10)
}

func mp4aLabel(codec string) string {
	parts := strings.Split(codec, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return ""
	}
	oti := parts[1]
	if len(parts) == 2 {
		if oti == "40" {
			return "MPEG-4 Audio"
		}
		return "MPEG-4 Audio (mp4a." + oti + ")"
	}
	aot, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return ""
	}
	names := map[uint64]string{
		1: "AAC Main", 2: "AAC LC", 3: "AAC SSR", 4: "AAC LTP", 5: "HE-AAC v1", 6: "AAC Scalable",
		17: "ER AAC LC", 19: "ER AAC LTP", 20: "ER AAC Scalable", 23: "ER AAC ELD", 29: "HE-AAC v2",
	}
	name, ok := names[aot]
	if !ok {
		return fmt.Sprintf("MPEG-4 Audio (mp4a.%s.%d)", oti, aot)
	}
	if oti == "40" {
		return name
	}
	return fmt.Sprintf("%s (mp4a.%s.%d)", name, oti, aot)
}
