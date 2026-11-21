package core

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/binaries"
	"github.com/pandyzhou/youtube-dl-gui/internal/httpx"
)

// ToolsPhase is where the tool check is.
type ToolsPhase int

// Tool check phases.
const (
	ToolsIdle ToolsPhase = iota
	// ToolsChecking fetches the manifest.
	ToolsChecking
	// ToolsInstalling downloads.
	ToolsInstalling
	// ToolsReady has everything planned installed.
	ToolsReady
	// ToolsFailed stopped on ToolsState.Err.
	ToolsFailed
)

// ToolStatus is one tool being installed.
type ToolStatus struct {
	Name     string
	Version  string
	Missing  bool // a first install, not an update
	Received int64
	Total    int64
	Done     bool
	Err      string
}

// Fraction is the downloaded share: 0 while waiting, -1 while downloading
// a file of unknown size.
func (s ToolStatus) Fraction() float64 {
	switch {
	case s.Done && s.Err == "":
		return 1
	case s.Received == 0 && s.Total <= 0:
		return 0
	case s.Total <= 0:
		return -1
	}
	return float64(s.Received) / float64(s.Total)
}

// ToolsState is a snapshot for the UI.
type ToolsState struct {
	Phase ToolsPhase
	Tools []ToolStatus
	Err   string
	// MissingRequired lists required tools that are not installed; while it
	// is not empty the app shows the install page instead of the queue.
	MissingRequired []string
}

// Tools checks and installs yt-dlp and the other tools in the background.
type Tools struct {
	core    *Core
	manager *binaries.Manager
	changed func()

	mu      sync.Mutex
	state   ToolsState
	running bool
}

// NewTools returns the tool controller; changed is called (from any
// goroutine) whenever the state changes, to redraw the window.
func NewTools(c *Core, changed func()) *Tools {
	if changed == nil {
		changed = func() {}
	}
	t := &Tools{core: c, manager: binaries.New(c.Dirs.Bin, nil), changed: changed}
	t.state.MissingRequired = t.manager.MissingRequired()
	return t
}

// Manager is the binaries manager, for the paths of the tools.
func (t *Tools) Manager() *binaries.Manager { return t.manager }

// State returns a snapshot.
func (t *Tools) State() ToolsState {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.state
	s.Tools = append([]ToolStatus(nil), s.Tools...)
	s.MissingRequired = append([]string(nil), s.MissingRequired...)
	return s
}

// Ready reports whether the required tools are installed.
func (t *Tools) Ready() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.state.MissingRequired) == 0
}

// Busy reports whether a check or install is running, so downloads wait
// rather than run a yt-dlp that is being replaced.
func (t *Tools) Busy() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running && t.state.Phase == ToolsInstalling
}

func (t *Tools) update(fn func(*ToolsState)) {
	t.mu.Lock()
	fn(&t.state)
	t.mu.Unlock()
	t.changed()
}

// Start checks the manifest and installs what is missing, plus updates when
// the settings allow them. It returns at once; a check already running is
// left alone.
func (t *Tools) Start() {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return
	}
	t.running = true
	t.mu.Unlock()
	go t.run()
}

func (t *Tools) run() {
	defer func() {
		t.mu.Lock()
		t.running = false
		t.mu.Unlock()
		t.changed()
	}()
	settings := t.core.Settings()
	t.manager.Client = httpx.Client(settings.Network, 0)
	t.update(func(s *ToolsState) { s.Phase, s.Err, s.Tools = ToolsChecking, "", nil })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	manifestCtx, cancelManifest := context.WithTimeout(ctx, 30*time.Second)
	manifest, err := t.manager.FetchManifest(manifestCtx)
	cancelManifest()
	if err != nil {
		t.fail(err)
		return
	}
	plan, err := t.manager.Plan(manifest, settings.Update.UpdateBinaries)
	if err != nil {
		t.fail(err)
		return
	}
	if len(plan) == 0 {
		t.finish()
		return
	}
	t.update(func(s *ToolsState) {
		s.Phase = ToolsInstalling
		for _, p := range plan {
			s.Tools = append(s.Tools, ToolStatus{Name: p.Name, Version: p.Version, Missing: p.Missing})
		}
	})
	slog.Info("installing tools", "count", len(plan))
	err = t.manager.Install(ctx, manifest, plan, func(p binaries.Progress) {
		t.update(func(s *ToolsState) {
			for i := range s.Tools {
				if s.Tools[i].Name != p.Tool {
					continue
				}
				st := &s.Tools[i]
				if p.Done {
					st.Done = true
					if p.Err != nil {
						st.Err = p.Err.Error()
					}
				} else {
					st.Received, st.Total = p.Received, p.Total
				}
			}
		})
	})
	if err != nil {
		t.fail(err)
		return
	}
	t.finish()
}

func (t *Tools) fail(err error) {
	slog.Error("tools", "err", err)
	missing := t.manager.MissingRequired()
	t.update(func(s *ToolsState) {
		s.Phase, s.Err, s.MissingRequired = ToolsFailed, err.Error(), missing
	})
}

func (t *Tools) finish() {
	missing := t.manager.MissingRequired()
	t.update(func(s *ToolsState) {
		s.Phase, s.MissingRequired = ToolsReady, missing
	})
}
