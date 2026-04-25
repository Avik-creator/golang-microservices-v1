package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"avikmukherjee/m/account-service/internal/middleware"
	"avikmukherjee/m/account-service/internal/model"
	"avikmukherjee/m/account-service/internal/repository"
	"avikmukherjee/m/account-service/internal/service"

	"github.com/go-chi/chi/v5"
)

type AccountHandler struct {
	svc *service.AccountService
}

func NewAccountHandler(svc *service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// POST /api/v1/accounts
func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())

	var req model.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	account, err := h.svc.CreateAccount(r.Context(), userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, account)
}

// GET /api/v1/accounts
func (h *AccountHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())

	accounts, err := h.svc.ListAccounts(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch accounts")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"accounts": accounts,
		"count":    len(accounts),
	})
}

// GET /api/v1/accounts/{accountID}
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())
	accountID := chi.URLParam(r, "accountID")

	account, err := h.svc.GetAccount(r.Context(), accountID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, "account not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, account)
}

// POST /internal/accounts/balance  (called by transaction-service, not via gateway)
func (h *AccountHandler) UpdateBalance(w http.ResponseWriter, r *http.Request) {
	// Validate internal secret header to prevent direct public access
	if r.Header.Get("X-Internal-Secret") != "internal-secret-change-me" {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req model.BalanceUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	account, err := h.svc.UpdateBalance(r.Context(), &req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "account not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, account)
}

// GET /api/v1/accounts/health
func (h *AccountHandler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "account-service",
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
