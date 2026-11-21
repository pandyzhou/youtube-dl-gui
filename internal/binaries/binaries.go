// Package binaries installs and updates the tools the app runs: yt-dlp,
// ffmpeg, ffprobe, deno and AtomicParsley. It reads the signed manifest the
// original project publishes, checks each file's SHA-256, and unpacks it
// into the tools folder, as the Tauri app's binaries manager did.
package binaries

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest locations and the key that signs it.
const (
	ManifestURL    = "https://jely2002.github.io/youtube-dl-gui/manifest/manifest.json"
	ManifestSigURL = "https://jely2002.github.io/youtube-dl-gui/manifest/manifest.sig"
	ManifestPubKey = "ae7988a00d92349e55ff560369e7ec6afdb4b22a7e1dbe62a6769e1f347fa073"
	// Platform is the manifest key of the only platform the app supports.
	Platform = "windows-x86_64"
)

// Tool names, in install order (the manifest order).
var Tools = []string{"yt-dlp", "ffmpeg", "ffprobe", "deno", "AtomicParsley"}

// Required tools must exist before anything can be downloaded.
var Required = []string{"yt-dlp", "ffmpeg", "ffprobe"}

// Manifest lists each tool's version and files per platform.
type Manifest struct {
	GeneratedAt string                  `json:"generatedAt"`
	Tools       map[string]ManifestTool `json:"tools"`
}

// ManifestTool is a tool in the manifest.
type ManifestTool struct {
	Version string                  `json:"version"`
	Files   map[string]ManifestFile `json:"files"`
}

// ManifestFile is a downloadable file: the tool itself, a zip with the tool
// at Entry, or a zip bundle whose contents all go to the tools folder.
type ManifestFile struct {
	URL    string  `json:"url"`
	SHA256 string  `json:"sha256"`
	Entry  string  `json:"entry"`
	Bundle *Bundle `json:"bundle"`
}

// Bundle describes a zip whose whole contents are installed.
type Bundle struct {
	KeepFolder  bool   `json:"keep_folder"`
	FolderName  string `json:"folder_name"`
	Entry       string `json:"entry"`
	RenameEntry string `json:"rename_entry_to"`
}

// Metadata records installed versions. Locked stops all installs and
// updates, for people who manage the tools themselves.
type Metadata struct {
	Versions map[string]string `json:"versions"`
	Locked   bool              `json:"is_locked"`
}

// Pending is a tool that needs installing.
type Pending struct {
	Name    string
	Version string
	Missing bool // false: installed but outdated
}

// Progress reports an install. Done is set once per tool, with Err when it
// failed.
type Progress struct {
	Tool     string
	Version  string
	Received int64
	Total    int64 // 0 when unknown
	Done     bool
	Err      error
}

// Manager installs tools into Dir.
type Manager struct {
	Dir         string
	Client      *http.Client
	ManifestURL string
	SigURL      string
	PublicKey   ed25519.PublicKey
	Platform    string
}

// New returns a manager for the published manifest.
func New(dir string, client *http.Client) *Manager {
	key, _ := hex.DecodeString(ManifestPubKey)
	return &Manager{Dir: dir, Client: client, ManifestURL: ManifestURL, SigURL: ManifestSigURL, PublicKey: key, Platform: Platform}
}

// Path is where a tool is installed.
func (m *Manager) Path(tool string) string { return filepath.Join(m.Dir, tool+".exe") }

// Installed reports whether a tool exists.
func (m *Manager) Installed(tool string) bool {
	info, err := os.Stat(m.Path(tool))
	return err == nil && !info.IsDir()
}

// MissingRequired lists the required tools that are not installed. It
// reads the disk only, so startup can call it without waiting for the
// network.
func (m *Manager) MissingRequired() []string {
	var missing []string
	for _, t := range Required {
		if !m.Installed(t) {
			missing = append(missing, t)
		}
	}
	return missing
}

func (m *Manager) metadataPath() string { return filepath.Join(m.Dir, "metadata.json") }

// LoadMetadata reads the installed versions; a missing file is empty.
func (m *Manager) LoadMetadata() (Metadata, error) {
	meta := Metadata{Versions: map[string]string{}}
	data, err := os.ReadFile(m.metadataPath())
	if errors.Is(err, os.ErrNotExist) {
		return meta, nil
	}
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{Versions: map[string]string{}}, fmt.Errorf("tools metadata: %w", err)
	}
	if meta.Versions == nil {
		meta.Versions = map[string]string{}
	}
	return meta, nil
}

