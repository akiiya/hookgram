package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateWritesChineseTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data", "config.yaml")

	manager, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}
	cfg := manager.Current()
	if cfg.Security.SessionSecret == "" || cfg.Security.TokenHashSecret == "" {
		t.Fatal("expected generated security secrets")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Hookgram 配置文件") {
		t.Fatalf("expected Chinese comments in config, got: %s", content)
	}
	if !strings.Contains(content, `host: "127.0.0.1"`) {
		t.Fatal("expected default host in config")
	}
}

func TestPathAndDatabaseDSNFromDataDirEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDataDir, dir)
	t.Setenv(EnvConfigPath, "")

	if got := PathFromEnv(); got != filepath.Join(dir, "config.yaml") {
		t.Fatalf("PathFromEnv() = %q, want config path under data dir", got)
	}
	if got := Default().Database.DSN; got != filepath.Join(dir, "hookgram.db") {
		t.Fatalf("Default database dsn = %q, want database under data dir", got)
	}
}
