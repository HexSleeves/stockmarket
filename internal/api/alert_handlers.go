package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"stockmarket/internal/models"
	"stockmarket/internal/web/pages"
)

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		alerts, err := s.db.GetActiveAlerts()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, alerts)

	case http.MethodPost:
		var alert models.PriceAlert
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			respondError(w, http.StatusBadRequest, INVALID_JSON)
			return
		}

		alert.Symbol = strings.ToUpper(strings.TrimSpace(alert.Symbol))
		if alert.Symbol == "" || alert.Price <= 0 {
			respondError(w, http.StatusBadRequest, "Symbol and price required")
			return
		}
		if alert.Condition != "above" && alert.Condition != "below" {
			respondError(w, http.StatusBadRequest, "Condition must be 'above' or 'below'")
			return
		}

		if err := s.db.SavePriceAlert(&alert); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, alert)

	default:
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
	}
}

// handleAlertDelete deletes a price alert
func (s *Server) handleAlertDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/alerts/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid alert ID")
		return
	}

	if err := s.db.DeletePriceAlert(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleNotificationChannels handles notification channel CRUD
func (s *Server) handleAlertsHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, INVALID_FORM_DATA)
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	condition := r.FormValue("condition")
	priceStr := r.FormValue("target_price")

	if symbol == "" || condition == "" || priceStr == "" {
		htmxError(w, ALL_FIELDS_REQUIRED)
		return
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		htmxError(w, INVALID_PRICE)
		return
	}

	alert := &models.PriceAlert{
		Symbol:    symbol,
		Condition: condition,
		Price:     price,
	}

	if err := s.db.SavePriceAlert(alert); err != nil {
		htmxError(w, err.Error())
		return
	}

	// Return updated alerts list
	s.renderAlertsList(w, r)
}

// handleAlertDeleteHTMX handles deleting alerts and returns updated list
func (s *Server) handleAlertDeleteHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/alerts/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		htmxError(w, "Invalid alert ID")
		return
	}

	if err := s.db.DeletePriceAlert(id); err != nil {
		htmxError(w, err.Error())
		return
	}

	s.renderAlertsList(w, r)
}

func (s *Server) renderAlertsList(w http.ResponseWriter, r *http.Request) {
	alertsRaw, _ := s.db.GetActiveAlerts()

	// Convert to pages.Alert
	alerts := make([]pages.Alert, len(alertsRaw))
	for i, a := range alertsRaw {
		alerts[i] = pages.Alert{
			ID:          a.ID,
			Symbol:      a.Symbol,
			Condition:   a.Condition,
			TargetPrice: a.Price,
			Triggered:   a.Triggered,
		}
	}

	w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
	pages.AlertsListPartial(alerts).Render(r.Context(), w)
}

// HTMX response helpers
