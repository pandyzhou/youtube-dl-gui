# AGENTS

## Overview
Open Video Downloader is a Go app with a native MyGo UI (Windows x64, Simplified Chinese).
The entry point is `cmd/ovd`; everything else lives in `internal/`:
`config` (settings, legacy import), `ytdlp` (arguments, parsing), `binaries` (tool installs),
`engine` (queue), `formats` (presets, inputs), `translate` (AI bilingual subtitles), `ui` (interface).

## Building and testing
Builds, vet, gofmt and tests run on GitHub Actions only; do not run go locally.
- `Go CI` checks formatting (gofmt), runs `go vet` (also for Windows) and the tests,
  and uploads UI screenshots as the `go-screenshots` artifact.
- `MyGo build (Windows)` builds a preview artifact on branches and publishes a release from `main`.

## Searching
Use `rg` for searching the codebase; avoid recursive `grep` or `ls` commands.

## Commit style
Write commit messages in the form `type: summary`, e.g. `fix: update progress bar logic`.
