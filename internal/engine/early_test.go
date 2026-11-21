package engine

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBeforeDownloadRunsBesideTheDownload(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/a"] = videoJSON("a", "Video A")
	var mu sync.Mutex
	var started Started
	var got Finished
	h.e.d.BeforeDownload = func(ctx context.Context, s Started, r Reporter) any {
		mu.Lock()
		started = s
		mu.Unlock()
		r.Progress(40) // not shown while the download runs
		time.Sleep(20 * time.Millisecond)
		return "translated"
	}
	h.e.d.AfterDownload = func(ctx context.Context, d Finished, r Reporter) {
		mu.Lock()
		got = d
		mu.Unlock()
	}
	id := h.e.Add("https://v/a", AddOptions{Start: true})
	h.waitStatus(id, StatusDone)
	mu.Lock()
	defer mu.Unlock()
	if started.URL != "https://v/a" || started.Title != "Video A" || started.Video == nil {
		t.Errorf("started %+v", started)
	}
	if got.Early != "translated" || got.Destination == "" {
		t.Errorf("finished %+v", got)
	}
}

func TestBeforeDownloadCanceledWhenTheDownloadFails(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/bad"] = videoJSON("bad", "Bad")
	h.fake.failDown["https://v/bad"] = []string{"ERROR: boom"}
	canceled := make(chan bool, 1)
	h.e.d.BeforeDownload = func(ctx context.Context, s Started, r Reporter) any {
		select {
		case <-ctx.Done():
			canceled <- true
		case <-time.After(5 * time.Second):
			canceled <- false
		}
		return nil
	}
	h.e.d.AfterDownload = func(ctx context.Context, d Finished, r Reporter) {
		t.Error("AfterDownload ran for a failed download")
	}
	id := h.e.Add("https://v/bad", AddOptions{Start: true})
	h.waitStatus(id, StatusError)
	if !<-canceled {
		t.Error("BeforeDownload went on after the download failed")
	}
}
