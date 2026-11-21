package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// File names in the app's data folder.
const (
	SettingsFile    = "settings.json"
	PreferencesFile = "preferences.json"
)

// Load reads a JSON file over defaults: fields the file lacks keep their
// default values, unknown fields are ignored. A missing file returns the
// defaults and fs.ErrNotExist.
func Load[T any](path string, defaults T) (T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return defaults, err
	}
	value := defaults
	if err := json.Unmarshal(data, &value); err != nil {
		return defaults, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return value, nil
}

// Save writes value as indented JSON, through a temporary file so a crash
// never leaves half a file.
func Save[T any](path string, value T) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadSettings reads the settings in dir, importing the Tauri app's files
// from legacyDir the first time. imported reports that an import happened.
func LoadSettings(dir, legacyDir, downloadDir string) (s Settings, p Preferences, imported bool, err error) {
	defaults := DefaultSettings(downloadDir)
	settingsPath := filepath.Join(dir, SettingsFile)
	preferencesPath := filepath.Join(dir, PreferencesFile)

	s, err = Load(settingsPath, defaults)
	if errors.Is(err, fs.ErrNotExist) && legacyDir != "" {
		if legacy, ok := ImportLegacy(legacyDir, downloadDir); ok {
			s, p, imported = legacy.Settings, legacy.Preferences, true
			err = errors.Join(Save(settingsPath, s), Save(preferencesPath, p))
			return Normalize(s, downloadDir), p, imported, err
		}
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Normalize(defaults, downloadDir), DefaultPreferences(), false, err
	}
	p, perr := Load(preferencesPath, DefaultPreferences())
	if perr != nil && !errors.Is(perr, fs.ErrNotExist) {
		err = perr
	} else {
		err = nil
	}
	return Normalize(s, downloadDir), p, false, err
}

// Normalize fixes values a file may hold that the app cannot use: empty
// enums, out-of-range numbers, the legacy subtitle format "json".
func Normalize(s Settings, downloadDir string) Settings {
	d := DefaultSettings(downloadDir)
	if s.Output.DownloadDir == "" {
		s.Output.DownloadDir = d.Output.DownloadDir
	}
	if s.Output.FileNameTemplate == "" {
		s.Output.FileNameTemplate = DefaultFileNameTemplate
	}
	if s.Output.AudioFileNameTemplate == "" {
		s.Output.AudioFileNameTemplate = DefaultAudioFileNameTemplate
	}
	for _, policy := range []*string{&s.Output.Video.Policy, &s.Output.Audio.Policy} {
		if *policy != PolicyNever {
			*policy = PolicyAllowReencode // includes the legacy "remuxOnly"
		}
	}
	if s.Performance.MaxConcurrency < 1 {
		s.Performance.MaxConcurrency = d.Performance.MaxConcurrency
	}
	if s.Performance.MaxConcurrency > 32 {
		s.Performance.MaxConcurrency = 32
	}
	if s.Performance.ConcurrentFragments < 1 {
		s.Performance.ConcurrentFragments = 1
	}
	if s.Performance.ConcurrentFragments > 16 {
		s.Performance.ConcurrentFragments = 16
	}
	if s.Performance.SplitPlaylistThreshold < 1 {
		s.Performance.SplitPlaylistThreshold = 1
	}
	formats := make([]string, 0, len(s.Subtitles.FormatPreference))
	for _, f := range s.Subtitles.FormatPreference {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "json" {
			f = "json3"
		}
		if f != "" {
			formats = append(formats, f)
		}
	}
	if len(formats) == 0 {
		formats = append(formats, DefaultSubtitleFormats...)
	}
	s.Subtitles.FormatPreference = formats
	if s.Input.DefaultPreset == "" {
		s.Input.DefaultPreset = d.Input.DefaultPreset
	}
	if s.System.CloseBehavior != CloseHide {
		s.System.CloseBehavior = CloseExit
	}
	if s.Notifications.DisabledNotifications == nil {
		s.Notifications.DisabledNotifications = []string{}
	}
	return s
}
