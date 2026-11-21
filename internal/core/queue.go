package core

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/httpx"
	"github.com/pandyzhou/youtube-dl-gui/internal/secrets"
	"github.com/pandyzhou/youtube-dl-gui/internal/translate"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// AuthSecrets reads the sign-in secrets for yt-dlp; a secret that cannot
// be read is logged and left out.
func (c *Core) AuthSecrets() ytdlp.AuthSecrets {
	get := func(key string) string {
		v, err := c.Secrets.Get(key)
		if err != nil {
			slog.Warn("read secret", "key", key, "err", err)
		}
		return v
	}
	return ytdlp.AuthSecrets{
		Username:      get(secrets.Username),
		Password:      get(secrets.Password),
		VideoPassword: get(secrets.VideoPassword),
		BearerToken:   get(secrets.BearerToken),
		Headers:       get(secrets.Headers),
	}
}

// NewEngine starts the download queue on the installed yt-dlp; downloads
// wait while the tools are being installed or updated.
func NewEngine(c *Core, tools *Tools, changed func(), events func(engine.Event)) *engine.Engine {
	return engine.New(engine.Deps{
		Runner:      engine.YtdlpRunner{Path: tools.Manager().Path("yt-dlp"), BinDir: c.Dirs.Bin},
		Settings:    c.Settings,
		Preferences: c.Preferences,
		Secrets:     c.AuthSecrets,
		FallbackDir: c.Dirs.Downloads,
		WaitTools: func(ctx context.Context) error {
			for !tools.Ready() || tools.Busy() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(500 * time.Millisecond):
				}
			}
			return nil
		},
		BeforeDownload: func(ctx context.Context, st engine.Started, r engine.Reporter) any {
			if !c.Settings().Translation.Enabled {
				return nil
			}
			return translate.Prepare(ctx, translateEnv(c, tools), st, r)
		},
		AfterDownload: func(ctx context.Context, d engine.Finished, r engine.Reporter) {
			if !c.Settings().Translation.Enabled && !d.SubtitlesOnly {
				return
			}
			translate.AfterDownload(ctx, translateEnv(c, tools), d, r)
		},
		OnChange: changed,
		OnEvent:  events,
		Throttle: 100 * time.Millisecond,
	})
}

// translateEnv is what a translation needs, as the settings are now.
func translateEnv(c *Core, tools *Tools) translate.Env {
	settings := c.Settings()
	key, err := c.Secrets.Get(secrets.TranslationAPIKey)
	if err != nil {
		slog.Warn("read translation key", "err", err)
	}
	return translate.Env{
		Runner:   engine.YtdlpRunner{Path: tools.Manager().Path("yt-dlp"), BinDir: c.Dirs.Bin},
		Settings: settings,
		Secrets:  c.AuthSecrets(),
		APIKey:   key,
		Client:   httpx.Client(settings.Network, 5*time.Minute),
		CacheDir: filepath.Join(c.Dirs.Data, "translate-cache"),
		FFmpeg:   tools.Manager().Path("ffmpeg"),
		FFprobe:  tools.Manager().Path("ffprobe"),
	}
}
