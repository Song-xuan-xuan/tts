package config

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

const initialConfigYAML = `
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

const reloadedConfigYAML = `
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

const invalidConfigYAML = `
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

func TestStoreReloadSwapsToNewConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	if err := os.WriteFile(path, []byte(initialConfigYAML), 0o600); err != nil {
		t.Fatalf("write first config: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := os.WriteFile(path, []byte(reloadedConfigYAML), 0o600); err != nil {
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

	if err := os.WriteFile(path, []byte(initialConfigYAML), 0o600); err != nil {
		t.Fatalf("write valid config: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := os.WriteFile(path, []byte(invalidConfigYAML), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	if err := store.Reload(); err == nil {
		t.Fatal("expected reload error")
	}

	if got := store.Current().Tokens[0].Defaults.Voice; got != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("expected previous config to remain active, got %q", got)
	}
}

func TestStoreCurrentReturnsIndependentCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	if err := os.WriteFile(path, []byte(initialConfigYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	current := store.Current()
	current.Tokens[0].Defaults.Voice = "mutated"
	current.Tokens[0].AllowedVoices[0] = "mutated"

	fresh := store.Current()
	if got := fresh.Tokens[0].Defaults.Voice; got != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("expected stored voice to remain unchanged, got %q", got)
	}
	if got := fresh.Tokens[0].AllowedVoices[0]; got != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("expected stored allowed voice to remain unchanged, got %q", got)
	}
}

func TestWatchReloadsConfigOnAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	if err := os.WriteFile(path, []byte(initialConfigYAML), 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Watch(ctx, logger, store, path); err != nil {
		t.Fatalf("watch: %v", err)
	}

	tmpPath := filepath.Join(dir, "gateway.yaml.next")
	if err := os.WriteFile(tmpPath, []byte(reloadedConfigYAML), 0o600); err != nil {
		t.Fatalf("write replacement config: %v", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		t.Fatalf("replace config atomically: %v", err)
	}

	waitForVoice(t, store, "zh-CN-YunxiNeural")
}

func TestShouldReloadOnEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	tests := []struct {
		name  string
		event string
		op    fsnotify.Op
		want  bool
	}{
		{
			name:  "write to config path",
			event: path,
			op:    fsnotify.Write,
			want:  true,
		},
		{
			name:  "create target after atomic replace",
			event: path,
			op:    fsnotify.Create,
			want:  true,
		},
		{
			name:  "rename temp file in config dir",
			event: filepath.Join(dir, ".gateway.yaml.tmp"),
			op:    fsnotify.Rename,
			want:  true,
		},
		{
			name:  "remove target during replace",
			event: path,
			op:    fsnotify.Remove,
			want:  true,
		},
		{
			name:  "chmod target ignored",
			event: path,
			op:    fsnotify.Chmod,
			want:  false,
		},
		{
			name:  "event outside config dir ignored",
			event: filepath.Join(t.TempDir(), "gateway.yaml"),
			op:    fsnotify.Write,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := watchEvent{Name: tt.event, Op: tt.op}
			if got := shouldReloadOnEvent(path, event); got != tt.want {
				t.Fatalf("shouldReloadOnEvent(%q, %+v) = %v, want %v", path, event, got, tt.want)
			}
		})
	}
}

func waitForVoice(t *testing.T, store *Store, want string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := store.Current().Tokens[0].Defaults.Voice; got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for voice %q, last=%q", want, store.Current().Tokens[0].Defaults.Voice)
}
