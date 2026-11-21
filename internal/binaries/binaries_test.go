package binaries

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	t       *testing.T
	files   map[string][]byte // path → body
	manager *Manager
	priv    ed25519.PrivateKey
	server  *httptest.Server
}

func newFixture(t *testing.T) *fixture {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, files: map[string][]byte{}, priv: priv}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := f.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(f.server.Close)
	f.manager = &Manager{
		Dir: t.TempDir(), Client: f.server.Client(), Platform: Platform, PublicKey: pub,
		ManifestURL: f.server.URL + "/manifest.json", SigURL: f.server.URL + "/manifest.sig",
	}
	return f
}

func zipBytes(t *testing.T, entries map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// serve adds a file and returns its manifest entry.
func (f *fixture) serve(name string, body []byte) ManifestFile {
	f.files["/"+name] = body
	sum := sha256.Sum256(body)
	return ManifestFile{URL: f.server.URL + "/" + name, SHA256: hex.EncodeToString(sum[:])}
}

func (f *fixture) publish(m Manifest) {
	body, err := json.Marshal(m)
	if err != nil {
		f.t.Fatal(err)
	}
	f.files["/manifest.json"] = body
	f.files["/manifest.sig"] = []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, body)))
}

func (f *fixture) standardManifest() Manifest {
	raw := f.serve("ffmpeg-win32-x64", []byte("ffmpeg binary"))
	entry := f.serve("deno.zip", zipBytes(f.t, map[string]string{"deno.exe": "deno binary", "README.md": "x"}))
	entry.Entry = "deno.exe"
	bundle := f.serve("yt-dlp_win.zip", zipBytes(f.t, map[string]string{"yt-dlp/yt-dlp.exe": "yt-dlp binary", "yt-dlp/_internal/python.dll": "dll"}))
	bundle.Bundle = &Bundle{KeepFolder: true, FolderName: "yt-dlp", Entry: "yt-dlp.exe", RenameEntry: "yt-dlp.exe"}
	return Manifest{GeneratedAt: "now", Tools: map[string]ManifestTool{
		"yt-dlp": {Version: "2026.10.26", Files: map[string]ManifestFile{Platform: bundle}},
		"ffmpeg": {Version: "b6.1.1", Files: map[string]ManifestFile{Platform: raw}},
		"deno":   {Version: "v2.9.8", Files: map[string]ManifestFile{Platform: entry}},
		"other":  {Version: "1", Files: map[string]ManifestFile{"darwin-aarch64": raw}},
	}}
}

func (f *fixture) installAll(updates bool) ([]Pending, []Progress, error) {
	ctx := context.Background()
	manifest, err := f.manager.FetchManifest(ctx)
	if err != nil {
		return nil, nil, err
	}
	plan, err := f.manager.Plan(manifest, updates)
	if err != nil {
		return nil, nil, err
	}
	var events []Progress
	err = f.manager.Install(ctx, manifest, plan, func(p Progress) { events = append(events, p) })
	return plan, events, err
}

