package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pandyzhou/lumadl/internal/config"
	"github.com/pandyzhou/lumadl/internal/secrets"
)

type fakeSettings struct {
	s       config.Settings
	p       config.Preferences
	saves   int
	secrets map[string]string
}

func newFakeSettings() *fakeSettings {
	return &fakeSettings{s: config.DefaultSettings(`C:\Users\me\Downloads`), secrets: map[string]string{secrets.Username: "me"}}
}

func (f *fakeSettings) deps() Deps {
	return Deps{
		Queue:       &fakeQueue{},
		Settings:    func() config.Settings { return f.s },
		Preferences: func() config.Preferences { return f.p },
		UpdateSettings: func(change func(*config.Settings)) error {
			change(&f.s)
			f.saves++
			return nil
		},
		UpdatePreferences: func(change func(*config.Preferences)) error {
			change(&f.p)
			return nil
		},
		GetSecret: func(key string) string { return f.secrets[key] },
		SetSecret: func(key, value string) error {
			f.secrets[key] = value
			return nil
		},
	}
}

// clickRole clicks the control with a role and name, as the settings rows
// also show the name as text beside it.
func clickRole(t *testing.T, tt *ui.Tester, role ui.Role, name string) {
	t.Helper()
	info, ok := tt.AccessByRole(role, name)
	if !ok {
		t.Fatalf("no %v named %q", role, name)
	}
	b := info.Bounds
	tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
}

func TestSettingsSaveImmediately(t *testing.T) {
	f := newFakeSettings()
	a := New(f.deps())
	a.router.Replace("/settings")
	tt := ui.NewTester(a.View, 1000, 900)
	if !tt.HasText("下载位置") || !tt.HasText("加入后自动开始下载") {
		t.Fatalf("general section missing, texts %q", tt.Texts())
	}
	if f.saves != 0 {
		t.Fatalf("saved %d times without a change", f.saves)
	}
	clickRole(t, tt, ui.RoleSwitch, "加入后自动开始下载")
	tt.Frame()
	if f.s.Input.AutoDownload || f.saves != 1 {
		t.Fatalf("toggle not saved: autoDownload %v, saves %d", f.s.Input.AutoDownload, f.saves)
	}
	if err := tt.Click("深色"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if f.s.Appearance.Theme != "dark" {
		t.Errorf("theme %q", f.s.Appearance.Theme)
	}
}

func TestSettingsSearchAndSections(t *testing.T) {
	f := newFakeSettings()
	a := New(f.deps())
	a.router.Replace("/settings")
	tt := ui.NewTester(a.View, 1000, 900)
	if err := tt.Click("登录与网络"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("使用代理") || tt.HasText("下载位置") {
		t.Fatalf("network section wrong, texts %q", tt.Texts())
	}
	a.s.query = "代理"
	tt.Frame()
	if !tt.HasText("代理地址") || tt.HasText("用户名") || !tt.HasText("登录与网络") {
		t.Fatalf("search wrong, texts %q", tt.Texts())
	}
	a.s.query = "不存在的设置"
	tt.Frame()
	if !tt.HasText("没有找到“不存在的设置”相关的设置。") {
		t.Fatalf("empty search, texts %q", tt.Texts())
	}
}

func TestFetchModels(t *testing.T) {
	f := newFakeSettings()
	f.s.Translation.BaseURL = "https://api.example.com/v1"
	f.secrets[secrets.TranslationAPIKey] = "sk-test"
	d := f.deps()
	var asked [2]string
	d.ListModels = func(baseURL, apiKey string, done func([]string, error)) {
		asked = [2]string{baseURL, apiKey}
		done([]string{"model-a", "model-b"}, nil)
	}
	a := New(d)
	a.router.Replace("/settings")
	a.s.section = 2
	tt := ui.NewTester(a.View, 1000, 1400)
	if !tt.HasText("思考强度") {
		t.Fatalf("no reasoning effort row, texts %q", tt.Texts())
	}
	if err := tt.Click("获取模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if asked != [2]string{"https://api.example.com/v1", "sk-test"} || !tt.HasText("接口提供 2 个模型。") {
		t.Fatalf("asked %q, texts %q", asked, tt.Texts())
	}
	if !tt.HasText("选择模型…") {
		t.Errorf("no model list, texts %q", tt.Texts())
	}

	tested := false
	a.deps.TestModel = func(s config.TranslationSettings, apiKey string, done func(string, time.Duration, error)) {
		tested = s.BaseURL == "https://api.example.com/v1" && apiKey == "sk-test"
		done("大家好", 1500*time.Millisecond, nil)
	}
	tt.Frame()
	if err := tt.Click("测试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tested || !tt.HasText("测试成功，用时 1.5 秒：大家好") {
		t.Errorf("tested %v, texts %q", tested, tt.Texts())
	}
}

func TestSettingsSecrets(t *testing.T) {
	f := newFakeSettings()
	a := New(f.deps())
	a.router.Replace("/settings")
	a.s.section = 3
	tt := ui.NewTester(a.View, 1000, 1200)
	clickRole(t, tt, ui.RoleTextField, "密码")
	tt.Type("hunter2")
	tt.Frame()
	if f.secrets[secrets.Password] != "hunter2" {
		t.Fatalf("password %q", f.secrets[secrets.Password])
	}
	if tt.HasText("hunter2") {
		t.Error("password shown in clear")
	}
}

func TestSettingsScreenshots(t *testing.T) {
	dir := os.Getenv("SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	for _, dark := range []bool{true, false} {
		scheme := map[bool]string{true: "dark", false: "light"}[dark]
		for i := range settingSections {
			f := newFakeSettings()
			a := New(f.deps())
			a.router.Replace("/settings")
			a.s.section = i
			tt := ui.NewTesterWith(a.View, 1000, 1000, ui.TesterOptions{Dark: dark})
			tt.Frame()
			out, err := os.Create(filepath.Join(dir, scheme+"-settings-"+string(rune('0'+i))+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(out, tt.Image()); err != nil {
				t.Fatal(err)
			}
			out.Close()
		}
	}
}

func TestNotificationKinds(t *testing.T) {
	f := newFakeSettings()
	f.s.Notifications.NotificationBehavior = config.NotifyAlways // "never" disables the switches
	a := New(f.deps())
	a.router.Replace("/settings")
	a.s.section = 5
	tt := ui.NewTester(a.View, 1000, 1400)
	clickRole(t, tt, ui.RoleCheckBox, "下载失败")
	tt.Frame()
	if len(f.s.Notifications.DisabledNotifications) != 1 || f.s.Notifications.DisabledNotifications[0] != config.NoticeDownloadFailed {
		t.Fatalf("disabled %q", f.s.Notifications.DisabledNotifications)
	}
	clickRole(t, tt, ui.RoleCheckBox, "下载失败")
	tt.Frame()
	if len(f.s.Notifications.DisabledNotifications) != 0 {
		t.Fatalf("re-enabled %q", f.s.Notifications.DisabledNotifications)
	}
}
