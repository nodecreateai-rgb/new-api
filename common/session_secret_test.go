package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSessionSecretStableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session_secret")
	first := resolveSessionSecret("", path)
	second := resolveSessionSecret("", path)
	if first == "" || first != second {
		t.Fatalf("secret changed across reads: %q %q", first, second)
	}
	if got := resolveSessionSecret("from-env", path); got != "from-env" {
		t.Fatalf("env = %q", got)
	}
	if got := resolveSessionSecret("random_string", path); got != first {
		t.Fatalf("default env should fall through, got %q", got)
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		t.Fatal(err)
	}
}
