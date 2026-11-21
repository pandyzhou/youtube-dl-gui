package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// The queue is kept between runs as JSON: each job with what it fetched,
// so it comes back without asking the site again. Downloads that were
// running come back paused; starting them again lets yt-dlp go on from
// its partial file.

const savedVersion = 1

type savedQueue struct {
	Version int        `json:"version"`
	Jobs    []savedJob `json:"jobs"`
}

type savedJob struct {
	URL      string                   `json:"url"`
	Phase    Status                   `json:"phase"`
	Err      string                   `json:"err,omitempty"`
	Playlist *ytdlp.Playlist          `json:"playlist,omitempty"`
	Combined bool                     `json:"combined,omitempty"`
	Preset   string                   `json:"preset"`
	Custom   *ytdlp.FormatOptions     `json:"custom,omitempty"`
	Existing string                   `json:"existing,omitempty"`
	Filters  ytdlp.InputFilterOptions `json:"filters"`
	Added    time.Time                `json:"added"`
	Items    []savedItem              `json:"items"`
}

type savedItem struct {
	URL           string               `json:"url"`
	Video         *ytdlp.Video         `json:"video,omitempty"`
	PlaylistIndex int                  `json:"playlistIndex"`
	Status        Status               `json:"status"`
	Destination   ytdlp.Destination    `json:"destination"`
	Diagnostics   []ytdlp.Diagnostic   `json:"diagnostics,omitempty"`
	Err           string               `json:"err,omitempty"`
	Template      ytdlp.TemplateValues `json:"template,omitempty"`
}

// Snapshot is the queue as Restore takes it back.
func (e *Engine) Snapshot() ([]byte, error) {
	e.mu.Lock()
	q := savedQueue{Version: savedVersion, Jobs: make([]savedJob, 0, len(e.jobs))}
	for _, j := range e.jobs {
		sj := savedJob{
			URL: j.url, Phase: j.phase, Err: j.err, Playlist: j.playlist, Combined: j.combined,
			Preset: j.preset, Custom: j.custom, Existing: j.existing, Filters: j.filters, Added: j.added,
		}
		for _, it := range j.items {
			status := it.status
			if status.Active() {
				status = StatusPaused
			}
			sj.Items = append(sj.Items, savedItem{
				URL: it.url, Video: it.video, PlaylistIndex: it.playlistIndex, Status: status,
				Destination: it.destination, Diagnostics: it.diagnostics, Err: it.err, Template: it.template,
			})
		}
		q.Jobs = append(q.Jobs, sj)
	}
	e.mu.Unlock()
	return json.Marshal(q)
}

// Restore adds the jobs of a snapshot after any already queued and
// returns how many. Jobs still getting their information get it again.
func (e *Engine) Restore(data []byte) (int, error) {
	var q savedQueue
	if err := json.Unmarshal(data, &q); err != nil {
		return 0, err
	}
	if q.Version != savedVersion {
		return 0, fmt.Errorf("queue version %d", q.Version)
	}
	var refetch []*job
	e.mu.Lock()
	for _, sj := range q.Jobs {
		j := &job{
			id: e.newID("j"), url: sj.URL, phase: sj.Phase, err: sj.Err, playlist: sj.Playlist,
			combined: sj.Combined, preset: sj.Preset, custom: sj.Custom, existing: sj.Existing,
			autoStart: sj.Existing != "", filters: sj.Filters, added: sj.Added,
		}
		j.fetchCtx, j.fetchCancel = context.WithCancel(context.Background())
		for _, si := range sj.Items {
			it := e.newItem(si.URL, si.PlaylistIndex)
			it.video, it.status, it.destination = si.Video, si.Status, si.Destination
			it.diagnostics, it.err, it.template = si.Diagnostics, si.Err, si.Template
			if it.status.Active() {
				it.status = StatusPaused
			}
			if it.video == nil && it.status == StatusFetching {
				it.status, it.err = StatusError, "上次退出时还没获取到视频信息，请重试"
			}
			j.items = append(j.items, it)
		}
		switch {
		case j.phase == StatusFetching || j.phase == StatusFetchingList || len(j.items) == 0 && j.phase != StatusSelecting:
			// Fetch it all again, as Retry does.
			j.phase, j.err, j.playlist, j.combined = StatusFetching, "", nil, false
			j.items = []*item{e.newItem(j.url, -1)}
			refetch = append(refetch, j)
		case j.phase == StatusSelecting && j.playlist == nil:
			j.phase, j.err = StatusFetching, ""
			j.items = []*item{e.newItem(j.url, -1)}
			refetch = append(refetch, j)
		}
		e.jobs = append(e.jobs, j)
	}
	e.mu.Unlock()
	for _, j := range refetch {
		e.fetchJob(j)
	}
	e.changed()
	return len(q.Jobs), nil
}
