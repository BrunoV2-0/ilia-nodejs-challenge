package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	domain "github.com/ilia/ms-transactions/internal/domain/wallet"
	"github.com/ilia/ms-transactions/internal/http/middleware"
)

type walletService interface {
	CreateTransaction(userID string, amount float64, txType domain.TransactionType) (domain.Transaction, error)
	ListTransactions(userID string, txType string) ([]domain.Transaction, error)
	GetWallet(userID string) (domain.Wallet, error)
}

type Handler struct {
	svc walletService
}

func New(svc walletService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/transactions", h.createTransaction)
	r.Get("/transactions", h.listTransactions)
}

func (h *Handler) InternalRoutes(r chi.Router) {
	r.Get("/wallets/balance", h.getInternalBalance)
}

type createRequest struct {
	Amount float64              `json:"amount"`
	Type   domain.TransactionType `json:"type"`
}

func (h *Handler) createTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := h.svc.CreateTransaction(userID, req.Amount, req.Type)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInsufficientFunds):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, domain.ErrConflict):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, tx)
}

func (h *Handler) listTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	txType := r.URL.Query().Get("type")
	txs, err := h.svc.ListTransactions(userID, txType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, txs)
}

// getInternalBalance serves the service-to-service route.
// The caller is authenticated by the internal JWT; the target user is passed
// as a query parameter rather than the JWT sub (which identifies the calling service).
func (h *Handler) getInternalBalance(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id query parameter is required")
		return
	}

	wallet, err := h.svc.GetWallet(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]float64{"balance": wallet.Balance})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
