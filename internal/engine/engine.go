package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
	"github.com/pandyzhou/youtube-dl-gui/internal/proc"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// Limits on what a job keeps.
const (
	maxLogLines    = 4000
	maxDiagnostics = 50
)

type item struct {
	id            string
	url           string
	video         *ytdlp.Video
	playlistIndex int
	status        Status
	stage         ytdlp.Stage
	percent       float64
	speed         float64
	eta           int64
	destination   ytdlp.Destination
	diagnostics   []ytdlp.Diagnostic
	err           string
	template      ytdlp.TemplateValues
}

type job struct {
	id        string
	url       string
	phase     Status // fetching, selecting, fetchingList, ready or error
	err       string
	playlist  *ytdlp.Playlist
	combined  bool
	items     []*item
	preset    string
	custom    *ytdlp.FormatOptions
	autoStart bool
	filters   ytdlp.InputFilterOptions
	added     time.Time
	log       []string

	fetchCtx    context.Context
	fetchCancel context.CancelFunc
	runCtx      context.Context
	runCancel   context.CancelFunc
	pending     int // playlist videos still being fetched
}

// Engine is the queue. Its methods are safe from any goroutine.
type Engine struct {
	d Deps

	mu        sync.Mutex
	jobs      []*job
	nextID    int
	autonum   int
	perList   map[string]int
	downloads *pool
	fetches   *pool

	notifyMu      sync.Mutex
	notifyPending bool
}

// New starts an engine.
func New(d Deps) *Engine {
	limit := max(d.Settings().Performance.MaxConcurrency, 1)
	return &Engine{d: d, perList: map[string]int{}, downloads: newPool(limit), fetches: newPool(limit)}
}

// Close cancels everything and stops the engine.
func (e *Engine) Close() {
	e.mu.Lock()
	for _, j := range e.jobs {
		j.cancelAll()
	}
	e.mu.Unlock()
	e.downloads.close()
	e.fetches.close()
}

// SetConcurrency changes how many fetches and downloads run at once.
func (e *Engine) SetConcurrency(n int) {
	e.downloads.setLimit(n)
	e.fetches.setLimit(n)
}

func (e *Engine) newID(prefix string) string {
	e.nextID++
	return prefix + strconv.Itoa(e.nextID)
}

// changed asks for a redraw, at most once per Throttle.
func (e *Engine) changed() {
	if e.d.OnChange == nil {
		return
	}
	if e.d.Throttle <= 0 {
		e.d.OnChange()
		return
	}
	e.notifyMu.Lock()
	defer e.notifyMu.Unlock()
	if e.notifyPending {
		return
	}
	e.notifyPending = true
	time.AfterFunc(e.d.Throttle, func() {
		e.notifyMu.Lock()
		e.notifyPending = false
		e.notifyMu.Unlock()
		e.d.OnChange()
	})
}

func (e *Engine) event(ev Event) {
	if e.d.OnEvent != nil {
		e.d.OnEvent(ev)
	}
}

// Add queues a link and starts fetching its information. Jobs are listed
// newest first.
func (e *Engine) Add(url string, o AddOptions) string {
	settings := e.d.Settings()
	preset := o.Preset
	if preset == "" {
		preset = settings.Input.DefaultPreset
	}
	e.mu.Lock()
	j := &job{
		id: e.newID("j"), url: strings.TrimSpace(url), phase: StatusFetching, preset: preset,
		autoStart: o.Start, filters: formats.InputFilters(settings.InputFilters), added: time.Now(),
	}
	j.fetchCtx, j.fetchCancel = context.WithCancel(context.Background())
	j.items = []*item{e.newItem(j.url, -1)}
	e.jobs = append([]*job{j}, e.jobs...)
	e.mu.Unlock()
	e.changed()
	e.fetchJob(j)
	return j.id
}

func (e *Engine) newItem(url string, playlistIndex int) *item {
	return &item{id: e.newID("i"), url: url, playlistIndex: playlistIndex, status: StatusFetching, eta: -1}
}

func (e *Engine) find(id string) *job {
	for _, j := range e.jobs {
		if j.id == id {
			return j
		}
	}
	return nil
}

func (e *Engine) alive(j *job) bool { return e.find(j.id) == j }

