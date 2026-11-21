package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNavigation(t *testing.T) {
	a := New()
	tt := ui.NewTester(a.View, 1000, 760)
	if !tt.HasText("队列为空") {
		t.Fatalf("queue page not shown, texts %q", tt.Texts())
	}
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	if a.router.Path() != "/about" || !tt.HasText("版本 dev") {
		t.Fatalf("about page not shown: path %q, texts %q", a.router.Path(), tt.Texts())
	}
}

// TestScreenshots writes PNGs of the main pages to $SCREENSHOT_DIR, which CI
// uploads as an artifact so the layout can be reviewed without running the app.
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{true, false} {
		scheme := "light"
		if dark {
			scheme = "dark"
		}
		for _, page := range []string{"/queue", "/settings", "/about"} {
			a := New()
			a.router.Replace(page)
			tt := ui.NewTesterWith(a.View, 1000, 760, ui.TesterOptions{Dark: dark})
			tt.Frame()
			name := filepath.Join(dir, scheme+"-"+filepath.Base(page)+".png")
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
