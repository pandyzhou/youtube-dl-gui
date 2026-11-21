// Package httpx makes the HTTP client the app uses for its own requests
// (tools, thumbnails, translation), honoring the proxy setting.
package httpx

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pandyzhou/youtube-dl-gui/internal/config"
)

// Client returns a client that uses the configured proxy when it is on and
// parses, and the system proxy (environment variables) otherwise. Timeout
// bounds a whole request; 0 means none (large downloads).
func Client(network config.NetworkSettings, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if network.EnableProxy {
		if u, err := url.Parse(strings.TrimSpace(network.Proxy)); err == nil && u.Scheme != "" && u.Host != "" {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}
