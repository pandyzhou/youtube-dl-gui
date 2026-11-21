package ui

import "github.com/egoist/mygo/ui"

// Colors from the redesign: slate surfaces with a green accent.
var (
	darkBackground = ui.Hex("#0f172a")
	darkSurface    = ui.Hex("#172033")
	darkSidebar    = ui.Hex("#0b1220")
	darkBorder     = ui.Hex("#263247")
	darkText       = ui.Hex("#e2e8f0")
	darkMuted      = ui.Hex("#94a3b8")
	darkAccent     = ui.Hex("#22c55e")

	lightBackground = ui.Hex("#f1f5f9")
	lightSurface    = ui.Hex("#ffffff")
	lightSidebar    = ui.Hex("#e7ecf3")
	lightBorder     = ui.Hex("#e2e8f0")
	lightText       = ui.Hex("#0f172a")
	lightMuted      = ui.Hex("#64748b")
	lightAccent     = ui.Hex("#15803d")
)

// fontFamilies draws Latin text in Segoe UI and Chinese in Microsoft YaHei
// UI on Windows; the Noto fallback is what the Linux CI renders with.
const fontFamilies = "Segoe UI, Microsoft YaHei UI, Noto Sans CJK SC, sans-serif"

// palette holds the colors the app uses outside the widgets' own theme.
type palette struct {
	sidebar ui.Color
	border  ui.Color
	muted   ui.Color
	surface ui.Color
}

// applyTheme sets the window's theme for this frame, following the
// system's light or dark appearance, and returns the extra colors.
func applyTheme(c *ui.Context) palette {
	base := c.Theme()
	var t ui.Theme
	var p palette
	if base.Dark {
		t = *base.WithAccent(darkAccent)
		t.Background, t.Surface, t.Border = darkBackground, darkSurface, darkBorder
		t.Text, t.TextMuted = darkText, darkMuted
		p = palette{sidebar: darkSidebar, border: darkBorder, muted: darkMuted, surface: darkSurface}
	} else {
		t = *base.WithAccent(lightAccent)
		t.Background, t.Surface, t.Border = lightBackground, lightSurface, lightBorder
		t.Text, t.TextMuted = lightText, lightMuted
		p = palette{sidebar: lightSidebar, border: lightBorder, muted: lightMuted, surface: lightSurface}
	}
	t.Font = fontFamilies
	t.Radius = 8
	c.SetTheme(&t)
	return p
}
