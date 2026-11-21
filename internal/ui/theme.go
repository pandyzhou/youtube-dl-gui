package ui

import "github.com/egoist/mygo/ui"

// The look follows the "macOS Vibrancy" style (stylekit.top): three depths
// of neutral gray, thin white or black hairlines, the system blue accent and
// a translucent sidebar over the window material (Mica on Windows 11).
// The style only defines a dark palette; the light one uses macOS's light
// system colors.
var (
	darkSidebar  = ui.RGBA(28, 28, 30, 0.8) // vibrancy, used only without the material
	darkContent  = ui.Hex("#1e1e20")
	darkSurface  = ui.Hex("#2c2c2e")
	darkControl  = ui.Hex("#3a3a3c")
	darkBorder   = ui.RGBA(255, 255, 255, 0.08)
	darkText     = ui.RGBA(255, 255, 255, 0.92)
	darkMuted    = ui.RGBA(255, 255, 255, 0.55)
	darkAccent   = ui.Hex("#0a84ff")
	darkSelected = ui.RGBA(10, 132, 255, 0.18)

	lightSidebar  = ui.Hex("#ececee")
	lightContent  = ui.Hex("#ffffff")
	lightSurface  = ui.Hex("#f5f5f7")
	lightControl  = ui.Hex("#e8e8ed")
	lightBorder   = ui.RGBA(0, 0, 0, 0.1)
	lightText     = ui.Hex("#1d1d1f")
	lightMuted    = ui.Hex("#6e6e73")
	lightAccent   = ui.Hex("#007aff")
	lightSelected = ui.RGBA(0, 122, 255, 0.14)
)

// fontFamilies draws Latin text in Segoe UI and Chinese in Microsoft YaHei
// UI on Windows; the Noto fallback is what the Linux CI renders with.
const fontFamilies = "Segoe UI Variable Text, Segoe UI, Microsoft YaHei UI, Noto Sans CJK SC, sans-serif"

// palette holds the colors the app uses outside the widgets' own theme.
type palette struct {
	sidebar  ui.Color // sidebar background where the window has no material
	content  ui.Color // the page behind the content
	surface  ui.Color // cards and panels
	control  ui.Color // raised controls, the selected segment
	border   ui.Color
	muted    ui.Color
	selected ui.Color // selected rows and items
}

// applyTheme sets the window's theme for this frame: the system's light or
// dark look, or the one the appearance setting forces. forced reports a
// look other than the system's, over which the window material (tinted
// for the system's look) must not show.
func applyTheme(c *ui.Context, mode string) (p palette, forced bool) {
	base := c.Theme()
	switch {
	case mode == "dark" && !base.Dark:
		base, forced = ui.DarkTheme(), true
	case mode == "light" && base.Dark:
		base, forced = ui.LightTheme(), true
	}
	var t ui.Theme
	if base.Dark {
		t = *base.WithAccent(darkAccent)
		t.Background, t.Surface, t.Border = darkContent, darkSurface, darkBorder
		t.SurfaceHover = ui.RGBA(255, 255, 255, 0.08)
		t.Text, t.TextMuted = darkText, darkMuted
		t.Selection = darkSelected
		t.Success, t.Warning, t.Danger = ui.Hex("#30d158"), ui.Hex("#ff9f0a"), ui.Hex("#ff453a")
		p = palette{darkSidebar, darkContent, darkSurface, darkControl, darkBorder, darkMuted, darkSelected}
	} else {
		t = *base.WithAccent(lightAccent)
		t.Background, t.Surface, t.Border = lightContent, lightSurface, lightBorder
		t.SurfaceHover = ui.RGBA(0, 0, 0, 0.05)
		t.Text, t.TextMuted = lightText, lightMuted
		t.Selection = lightSelected
		t.Success, t.Warning, t.Danger = ui.Hex("#34c759"), ui.Hex("#ff9500"), ui.Hex("#ff3b30")
		p = palette{lightSidebar, lightContent, lightSurface, lightControl, lightBorder, lightMuted, lightSelected}
	}
	t.Font = fontFamilies
	t.Radius = 6
	c.SetTheme(&t)
	return p, forced
}