// infoArgs are the arguments of an information fetch.
func (e *Engine) infoArgs(url string, filters ytdlp.InputFilterOptions, settings config.Settings) []string {
	args := []string{"--encoding", "utf-8"}
	args = append(args, ytdlp.FormatArgs(ytdlp.FormatOptions{TrackType: ytdlp.TrackBoth}, settings.Output)...)
	args = append(args, ytdlp.InputArgs(filters.PlaylistMode, settings.Input)...)
	args = append(args, ytdlp.InputFilterArgs(filters)...)
	args = append(args, ytdlp.AuthArgs(settings.Auth, e.secrets())...)
	args = append(args, ytdlp.NetworkArgs(settings.Network)...)
	return append(args, "-J", "--flat-playlist", url)
}

func (e *Engine) secrets() ytdlp.AuthSecrets {
	if e.d.Secrets == nil {
		return ytdlp.AuthSecrets{}
	}
	return e.d.Secrets()
}

// fetchInfo runs yt-dlp -J. It returns the parsed media, or an error
// message with the diagnostics yt-dlp printed.
func (e *Engine) fetchInfo(ctx context.Context, j *job, url string, filters ytdlp.InputFilterOptions) (*ytdlp.Media, []ytdlp.Diagnostic, error) {
	settings := e.d.Settings()
	var stdout strings.Builder
	var diags []ytdlp.Diagnostic
	code, err := e.d.Runner.Run(ctx, e.infoArgs(url, filters, settings), func(l proc.Line) {
		if !l.Stderr && strings.HasPrefix(l.Text, "{") {
			stdout.Reset()
			stdout.WriteString(l.Text)
			return
		}
		e.appendLog(j, l.Text)
		if d := ytdlp.ParseDiagnostic(l.Text); d != nil {
			diags = append(diags, *d)
		}
	})
	if err != nil {
		return nil, diags, err
	}
	if code != 0 {
		return nil, diags, fmt.Errorf("yt-dlp 退出代码 %d", code)
	}
	media, err := ytdlp.ParseInfo([]byte(stdout.String()))
	if err != nil {
		return nil, diags, err
	}
	return media, diags, nil
}

func (e *Engine) appendLog(j *job, line string) {
	if line == "" || strings.HasPrefix(line, "RAW|") {
		return
	}
	e.mu.Lock()
	j.log = append(j.log, line)
	if over := len(j.log) - maxLogLines; over > 0 {
		j.log = append(j.log[:0:0], j.log[over:]...)
	}
	e.mu.Unlock()
}

// fetchJob gets the information of a job's link.
func (e *Engine) fetchJob(j *job) {
	e.fetches.submit(j.id, j.fetchCtx, func(ctx context.Context) {
		media, diags, err := e.fetchInfo(ctx, j, j.url, j.filters)
		if ctx.Err() != nil {
			return
		}
		e.mu.Lock()
		if !e.alive(j) {
			e.mu.Unlock()
			return
		}
		var after func()
		switch {
		case err != nil:
			j.fail(errorMessage(err, diags), diags)
		case media.Kind == ytdlp.KindLivestream:
			j.fail("暂不支持下载直播", nil)
		case media.Kind == ytdlp.KindPlaylist && len(media.Playlist.Entries) == 0:
			j.fail("播放列表是空的", nil)
		case media.Kind == ytdlp.KindPlaylist:
			j.playlist = media.Playlist
			j.items = nil
			if j.autoStart {
				after = e.expandLocked(j, nil)
			} else {
				j.phase = StatusSelecting
			}
		default:
			it := j.items[0]
			it.video, it.status, it.diagnostics = media.Video, StatusReady, diags
			j.phase = StatusReady
			title := media.Video.Title
			after = func() { e.event(Event{Kind: EventVideoReady, JobID: j.id, Title: title, Count: 1}) }
			if j.autoStart {
				e.startLocked(j)
			}
		}
		e.mu.Unlock()
		if after != nil {
			after()
		}
		e.changed()
	})
}

// errorMessage picks the most telling message for a failed run: the last
// error yt-dlp printed, or err.
func errorMessage(err error, diags []ytdlp.Diagnostic) string {
	for i := len(diags) - 1; i >= 0; i-- {
		if diags[i].Level == ytdlp.LevelError {
			return diags[i].Message
		}
	}
	return err.Error()
}

func (j *job) fail(msg string, diags []ytdlp.Diagnostic) {
	j.phase, j.err = StatusError, msg
	for _, it := range j.items {
		it.status, it.err = StatusError, msg
		it.diagnostics = appendDiagnostics(it.diagnostics, diags...)
	}
}

func (j *job) cancelAll() {
	if j.fetchCancel != nil {
		j.fetchCancel()
	}
	if j.runCancel != nil {
		j.runCancel()
	}
}

