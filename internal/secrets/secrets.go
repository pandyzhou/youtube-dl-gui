// Package secrets keeps passwords, tokens, custom headers and the
// translation API key out of the settings file, in the system's credential
// store (the Credential Manager on Windows).
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Keys of the secrets the app keeps; they match the Tauri app's names.
const (
	Username          = "auth.username"
	Password          = "auth.password"
	VideoPassword     = "video.password"
	BearerToken       = "auth.bearer"
	Headers           = "auth.headers" // one "Name: value" per line
	TranslationAPIKey = "translate.apiKey"
)

// AuthKeys are the secrets that sign in to sites.
var AuthKeys = []string{Username, Password, VideoPassword, BearerToken, Headers}

// ErrNotFound reports a secret that is not set.
var ErrNotFound = errors.New("secret not found")

// Backend is a credential store: the system's (System) or one in memory.
type Backend interface {
	Get(key string) ([]byte, error) // ErrNotFound when missing
	Set(key string, value []byte) error
	Delete(key string) error // nil when missing
}

// MaxBackendSize is the largest value a Backend keeps (the Credential
// Manager's limit). Larger values go to an encrypted file.
const MaxBackendSize = 2560

// Store reads and writes secrets as strings. Values too large for the
// backend are encrypted with AES-GCM into a file in dir, under a random
// key that the backend keeps.
type Store struct {
	backend Backend
	dir     string
	mu      sync.Mutex
}

// New returns a store on backend, with large values in dir.
func New(backend Backend, dir string) *Store {
	return &Store{backend: backend, dir: dir}
}

const fileKeySuffix = ".filekey"

// Get returns a secret, or "" when it is not set.
func (s *Store) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.backend.Get(key)
	if err == nil {
		return string(value), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return "", err
	}
	fileKey, err := s.backend.Get(key + fileKeySuffix)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.readFile(key, fileKey)
}

// Set stores a secret; an empty value deletes it.
func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(value) == "" {
		return s.deleteLocked(key)
	}
	if len(value) <= MaxBackendSize {
		if err := s.backend.Set(key, []byte(value)); err != nil {
			return err
		}
		s.removeFile(key)
		return s.backend.Delete(key + fileKeySuffix)
	}
	fileKey := make([]byte, 32)
	if _, err := rand.Read(fileKey); err != nil {
		return err
	}
	if err := s.writeFile(key, fileKey, []byte(value)); err != nil {
		return err
	}
	if err := s.backend.Set(key+fileKeySuffix, fileKey); err != nil {
		return err
	}
	return s.backend.Delete(key)
}

// Delete removes a secret.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteLocked(key)
}

func (s *Store) deleteLocked(key string) error {
	s.removeFile(key)
	return errors.Join(s.backend.Delete(key), s.backend.Delete(key+fileKeySuffix))
}

// HasAny reports whether any of keys is set.
func (s *Store) HasAny(keys ...string) bool {
	for _, key := range keys {
		if value, err := s.Get(key); err == nil && value != "" {
			return true
		}
	}
	return false
}

func (s *Store) filePath(key string) string {
	return filepath.Join(s.dir, "secret-"+hex.EncodeToString([]byte(key))+".bin")
}

func (s *Store) removeFile(key string) {
	_ = os.Remove(s.filePath(key))
}

func (s *Store) writeFile(key string, fileKey, plain []byte) error {
	gcm, err := newGCM(fileKey)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	sealed := gcm.Seal(nonce, nonce, plain, []byte(key))
	return os.WriteFile(s.filePath(key), sealed, 0o600)
}

func (s *Store) readFile(key string, fileKey []byte) (string, error) {
	sealed, err := os.ReadFile(s.filePath(key))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	gcm, err := newGCM(fileKey)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", fmt.Errorf("secret %s: file too short", key)
	}
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte(key))
	if err != nil {
		return "", fmt.Errorf("secret %s: %w", key, err)
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Memory is a Backend in memory, for tests.
type Memory struct {
	mu     sync.Mutex
	values map[string][]byte
}

// NewMemory returns an empty in-memory backend.
func NewMemory() *Memory { return &Memory{values: map[string][]byte{}} }

// Get implements Backend.
func (m *Memory) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

// Set implements Backend.
func (m *Memory) Set(key string, value []byte) error {
	if len(value) > MaxBackendSize {
		return fmt.Errorf("secret %s: %d bytes is over the limit", key, len(value))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = append([]byte(nil), value...)
	return nil
}

// Delete implements Backend.
func (m *Memory) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	return nil
}
