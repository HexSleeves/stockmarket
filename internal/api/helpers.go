package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_JSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// htmxSuccess sends a success notification via HTMX
func htmxSuccess(w http.ResponseWriter, message string) {
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": {"message": "%s", "type": "success"}}`, message))
	w.WriteHeader(http.StatusOK)
}

// htmxError sends an error notification via HTMX
func htmxError(w http.ResponseWriter, message string) {
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": {"message": "%s", "type": "error"}}`, message))
	w.WriteHeader(http.StatusBadRequest)
}
