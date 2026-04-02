package httpapi

import (
	"encoding/json"
	"net/http"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

func writeError(w http.ResponseWriter, status int, errorType string, code string, message string) {
	writeJSON(w, status, errorEnvelope{
		Error: apiError{
			Message: message,
			Type:    errorType,
			Code:    code,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}
