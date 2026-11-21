package translate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// checkpoint keeps the finished batches and the notes of one video's
// translation on disk, so a translation that was stopped goes on where it
// was. It belongs to one list of source lines and one target language;
// anything else starts over. A nil checkpoint keeps nothing.
type checkpoint struct {
	path string
	mu   sync.Mutex
	data checkpointData
}

type checkpointData struct {
	Lines   string              `json:"lines"` // hash of the source lines
	Target  string              `json:"target"`
	Notes   string              `json:"notes,omitempty"`
	Batches map[string][]string `json:"batches"`
}

// openCheckpoint loads the checkpoint for key (the video and its subtitle
// language) from dir; "" turns checkpoints off.
func openCheckpoint(dir, key string, texts []string, target string) *checkpoint {
	if dir == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(key))
	c := &checkpoint{path: filepath.Join(dir, hex.EncodeToString(sum[:12])+".json")}
	fresh := checkpointData{Lines: linesHash(texts), Target: target, Batches: map[string][]string{}}
	if data, err := os.ReadFile(c.path); err == nil && json.Unmarshal(data, &c.data) == nil &&
		c.data.Lines == fresh.Lines && c.data.Target == fresh.Target && c.data.Batches != nil {
		return c
	}
	c.data = fresh
	return c
}

func linesHash(texts []string) string {
	sum := sha256.Sum256([]byte(strings.Join(texts, "\n")))
	return hex.EncodeToString(sum[:])
}

// batch returns the saved translation of the n lines from start.
func (c *checkpoint) batch(start, n int) ([]string, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lines, ok := c.data.Batches[strconv.Itoa(start)]
	return lines, ok && len(lines) == n
}

// count is how many batches are saved.
func (c *checkpoint) count() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data.Batches)
}

func (c *checkpoint) notes() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data.Notes
}

func (c *checkpoint) setNotes(notes string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.data.Notes = notes
	c.saveLocked()
	c.mu.Unlock()
}

// save keeps a finished batch.
func (c *checkpoint) save(start int, lines []string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.data.Batches[strconv.Itoa(start)] = append([]string(nil), lines...)
	c.saveLocked()
	c.mu.Unlock()
}

// saveLocked writes the file; a checkpoint that cannot be written only
// costs a fresh start next time.
func (c *checkpoint) saveLocked() {
	data, err := json.Marshal(c.data)
	if err != nil || os.MkdirAll(filepath.Dir(c.path), 0o755) != nil {
		return
	}
	tmp := c.path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, c.path)
	}
}

// remove deletes the checkpoint once the translation is complete.
func (c *checkpoint) remove() {
	if c == nil {
		return
	}
	c.mu.Lock()
	_ = os.Remove(c.path)
	c.mu.Unlock()
}
