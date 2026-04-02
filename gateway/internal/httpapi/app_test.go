package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songtf/tts-stack/gateway/internal/config"
	"github.com/songtf/tts-stack/gateway/internal/upstream"
)

type fakeUpstream struct {
	voices           []upstream.Voice
	listVoicesCalled int
	synthCalled      int
	lastSynthesize   upstream.SynthesizeParams
}

func (f *fakeUpstream) ListVoices(context.Context) ([]upstream.Voice, error) {
	f.listVoicesCalled++
	return append([]upstream.Voice(nil), f.voices...), nil
}

func (f *fakeUpstream) Synthesize(_ context.Context, params upstream.SynthesizeParams) (*http.Response, error) {
	f.synthCalled++
	f.lastSynthesize = params

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"audio/mpeg"}},
		Body:       io.NopCloser(strings.NewReader("mp3-bytes")),
	}, nil
}

func TestHealthzReturnsOK(t *testing.T) {
	store := newTestStore(t)
	app := New(store, &fakeUpstream{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("unexpected body %s", got)
	}
}

func TestSpeechRejectsUnknownToken(t *testing.T) {
	store := newTestStore(t)
	up := &fakeUpstream{}
	app := New(store, up)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/audio/speech",
		strings.NewReader(`{"model":"tts-1","input":"hello world"}`),
	)
	req.Header.Set("Authorization", "Bearer sk_unknown")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	if up.synthCalled != 0 {
		t.Fatalf("expected synthesize not to be called, got %d", up.synthCalled)
	}
}

func TestSpeechUsesDefaultVoiceWhenVoiceMissing(t *testing.T) {
	store := newTestStore(t)
	up := &fakeUpstream{}
	app := New(store, up)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/audio/speech",
		strings.NewReader(`{"model":"tts-1","input":"hello world"}`),
	)
	req.Header.Set("Authorization", "Bearer sk_test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	if up.synthCalled != 1 {
		t.Fatalf("expected synthesize called once, got %d", up.synthCalled)
	}

	if got := up.lastSynthesize.Voice; got != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("expected default voice, got %q", got)
	}

	if got := up.lastSynthesize.Thread; got != 2 {
		t.Fatalf("expected thread 2, got %d", got)
	}

	if got := up.lastSynthesize.ShardLength; got != 300 {
		t.Fatalf("expected shard length 300, got %d", got)
	}

	if got := rec.Body.String(); got != "mp3-bytes" {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestVoicesReturnsTokenFilteredCatalog(t *testing.T) {
	store := newTestStore(t)
	up := &fakeUpstream{
		voices: []upstream.Voice{
			{ShortName: "zh-CN-XiaoxiaoNeural", Locale: "zh-CN", Gender: "Female"},
			{ShortName: "zh-CN-YunxiNeural", Locale: "zh-CN", Gender: "Male"},
			{ShortName: "en-US-AriaNeural", Locale: "en-US", Gender: "Female"},
		},
	}
	app := New(store, up)

	req := httptest.NewRequest(http.MethodGet, "/api/voices", nil)
	req.Header.Set("Authorization", "Bearer sk_test")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	want := `{"default_voice":"zh-CN-XiaoxiaoNeural","voices":[{"short_name":"zh-CN-XiaoxiaoNeural","locale":"zh-CN","gender":"Female"},{"short_name":"zh-CN-YunxiNeural","locale":"zh-CN","gender":"Male"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("unexpected body %s", got)
	}

	if up.listVoicesCalled != 1 {
		t.Fatalf("expected list voices called once, got %d", up.listVoicesCalled)
	}
}

func newTestStore(t *testing.T) *config.Store {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	body := `
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
      thread: 2
      shard_length: 300
      max_text_length: 5000
    allowed_voices:
      - zh-CN-XiaoxiaoNeural
      - zh-CN-YunxiNeural
`

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store, err := config.NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	return store
}