func (m *Manager) saveMetadata(meta Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.metadataPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.metadataPath())
}

// FetchManifest downloads the manifest and checks its signature.
func (m *Manager) FetchManifest(ctx context.Context) (*Manifest, error) {
	body, err := m.get(ctx, m.ManifestURL)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	sigText, err := m.get(ctx, m.SigURL)
	if err != nil {
		return nil, fmt.Errorf("manifest signature: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil {
		return nil, fmt.Errorf("manifest signature: %w", err)
	}
	if len(m.PublicKey) != ed25519.PublicKeySize || !ed25519.Verify(m.PublicKey, body, sig) {
		return nil, errors.New("manifest signature does not verify")
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	return &manifest, nil
}

func (m *Manager) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// Plan lists the tools to install: missing ones always, outdated ones when
// updates is set. Nothing is planned while the metadata is locked.
func (m *Manager) Plan(manifest *Manifest, updates bool) ([]Pending, error) {
	meta, err := m.LoadMetadata()
	if err != nil {
		return nil, err
	}
	if meta.Locked {
		return nil, nil
	}
	var plan []Pending
	for _, name := range m.toolOrder(manifest) {
		tool := manifest.Tools[name]
		if _, ok := tool.Files[m.Platform]; !ok {
			continue
		}
		missing := !m.Installed(name)
		outdated := meta.Versions[name] != tool.Version
		if missing || (updates && outdated) {
			plan = append(plan, Pending{Name: name, Version: tool.Version, Missing: missing})
		}
	}
	return plan, nil
}

// toolOrder is Tools first, then any other manifest tools by name.
func (m *Manager) toolOrder(manifest *Manifest) []string {
	var names []string
	for _, t := range Tools {
		if _, ok := manifest.Tools[t]; ok {
			names = append(names, t)
		}
	}
	var extra []string
	for name := range manifest.Tools {
		if !contains(names, name) {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	return append(names, extra...)
}

// Install installs plan one tool at a time, reporting progress, and records
// the versions that installed. It returns the first error; tools after a
// failed one are still tried.
func (m *Manager) Install(ctx context.Context, manifest *Manifest, plan []Pending, progress func(Progress)) error {
	if progress == nil {
		progress = func(Progress) {}
	}
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return err
	}
	meta, err := m.LoadMetadata()
	if err != nil {
		meta = Metadata{Versions: map[string]string{}}
	}
	var firstErr error
	for _, p := range plan {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		file := manifest.Tools[p.Name].Files[m.Platform]
		err := m.installTool(ctx, p, file, progress)
		if err == nil {
			meta.Versions[p.Name] = p.Version
		} else if firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", p.Name, err)
		}
		progress(Progress{Tool: p.Name, Version: p.Version, Done: true, Err: err})
	}
	if err := m.saveMetadata(meta); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (m *Manager) installTool(ctx context.Context, p Pending, file ManifestFile, progress func(Progress)) error {
	download := filepath.Join(m.Dir, ".download-"+p.Name+".tmp")
	defer os.Remove(download)
	if err := m.download(ctx, p, file, download, progress); err != nil {
		return err
	}
	canonical := m.Path(p.Name)
	var err error
	switch {
	case !strings.HasSuffix(strings.ToLower(path.Base(file.URL)), ".zip"):
		err = replaceFile(download, canonical)
	case file.Bundle != nil:
		err = m.installBundle(download, *file.Bundle)
	default:
		err = extractEntry(download, p.Name, file.Entry, canonical)
	}
	if err != nil {
		return err
	}
	if !m.Installed(p.Name) {
		return fmt.Errorf("%s is missing after install", canonical)
	}
	return nil
}

func (m *Manager) download(ctx context.Context, p Pending, file ManifestFile, dest string, progress func(Progress)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URL, nil)
	if err != nil {
		return err
	}
	resp, err := m.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	hash := sha256.New()
	report := &progressWriter{p: Progress{Tool: p.Name, Version: p.Version, Total: max(resp.ContentLength, 0)}, fn: progress}
	progress(report.p)
	_, err = io.Copy(io.MultiWriter(out, hash, report), resp.Body)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, file.SHA256) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, file.SHA256)
	}
	return nil
}

