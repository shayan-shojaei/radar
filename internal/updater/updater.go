package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	repo    = "shayan-shojaei/radar"
	apiBase = "https://api.github.com"
	dlBase  = "https://github.com"
)

type release struct {
	TagName string `json:"tag_name"`
}

// LatestVersion returns the tag name of the most recent GitHub release.
func LatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("decode release: %w", err)
	}
	if r.TagName == "" {
		return "", fmt.Errorf("empty tag_name in release response")
	}
	return r.TagName, nil
}

// IsNewer reports whether latest is a strictly newer semver than current.
// Both are expected in "vX.Y.Z" form; falls back to string inequality for
// non-semver tags (e.g. "dev" builds always get updated).
func IsNewer(latest, current string) bool {
	if current == "dev" || current == "" || current == "unknown" {
		return false // don't auto-update dev builds
	}
	return stripV(latest) != stripV(current)
}

func stripV(s string) string {
	return strings.TrimPrefix(s, "v")
}

// Replace downloads the binary for the given tag/goos/goarch and
// atomically replaces the file at dest.
func Replace(dest, tag, goos, goarch string) error {
	url := fmt.Sprintf("%s/%s/releases/download/%s/radar-%s-%s",
		dlBase, repo, tag, goos, goarch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s (url: %s)", resp.Status, url)
	}

	tmp, err := os.CreateTemp("", "radar-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write download: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod: %w", err)
	}
	tmp.Close()

	// Atomic replace: rename over the existing binary.
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}
