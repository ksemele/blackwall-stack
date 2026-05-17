package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_WritesJSONToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "blackwall.log")

	logger, closer, err := Setup(Options{File: logPath, Level: "info"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	t.Cleanup(func() { _ = closer.Close() })

	logger.Info("hello", "key", "value")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(raw), `"msg":"hello"`) {
		t.Errorf("missing msg in log: %s", raw)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(string(raw), "\n", 2)[0]), &parsed); err != nil {
		t.Errorf("log line is not JSON: %v", err)
	}
}

func TestSetup_LevelParsing(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		logPath := filepath.Join(t.TempDir(), "x.log")
		_, closer, err := Setup(Options{File: logPath, Level: level})
		if err != nil {
			t.Errorf("Setup(%s): %v", level, err)
		}
		_ = closer.Close()
	}
}

func TestSetup_UnknownLevelDefaultsWarn(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "x.log")
	_, closer, err := Setup(Options{File: logPath, Level: "bogus"})
	if err != nil {
		t.Errorf("Setup: %v", err)
	}
	_ = closer.Close()
}