// SelectPlaylist takes the chosen videos of a playlist (no rows: all) and
// fetches their information; start downloads them once it is in.
func (e *Engine) SelectPlaylist(jobID string, rows []formats.SelectionRow, start bool) error {
	e.mu.Lock()
	j := e.find(jobID)
	if j == nil || j.phase != StatusSelecting {
		e.mu.Unlock()
		return errors.New("这个项目不在选择播放列表的步骤")
	}
	if len(formats.SelectEntries(j.playlist.Entries, rows)) == 0 {
		e.mu.Unlock()
		return errors.New("没有选中任何视频")
	}
	j.autoStart = j.autoStart || start
	after := e.expandLocked(j, rows)
	e.mu.Unlock()
	if after != nil {
		after()
	}
	e.changed()
	return nil
}

// expandLocked starts fetching the selected videos of a playlist. It
// returns nothing to run after unlocking; the signature matches the other
// steps.
func (e *Engine) expandLocked(j *job, rows []formats.SelectionRow) func() {
	entries := formats.SelectEntries(j.playlist.Entries, rows)
	j.filters.PlaylistItems = formats.PlaylistItemsSpec(rows)
	j.phase = StatusFetchingList
	j.items = nil
	for _, entry := range entries {
		j.items = append(j.items, e.newItem(entry.URL, entry.Index))
	}
	j.pending = len(j.items)
	filters := j.filters
	filters.PlaylistItems, filters.PlaylistMode = "", ytdlp.PlaylistModeSingle
	for _, it := range j.items {
		it := it
		e.fetches.submit(j.id, j.fetchCtx, func(ctx context.Context) { e.fetchEntry(ctx, j, it, filters) })
	}
	return nil
}

func (e *Engine) fetchEntry(ctx context.Context, j *job, it *item, filters ytdlp.InputFilterOptions) {
	media, diags, err := e.fetchInfo(ctx, j, it.url, filters)
	if ctx.Err() != nil {
		return
	}
	e.mu.Lock()
	if !e.alive(j) {
		e.mu.Unlock()
		return
	}
	switch {
	case err != nil:
		it.status, it.err = StatusError, errorMessage(err, diags)
		it.diagnostics = appendDiagnostics(it.diagnostics, diags...)
	case media.Kind != ytdlp.KindSingle:
		it.status, it.err = StatusError, "不是单个视频"
	default:
		it.video, it.status, it.diagnostics = media.Video, StatusReady, diags
	}
	j.pending--
	var after func()
	if j.pending == 0 && j.phase == StatusFetchingList {
		after = e.finishPlaylistLocked(j)
	}
	e.mu.Unlock()
	if after != nil {
		after()
	}
	e.changed()
}

// finishPlaylistLocked splits a playlist below the split threshold into a
// card per video, in playlist order where the playlist was, or keeps it as
// one card.
func (e *Engine) finishPlaylistLocked(j *job) func() {
	threshold := e.d.Settings().Performance.SplitPlaylistThreshold
	title, count := j.playlist.Title, len(j.items)
	ready := func() { e.event(Event{Kind: EventPlaylistReady, JobID: j.id, Title: title, Count: count}) }
	if count >= threshold {
		j.combined, j.phase = true, StatusReady
		if j.autoStart {
			e.startLocked(j)
		}
		return ready
	}
	pos := 0
	for i, other := range e.jobs {
		if other == j {
			pos = i
		}
	}
	var split []*job
	for _, it := range j.items {
		nj := &job{
			id: e.newID("j"), url: it.url, phase: StatusReady, playlist: j.playlist, items: []*item{it},
			preset: j.preset, custom: j.custom, autoStart: j.autoStart, filters: j.filters, added: j.added,
		}
		nj.filters.PlaylistItems = ""
		if it.status == StatusError {
			nj.phase, nj.err = StatusError, it.err
		}
		nj.fetchCtx, nj.fetchCancel = context.WithCancel(context.Background())
		split = append(split, nj)
	}
	j.cancelAll()
	rest := append(split, e.jobs[pos+1:]...)
	e.jobs = append(e.jobs[:pos:pos], rest...)
	for _, nj := range split {
		if nj.autoStart && nj.phase == StatusReady {
			e.startLocked(nj)
		}
	}
	return ready
}

// SetPreset changes a job's quality preset; it applies to videos that have
// not started.
func (e *Engine) SetPreset(jobID, preset string) {
	e.mu.Lock()
	if j := e.find(jobID); j != nil {
		j.preset, j.custom = preset, nil
	}
	e.mu.Unlock()
	e.changed()
}

