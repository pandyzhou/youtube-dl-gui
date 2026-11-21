package engine

import (
	"testing"
)

func TestSnapshotAndRestore(t *testing.T) {
	h := newHarness(t, nil)
	h.fake.infos["https://v/done"] = videoJSON("done", "Done Video")
	h.fake.infos["https://v/ready"] = videoJSON("ready", "Ready Video")
	h.fake.infos["https://v/slow"] = videoJSON("slow", "Slow Video")
	h.fake.block["https://v/slow"] = true // its download hangs until canceled
	done := h.e.Add("https://v/done", AddOptions{Preset: "720p", Start: true})
	h.waitStatus(done, StatusDone)
	ready := h.e.Add("https://v/ready", AddOptions{Preset: "480p"})
	h.waitStatus(ready, StatusReady)
	slow := h.e.Add("https://v/slow", AddOptions{Start: true})
	h.waitStatus(slow, StatusDownloading)

	data, err := h.e.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	h2 := newHarness(t, nil)
	h2.fake.infos["https://v/slow"] = videoJSON("slow", "Slow Video")
	n, err := h2.e.Restore(data)
	if err != nil || n != 3 {
		t.Fatalf("restored %d, %v", n, err)
	}
	jobs := h2.e.Jobs()
	byURL := map[string]JobView{}
	for _, j := range jobs {
		byURL[j.URL] = j
	}
	if j := byURL["https://v/done"]; j.Status != StatusDone || j.Title != "Done Video" || j.Items[0].Destination == "" {
		t.Errorf("done job %+v", j)
	}
	if j := byURL["https://v/ready"]; j.Status != StatusReady || j.Preset != "480p" {
		t.Errorf("ready job %+v", j)
	}
	if j := byURL["https://v/slow"]; j.Status != StatusPaused {
		t.Errorf("running download should come back paused: %+v", j)
	}
	if len(h2.fake.downloads()) != 0 {
		t.Errorf("restoring downloaded %q", h2.fake.downloads())
	}
	// A paused one goes on when started.
	h2.e.Start(byURL["https://v/slow"].ID)
	h2.waitStatus(byURL["https://v/slow"].ID, StatusDone)

	if _, err := h2.e.Restore([]byte(`{"version": 99}`)); err == nil {
		t.Error("restored an unknown version")
	}
}
