package ytdlp

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pandyzhou/lumadl/internal/config"
)

// ProgressTemplate makes yt-dlp print one machine-readable progress line
// per update; ParseProgress reads it.
const ProgressTemplate = "RAW|%(progress.percent|)s|%(progress._percent_str|)s|%(progress.speed|)s|%(progress.eta|)s|%(progress.downloaded_bytes|)s|%(progress.total_bytes|)s|%(progress.total_bytes_estimate|)s|%(progress.fragment_index|)s|%(progress.fragment_count|)s"

// ProgressArgs asks for progress lines every half second.
func ProgressArgs() []string {
	return []string{"--newline", "--progress", "--no-color", "--progress-template", ProgressTemplate, "--progress-delta", "0.5"}
}

// MaxConcurrentFragments caps --concurrent-fragments, so a bad setting
// cannot open hundreds of connections.
const MaxConcurrentFragments = 16

// PerformanceArgs downloads several fragments of HLS/DASH formats at once;
// 0 or 1 keeps yt-dlp's one at a time. Single-file downloads ignore it.
func PerformanceArgs(fragments int) []string {
	if fragments <= 1 {
		return nil
	}
	if fragments > MaxConcurrentFragments {
		fragments = MaxConcurrentFragments
	}
	return []string{"--concurrent-fragments", strconv.Itoa(fragments)}
}

// NetworkArgs returns the proxy, impersonation and extractor arguments.
// Each non-empty line of the extractor arguments is its own flag.
func NetworkArgs(network config.NetworkSettings) []string {
	var args []string
	if network.EnableProxy && strings.TrimSpace(network.Proxy) != "" {
		args = append(args, "--proxy", strings.TrimSpace(network.Proxy))
	}
	switch network.Impersonate {
	case "", "none":
	case "any":
		args = append(args, "--impersonate", "")
	default:
		args = append(args, "--impersonate", network.Impersonate)
	}
	for _, line := range ExtractorArgLines(network.ExtractorArgs) {
		args = append(args, "--extractor-args", line)
	}
	return args
}

// DownloadNetworkArgs are NetworkArgs for a download. With several
// fragments at once, YouTube's plain HTTPS formats are fetched as DASH
// fragments (youtube:formats=dashy) so the fragments run in parallel;
// otherwise yt-dlp downloads them over a single connection.
func DownloadNetworkArgs(network config.NetworkSettings, fragments int) []string {
	if fragments > 1 {
		network.ExtractorArgs = withDashy(network.ExtractorArgs)
	}
	return NetworkArgs(network)
}

// withDashy adds formats=dashy to the youtube extractor arguments, unless
// they already choose formats.
func withDashy(value string) string {
	lines := ExtractorArgLines(value)
	for i, line := range lines {
		ie, args, _ := strings.Cut(line, ":")
		if !strings.EqualFold(strings.TrimSpace(ie), "youtube") {
			continue
		}
		switch {
		case strings.Contains(args, "formats="):
		case strings.TrimSpace(args) == "":
			lines[i] = "youtube:formats=dashy"
		default:
			lines[i] = line + ";formats=dashy"
		}
		return strings.Join(lines, "\n")
	}
	return strings.Join(append(lines, "youtube:formats=dashy"), "\n")
}

// ExtractorArgLines returns the trimmed non-empty lines of value.
func ExtractorArgLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// AuthArgs returns the cookie options and the sign-in secrets. Header lines
// without a colon are skipped.
func AuthArgs(auth config.AuthSettings, secrets AuthSecrets) []string {
	var args []string
	if auth.CookieBrowser != "" && auth.CookieBrowser != "none" {
		args = append(args, "--cookies-from-browser", auth.CookieBrowser)
	}
	if strings.TrimSpace(auth.CookieFile) != "" {
		args = append(args, "--cookies", auth.CookieFile)
	}
	if secrets.Username != "" {
		args = append(args, "--username", secrets.Username)
	}
	if secrets.Password != "" {
		args = append(args, "--password", secrets.Password)
	}
	if secrets.VideoPassword != "" {
		args = append(args, "--video-password", secrets.VideoPassword)
	}
	if secrets.BearerToken != "" {
		args = append(args, "--add-header", "Authorization:Bearer "+secrets.BearerToken)
	}
	for _, line := range strings.Split(secrets.Headers, "\n") {
		if line = strings.TrimSpace(line); line != "" && strings.Contains(line, ":") {
			args = append(args, "--add-header", line)
		}
	}
	return args
}

// LocationArgs returns -o with the output path: the per-kind folder (or the
// download folder, or fallbackDir), the per-kind folder template and the
// file name template, with the fields values knows filled in.
func LocationArgs(trackType TrackType, values TemplateValues, output config.OutputSettings, paths config.PathPreferences, fallbackDir string) []string {
	globalDir := output.DownloadDir
	if globalDir == "" {
		globalDir = fallbackDir
	}
	preferred, prefix, filename := paths.VideoDownloadDir, paths.VideoDirectoryTemplate, output.FileNameTemplate
	if trackType == TrackAudio {
		preferred, prefix, filename = paths.AudioDownloadDir, paths.AudioDirectoryTemplate, output.AudioFileNameTemplate
	}
	base := preferred
	if base == "" {
		base = globalDir
	}
	return []string{"-o", values.Render(filepath.Join(base, prefix, filename))}
}

// RunSummary says which sensitive options a command line has, without
// their values, for logging.
type RunSummary struct {
	ArgCount          int
	HasProxy          bool
	HasCookies        bool
	HasBrowserCookies bool
	HasAuth           bool
}

// Summarize inspects flags only, never values, so it cannot leak secrets.
func Summarize(args []string) RunSummary {
	is := func(arg string, flags ...string) bool {
		for _, f := range flags {
			if arg == f || strings.HasPrefix(arg, f+"=") {
				return true
			}
		}
		return false
	}
	s := RunSummary{ArgCount: len(args)}
	for _, arg := range args {
		s.HasProxy = s.HasProxy || is(arg, "--proxy")
		s.HasCookies = s.HasCookies || is(arg, "--cookies")
		s.HasBrowserCookies = s.HasBrowserCookies || is(arg, "--cookies-from-browser")
		s.HasAuth = s.HasAuth || is(arg, "--username", "--password", "--video-password", "--add-header")
	}
	return s
}