// SetCustom gives a job an explicit format instead of a preset.
func (e *Engine) SetCustom(jobID string, f ytdlp.FormatOptions) {
	e.mu.Lock()
	if j := e.find(jobID); j != nil {
		j.preset, j.custom = formats.PresetCustom, &f
	}
	e.mu.Unlock()
	e.changed()
}

// Start downloads a job's videos that are ready, paused or failed.
func (e *Engine) Start(jobID string) {
	e.mu.Lock()
	if j := e.find(jobID); j != nil {
		e.startLocked(j)
	}
	e.mu.Unlock()
	e.changed()
}

// StartAll starts every job that is ready or paused and returns how many.
func (e *Engine) StartAll() int {
	e.mu.Lock()
	n := 0
	for i := len(e.jobs) - 1; i >= 0; i-- { // oldest first
		j := e.jobs[i]
		if s := j.status(); s == StatusReady || s == StatusPaused {
			e.startLocked(j)
			n++
		}
	}
	e.mu.Unlock()
	e.changed()
	return n
}

func (e *Engine) startLocked(j *job) {
	if j.phase != StatusReady {
		return
	}
	if j.runCtx == nil || j.runCtx.Err() != nil {
		j.runCtx, j.runCancel = context.WithCancel(context.Background())
	}
	settings := e.d.Settings()
	for _, it := range j.items {
		if it.video == nil || it.status.Active() || it.status == StatusDone || it.status == StatusSkipped {
			continue
		}
		it.status, it.err, it.stage, it.percent, it.speed, it.eta = StatusQueued, "", "", 0, 0, -1
		it.template = e.templateValues(j, it, settings)
		it := it
		e.downloads.submit(j.id, j.runCtx, func(ctx context.Context) { e.download(ctx, j, it) })
	}
}

// templateValues fills the output template fields of a playlist video;
// autonumber counts downloads started this session.
func (e *Engine) templateValues(j *job, it *item, settings config.Settings) ytdlp.TemplateValues {
	e.autonum++
	v := ytdlp.TemplateValues{"autonumber": strconv.Itoa(e.autonum)}
	p := j.playlist
	if p == nil || it.playlistIndex < 0 {
		return v
	}
	count := 0
	if p.PlaylistCount != nil {
		count = int(*p.PlaylistCount)
	}
	e.perList[p.PlaylistID]++
	v["playlist_autonumber"] = strconv.Itoa(e.perList[p.PlaylistID])
	v["playlist_index"] = strconv.Itoa(formats.PlaylistIndex(it.playlistIndex, count, settings.Output.ReversePlaylistNumbering))
	v["playlist_id"] = p.PlaylistID
	v["playlist_title"] = p.Title
	v["playlist"] = p.Title
	v["playlist_uploader"] = p.Uploader
	v["playlist_uploader_id"] = p.UploaderID
	if count > 0 {
		v["playlist_count"] = strconv.Itoa(count)
	}
	v["n_entries"] = strconv.Itoa(len(j.items))
	for k, val := range v {
		if val == "" {
			delete(v, k) // unknown: leave the field to yt-dlp
		}
	}
	return v
}

// choice resolves a job's quality for one video.
func choice(j *job, video *ytdlp.Video) formats.Choice {
	if j.custom != nil {
		return formats.Choice{Format: *j.custom}
	}
	var fs []ytdlp.Format
	if video != nil {
		fs = video.Formats
	}
	return formats.Resolve(formats.PresetByID(j.preset), fs)
}

// downloadArgs are the arguments of a download, in the old app's order.
func (e *Engine) downloadArgs(j *job, it *item, settings config.Settings) ([]string, ytdlp.TrackType, error) {
	c := choice(j, it.video)
	output := c.ApplyAudioFormat(settings.Output)
	filters := j.filters
	filters.PlaylistItems = ""
	if it.playlistIndex >= 0 {
		filters.PlaylistMode = ytdlp.PlaylistModeSingle
	}
	var inventory *ytdlp.SubtitleInventory
	if it.video != nil {
		inventory = &it.video.SubtitleInventory
	}
	args := []string{"--encoding", "utf-8"}
	args = append(args, ytdlp.ProgressArgs()...)
	args = append(args, ytdlp.PerformanceArgs(settings.Performance.ConcurrentFragments)...)
	args = append(args, ytdlp.NetworkArgs(settings.Network)...)
	args = append(args, ytdlp.AuthArgs(settings.Auth, e.secrets())...)
	args = append(args, ytdlp.SubtitleArgs(settings.Subtitles, inventory)...)
	args = append(args, ytdlp.FormatArgs(c.Format, output)...)
	args = append(args, ytdlp.InputFilterArgs(filters)...)
	args = append(args, ytdlp.InputArgs(filters.PlaylistMode, settings.Input)...)
	outArgs, err := ytdlp.OutputArgs(c.Format, output)
	if err != nil {
		return nil, "", err
	}
	args = append(args, outArgs...)
	prefs := config.Preferences{}
	if e.d.Preferences != nil {
		prefs = e.d.Preferences()
	}
	args = append(args, ytdlp.LocationArgs(c.Format.TrackType, it.template, output, prefs.Paths, e.d.FallbackDir)...)
	return append(args, it.url), c.Format.TrackType, nil
}

