package httpapi

import "net/http"

type voicesResponse struct {
	DefaultVoice string        `json:"default_voice"`
	Voices       []voiceRecord `json:"voices"`
}

type voiceRecord struct {
	ShortName string `json:"short_name"`
	Locale    string `json:"locale"`
	Gender    string `json:"gender"`
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
		writeUpstreamError(w, err, "voices_unavailable", "upstream voices unavailable")
		return
	}

	filtered := make([]voiceRecord, 0, len(token.AllowedVoices))
	for _, voice := range voices {
		if !containsVoice(token.AllowedVoices, voice.ShortName) {
			continue
		}

		filtered = append(filtered, voiceRecord{
			ShortName: voice.ShortName,
			Locale:    voice.Locale,
			Gender:    voice.Gender,
		})
	}

	writeJSON(w, http.StatusOK, voicesResponse{
		DefaultVoice: token.Defaults.Voice,
		Voices:       filtered,
	})
}
