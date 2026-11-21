package ui

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// Icons from Lucide (ISC license), drawn in the color of the text.
var (
	//go:embed icons/list-video.svg
	queueSVG []byte
	//go:embed icons/settings.svg
	settingsSVG []byte
	//go:embed icons/info.svg
	infoSVG []byte
	//go:embed icons/download.svg
	downloadSVG []byte
	//go:embed icons/link.svg
	linkSVG []byte
	//go:embed icons/clipboard-list.svg
	clipboardSVG []byte
	//go:embed icons/folder-down.svg
	folderSVG []byte
	//go:embed icons/funnel.svg
	funnelSVG []byte

	iconQueue     = ui.MustParseSVG(queueSVG)
	iconSettings  = ui.MustParseSVG(settingsSVG)
	iconInfo      = ui.MustParseSVG(infoSVG)
	iconDownload  = ui.MustParseSVG(downloadSVG)
	iconLink      = ui.MustParseSVG(linkSVG)
	iconClipboard = ui.MustParseSVG(clipboardSVG)
	iconFolder    = ui.MustParseSVG(folderSVG)
	iconFunnel    = ui.MustParseSVG(funnelSVG)
)
