package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"stockmarket/internal/models"
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
			respondError(w, http.StatusBadRequest, "Invalid JSON")
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
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleAlertDelete deletes a price alert
func (s *Server) handleAlertDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	condition := r.FormValue("condition")
	priceStr := r.FormValue("target_price")

	if symbol == "" || condition == "" || priceStr == "" {
		htmxError(w, "All fields are required")
		return
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		htmxError(w, "Invalid price")
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
	s.renderAlertsList(w)
}

// handleAlertDeleteHTMX handles deleting alerts and returns updated list
func (s *Server) handleAlertDeleteHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	s.renderAlertsList(w)
}

func (s *Server) renderAlertsList(w http.ResponseWriter) {
	alerts, _ := s.db.GetActiveAlerts()

	w.Header().Set("Content-Type", "text/html")

	if len(alerts) == 0 {
		w.Write([]byte(`
<div class="text-center py-12">
    <div class="text-5xl mb-3">🔔</div>
    <p class="text-slate-400">No active alerts</p>
    <p class="text-slate-500 text-sm mt-1">Create an alert to get notified when prices change</p>
</div>
`))
		return
	}

	// Use strings.Builder to avoid repeated string concatenation allocations
	var sb strings.Builder
	sb.Grow(len(alerts) * 512) // Pre-allocate estimated size

	sb.WriteString(`<div class="space-y-3">`)
	for _, a := range alerts {
		icon := "⬆️"
		if a.Condition == "below" {
			icon = "⬇️"
		}
		fmt.Fprintf(&sb, `
    <div class="flex items-center justify-between p-4 bg-slate-700/50 rounded-lg">
        <div class="flex items-center gap-4">
            <span class="text-2xl">%s</span>
            <div>
                <div class="font-medium text-white">%s</div>
                <div class="text-sm text-slate-400">
                    Price %s $%.2f
                </div>
            </div>
        </div>
        <div class="flex items-center gap-4">
            <span class="px-2 py-1 rounded text-xs font-medium bg-slate-600 text-slate-300">Active</span>
            <button hx-delete="/api/alerts/%d"
                    hx-target="#alerts-list"
                    hx-swap="innerHTML"
                    hx-confirm="Delete this alert?"
                    class="text-red-400 hover:text-red-300 text-sm">
                Delete
            </button>
        </div>
    </div>
`, icon, a.Symbol, a.Condition, a.Price, a.ID)
	}
	sb.WriteString(`</div>`)

	w.Write([]byte(sb.String()))
}

// HTMX response helpers
