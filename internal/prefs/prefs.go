package prefs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Prefs holds global UI preferences that persist across sessions.
type Prefs struct {
	SummaryMode   int                 `json:"summary_mode"`
	CollapsedTags map[string][]string `json:"collapsed_tags"` // spec URL → collapsed tag names
}

func filePath(storageDir string) string {
	return filepath.Join(storageDir, "prefs.json")
}

// Load reads Prefs from disk, returning defaults if no file exists.
func Load(storageDir string) (*Prefs, error) {
	p := &Prefs{
		SummaryMode:   1,
		CollapsedTags: make(map[string][]string),
	}
	data, err := os.ReadFile(filePath(storageDir))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, fmt.Errorf("prefs: read: %w", err)
	}
	if err := json.Unmarshal(data, p); err != nil {
		return p, fmt.Errorf("prefs: unmarshal: %w", err)
	}
	if p.CollapsedTags == nil {
		p.CollapsedTags = make(map[string][]string)
	}
	return p, nil
}

// Save writes Prefs to disk as plain JSON.
func Save(p *Prefs, storageDir string) error {
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
		return fmt.Errorf("prefs: mkdir: %w", err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("prefs: marshal: %w", err)
	}
	if err := os.WriteFile(filePath(storageDir), data, 0o600); err != nil {
		return fmt.Errorf("prefs: write: %w", err)
	}
	return nil
}
