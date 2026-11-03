package common

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// resolveSessionSecret keeps the login cookie valid across restarts. An
// explicit SESSION_SECRET wins. Otherwise the secret is read from path, and
// created there the first time so the next boot signs cookies the same way.
func resolveSessionSecret(envValue, path string) string {
	envValue = strings.TrimSpace(envValue)
	if envValue != "" && envValue != "random_string" {
		return envValue
	}
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" && s != "random_string" {
			return s
		}
	}
	s := uuid.New().String()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(s+"\n"), 0o600)
	}
	return s
}
