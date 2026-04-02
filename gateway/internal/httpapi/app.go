package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/songtf/tts-stack/gateway/internal/config"
	"github.com/songtf/tts-stack/gateway/internal/upstream"
)

const speechModel = "tts-1"

type upstreamClient interface {
	ListVoices(rctx context.Context) ([]upstream.Voice, error)
	Synthesize(rctx context.Context, params upstream.SynthesizeParams) (*http.Response, error)
}

type configProvider interface {
	Current() *config.RuntimeConfig
}

type App struct {
	cfg      configProvider
	upstream upstreamClient
	mux      *http.ServeMux
}

func New(store *config.Store, upstream upstreamClient) *App {
	return newWithProvider(store, upstream)
}

func newWithProvider(cfg configProvider, upstream upstreamClient) *App {
	app := &App{
		cfg:      cfg,
		upstream: upstream,
		mux:      http.NewServeMux(),
	}

	app.mux.HandleFunc("GET /healthz", app.handleHealthz)
	app.mux.HandleFunc("POST /v1/audio/speech", app.handleSpeech)
	app.mux.HandleFunc("GET /api/voices", app.handleVoices)

	return app
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *App) currentConfig() (*config.RuntimeConfig, error) {
	cfg := a.cfg.Current()
	if cfg == nil {
		return nil, fmt.Errorf("config unavailable")
	}

	return cfg, nil
}

func (a *App) authorize(w http.ResponseWriter, r *http.Request) (*config.RuntimeConfig, *config.TokenConfig, bool) {
	cfg, err := a.currentConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "config_unavailable", "server configuration unavailable")
		return nil, nil, false
	}

	secret, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return cfg, nil, false
	}

	for i := range cfg.Tokens {
		token := &cfg.Tokens[i]
		if token.Enabled && token.Token == secret {
			return cfg, token, true
		}
	}

	return cfg, nil, false
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}

	return strings.TrimSpace(token), true
}

func containsVoice(allowed []string, voice string) bool {
	for _, candidate := range allowed {
		if candidate == voice {
			return true
		}
	}

	return false
}

func writeUpstreamError(w http.ResponseWriter, err error, code string, message string) {
	status := http.StatusBadGateway
	if isTimeoutError(err) {
		status = http.StatusGatewayTimeout
	}

	writeError(w, status, "upstream_error", code, message)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func proxyAudio(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "upstream_error", "synthesis_failed", "upstream synthesis failed")
		return
	}

	if !isMP3ContentType(resp.Header.Get("Content-Type")) {
		writeError(w, http.StatusBadGateway, "upstream_error", "invalid_audio_response", "upstream did not return MP3 audio")
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func isMP3ContentType(contentType string) bool {
	baseType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return baseType == "audio/mpeg" || baseType == "audio/mp3"
}
