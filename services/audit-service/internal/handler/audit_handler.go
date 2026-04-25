package handler

import (
	"encoding/json"
	"net/http"

	"avikmukherjee/m/audit-service/internal/service"
)

type AuditHandler struct {
	store *service.AuditStore
}

func NewAuditHandler(store *service.AuditStore) *AuditHandler {
	return &AuditHandler{store: store}
}

// GET /api/v1/audit/health
func (h *AuditHandler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "audit-service",
	})
}

// GET /api/v1/audit/logs?prefix=transaction/2026/04/20/
// Lists all audit log object keys under the given MinIO prefix.
// Useful for querying logs by event type and date.
func (h *AuditHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	// Default to listing everything if no prefix given
	keys, err := h.store.List(r.Context(), prefix)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "could not list audit logs",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"prefix": prefix,
		"count":  len(keys),
		"keys":   keys,
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
