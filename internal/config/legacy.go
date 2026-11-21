package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// LegacyIdentifier names the Tauri app's data folder under %APPDATA%.
const LegacyIdentifier = "com.jelleglebbeek.youtube-dl-gui"

// Legacy is what ImportLegacy read from the Tauri app.
type Legacy struct {
	Settings    Settings
	Preferences Preferences
}

// legacyConfig reads the proxy switch, which the Tauri app left null until
// the user set it, meaning "on when a proxy is filled in".
type legacyConfig struct {
	Network struct {
		EnableProxy *bool   `json:"enableProxy"`
		Proxy       *string `json:"proxy"`
	} `json:"network"`
}

// ImportLegacy reads the Tauri app's config.store.json and
// preferences.store.json from dir (both wrap their value in a root key).
// ok is false when there is no config file to import. Removed features
// (SponsorBlock, precise cuts, window geometry) are dropped; secrets are
// not imported.
func ImportLegacy(dir, downloadDir string) (Legacy, bool) {
	var root struct {
		Config json.RawMessage `json:"config"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.store.json"))
	if err != nil || json.Unmarshal(data, &root) != nil || len(root.Config) == 0 {
		return Legacy{}, false
	}
	result := Legacy{Settings: DefaultSettings(downloadDir), Preferences: DefaultPreferences()}

	// Null strings fail to decode into Go strings, so turn every null into
	// a missing field first; the defaults then stay.
	cfg := stripNulls(root.Config)
	_ = json.Unmarshal(cfg, &result.Settings)

	var shape legacyConfig
	_ = json.Unmarshal(root.Config, &shape)
	if shape.Network.EnableProxy == nil {
		result.Settings.Network.EnableProxy = shape.Network.Proxy != nil && strings.TrimSpace(*shape.Network.Proxy) != ""
	}

	var prefsRoot struct {
		Preferences json.RawMessage `json:"preferences"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "preferences.store.json")); err == nil &&
		json.Unmarshal(data, &prefsRoot) == nil && len(prefsRoot.Preferences) > 0 {
		_ = json.Unmarshal(stripNulls(prefsRoot.Preferences), &result.Preferences)
	}
	if result.Preferences.Recents.Recent == nil {
		result.Preferences.Recents.Recent = map[string][]string{}
	}
	result.Settings = Normalize(result.Settings, downloadDir)
	return result, true
}

// stripNulls removes every null-valued object member from a JSON document.
func stripNulls(raw json.RawMessage) json.RawMessage {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return raw
	}
	out, err := json.Marshal(dropNulls(value))
	if err != nil {
		return raw
	}
	return out
}

func dropNulls(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if child == nil {
				delete(v, key)
				continue
			}
			v[key] = dropNulls(child)
		}
		return v
	case []any:
		for i, child := range v {
			v[i] = dropNulls(child)
		}
		return v
	default:
		return v
	}
}