func (e *Engine) download(ctx context.Context, j *job, it *item) {
	if e.d.WaitTools != nil {
		if err := e.d.WaitTools(ctx); err != nil {
			e.finishDownload(ctx, j, it, -1, err)
			return
		}
	}
	settings := e.d.Settings()
	e.mu.Lock()
	if !e.alive(j) || it.status != StatusQueued {
		e.mu.Unlock()
		return
	}
	args, track, err := e.downloadArgs(j, it, settings)
	if err == nil {
		it.status, it.stage = StatusDownloading, ytdlp.StageInitializing
	}
	e.mu.Unlock()
	if err != nil {
		e.finishDownload(ctx, j, it, -1, fmt.Errorf("下载设置有误：%w", err))
		return
	}
	e.changed()
	slog.Info("download", "job", j.id, "item", it.id, "summary", ytdlp.Summarize(args))

	parser := ytdlp.NewProgressParser(track)
	code, err := e.d.Runner.Run(ctx, args, func(l proc.Line) {
		e.appendLog(j, l.Text)
		ev := parser.ParseLine(l.Text)
		d := ytdlp.ParseDiagnostic(l.Text)
		if ev.Destination == nil && ev.Stage == "" && ev.Progress == nil && d == nil {
			return
		}
		e.mu.Lock()
		if ev.Destination != nil && ev.Destination.Confidence >= it.destination.Confidence {
			it.destination = *ev.Destination
		}
		if ev.Stage != "" {
			it.stage = ev.Stage
		}
		if p := ev.Progress; p != nil && (p.Category == ytdlp.CategoryVideo || p.Category == ytdlp.CategoryAudio) {
			if p.Percent != nil {
				it.percent = *p.Percent
			}
			if p.SpeedBps != nil {
				it.speed = *p.SpeedBps
			}
			if p.EtaSecs != nil {
				it.eta = *p.EtaSecs
			}
		}
		if d != nil {
			it.diagnostics = appendDiagnostics(it.diagnostics, *d)
		}
		e.mu.Unlock()
		e.changed()
	})
	if err == nil && code == 0 && e.d.AfterDownload != nil && ctx.Err() == nil {
		e.mu.Lock()
		fin := Finished{JobID: j.id, ItemID: it.id, Destination: it.destination.Path, Video: it.video}
		if it.video != nil {
			fin.Title = it.video.Title
		}
		e.mu.Unlock()
		err = e.d.AfterDownload(ctx, fin, func(s ytdlp.Stage) {
			e.mu.Lock()
			it.stage = s
			e.mu.Unlock()
			e.changed()
		})
		if err != nil {
			err = fmt.Errorf("下载完成，但后续处理失败：%w", err)
		}
	}
	e.finishDownload(ctx, j, it, code, err)
}

