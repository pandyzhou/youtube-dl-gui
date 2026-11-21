package formats

import (
	"testing"

	"github.com/pandyzhou/lumadl/internal/ytdlp"
)

func TestMixedLink(t *testing.T) {
	cases := []struct {
		link       string
		mixed, mix bool
	}{
		{"https://www.youtube.com/watch?v=TAMyLwW9HW8&list=RDTAMyLwW9HW8&start_radio=1", true, true},
		{"https://www.youtube.com/watch?v=abc&list=PLxyz", true, false},
		{"https://youtu.be/abc?list=PLxyz", true, false},
		{"https://music.youtube.com/watch?v=abc&list=RDAMVMabc", true, true},
		{"https://www.youtube.com/playlist?list=PLxyz", false, false},
		{"https://www.youtube.com/watch?v=abc", false, false},
		{"https://example.com/watch?v=abc&list=PL", false, false},
		{"not a link", false, false},
	}
	for _, c := range cases {
		if mixed, mix := MixedLink(c.link); mixed != c.mixed || mix != c.mix {
			t.Errorf("%s: %v %v", c.link, mixed, mix)
		}
	}
	mixed := "https://www.youtube.com/watch?v=abc&list=PLxyz"
	if MixedMode(mixed, MixedAsk) != ytdlp.PlaylistModeSingle || MixedMode(mixed, MixedList) != ytdlp.PlaylistModeAll ||
		MixedMode("https://www.youtube.com/watch?v=abc", MixedList) != ytdlp.PlaylistModeNone {
		t.Error("modes")
	}
}
