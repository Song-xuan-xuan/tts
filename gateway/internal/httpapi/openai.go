package httpapi

import (
	"encoding/json"
	"net/http"
	"unicode/utf8"

	"github.com/songtf/tts-stack/gateway/internal/upstream"
)

type speechRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

func (a *App) handleSpeech(w http.ResponseWriter, r *http.Request) {
	cfg, token, ok := a.authorize(w, r)
	if !ok {
		if cfg == nil {
			return
		}
		writeError(w, http.StatusUnauthorized, "authentication_error", "invalid_api_key", "invalid bearer token")
		return
	}

	var req speechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid_json", "request body must be valid JSON")
		return
	}

	if req.Model != speechModel {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "unsupported_model", "model must be tts-1")
		return
	}

	if req.Input == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "input_required", "input is required")
		return
	}

	if utf8.RuneCountInString(req.Input) > token.Defaults.MaxTextLength {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "input_too_long", "input exceeds max_text_length")
		return
	}

	voice := req.Voice
	if voice == "" {
		voice = token.Defaults.Voice
	}
	if !containsVoice(token.AllowedVoices, voice) {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "voice_not_allowed", "voice is not allowed for this token")
		return
	}

	resp, err := a.upstream.Synthesize(r.Context(), upstream.SynthesizeParams{
		Text:        req.Input,
		Voice:       voice,
		Thread:      token.Defaults.Thread,
		ShardLength: token.Defaults.ShardLength,
	})
	if err != nil {
		writeUpstreamError(w, err, "synthesis_failed", "upstream synthesis failed")
		return
	}

	proxyAudio(w, resp)
}
