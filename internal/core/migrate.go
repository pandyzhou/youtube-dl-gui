package core

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/pandyzhou/lumadl/internal/config"
)

// PreviousName is the app's name before it became LumaDL (流光下载). Its
// data folder, %AppData%\Open Video Downloader, moves to the new one the
// first time the renamed app starts.
const PreviousName = "Open Video Downloader"

// MoveOldData moves the settings, queue, tools, caches and window state of
// the app's previous name into dataDir, when dataDir has no settings yet
// and the old folder has. It reports whether it moved anything. Entries
// that cannot be moved (the old app running, say) are copied when they
// are files and otherwise left, to be fetched again.
func MoveOldData(dataDir string) (bool, error) {
	if dataDir == "" {
		return false, nil
	}
	old := filepath.Join(filepath.Dir(dataDir), PreviousName)
	if filepath.Clean(old) == filepath.Clean(dataDir) ||
		!isFile(filepath.Join(old, config.SettingsFile)) || isFile(filepath.Join(dataDir, config.SettingsFile)) {
		return false, nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(old)
	if err != nil {
		return false, err
	}
	var errs []error
	for _, e := range entries {
		src, dst := filepath.Join(old, e.Name()), filepath.Join(dataDir, e.Name())
		// The renamed app may already have made some, such as the update
		// window's preferences: the old ones win.
		_ = os.RemoveAll(dst)
		if err := os.Rename(src, dst); err != nil {
			if e.Type().IsRegular() {
				err = copyFile(src, dst)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	_ = os.Remove(old) // only when empty
	return true, errors.Join(errs...)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