// progressWriter reports every 256 KiB received.
type progressWriter struct {
	p    Progress
	last int64
	fn   func(Progress)
}

func (w *progressWriter) Write(b []byte) (int, error) {
	w.p.Received += int64(len(b))
	if w.p.Received-w.last >= 256<<10 || (w.p.Total > 0 && w.p.Received >= w.p.Total) {
		w.last = w.p.Received
		w.fn(w.p)
	}
	return len(b), nil
}

// extractEntry writes one file of a zip to dest: the file named entry, or
// else the one named after the tool, or else the only file.
func extractEntry(archive, tool, entry, dest string) error {
	zr, err := openZip(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	var files []*zip.File
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			files = append(files, f)
		}
	}
	var chosen *zip.File
	for _, f := range files {
		base := path.Base(f.Name)
		switch {
		case entry != "" && f.Name == entry:
			chosen = f
		case entry == "" && chosen == nil && (base == tool || base == tool+".exe"):
			chosen = f
		}
	}
	if chosen == nil && entry == "" && len(files) == 1 {
		chosen = files[0]
	}
	if chosen == nil {
		return fmt.Errorf("entry %q not found in %s", entry, filepath.Base(archive))
	}
	rc, err := chosen.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, rc)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return replaceFile(tmp, dest)
}

// installBundle unpacks a zip into a temporary folder, renames the entry,
// and moves everything in the bundle's top folder into the tools folder
// (yt-dlp's build is an exe with an _internal folder beside it).
func (m *Manager) installBundle(archive string, b Bundle) error {
	tmp, err := os.MkdirTemp(m.Dir, ".extract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := unzipAll(archive, tmp); err != nil {
		return err
	}
	root := tmp
	if entries, err := os.ReadDir(tmp); err == nil && len(entries) == 1 && entries[0].IsDir() {
		root = filepath.Join(tmp, entries[0].Name())
	}
	entry, err := safeJoin(root, b.Entry)
	if err != nil {
		return err
	}
	if _, err := os.Stat(entry); err != nil {
		return fmt.Errorf("bundle entry %q not found", b.Entry)
	}
	if b.RenameEntry != "" && filepath.Base(entry) != b.RenameEntry {
		if err := os.Rename(entry, filepath.Join(filepath.Dir(entry), b.RenameEntry)); err != nil {
			return err
		}
	}
	children, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, child := range children {
		dest := filepath.Join(m.Dir, child.Name())
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(root, child.Name()), dest); err != nil {
			return err
		}
	}
	return nil
}

// unzipAll extracts every file of a zip under dir, refusing paths that
// would leave it and anything but files and folders.
func unzipAll(archive, dir string) error {
	zr, err := openZip(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		out, err := safeJoin(dir, f.Name)
		if err != nil {
			return err
		}
		mode := f.FileInfo().Mode()
		if mode.IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			continue
		}
		if !mode.IsRegular() {
			return fmt.Errorf("unsupported archive entry %q", f.Name)
		}
		if err := writeZipFile(f, out); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(f *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, rc)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// openZip opens a zip even when it has unsafe paths (zip.ErrInsecurePath);
// safeJoin checks every path before anything is written.
func openZip(archive string) (*zip.ReadCloser, error) {
	zr, err := zip.OpenReader(archive)
	if errors.Is(err, zip.ErrInsecurePath) && zr != nil {
		return zr, nil
	}
	return zr, err
}

// ErrPathTraversal is returned for archive paths that leave the folder.
var ErrPathTraversal = errors.New("archive path leaves the target folder")

// safeJoin joins an archive path (with / or \ separators) to root, refusing
// absolute paths, drive letters and "..".
func safeJoin(root, name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':') {
		return "", fmt.Errorf("%w: %s", ErrPathTraversal, name)
	}
	out := root
	for _, part := range strings.Split(name, "/") {
		switch part {
		case "", ".":
		case "..":
			return "", fmt.Errorf("%w: %s", ErrPathTraversal, name)
		default:
			out = filepath.Join(out, part)
		}
	}
	return out, nil
}

// replaceFile moves src over dst.
func replaceFile(src, dst string) error {
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace %s: %w", filepath.Base(dst), err)
	}
	return os.Rename(src, dst)
}

func contains(items []string, s string) bool {
	for _, item := range items {
		if item == s {
			return true
		}
	}
	return false
}
