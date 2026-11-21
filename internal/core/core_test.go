package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
	"github.com/pandyzhou/youtube-dl-gui/internal/secrets"
)

func TestCoreSavesAndReloads(t *testing.T) {
	root := t.TempDir()
	dirs := Dirs{
		Data:      filepath.Join(root, "data"),
		Bin:       filepath.Join(root, "data", "bin"),
		Logs:      filepath.Join(root, "logs"),
		Downloads: filepath.Join(root, "Downloads"),
	}
	c := Open(dirs, secrets.NewMemory())
	if c.Settings().Output.DownloadDir != dirs.Downloads {
		t.Fatalf("download dir %q", c.Settings().Output.DownloadDir)
	}
	if err := c.UpdateSettings(func(s *config.Settings) { s.Performance.MaxConcurrency = 3 }); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdatePreferences(func(p *config.Preferences) { p.AddRecent("downloadDir", "D:\\x") }); err != nil {
		t.Fatal(err)
	}

	// A copy handed out is not the core's own value.
	s := c.Settings()
	s.Subtitles.Languages[0] = "changed"
	if c.Settings().Subtitles.Languages[0] != "en" {
		t.Fatal("Settings returned shared slices")
	}

	reopened := Open(dirs, secrets.NewMemory())
	if reopened.Settings().Performance.MaxConcurrency != 3 {
		t.Fatalf("setting not saved: %+v", reopened.Settings().Performance)
	}
	if got := reopened.Preferences().Recents.Recent["downloadDir"]; len(got) != 1 || got[0] != "D:\\x" {
		t.Fatalf("preferences not saved: %v", got)
	}
	for _, dir := range []string{dirs.Data, dirs.Bin, dirs.Logs} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("folder not created: %v", err)
		}
	}
}

func TestSetupLogging(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ovd.log"), make([]byte, maxLogSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	closer, err := SetupLogging(dir)
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("hello from the test")
	closer.Close()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if _, err := os.Stat(filepath.Join(dir, "ovd.1.log")); err != nil {
		t.Error("old log not rotated")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "ovd.log"))
	if len(data) == 0 {
		t.Error("nothing logged")
	}
}
