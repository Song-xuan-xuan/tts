package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/songtf/tts-stack/gateway/internal/config"
	"github.com/songtf/tts-stack/gateway/internal/upstream"
)

type fakeUpstream struct {
	voices           []upstream.Voice
	listVoicesErr    error
	listVoicesCalled int
	synthErr         error
	synthResp        *http.Response
	synthCalled      int
	lastSynthesize   upstream.SynthesizeParams
}

func (f *fakeUpstream) ListVoices(context.Context) ([]upstream.Voice, error) {
	f.listVoicesCalled++
	if f.listVoicesErr != nil {
		return nil, f.listVoicesErr
	}
	return append([]upstream.Voice(nil), f.voices...), nil
}

func (f *fakeUpstream) Synthesize(_ context.Context, params upstream.SynthesizeParams) (*http.Response, error) {
	f.synthCalled++
	f.lastSynthesize = params
	if f.synthErr != nil {
		return nil, f.synthErr
	}
	if f.synthResp != nil {
		return f.synthResp, nil
	}

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

	if got := rec.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("expected audio/mpeg content-type, got %q", got)
	}
}

func TestSpeechRejectsNonAudioUpstreamSuccess(t *testing.T) {
	store := newTestStore(t)
	up := &fakeUpstream{
		synthResp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		},
	}
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

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSpeechMapsUpstreamTimeoutToGatewayTimeout(t *testing.T) {
	store := newTestStore(t)
	up := &fakeUpstream{synthErr: context.DeadlineExceeded}
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

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status 504, got %d body=%s", rec.Code, rec.Body.String())
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

func TestVoicesMapsUpstreamTimeoutToGatewayTimeout(t *testing.T) {
	store := newTestStore(t)
	up := &fakeUpstream{listVoicesErr: context.DeadlineExceeded}
	app := New(store, up)

	req := httptest.NewRequest(http.MethodGet, "/api/voices", nil)
	req.Header.Set("Authorization", "Bearer sk_test")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status 504, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAppUsesReloadedTokenConfigForNewRequests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	writeConfig(t, path, testConfigYAML(
		"sk_test",
		"zh-CN-XiaoxiaoNeural",
		[]string{"zh-CN-XiaoxiaoNeural", "zh-CN-YunxiNeural"},
		2,
		300,
	))

	store, err := config.NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	up := &fakeUpstream{}
	app := New(store, up)

	firstReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/audio/speech",
		strings.NewReader(`{"model":"tts-1","input":"hello world"}`),
	)
	firstReq.Header.Set("Authorization", "Bearer sk_test")
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()

	app.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first request status 200, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	if got := up.lastSynthesize.Voice; got != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("expected initial default voice, got %q", got)
	}

	writeConfig(t, path, testConfigYAML(
		"sk_reloaded",
		"zh-CN-YunxiNeural",
		[]string{"zh-CN-YunxiNeural"},
		4,
		250,
	))
	if err := store.Reload(); err != nil {
		t.Fatalf("reload store: %v", err)
	}

	secondReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/audio/speech",
		strings.NewReader(`{"model":"tts-1","input":"hello world"}`),
	)
	secondReq.Header.Set("Authorization", "Bearer sk_reloaded")
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()

	app.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected second request status 200, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	if got := up.lastSynthesize.Voice; got != "zh-CN-YunxiNeural" {
		t.Fatalf("expected reloaded default voice, got %q", got)
	}
	if got := up.lastSynthesize.Thread; got != 4 {
		t.Fatalf("expected reloaded thread 4, got %d", got)
	}
	if got := up.lastSynthesize.ShardLength; got != 250 {
		t.Fatalf("expected reloaded shard length 250, got %d", got)
	}

	oldReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/audio/speech",
		strings.NewReader(`{"model":"tts-1","input":"hello world"}`),
	)
	oldReq.Header.Set("Authorization", "Bearer sk_test")
	oldReq.Header.Set("Content-Type", "application/json")
	oldRec := httptest.NewRecorder()

	app.ServeHTTP(oldRec, oldReq)

	if oldRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old token to be unauthorized after reload, got %d body=%s", oldRec.Code, oldRec.Body.String())
	}
}

func TestIsTimeoutErrorRecognizesNetError(t *testing.T) {
	if !isTimeoutError(timeoutErr{}) {
		t.Fatal("expected timeoutErr to be recognized as timeout")
	}
	if isTimeoutError(errors.New("plain error")) {
		t.Fatal("expected plain error not to be recognized as timeout")
	}
}

func newTestStore(t *testing.T) *config.Store {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	writeConfig(t, path, testConfigYAML(
		"sk_test",
		"zh-CN-XiaoxiaoNeural",
		[]string{"zh-CN-XiaoxiaoNeural", "zh-CN-YunxiNeural"},
		2,
		300,
	))

	store, err := config.NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	return store
}

func writeConfig(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func testConfigYAML(token string, voice string, allowedVoices []string, thread int, shardLength int) string {
	allowed := make([]string, 0, len(allowedVoices))
	for _, item := range allowedVoices {
		allowed = append(allowed, "      - "+item)
	}

	return `
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
    token: ` + token + `
    enabled: true
    defaults:
      voice: ` + voice + `
      thread: ` + strconv.Itoa(thread) + `
      shard_length: ` + strconv.Itoa(shardLength) + `
      max_text_length: 5000
    allowed_voices:
` + strings.Join(allowed, "\n") + `
`
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }
