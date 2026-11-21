// Package proc runs the command line tools (yt-dlp, ffmpeg) the app drives:
// without a console window, with the tools folder first on PATH, line by
// line output, and the whole process tree ended when the run is canceled
// or the app exits.
package proc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Spec describes a run.
type Spec struct {
	Path string   // absolute path of the program
	Args []string // arguments, without the program
	Dir  string   // working directory; "" for the app's
	// PathPrepend goes first on the child's PATH, so the program finds the
	// tools next to it (yt-dlp finds ffmpeg, ffprobe and deno this way).
	PathPrepend string
	Env         []string // extra KEY=value entries
}

// Line is one line of output.
type Line struct {
	Stderr bool
	Text   string
}

// Run starts spec, calls onLine for each line of output from one goroutine
// at a time, and waits for the program to end. It returns the exit code.
// Canceling ctx ends the program and everything it started, and Run then
// returns ctx's error.
func Run(ctx context.Context, spec Spec, onLine func(Line)) (int, error) {
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = buildEnv(os.Environ(), spec.PathPrepend, spec.Env)
	prepare(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	tree, err := attach(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return -1, err
	}
	defer tree.close()

	lines := make(chan Line, 64)
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); SplitLines(stdout, func(s string) { lines <- Line{Text: s} }) }()
	go func() {
		defer readers.Done()
		SplitLines(stderr, func(s string) { lines <- Line{Stderr: true, Text: s} })
	}()
	go func() { readers.Wait(); close(lines) }()

	stop := context.AfterFunc(ctx, tree.kill)
	defer stop()

	for line := range lines {
		if onLine != nil {
			onLine(line)
		}
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	if waitErr != nil {
		return -1, waitErr
	}
	return 0, nil
}

// SplitLines calls fn for each line of r, splitting on "\n" and on "\r"
// (progress output redraws a line with "\r"), skipping empty lines and
// replacing invalid UTF-8.
func SplitLines(r io.Reader, fn func(string)) {
	br := bufio.NewReaderSize(r, 64<<10)
	var line []byte
	flush := func() {
		if len(line) > 0 {
			fn(strings.ToValidUTF8(string(line), "�"))
			line = line[:0]
		}
	}
	for {
		b, err := br.ReadByte()
		if err != nil {
			flush()
			return
		}
		if b == '\n' || b == '\r' {
			flush()
			continue
		}
		line = append(line, b)
	}
}

// buildEnv returns env with prepend first on PATH (matched case-insensitively,
// as Windows names it "Path") and extra appended.
func buildEnv(env []string, prepend string, extra []string) []string {
	out := make([]string, 0, len(env)+len(extra)+1)
	found := false
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if ok && strings.EqualFold(key, "PATH") && prepend != "" {
			found = true
			if value == "" {
				kv = key + "=" + prepend
			} else {
				kv = key + "=" + prepend + string(os.PathListSeparator) + value
			}
		}
		out = append(out, kv)
	}
	if !found && prepend != "" {
		out = append(out, "PATH="+prepend)
	}
	return append(out, extra...)
}
