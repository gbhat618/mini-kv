package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCLIConfig(t *testing.T) {
	content := `
[server]
host = 192.168.1.100
port = 6380

[security]
password = cli_secret

[tls]
enabled = true
skip_verify = true
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cli.ini")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Host != "192.168.1.100" {
		t.Errorf("Expected host 192.168.1.100, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 6380 {
		t.Errorf("Expected port 6380, got %d", cfg.Server.Port)
	}
	if cfg.Security.Password != "cli_secret" {
		t.Errorf("Expected password cli_secret, got %s", cfg.Security.Password)
	}
	if !cfg.TLS.Enabled {
		t.Error("Expected TLS enabled")
	}
	if !cfg.TLS.SkipVerify {
		t.Error("Expected TLS skip_verify")
	}
}

func TestLoadCLIConfigNotFound(t *testing.T) {
	cfg, err := loadConfig("/nonexistent/cli.ini")
	if err != nil {
		t.Fatalf("Expected no error for nonexistent file, got %v", err)
	}
	if cfg != nil {
		t.Error("Expected nil config for nonexistent file")
	}
}

func TestApplyCLIConfig(t *testing.T) {
	cfg := &CLIConfig{}
	cfg.Server.Host = "10.0.0.1"
	cfg.Server.Port = 6390
	cfg.Security.Password = "applied_pass"
	cfg.TLS.Enabled = true
	cfg.TLS.SkipVerify = true

	host = DefaultHost
	port = DefaultPort
	password = ""
	tlsFlag = false
	tlsSkipVerify = false

	applyConfig(cfg)

	if host != "10.0.0.1" {
		t.Errorf("Expected host 10.0.0.1, got %s", host)
	}
	if port != 6390 {
		t.Errorf("Expected port 6390, got %d", port)
	}
	if password != "applied_pass" {
		t.Errorf("Expected password applied_pass, got %s", password)
	}
	if !tlsFlag {
		t.Error("Expected tlsFlag to be true")
	}
	if !tlsSkipVerify {
		t.Error("Expected tlsSkipVerify to be true")
	}
}

func TestApplyCLIConfigNil(t *testing.T) {
	host = "original-host"
	port = 1111
	password = "original-pass"
	tlsFlag = true

	applyConfig(nil)

	if host != "original-host" {
		t.Errorf("Expected host unchanged, got %s", host)
	}
	if port != 1111 {
		t.Errorf("Expected port unchanged, got %d", port)
	}
	if password != "original-pass" {
		t.Errorf("Expected password unchanged, got %s", password)
	}
	if !tlsFlag {
		t.Error("Expected tlsFlag unchanged")
	}
}
