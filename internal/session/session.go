package session

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/shayan-shojaei/radar/internal/crypto"
	"github.com/shayan-shojaei/radar/pkg/models"
)

// sessionsDir returns the directory used for session files.
func sessionsDir(storageDir string) string {
	return filepath.Join(storageDir, "sessions")
}

// sessionPath returns the path to the session file for a given base URL.
func sessionPath(storageDir, baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("session: parse base URL: %w", err)
	}
	hostname := u.Hostname()
	if hostname == "" {
		hostname = "local"
	}
	return filepath.Join(sessionsDir(storageDir), hostname+".age"), nil
}

// Save encrypts and writes a Session to disk.
func Save(session *models.Session, storageDir, passphrase string) error {
	path, err := sessionPath(storageDir, session.BaseURL)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("session: create storage dir: %w", err)
	}

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("session: marshal: %w", err)
	}

	ciphertext, err := crypto.Encrypt(data, passphrase)
	if err != nil {
		return fmt.Errorf("session: encrypt: %w", err)
	}

	if err := os.WriteFile(path, ciphertext, 0o600); err != nil {
		return fmt.Errorf("session: write file: %w", err)
	}

	return nil
}

// Load decrypts and reads a Session from disk.
// Returns a new empty session if no file exists for the given base URL.
func Load(baseURL, storageDir, passphrase string) (*models.Session, error) {
	path, err := sessionPath(storageDir, baseURL)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &models.Session{
			BaseURL:  baseURL,
			Requests: make(map[string]models.RequestData),
		}, nil
	}

	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session: read file: %w", err)
	}

	plaintext, err := crypto.Decrypt(ciphertext, passphrase)
	if err != nil {
		return nil, fmt.Errorf("session: decrypt: %w", err)
	}

	var session models.Session
	if err := json.Unmarshal(plaintext, &session); err != nil {
		return nil, fmt.Errorf("session: unmarshal: %w", err)
	}

	return &session, nil
}
