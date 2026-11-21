// Package core ties the app's state together: its folders, settings,
// preferences and secrets. The engine and the UI get it from main.
package core

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/secrets"
)

// Dirs are the folders the app uses.
type Dirs struct {
	Data      string // settings, preferences, large secrets
	Bin       string // yt-dlp, ffmpeg and the other tools
	Logs      string
	Downloads string // the system's Downloads folder, the default target
	Legacy    string // the Tauri app's data folder, imported from once
}

// LegacyDir returns the Tauri app's data folder on Windows, or "".
func LegacyDir() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, config.LegacyIdentifier)
}

// Core holds the loaded state. Its methods are safe from any goroutine.
type Core struct {
	Dirs    Dirs
	Secrets *secrets.Store

	mu          sync.RWMutex
	settings    config.Settings
	preferences config.Preferences
}

// Open loads the settings (importing the Tauri app's on first run) and
// opens the secrets store. A settings file that cannot be read is logged
// and replaced by the defaults, so the app still starts.
func Open(dirs Dirs, backend secrets.Backend) *Core {
	for _, dir := range []string{dirs.Data, dirs.Bin, dirs.Logs} {
		if dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
	}
	s, p, imported, err := config.LoadSettings(dirs.Data, dirs.Legacy, dirs.Downloads)
	if err != nil {
		slog.Error("load settings", "err", err)
	}
	if imported {
		slog.Info("imported settings from the previous app", "from", dirs.Legacy)
	}
	return &Core{Dirs: dirs, Secrets: secrets.New(backend, dirs.Data), settings: s, preferences: p}
}

// Settings returns a copy of the settings.
func (c *Core) Settings() config.Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return clone(c.settings)
}

// Preferences returns a copy of the preferences.
func (c *Core) Preferences() config.Preferences {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return clonePreferences(c.preferences)
}

// UpdateSettings applies change to the settings and saves them.
func (c *Core) UpdateSettings(change func(*config.Settings)) error {
	c.mu.Lock()
	next := clone(c.settings)
	change(&next)
	next = config.Normalize(next, c.Dirs.Downloads)
	c.settings = next
	c.mu.Unlock()
	if err := config.Save(filepath.Join(c.Dirs.Data, config.SettingsFile), next); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// UpdatePreferences applies change to the preferences and saves them.
func (c *Core) UpdatePreferences(change func(*config.Preferences)) error {
	c.mu.Lock()
	next := clonePreferences(c.preferences)
	change(&next)
	c.preferences = next
	c.mu.Unlock()
	if err := config.Save(filepath.Join(c.Dirs.Data, config.PreferencesFile), next); err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	return nil
}

func clone(s config.Settings) config.Settings {
	s.Subtitles.Languages = append([]string(nil), s.Subtitles.Languages...)
	s.Subtitles.FormatPreference = append([]string(nil), s.Subtitles.FormatPreference...)
	s.Notifications.DisabledNotifications = append([]string{}, s.Notifications.DisabledNotifications...)
	if v := s.InputFilters.MinSize.Value; v != nil {
		n := *v
		s.InputFilters.MinSize.Value = &n
	}
	if v := s.InputFilters.MaxSize.Value; v != nil {
		n := *v
		s.InputFilters.MaxSize.Value = &n
	}
	return s
}

func clonePreferences(p config.Preferences) config.Preferences {
	recent := make(map[string][]string, len(p.Recents.Recent))
	for k, v := range p.Recents.Recent {
		recent[k] = append([]string(nil), v...)
	}
	p.Recents.Recent = recent
	return p
}
