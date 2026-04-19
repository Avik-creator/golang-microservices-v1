package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"avikmukherjee.com/m/transaction-service/internal/middleware"
	"avikmukherjee.com/m/transaction-service/internal/model"
	"avikmukherjee.com/m/transaction-service/internal/repository"
	"avikmukherjee.com/m/transaction-service/internal/service"
	"github.com/go-chi/chi/v5"
)

type TransactionHandler struct {
	svc *service.TransactionService
}

func NewTransactionHandler(svc *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

// POST /api/v1/transactions
func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	// Confirm the caller is authenticated (user ID injected by middleware)
	_, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := h.svc.ProcessPayment(r.Context(), &req)
	if err != nil {
		if tx != nil {
			// Payment was attempted but failed — return 422 with the failed transaction
			respondJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":       err.Error(),
				"transaction": tx,
			})
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, tx)
}

// GET /api/v1/transactions/{txID}
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "txID")

	tx, err := h.svc.GetTransaction(r.Context(), txID)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, "transaction not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, tx)
}

// GET /api/v1/transactions?account_id=xxx
func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}

	txs, err := h.svc.ListByAccount(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch transactions")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"transactions": txs,
		"count":        len(txs),
	})
}

// GET /api/v1/transactions/health
func (h *TransactionHandler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "transaction-service",
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
