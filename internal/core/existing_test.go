package core

import "testing"

func TestURLFromProbe(t *testing.T) {
	probe := []byte(`{"format": {"tags": {"title": "GTA 6", "PURL": "https://www.youtube.com/watch?v=dBCh9I6Ui7Y", "COMMENT": "https://www.youtube.com/watch?v=dBCh9I6Ui7Y"}}}`)
	if u, ok := URLFromProbe(probe, `F:\v\GTA 6.mkv`); !ok || u != "https://www.youtube.com/watch?v=dBCh9I6Ui7Y" {
		t.Errorf("tags: %q %v", u, ok)
	}
	comment := []byte(`{"format": {"tags": {"comment": "just a note", "description": "x"}}}`)
	if u, ok := URLFromProbe(comment, `F:\v\My video [abcDEF12_-3].mp4`); !ok || u != "https://www.youtube.com/watch?v=abcDEF12_-3" {
		t.Errorf("file name: %q %v", u, ok)
	}
	if _, ok := URLFromProbe([]byte("not json"), `F:\v\clip.mp4`); ok {
		t.Error("found a link in nothing")
	}
	if !IsVideoFile(`F:\v\a.MKV`) || IsVideoFile(`F:\v\a.srt`) {
		t.Error("video extensions")
	}
}
