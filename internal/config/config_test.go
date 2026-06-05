package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig(t *testing.T) {
	const content = `
server:
  port: "10000"
  interface: "0.0.0.0"
  debug: true
signal:
  number: "+421918533760"
  send: "http://signal-api:8080/v2/send"
  textmodeNormal: false
  recipients:
    - "group.DEFAULT"
alertmanager:
  ignoreLabels:
    - "alertname"
    - "recipients"
  ignoreAnnotations: []
  generatorURL: true
recipients:
  proxmox: "group.PROXMOX"
  critical: "group.CRITICAL"
`
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("could not write temp config: %v", err)
	}

	cfg, err := NewConfig(configPath)
	if err != nil {
		t.Fatalf("NewConfig returned error: %v", err)
	}

	if cfg.Signal.Number != "+421918533760" {
		t.Errorf("Signal.Number = %q, want %q", cfg.Signal.Number, "+421918533760")
	}
	if cfg.Signal.Send != "http://signal-api:8080/v2/send" {
		t.Errorf("Signal.Send = %q", cfg.Signal.Send)
	}
	if cfg.Signal.TextModeNormal {
		t.Error("Signal.TextModeNormal = true, want false")
	}
	if len(cfg.Signal.Recipients) != 1 || cfg.Signal.Recipients[0] != "group.DEFAULT" {
		t.Errorf("Signal.Recipients = %v, want [group.DEFAULT]", cfg.Signal.Recipients)
	}
	if !cfg.Server.Debug {
		t.Error("Server.Debug = false, want true")
	}
	if !cfg.AMConfig.GeneratorURL {
		t.Error("AMConfig.GeneratorURL = false, want true")
	}
	if cfg.Recipients["proxmox"] != "group.PROXMOX" {
		t.Errorf("Recipients[proxmox] = %q, want %q", cfg.Recipients["proxmox"], "group.PROXMOX")
	}
	if cfg.Recipients["critical"] != "group.CRITICAL" {
		t.Errorf("Recipients[critical] = %q, want %q", cfg.Recipients["critical"], "group.CRITICAL")
	}
}

func TestNewConfigMissingFile(t *testing.T) {
	if _, err := NewConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml")); err == nil {
		t.Error("expected an error for a missing config file, got nil")
	}
}

func TestNewConfigInvalidYAML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(configPath, []byte("signal: [this is not valid"), 0o600); err != nil {
		t.Fatalf("could not write temp config: %v", err)
	}
	if _, err := NewConfig(configPath); err == nil {
		t.Error("expected an error for invalid YAML, got nil")
	}
}
