package ui

import (
	"fmt"
	"sort"

	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/youtube-dl-gui/internal/engine"
	"github.com/pandyzhou/youtube-dl-gui/internal/formats"
	"github.com/pandyzhou/youtube-dl-gui/internal/ytdlp"
)

// formatOffer is what a card's videos offer to choose from: their formats,
// codecs and tracks, merged across the videos of a playlist card.
type formatOffer struct {
	video       []ytdlp.Format // one per height and frame rate, highest first
	audio       []ytdlp.Format // one per bitrate, highest first
	videoCodecs []ytdlp.Codec
	audioCodecs []ytdlp.Codec
	audioTracks []ytdlp.Track
	videoTracks []ytdlp.Track
}

func offerOf(j engine.JobView) formatOffer {
	var o formatOffer
	seenFormat := map[string]bool{}
	seenCodec := map[string]bool{}
	seenTrack := map[string]bool{}
	addCodecs := func(list *[]ytdlp.Codec, kind string, codecs []ytdlp.Codec) {
		for _, c := range codecs {
			if key := kind + c.ID; !seenCodec[key] {
				seenCodec[key] = true
				*list = append(*list, c)
			}
		}
	}
	addTracks := func(list *[]ytdlp.Track, kind string, tracks []ytdlp.Track) {
		for _, t := range tracks {
			if key := kind + t.ID; t.ID != "auto" && !seenTrack[key] {
				seenTrack[key] = true
				*list = append(*list, t)
			}
		}
	}
	for _, it := range j.Items {
		v := it.Video
		if v == nil {
			continue
		}
		for _, f := range v.Formats {
			key := fmt.Sprintf("%d/%d/%d", f.Height, f.Fps, f.Abr)
			if seenFormat[key] {
				continue
			}
			seenFormat[key] = true
			if f.Height > 0 {
				o.video = append(o.video, f)
			} else if f.Abr > 0 {
				o.audio = append(o.audio, f)
			}
		}
		addCodecs(&o.videoCodecs, "v", v.VideoCodecs)
		addCodecs(&o.audioCodecs, "a", v.AudioCodecs)
		addTracks(&o.audioTracks, "a", v.AudioTracks)
		addTracks(&o.videoTracks, "v", v.VideoTracks)
	}
	sort.SliceStable(o.video, func(a, b int) bool {
		if o.video[a].Height != o.video[b].Height {
			return o.video[a].Height > o.video[b].Height
		}
		return o.video[a].Fps > o.video[b].Fps
	})
	sort.SliceStable(o.audio, func(a, b int) bool { return o.audio[a].Abr > o.audio[b].Abr })
	return o
}

// customStart is the format a card gets when "自定义…" is chosen: what its
// preset resolved to, so the panel opens on the current quality.
func customStart(j engine.JobView) ytdlp.FormatOptions {
	p := formats.PresetByID(j.Preset)
	o := offerOf(j)
	if p.Audio {
		return ytdlp.FormatOptions{TrackType: ytdlp.TrackAudio}
	}
	c := formats.Resolve(p, o.video)
	return c.Format
}

// customPanel picks the type, resolution, codecs and tracks of a card that
// uses a custom format. Every change applies at once.
func (a *App) customPanel(c *ui.Context, p palette, j engine.JobView) {
	f := *j.Custom
	o := offerOf(j)
	changed := false

	ui.Column(c).Gap(10).AlignItems(ui.Stretch).Padding(10, 0, 0, thumbWidth+cardPad).Children(func() {
		ui.Box(c).Height(1).Background(p.border)
		ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
			types := []ytdlp.TrackType{ytdlp.TrackBoth, ytdlp.TrackVideo, ytdlp.TrackAudio}
			selected := 0
			for i, t := range types {
				if t == f.TrackType {
					selected = i
				}
			}
			if ui.Segmented(c, &selected, "视频", "仅视频（无声）", "仅音频").Label("下载类型").Changed() {
				f.TrackType = types[selected]
				f.Height, f.Fps, f.Abr = 0, 0, 0
				changed = true
			}

			if f.TrackType == ytdlp.TrackAudio {
				labels, values := []string{"最佳音质"}, []int{0}
				for _, af := range o.audio {
					labels, values = append(labels, fmt.Sprintf("%d kbps", af.Abr)), append(values, af.Abr)
				}
				current := labels[0]
				for i, v := range values {
					if v == f.Abr {
						current = labels[i]
					}
				}
				if ui.Select(c, &current, labels).Width(130).Height(32).Label("音质").Changed() {
					for i, l := range labels {
						if l == current {
							f.Abr = values[i]
						}
					}
					changed = true
				}
			} else {
				labels := []string{"最高分辨率"}
				type hf struct{ h, fps int }
				values := []hf{{}}
				for _, vf := range o.video {
					labels, values = append(labels, formats.ResolutionLabel(vf)), append(values, hf{vf.Height, vf.Fps})
				}
				current := labels[0]
				for i, v := range values {
					if v.h == f.Height && v.fps == f.Fps {
						current = labels[i]
					}
				}
				if ui.Select(c, &current, labels).Width(130).Height(32).Label("分辨率").Changed() {
					for i, l := range labels {
						if l == current {
							f.Height, f.Fps = values[i].h, values[i].fps
						}
					}
					changed = true
				}
				if codecSelect(c, "视频编码", &f.VideoEncoding, o.videoCodecs) {
					changed = true
				}
			}
			if f.TrackType != ytdlp.TrackVideo && codecSelect(c, "音频编码", &f.AudioEncoding, o.audioCodecs) {
				changed = true
			}
			if f.TrackType != ytdlp.TrackVideo && len(o.audioTracks) > 1 && trackSelect(c, "音轨", &f.AudioTrack, o.audioTracks) {
				changed = true
			}
			if f.TrackType != ytdlp.TrackAudio && len(o.videoTracks) > 1 && trackSelect(c, "视频语言", &f.VideoTrack, o.videoTracks) {
				changed = true
			}
		})
	})
	if changed {
		a.queue.SetCustom(j.ID, f)
	}
}

// codecSelect chooses a codec by ID, "" being automatic.
func codecSelect(c *ui.Context, label string, value *string, codecs []ytdlp.Codec) bool {
	if len(codecs) == 0 {
		return false
	}
	labels, ids := []string{label + "：自动"}, []string{""}
	for _, codec := range codecs {
		labels, ids = append(labels, label+"："+codec.Label), append(ids, codec.ID)
	}
	return pick(c, label, value, labels, ids, 190)
}

// trackSelect chooses a track by ID, "" being automatic.
func trackSelect(c *ui.Context, label string, value *string, tracks []ytdlp.Track) bool {
	labels, ids := []string{label + "：自动"}, []string{""}
	for _, t := range tracks {
		labels, ids = append(labels, label+"："+t.Label), append(ids, t.ID)
	}
	return pick(c, label, value, labels, ids, 150)
}

func pick(c *ui.Context, label string, value *string, labels, ids []string, width float32) bool {
	current := labels[0]
	for i, id := range ids {
		if id == *value {
			current = labels[i]
		}
	}
	if !ui.Select(c, &current, labels).Width(width).Height(32).Label(label).Changed() {
		return false
	}
	for i, l := range labels {
		if l == current {
			*value = ids[i]
		}
	}
	return true
}
