package ytdlp

import "testing"

// Ported from src-tauri/src/runners/template_context.rs.
func TestRender(t *testing.T) {
	cases := []struct {
		name   string
		values TemplateValues
		tmpl   string
		want   string
	}{
		{"unknown field untouched", TemplateValues{}, "%(title)s", "%(title)s"},
		{"unknown numeric untouched", TemplateValues{}, "%(fps)s", "%(fps)s"},
		{"known index padded, unknown kept", TemplateValues{"playlist_index": "5"}, "%(playlist_index)03d - %(title)s", "005 - %(title)s"},
		{"known but empty numeric", TemplateValues{"fps": ""}, "%(fps).0d", ""},
		{"simple", TemplateValues{"title": "My Video"}, "File: %(title)s.mp4", "File: My Video.mp4"},
		{"fallback primary present", TemplateValues{"title": "Fallback Title", "primary": "Primary Value"}, "%(primary,title)s", "Primary Value"},
		{"fallback only secondary", TemplateValues{"title": "Only Title"}, "%(primary,title)s", "%(primary|Only Title)s"},
		{"replacement present", TemplateValues{"playlist_index": "3"}, "%(playlist_index& - )sTitle", " - Title"},
		{"replacement absent", TemplateValues{}, "%(playlist_index& - )sTitle", "%(playlist_index& - )sTitle"},
		{"replacement with default", TemplateValues{"playlist_index": "7"}, "%(playlist_index& - |)sTitle", " - Title"},
		{"zero pad", TemplateValues{"playlist_index": "5", "title": "My Video"}, "%(playlist_index)03d - %(title)s", "005 - My Video"},
		{"space pad", TemplateValues{"playlist_index": "5"}, "%(playlist_index)2d", " 5"},
		{"non numeric with d", TemplateValues{"index": "abc"}, "%(index)03d", "abc"},
		{"date modifier ignored", TemplateValues{"upload_date": "20250101"}, "%(upload_date>%Y-%m-%d)s", "20250101"},
		{"literal percent", TemplateValues{"title": "My Video"}, "Progress: 50%% - %(title)s", "Progress: 50% - My Video"},
		{"percent around field", TemplateValues{"title": "My Video"}, "%%%(title)s%%", "%My Video%"},
		{"same field twice", TemplateValues{"title": "My Video"}, "%(title)s - %(title)s", "My Video - My Video"},
		{"realistic", TemplateValues{"playlist_title": "My Playlist", "playlist_index": "5", "autonumber": "12"},
			"%(playlist_title|)s/%(playlist_index)03d - %(title)s.%(ext)s", "My Playlist/005 - %(title)s.%(ext)s"},
		{"realistic unknown", TemplateValues{}, "%(playlist_title|)s/%(playlist_index)03d - %(title)s.%(ext)s",
			"%(playlist_title|)s/%(playlist_index)03d - %(title)s.%(ext)s"},
		{"dot zero d", TemplateValues{"playlist_count": "4"}, "%(playlist_count).0d-%(title).200s", "4-%(title).200s"},
		{"combined right", TemplateValues{"playlist_title": "my playlist"}, "%(album,playlist_title).s-%(title).200s", "%(album|my playlist).s-%(title).200s"},
		{"combined left", TemplateValues{"playlist_title": "my playlist"}, "%(playlist_title,album)s-%(title).200s", "my playlist-%(title).200s"},
		{"combined center", TemplateValues{"playlist_title": "my playlist"}, "%(uploader,playlist_title,album)s-%(title).200s", "%(uploader|my playlist)s-%(title).200s"},
		{"combined unknown", TemplateValues{}, "%(album,playlist_title)s-%(title)s", "%(album,playlist_title)s-%(title)s"},
		{"path traversal", TemplateValues{"playlist_title": "../../malicious"}, "/downloads/%(playlist_title)s/video.mp4", "/downloads/.._.._malicious/video.mp4"},
	}
	for _, c := range cases {
		if got := c.values.Render(c.tmpl); got != c.want {
			t.Errorf("%s: Render(%q) = %q, want %q", c.name, c.tmpl, got, c.want)
		}
	}
}
