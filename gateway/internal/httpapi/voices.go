package httpapi

import (
	"net/http"
)

type voiceResponse struct {
	ID      string `json:"id"`
	Locale  string `json:"locale,omitempty"`
	Gender  string `json:"gender,omitempty"`
	Default bool   `json:"default"`
}

func (a *App) handleVoices(w http.ResponseWriter, r *http.Request) {
	cfg, token, ok := a.authorize(w, r)
	if !ok {
		if cfg == nil {
			return
		}
		writeError(w, http.StatusUnauthorized, "authentication_error", "invalid_api_key", "invalid bearer token")
		return
	}

	voices, err := a.upstream.ListVoices(r.Context())
	if err != nil {
		_ = cfg
		writeError(w, http.StatusBadGateway, "upstream_error", "voices_unavailable", "upstream voices unavailable")
		return
	}

	filtered := make([]voiceResponse, 0, len(token.AllowedVoices))
	seen := make(map[string]struct{}, len(token.AllowedVoices))
	for _, voice := range voices {
		if !containsVoice(token.AllowedVoices, voice.ShortName) {
			continue
		}

		filtered = append(filtered, voiceResponse{
			ID:      voice.ShortName,
			Locale:  voice.Locale,
			Gender:  voice.Gender,
			Default: voice.ShortName == token.Defaults.Voice,
		})
		seen[voice.ShortName] = struct{}{}
	}

	if _, ok := seen[token.Defaults.Voice]; !ok {
		filtered = append(filtered, voiceResponse{
			ID:      token.Defaults.Voice,
			Default: true,
		})
	}

	writeJSON(w, http.StatusOK, filtered)
}
