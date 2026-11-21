package engine

import "github.com/pandyzhou/lumadl/internal/ytdlp"

// status is a job's status: its phase until it is ready, then what its
// videos are doing, the most urgent first.
func (j *job) status() Status {
	if j.phase != StatusReady {
		return j.phase
	}
	counts := map[Status]int{}
	for _, it := range j.items {
		counts[it.status]++
	}
	n := len(j.items)
	switch {
	case counts[StatusDownloading] > 0:
		return StatusDownloading
	case counts[StatusQueued] > 0:
		return StatusQueued
	case counts[StatusPaused] > 0:
		return StatusPaused
	case n > 0 && counts[StatusDone]+counts[StatusSkipped] == n:
		if counts[StatusDone] == 0 {
			return StatusSkipped
		}
		return StatusDone
	case counts[StatusError] > 0 && counts[StatusReady] == 0:
		return StatusError
	}
	return StatusReady
}

func (j *job) title() string {
	switch {
	case j.playlist != nil && (j.combined || len(j.items) != 1 || j.phase != StatusReady && j.phase != StatusError):
		if j.playlist.Title != "" {
			return j.playlist.Title
		}
	case len(j.items) == 1 && j.items[0].video != nil && j.items[0].video.Title != "":
		return j.items[0].video.Title
	}
	return j.url
}

// Jobs returns snapshots of the queue, newest first.
func (e *Engine) Jobs() []JobView {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]JobView, len(e.jobs))
	for i, j := range e.jobs {
		out[i] = j.view()
	}
	return out
}

// Job returns one job's snapshot.
func (e *Engine) Job(id string) (JobView, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if j := e.find(id); j != nil {
		return j.view(), true
	}
	return JobView{}, false
}

func (j *job) view() JobView {
	v := JobView{
		ID: j.id, URL: j.url, Status: j.status(), Title: j.title(), Playlist: j.playlist,
		Combined: j.combined, Preset: j.preset, Custom: j.custom, Err: j.err, Added: j.added,
		Existing: j.existing,
	}
	if j.playlist != nil {
		v.Thumbnail, v.Uploader = j.playlist.Thumbnail, j.playlist.Uploader
	}
	var percent float64
	for _, it := range j.items {
		iv := it.view(j)
		v.Items = append(v.Items, iv)
		v.Size += iv.Size
		switch it.status {
		case StatusDone, StatusSkipped:
			v.Done++
			percent += 100
		case StatusError:
			v.Failed++
		default:
			percent += iv.Percent
		}
		if it.video != nil {
			if it.video.Duration != nil {
				v.Duration += *it.video.Duration
			}
			if v.Thumbnail == "" || (!j.combined && len(j.items) == 1) {
				if it.video.Thumbnail != "" {
					v.Thumbnail = it.video.Thumbnail
				}
			}
			if v.Uploader == "" {
				v.Uploader = it.video.Uploader
			}
		}
	}
	if len(j.items) > 0 {
		v.Percent = percent / float64(len(j.items))
	}
	return v
}

func (it *item) view(j *job) ItemView {
	v := ItemView{
		ID: it.id, URL: it.url, Video: it.video, PlaylistIndex: it.playlistIndex, Status: it.status,
		Stage: it.stage, Percent: it.percent, SpeedBps: it.speed, EtaSecs: it.eta,
		Destination: it.destination.Path, Err: it.err,
		Diagnostics: append([]ytdlp.Diagnostic(nil), it.diagnostics...),
	}
	if it.video != nil {
		v.Title = it.video.Title
		c := choice(j, it.video)
		v.Choice, v.Size = c.Label(), c.Size
	} else {
		v.Title = it.url
	}
	return v
}
