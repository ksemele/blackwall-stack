package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureExists_CreatesFileWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	created, err := EnsureExists(path)
	if err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	if !created {
		t.Fatal("created = false on first run, want true")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if !strings.Contains(string(data), "example-master") {
		t.Errorf("template missing example-master section: %s", data)
	}

	// Second call is a no-op.
	created, err = EnsureExists(path)
	if err != nil {
		t.Fatalf("EnsureExists 2nd: %v", err)
	}
	if created {
		t.Error("created = true on second run, want false")
	}
}

func TestTemplate_HasLabIndexer(t *testing.T) {
	tpl := string(defaultTemplate)
	if !strings.Contains(tpl, "indexer:") || !strings.Contains(tpl, "port:     9200") {
		t.Error("template lab context must carry an indexer section on port 9200")
	}
}
