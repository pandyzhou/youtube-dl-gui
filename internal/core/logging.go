package core

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
)

// maxLogSize is the size at which the log moves to ovd.1.log at startup.
const maxLogSize = 4 << 20

// SetupLogging sends slog and the log package to ovd.log in dir, since a
// Windows GUI app has no console. The previous log is kept as ovd.1.log
// once it grows past maxLogSize. The returned closer flushes the file.
func SetupLogging(dir string) (io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "ovd.log")
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogSize {
		_ = os.Rename(path, filepath.Join(dir, "ovd.1.log"))
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	log.SetOutput(f)
	return f, nil
}
