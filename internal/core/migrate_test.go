package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pandyzhou/lumadl/internal/config"
)

func TestMoveOldData(t *testing.T) {
	appData := t.TempDir()
	old := filepath.Join(appData, PreviousName)
	data := filepath.Join(appData, "LumaDL")
	os.MkdirAll(filepath.Join(old, "bin"), 0o755)
	os.WriteFile(filepath.Join(old, config.SettingsFile), []byte(`{"old":true}`), 0o644)
	os.WriteFile(filepath.Join(old, QueueFile), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(old, "bin", "yt-dlp.exe"), []byte("x"), 0o644)
	os.MkdirAll(data, 0o755)
	os.WriteFile(filepath.Join(data, "updater.json"), []byte("new"), 0o644)

	moved, err := MoveOldData(data)
	if err != nil || !moved {
		t.Fatalf("moved %v, %v", moved, err)
	}
	for _, name := range []string{config.SettingsFile, QueueFile, filepath.Join("bin", "yt-dlp.exe"), "updater.json"} {
		if !isFile(filepath.Join(data, name)) {
			t.Errorf("%s not moved", name)
		}
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old folder left: %v", err)
	}

	// Once the new folder has settings, nothing moves again.
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, config.SettingsFile), []byte(`{}`), 0o644)
	if moved, _ := MoveOldData(data); moved {
		t.Error("moved over existing settings")
	}
}
