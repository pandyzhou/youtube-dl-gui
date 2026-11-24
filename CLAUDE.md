# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@AGENTS.md

## Commands

Nothing is built or tested locally (see AGENTS.md): push the branch and read the results on GitHub Actions.

- `gh run list --branch <branch>` / `gh run watch <id>` / `gh run view <id> --log-failed` to follow `Go CI` and `MyGo build (Windows)`.
- `gh run download <id> -n go-screenshots` fetches the UI screenshots (light and dark, every page and settings section) to review a UI change.
- Both workflows also have `workflow_dispatch`. They only trigger on changes to Go files, `go.mod`/`go.sum`, `mygo.json`, `resources/` or the workflow itself.
- What CI runs, for reference: `gofmt -l`, `go vet ./...` (again with `GOOS=windows`), `go test ./...` with `SCREENSHOT_DIR` set, and `go tool mygo build -platform windows/amd64`. A single test would be `go test ./internal/engine -run TestName`.
- CI runs `go mod tidy` and uploads the result as the `go-modules` artifact: after changing dependencies, take `go.mod`/`go.sum` from there instead of running go locally.

## Releases

Every push to `main` that touches code publishes a release: the workflow computes the next `v4.0.N` tag itself (the `version` in `mygo.json` is only the base for preview builds — do not bump it), builds the release notes from the `feat:`/`fix:`/`perf:` commit subjects since the last tag, and signs the update feed that installed apps read. So a commit on `main` ships to users; use `chore:`/`docs:`/`refactor:`/`test:` for anything that should not appear in the notes, and `[skip ci]` for changes that should not release.

## Architecture

Tests run on Linux in CI while the app only ships for Windows, so everything except `cmd/lumadl` must compile and test on Linux; Windows-only code sits behind build-tagged files (`internal/proc/proc_windows.go` vs `proc_unix.go`) and is only vetted, not tested.

### Wiring

`cmd/lumadl/main.go` is the only place that knows about everything; packages get what they need as `Deps` structs of functions and small interfaces, which is also how the tests fake them.

- `internal/core` is the glue below `main` (AGENTS.md does not list it): `Core` holds the folders, settings, preferences and secrets behind a mutex and hands out copies; `core.NewTools` drives `binaries`; `core.NewEngine` builds the `engine.Deps`, and is where `translate` is plugged into the engine; `RestoreQueue`/`KeepQueue` persist the queue; `migrate.go` and `config/legacy.go` import the data of the old Tauri app (Open Video Downloader).
- `cmd/lumadl/desktop.go` is the Windows integration (tray, global shortcuts, notifications, start at login, taskbar progress), re-applied after every settings change via `desktop.apply`.
- Supporting packages: `proc` (runs yt-dlp/ffmpeg without a console, tools folder first on PATH, kills the process tree on cancel), `secrets` (Credential Manager, keys named as in the Tauri app), `httpx` (HTTP client honouring the proxy settings).

### Threads

UI state lives on the main thread. The engine, tool installs and network calls run on goroutines; they change their own mutex-guarded state and call the `redraw` closure (`window.Invalidate`), and anything that touches UI state or MyGo objects goes through `mygo.RunOnMain`. Callbacks in `ui.Deps` (`PickPath`, `ListModels`, …) promise to call `done` on the main thread.

### Engine

`internal/engine` knows nothing of the UI or of translation. A `job` is a queue card (one link); it has `item`s (videos — several for a playlist). A job has a phase (fetching → selecting → fetchingList → ready) and then its status is derived from its items (`view.go`). The UI only sees snapshots (`JobView`/`ItemView` from `Jobs()`) and calls methods. Fetches and downloads go through two `pool`s sized by the concurrency setting. yt-dlp is behind the `Runner` interface; engine tests use a scripted fake (`fakeYtdlp` in `engine_test.go`) rather than a real binary.

Translation hooks into downloads through three `Deps` callbacks: `BeforeDownload` starts translating beside the download, `AfterDownload` waits for its result (`Finished.Early`), and `MergeTracks` + `FFmpeg` + `OneStep` enable the one-step path (`onestep.go`): when translated subtitles go into an MKV, yt-dlp only downloads the separate streams into a `.ovd-parts-*` folder and a single ffmpeg run merges video, audio, cover and subtitle tracks.

`persist.go` saves the queue as versioned JSON including the fetched info; running downloads come back paused.

### ytdlp, formats, translate, binaries

- `ytdlp` is pure logic with no process handling: building arguments from settings, parsing `-J` info and progress lines, output templates, subtitle selection. Errors and warnings printed by yt-dlp are classified by the embedded `diagnostic_rules.json` into codes that `ui/diagnostics.go` explains. Much of it, with its tests, is ported from the original project — keep behaviour compatible when touching it.
- `formats` holds the quality presets and link parsing (including the "video inside a playlist" choice in `mixed.go`).
- `translate` fetches source subtitles with yt-dlp (json3 with word timings for YouTube auto captions, re-segmented into sentences), translates in batches through an OpenAI-compatible chat API with checkpoints for resuming (`checkpoint.go`, cache under `<data>/translate-cache`), and writes SRT/ASS (`output.go`).
- `binaries` installs yt-dlp, ffmpeg, ffprobe, deno and AtomicParsley from the Ed25519-signed manifest still published by the upstream project (jely2002), verifying SHA-256.

### UI

`internal/ui` is immediate-mode MyGo: `App.View(c *ui.Context)` rebuilds the window each frame from `Deps` and the page states (`queueState`, `settingsState`), with a `ui.Router` over `/queue`, `/settings`, `/about`. All user-facing strings are Simplified Chinese, written inline (no i18n layer). Layout constants shared by the sidebar and pages are at the top of `app.go`; colours come from `theme.go`.

UI tests drive the real view headlessly with `ui.NewTester(a.View, w, h)` — `HasText`, `Click`, `Frame` — against fakes (`fakeTools`, `fakeQueue`, `newFakeSettings`); nil `Deps` fields are tolerated so a test can build one page. The `TestScreenshots*` tests skip unless `SCREENSHOT_DIR` is set; add new pages or states to them so they show up in the `go-screenshots` artifact.

## Leftovers

The repo was a Tauri/Vue app before the Go rewrite. `node_modules/` (git-ignored) is a leftover; there is no JS in the project. The app identifier `io.github.pandyzhou.ovd`, the `ovd-` artifact and folder prefixes and the secret key names are kept deliberately for compatibility with existing installs.
