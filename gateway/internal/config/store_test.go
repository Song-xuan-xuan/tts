package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreReloadSwapsToNewConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	first := `
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
    allowed_voices:
      - zh-CN-XiaoxiaoNeural
`

	second := `
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
`

	if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
		t.Fatalf("write first config: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := os.WriteFile(path, []byte(second), 0o600); err != nil {
		t.Fatalf("write second config: %v", err)
	}

	if err := store.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if got := store.Current().Tokens[0].Defaults.Voice; got != "zh-CN-YunxiNeural" {
		t.Fatalf("expected reloaded voice, got %q", got)
	}
}

func TestStoreReloadKeepsPreviousConfigOnInvalidUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	valid := `
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
    allowed_voices:
      - zh-CN-XiaoxiaoNeural
`

	invalid := `
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
      voice: ""
      thread: 1
      shard_length: 400
      max_text_length: 8000
    allowed_voices: []
`

	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatalf("write valid config: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	if err := store.Reload(); err == nil {
		t.Fatal("expected reload error")
	}

	if got := store.Current().Tokens[0].Defaults.Voice; got != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("expected previous config to remain active, got %q", got)
	}
}