func (e *Engine) finishDownload(ctx context.Context, j *job, it *item, code int, err error) {
	e.mu.Lock()
	if !e.alive(j) {
		e.mu.Unlock()
		return
	}
	var events []Event
	title := it.url
	if it.video != nil {
		title = it.video.Title
	}
	switch {
	case ctx.Err() != nil && j.runCtx != nil && j.runCtx != ctx && j.runCtx.Err() == nil:
		// Paused and started again while yt-dlp was ending: run again.
		it.status, it.stage, it.speed, it.eta = StatusQueued, "", 0, -1
		runCtx := j.runCtx
		e.downloads.submit(j.id, runCtx, func(ctx context.Context) { e.download(ctx, j, it) })
	case ctx.Err() != nil:
		it.status, it.stage, it.speed, it.eta = StatusPaused, "", 0, -1
	case err != nil:
		it.status, it.err = StatusError, err.Error()
		events = append(events, Event{Kind: EventDownloadError, JobID: j.id, Title: title})
	case code != 0:
		it.status = StatusError
		it.err = errorMessage(fmt.Errorf("yt-dlp 退出代码 %d", code), it.diagnostics)
		events = append(events, Event{Kind: EventDownloadError, JobID: j.id, Title: title})
	case it.destination.Path == "" && hasCode(it.diagnostics, ytdlp.CodeInputFilterSkipped):
		it.status, it.stage = StatusSkipped, ""
	default:
		it.status, it.stage, it.percent, it.speed, it.eta = StatusDone, "", 100, 0, -1
		if !j.combined || j.status() == StatusDone {
			n := 1
			if j.combined {
				n = len(j.items)
			}
			events = append(events, Event{Kind: EventDownloadDone, JobID: j.id, Title: j.title(), Count: n})
		}
	}
	if ctx.Err() == nil && !e.anyActiveLocked() {
		events = append(events, Event{Kind: EventQueueDone})
	}
	e.mu.Unlock()
	for _, ev := range events {
		e.event(ev)
	}
	e.changed()
}

func (e *Engine) anyActiveLocked() bool {
	for _, j := range e.jobs {
		for _, it := range j.items {
			if it.status.Active() {
				return true
			}
		}
	}
	return false
}

func hasCode(diags []ytdlp.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func appendDiagnostics(list []ytdlp.Diagnostic, add ...ytdlp.Diagnostic) []ytdlp.Diagnostic {
	for _, d := range add {
		dup := false
		for _, have := range list {
			dup = dup || (have.Code == d.Code && have.Message == d.Message)
		}
		if !dup && len(list) < maxDiagnostics {
			list = append(list, d)
		}
	}
	return list
}

// Pause stops a job's downloads; Start continues them, and yt-dlp resumes
// the partial files.
func (e *Engine) Pause(jobID string) {
	e.mu.Lock()
	if j := e.find(jobID); j != nil {
		e.pauseLocked(j)
	}
	e.mu.Unlock()
	e.changed()
}

// PauseAll pauses every running job.
func (e *Engine) PauseAll() {
	e.mu.Lock()
	for _, j := range e.jobs {
		e.pauseLocked(j)
	}
	e.mu.Unlock()
	e.changed()
}

func (e *Engine) pauseLocked(j *job) {
	if j.runCancel == nil {
		return
	}
	j.runCancel()
	for _, it := range j.items {
		if it.status == StatusQueued {
			it.status = StatusPaused // a running one turns paused when yt-dlp ends
		}
	}
}

// Retry fetches a failed link again, or restarts failed downloads.
func (e *Engine) Retry(jobID string) {
	e.mu.Lock()
	j := e.find(jobID)
	if j == nil {
		e.mu.Unlock()
		return
	}
	needsFetch := j.phase == StatusError && (len(j.items) == 0 || j.items[0].video == nil)
	if needsFetch {
		j.phase, j.err = StatusFetching, ""
		j.fetchCtx, j.fetchCancel = context.WithCancel(context.Background())
		j.items = []*item{e.newItem(j.url, playlistIndexOf(j))}
		e.mu.Unlock()
		e.changed()
		e.fetchJob(j)
		return
	}
	if j.phase == StatusReady {
		e.startLocked(j)
	}
	e.mu.Unlock()
	e.changed()
}

func playlistIndexOf(j *job) int {
	if len(j.items) == 1 {
		return j.items[0].playlistIndex
	}
	return -1
}

// Remove cancels a job and takes it off the queue.
func (e *Engine) Remove(jobID string) {
	e.mu.Lock()
	for i, j := range e.jobs {
		if j.id == jobID {
			j.cancelAll()
			e.jobs = append(e.jobs[:i], e.jobs[i+1:]...)
			e.downloads.drop(j.id)
			e.fetches.drop(j.id)
			break
		}
	}
	e.mu.Unlock()
	e.changed()
}

// RemoveWhere removes the jobs match reports true for and returns how many.
func (e *Engine) RemoveWhere(match func(JobView) bool) int {
	var ids []string
	for _, v := range e.Jobs() {
		if match(v) {
			ids = append(ids, v.ID)
		}
	}
	for _, id := range ids {
		e.Remove(id)
	}
	return len(ids)
}

// Log returns a job's yt-dlp output.
func (e *Engine) Log(jobID string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if j := e.find(jobID); j != nil {
		return append([]string(nil), j.log...)
	}
	return nil
}
