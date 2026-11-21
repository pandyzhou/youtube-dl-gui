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
	//go:embed icons/play.svg
	playSVG []byte
	//go:embed icons/pause.svg
	pauseSVG []byte
	//go:embed icons/rotate-ccw.svg
	retrySVG []byte
	//go:embed icons/folder-open.svg
	folderOpenSVG []byte
	//go:embed icons/x.svg
	closeSVG []byte
	//go:embed icons/trash-2.svg
	trashSVG []byte
	//go:embed icons/ellipsis.svg
	moreSVG []byte
	//go:embed icons/circle-alert.svg
	alertSVG []byte
	//go:embed icons/circle-check.svg
	checkSVG []byte
	//go:embed icons/list-checks.svg
	listChecksSVG []byte
	//go:embed icons/file-video.svg
	fileVideoSVG []byte
	//go:embed icons/arrow-left.svg
	backSVG []byte
	//go:embed icons/chevron-down.svg
	chevronSVG []byte
	//go:embed icons/image.svg
	imageSVG []byte

	iconQueue      = ui.MustParseSVG(queueSVG)
	iconSettings   = ui.MustParseSVG(settingsSVG)
	iconInfo       = ui.MustParseSVG(infoSVG)
	iconDownload   = ui.MustParseSVG(downloadSVG)
	iconLink       = ui.MustParseSVG(linkSVG)
	iconClipboard  = ui.MustParseSVG(clipboardSVG)
	iconFolder     = ui.MustParseSVG(folderSVG)
	iconFunnel     = ui.MustParseSVG(funnelSVG)
	iconPlay       = ui.MustParseSVG(playSVG)
	iconPause      = ui.MustParseSVG(pauseSVG)
	iconRetry      = ui.MustParseSVG(retrySVG)
	iconFolderOpen = ui.MustParseSVG(folderOpenSVG)
	iconClose      = ui.MustParseSVG(closeSVG)
	iconTrash      = ui.MustParseSVG(trashSVG)
	iconMore       = ui.MustParseSVG(moreSVG)
	iconAlert      = ui.MustParseSVG(alertSVG)
	iconCheck      = ui.MustParseSVG(checkSVG)
	iconListChecks = ui.MustParseSVG(listChecksSVG)
	iconFileVideo  = ui.MustParseSVG(fileVideoSVG)
	iconBack       = ui.MustParseSVG(backSVG)
	iconChevron    = ui.MustParseSVG(chevronSVG)
	iconImage      = ui.MustParseSVG(imageSVG)
)
	iconSettings  = ui.MustParseSVG(settingsSVG)
	iconInfo      = ui.MustParseSVG(infoSVG)
	iconDownload  = ui.MustParseSVG(downloadSVG)
	iconLink      = ui.MustParseSVG(linkSVG)
	iconClipboard = ui.MustParseSVG(clipboardSVG)
	iconFolder    = ui.MustParseSVG(folderSVG)
	iconFunnel    = ui.MustParseSVG(funnelSVG)
)

//go:embed icons/app.png
var appPNG []byte

// appIcon is the app's logo, drawn in the sidebar and the about page.
var appIcon = mustBitmap(appPNG)

func mustBitmap(data []byte) *ui.Bitmap {
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		panic(err)
	}
	return bmp
}
