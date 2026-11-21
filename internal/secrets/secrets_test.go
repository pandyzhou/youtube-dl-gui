package secrets

import (
	"strings"
	"testing"
)

func TestSmallSecretsLiveInTheBackend(t *testing.T) {
	backend := NewMemory()
	s := New(backend, t.TempDir())
	if v, err := s.Get(TranslationAPIKey); err != nil || v != "" {
		t.Fatalf("unset secret: %q %v", v, err)
	}
	if err := s.Set(TranslationAPIKey, "sk-123"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get(TranslationAPIKey); v != "sk-123" {
		t.Fatalf("got %q", v)
	}
	if raw, _ := backend.Get(TranslationAPIKey); string(raw) != "sk-123" {
		t.Fatalf("backend holds %q", raw)
	}
	if !s.HasAny(TranslationAPIKey) || s.HasAny(AuthKeys...) {
		t.Fatal("HasAny wrong")
	}
	if err := s.Set(TranslationAPIKey, "  "); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get(TranslationAPIKey); v != "" {
		t.Fatalf("blank value did not delete: %q", v)
	}
}

func TestLargeSecretsGoToAnEncryptedFile(t *testing.T) {
	backend := NewMemory()
	dir := t.TempDir()
	s := New(backend, dir)
	big := strings.Repeat("X-Header: value\n", 400) // ~6.4 KB, over the limit
	if err := s.Set(Headers, big); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get(Headers); v != big {
		t.Fatalf("large secret round trip failed (%d bytes)", len(v))
	}
	if _, err := backend.Get(Headers); err == nil {
		t.Fatal("large value kept in the backend")
	}

	// Back to a small value: the file and its key go away.
	if err := s.Set(Headers, "A: b"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get(Headers); v != "A: b" {
		t.Fatalf("got %q", v)
	}
	if _, err := backend.Get(Headers + fileKeySuffix); err == nil {
		t.Fatal("file key left behind")
	}
	if err := s.Delete(Headers); err != nil {
		t.Fatal(err)
	}
	if s.HasAny(Headers) {
		t.Fatal("deleted secret still set")
	}
}
