package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMongoURIFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mongo.uri")
	if err := os.WriteFile(path, []byte("mongodb://example:27017/admin\n"), 0o600); err != nil {
		t.Fatalf("write mongo.uri: %v", err)
	}

	t.Setenv("MOGOTOR_MONGO_URI", "")
	if got := resolveMongoURI(dir); got != "mongodb://example:27017/admin" {
		t.Fatalf("expected uri from file, got %q", got)
	}
}

func TestResolveMongoURIEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mongo.uri")
	if err := os.WriteFile(path, []byte("mongodb://file/admin"), 0o600); err != nil {
		t.Fatalf("write mongo.uri: %v", err)
	}

	t.Setenv("MOGOTOR_MONGO_URI", "mongodb://env/admin")
	if got := resolveMongoURI(dir); got != "mongodb://env/admin" {
		t.Fatalf("expected env uri, got %q", got)
	}
}