func readFile(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallAllKinds(t *testing.T) {
	f := newFixture(t)
	f.publish(f.standardManifest())
	if got := f.manager.MissingRequired(); len(got) != 3 {
		t.Fatalf("missing %q", got)
	}
	plan, events, err := f.installAll(false)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range plan {
		names = append(names, p.Name)
		if !p.Missing {
			t.Errorf("%s not marked missing", p.Name)
		}
	}
	if strings.Join(names, ",") != "yt-dlp,ffmpeg,deno" {
		t.Errorf("plan %q", names)
	}
	dir := f.manager.Dir
	if readFile(t, filepath.Join(dir, "yt-dlp.exe")) != "yt-dlp binary" || readFile(t, filepath.Join(dir, "_internal", "python.dll")) != "dll" {
		t.Error("bundle not hoisted into the tools folder")
	}
	if readFile(t, filepath.Join(dir, "ffmpeg.exe")) != "ffmpeg binary" || readFile(t, filepath.Join(dir, "deno.exe")) != "deno binary" {
		t.Error("raw or entry tool wrong")
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		t.Error("entry zip extracted more than the entry")
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left: %q", leftovers)
	}
	done := 0
	for _, e := range events {
		if e.Done {
			done++
			if e.Err != nil {
				t.Errorf("%s: %v", e.Tool, e.Err)
			}
		}
	}
	if done != 3 {
		t.Errorf("%d done events", done)
	}
	meta, _ := f.manager.LoadMetadata()
	if meta.Versions["yt-dlp"] != "2026.10.26" || meta.Versions["deno"] != "v2.9.8" {
		t.Errorf("metadata %+v", meta)
	}

	// Up to date: nothing to do, with or without updates.
	plan, _, err = f.installAll(true)
	if err != nil || len(plan) != 0 {
		t.Errorf("second run planned %+v (%v)", plan, err)
	}
}

func TestPlanUpdatesOnlyWhenAllowed(t *testing.T) {
	f := newFixture(t)
	m := f.standardManifest()
	f.publish(m)
	if _, _, err := f.installAll(false); err != nil {
		t.Fatal(err)
	}
	tool := m.Tools["ffmpeg"]
	tool.Version = "b7"
	m.Tools["ffmpeg"] = tool
	f.publish(m)
	if plan, _, _ := f.installAll(false); len(plan) != 0 {
		t.Errorf("updated without permission: %+v", plan)
	}
	plan, _, err := f.installAll(true)
	if err != nil || len(plan) != 1 || plan[0].Name != "ffmpeg" || plan[0].Missing {
		t.Errorf("update plan %+v (%v)", plan, err)
	}

	// A deleted tool comes back even with updates off.
	os.Remove(f.manager.Path("deno"))
	plan, _, _ = f.installAll(false)
	if len(plan) != 1 || plan[0].Name != "deno" || !plan[0].Missing {
		t.Errorf("missing tool plan %+v", plan)
	}
}

func TestLockedMetadataPlansNothing(t *testing.T) {
	f := newFixture(t)
	f.publish(f.standardManifest())
	os.WriteFile(filepath.Join(f.manager.Dir, "metadata.json"), []byte(`{"versions":{},"is_locked":true}`), 0o644)
	if plan, _, err := f.installAll(true); err != nil || len(plan) != 0 {
		t.Errorf("locked plan %+v (%v)", plan, err)
	}
}

func TestBadSignatureRejected(t *testing.T) {
	f := newFixture(t)
	f.publish(f.standardManifest())
	f.files["/manifest.json"] = append(f.files["/manifest.json"], ' ')
	if _, err := f.manager.FetchManifest(context.Background()); err == nil {
		t.Fatal("tampered manifest accepted")
	}
}

func TestChecksumMismatch(t *testing.T) {
	f := newFixture(t)
	m := f.standardManifest()
	f.publish(m)
	f.files["/ffmpeg-win32-x64"] = []byte("tampered")
	_, events, err := f.installAll(false)
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("err %v", err)
	}
	if f.manager.Installed("ffmpeg") {
		t.Error("tampered tool installed")
	}
	if !f.manager.Installed("yt-dlp") || !f.manager.Installed("deno") {
		t.Error("other tools not installed after one failed")
	}
	meta, _ := f.manager.LoadMetadata()
	if _, ok := meta.Versions["ffmpeg"]; ok {
		t.Error("failed tool recorded")
	}
	failed := false
	for _, e := range events {
		failed = failed || (e.Tool == "ffmpeg" && e.Done && e.Err != nil)
	}
	if !failed {
		t.Error("no failure event")
	}
}

func TestPathTraversalRejected(t *testing.T) {
	f := newFixture(t)
	evil := f.serve("evil.zip", zipBytes(t, map[string]string{"yt-dlp/../../escape.txt": "x", "yt-dlp/yt-dlp.exe": "y"}))
	evil.Bundle = &Bundle{Entry: "yt-dlp.exe"}
	f.publish(Manifest{Tools: map[string]ManifestTool{"yt-dlp": {Version: "1", Files: map[string]ManifestFile{Platform: evil}}}})
	_, _, err := f.installAll(false)
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(f.manager.Dir), "escape.txt")); err == nil {
		t.Error("file escaped the tools folder")
	}
}

func TestSafeJoin(t *testing.T) {
	for _, bad := range []string{"../x", "a/../../x", "/abs", `C:\x`, `a\..\..\x`} {
		if _, err := safeJoin("root", bad); !errors.Is(err, ErrPathTraversal) {
			t.Errorf("%q accepted", bad)
		}
	}
	if got, err := safeJoin("root", "a/./b"); err != nil || got != filepath.Join("root", "a", "b") {
		t.Errorf("a/./b: %q %v", got, err)
	}
}

func TestEntryZipAmbiguous(t *testing.T) {
	f := newFixture(t)
	z := f.serve("tools.zip", zipBytes(t, map[string]string{"a.exe": "a", "b.exe": "b"}))
	f.publish(Manifest{Tools: map[string]ManifestTool{"ffprobe": {Version: "1", Files: map[string]ManifestFile{Platform: z}}}})
	if _, _, err := f.installAll(false); err == nil {
		t.Fatal("ambiguous zip accepted")
	}
	z = f.serve("tools2.zip", zipBytes(t, map[string]string{"bin/ffprobe.exe": "probe", "b.exe": "b"}))
	f.publish(Manifest{Tools: map[string]ManifestTool{"ffprobe": {Version: "1", Files: map[string]ManifestFile{Platform: z}}}})
	if _, _, err := f.installAll(false); err != nil || readFile(t, f.manager.Path("ffprobe")) != "probe" {
		t.Fatalf("tool-named entry: %v", err)
	}
}
