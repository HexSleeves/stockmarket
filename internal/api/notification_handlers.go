package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"stockmarket/internal/models"
)

func (s *Server) handleNotificationChannels(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, cfg.NotificationChannels)

	case http.MethodPost:
		var channel models.NotificationConfig
		if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		if channel.Type == "" || channel.Target == "" {
			respondError(w, http.StatusBadRequest, "Type and target required")
			return
		}

		if err := s.db.SaveNotificationChannel(cfg.ID, &channel); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, channel)

	case http.MethodPut:
		var channel models.NotificationConfig
		if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		if channel.ID == 0 {
			respondError(w, http.StatusBadRequest, "Channel ID required")
			return
		}

		if err := s.db.SaveNotificationChannel(cfg.ID, &channel); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, channel)

	default:
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
	}
}

// handleNotificationChannelDelete deletes a notification channel
func (s *Server) handleNotificationChannelDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/notification-channels/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	if err := s.db.DeleteNotificationChannel(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleProfiles returns available risk and frequency profiles
