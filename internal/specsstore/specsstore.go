package specsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SavedSpec is a named, persisted OpenAPI spec URL.
type SavedSpec struct {
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	LastUsed time.Time `json:"last_used,omitempty"`
}

func specsFilePath(storageDir string) string {
	return filepath.Join(storageDir, "specs.json")
}

func recentFilePath(storageDir string) string {
	return filepath.Join(storageDir, "recent.json")
}

// Load reads saved specs from disk. Returns an empty slice if none exist.
// Results are sorted by LastUsed descending (most recently used first).
func Load(storageDir string) ([]SavedSpec, error) {
	path := specsFilePath(storageDir)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []SavedSpec{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("specsstore: read: %w", err)
	}
	var specs []SavedSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, fmt.Errorf("specsstore: unmarshal: %w", err)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].LastUsed.After(specs[j].LastUsed)
	})
	return specs, nil
}

// Save writes specs to disk.
func Save(storageDir string, specs []SavedSpec) error {
	path := specsFilePath(storageDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("specsstore: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return fmt.Errorf("specsstore: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("specsstore: write: %w", err)
	}
	return nil
}

// Add adds or updates a spec entry and touches its LastUsed timestamp.
func Add(storageDir, name, url string) error {
	specs, err := Load(storageDir)
	if err != nil {
		return err
	}
	now := time.Now()
	for i, s := range specs {
		if s.Name == name {
			specs[i].URL = url
			specs[i].LastUsed = now
			return Save(storageDir, specs)
		}
	}
	specs = append(specs, SavedSpec{Name: name, URL: url, LastUsed: now})
	return Save(storageDir, specs)
}

// Touch updates the LastUsed timestamp for a saved spec by name.
func Touch(storageDir, name string) error {
	specs, err := Load(storageDir)
	if err != nil {
		return err
	}
	for i, s := range specs {
		if s.Name == name {
			specs[i].LastUsed = time.Now()
			return Save(storageDir, specs)
		}
	}
	return nil
}

// Delete removes a spec by name. Returns nil if not found.
func Delete(storageDir, name string) error {
	specs, err := Load(storageDir)
	if err != nil {
		return err
	}
	for i, s := range specs {
		if s.Name == name {
			specs = append(specs[:i], specs[i+1:]...)
			return Save(storageDir, specs)
		}
	}
	return nil
}

// LoadRecent returns recently-used specs, newest first. Capped at 20 entries.
func LoadRecent(storageDir string) ([]SavedSpec, error) {
	path := recentFilePath(storageDir)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []SavedSpec{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("specsstore: read recent: %w", err)
	}
	var specs []SavedSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, fmt.Errorf("specsstore: unmarshal recent: %w", err)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].LastUsed.After(specs[j].LastUsed)
	})
	return specs, nil
}

// AddRecent records url as recently used. Upserts by URL and caps at 20 entries.
func AddRecent(storageDir, url string) error {
	recents, _ := LoadRecent(storageDir)
	now := time.Now()
	found := false
	for i, r := range recents {
		if r.URL == url {
			recents[i].LastUsed = now
			found = true
			break
		}
	}
	if !found {
		recents = append(recents, SavedSpec{Name: url, URL: url, LastUsed: now})
	}
	sort.Slice(recents, func(i, j int) bool {
		return recents[i].LastUsed.After(recents[j].LastUsed)
	})
	if len(recents) > 20 {
		recents = recents[:20]
	}
	return saveRecent(storageDir, recents)
}

// DeleteRecent removes a URL from the recent list.
func DeleteRecent(storageDir, url string) error {
	recents, err := LoadRecent(storageDir)
	if err != nil {
		return err
	}
	for i, r := range recents {
		if r.URL == url {
			recents = append(recents[:i], recents[i+1:]...)
			return saveRecent(storageDir, recents)
		}
	}
	return nil
}

func saveRecent(storageDir string, specs []SavedSpec) error {
	path := recentFilePath(storageDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("specsstore: mkdir recent: %w", err)
	}
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return fmt.Errorf("specsstore: marshal recent: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}
