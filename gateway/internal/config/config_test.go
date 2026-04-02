package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func TestLoadRequiresAllowedVoices(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 8080
upstream:
  base_url: http://tts:8080
  timeout_seconds: 90
defaults:
  thread: 1
  shard_length: 400
  max_text_length: 8000
tokens:
  - name: llm-prod
    token: sk_test
    enabled: true
    defaults:
      voice: zh-CN-XiaoxiaoNeural
      thread: 1
      shard_length: 400
      max_text_length: 8000
    allowed_voices: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "allowed_voices") {
		t.Fatalf("expected allowed_voices error, got %v", err)
	}
}

func TestLoadUsesTokenOverrides(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 8080
upstream:
  base_url: http://tts:8080
  timeout_seconds: 90
defaults:
  thread: 1
  shard_length: 400
  max_text_length: 8000
tokens:
  - name: llm-prod
    token: sk_test
    enabled: true
    defaults:
      voice: zh-CN-YunxiNeural
      thread: 2
      shard_length: 300
      max_text_length: 5000
    allowed_voices:
      - zh-CN-YunxiNeural
      - zh-CN-XiaoxiaoNeural
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("expected server port 8080, got %d", cfg.Server.Port)
	}

	if cfg.Tokens[0].Defaults.Thread != 2 {
		t.Fatalf("expected token thread override 2, got %d", cfg.Tokens[0].Defaults.Thread)
	}

	if cfg.Tokens[0].Defaults.Voice != "zh-CN-YunxiNeural" {
		t.Fatalf("unexpected default voice %q", cfg.Tokens[0].Defaults.Voice)
	}
}

func TestLoadRequiresTokenDefaultVoiceInAllowedVoices(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 8080
upstream:
  base_url: http://tts:8080
  timeout_seconds: 90
defaults:
  thread: 1
  shard_length: 400
  max_text_length: 8000
tokens:
  - name: llm-prod
    token: sk_test
    enabled: true
    defaults:
      voice: zh-CN-YunxiNeural
      thread: 2
      shard_length: 300
      max_text_length: 5000
    allowed_voices:
      - zh-CN-XiaoxiaoNeural
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "defaults.voice") {
		t.Fatalf("expected defaults.voice error, got %v", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 8080
upstream:
  base_url: http://tts:8080
  timeout_seconds: 90
defaults:
  thread: 1
  shard_length: 400
  max_text_length: 8000
  timeout_seconds: 60
tokens:
  - name: llm-prod
    token: sk_test
    enabled: true
    defaults:
      voice: zh-CN-YunxiNeural
      thread: 2
      shard_length: 300
      max_text_length: 5000
    allowed_voices:
      - zh-CN-YunxiNeural
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected decode error")
	}

	if !strings.Contains(err.Error(), "field timeout_seconds not found") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}
