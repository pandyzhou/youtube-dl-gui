package core

import (
	"context"
	"log/slog"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/secrets"
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
		OnChange: changed,
		OnEvent:  events,
		Throttle: 100 * time.Millisecond,
	})
}
