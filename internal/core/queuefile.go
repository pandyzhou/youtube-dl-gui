package core

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
)

// QueueFile keeps the download queue between runs.
const QueueFile = "queue.json"

// RestoreQueue brings back the queue the last run left.
func RestoreQueue(c *Core, q *engine.Engine) {
	data, err := os.ReadFile(filepath.Join(c.Dirs.Data, QueueFile))
	if err != nil {
		return
	}
	n, err := q.Restore(data)
	if err != nil {
		slog.Warn("restore queue", "err", err)
		return
	}
	slog.Info("restored queue", "jobs", n)
}

// KeepQueue saves the queue whenever it changes, looking every few seconds.
// The returned stop saves it one last time; call it before the engine
// closes, while running downloads still count as running.
func KeepQueue(c *Core, q *engine.Engine) (stop func()) {
	path := filepath.Join(c.Dirs.Data, QueueFile)
	var last []byte
	save := func() {
		data, err := q.Snapshot()
		if err != nil || bytes.Equal(data, last) {
			return
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			slog.Warn("save queue", "err", err)
			return
		}
		if err := os.Rename(tmp, path); err != nil {
			slog.Warn("save queue", "err", err)
			return
		}
		last = data
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(3 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				save()
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			wg.Wait()
			save()
		})
	}
}
