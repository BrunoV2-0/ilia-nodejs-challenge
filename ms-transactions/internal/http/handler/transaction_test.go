package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ilia/ms-transactions/internal/http/handler"
	"github.com/ilia/ms-transactions/internal/http/middleware"
	domain "github.com/ilia/ms-transactions/internal/domain/wallet"
)

// mockService satisfies handler's unexported walletService interface.
type mockService struct {
	createFn    func(userID string, amount float64, txType domain.TransactionType) (domain.Transaction, error)
	listFn      func(userID string, txType string) ([]domain.Transaction, error)
	getWalletFn func(userID string) (domain.Wallet, error)
}

func (m *mockService) CreateTransaction(userID string, amount float64, txType domain.TransactionType) (domain.Transaction, error) {
	return m.createFn(userID, amount, txType)
}

func (m *mockService) ListTransactions(userID string, txType string) ([]domain.Transaction, error) {
	return m.listFn(userID, txType)
}

func (m *mockService) GetWallet(userID string) (domain.Wallet, error) {
	return m.getWalletFn(userID)
}

func newRouter(svc *mockService) http.Handler {
	r := chi.NewRouter()
	h := handler.New(svc)
	h.Routes(r)
	h.InternalRoutes(r)
	return r
}

func withUser(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

// ── POST /transactions ────────────────────────────────────────────────────────

func TestCreateTransaction_Created(t *testing.T) {
	userID := uuid.New().String()
	txID := uuid.New()

	svc := &mockService{
		createFn: func(uid string, amount float64, txType domain.TransactionType) (domain.Transaction, error) {
			return domain.Transaction{ID: txID, UserID: uuid.MustParse(uid), Amount: amount, Type: txType, CreatedAt: time.Now()}, nil
		},
	}

	body, _ := json.Marshal(map[string]any{"amount": 100.0, "type": "CREDIT"})
	req := withUser(httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(body)), userID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var got domain.Transaction
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if got.ID != txID {
		t.Errorf("expected ID %v, got %v", txID, got.ID)
	}
}

func TestCreateTransaction_BadJSON(t *testing.T) {
	svc := &mockService{
		createFn: func(string, float64, domain.TransactionType) (domain.Transaction, error) {
			t.Error("service must not be called on bad JSON")
			return domain.Transaction{}, nil
		},
	}

	req := withUser(httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString("not json")), uuid.New().String())
	rec := httptest.NewRecorder()

	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateTransaction_InsufficientFunds(t *testing.T) {
	svc := &mockService{
		createFn: func(string, float64, domain.TransactionType) (domain.Transaction, error) {
			return domain.Transaction{}, domain.ErrInsufficientFunds
		},
	}

	body, _ := json.Marshal(map[string]any{"amount": 999.0, "type": "DEBIT"})
	req := withUser(httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(body)), uuid.New().String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestCreateTransaction_Conflict(t *testing.T) {
	svc := &mockService{
		createFn: func(string, float64, domain.TransactionType) (domain.Transaction, error) {
			return domain.Transaction{}, domain.ErrConflict
		},
	}

	body, _ := json.Marshal(map[string]any{"amount": 10.0, "type": "CREDIT"})
	req := withUser(httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(body)), uuid.New().String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestCreateTransaction_ValidationError(t *testing.T) {
	svc := &mockService{
		createFn: func(string, float64, domain.TransactionType) (domain.Transaction, error) {
			return domain.Transaction{}, errors.New("amount must be greater than zero")
		},
	}

	body, _ := json.Marshal(map[string]any{"amount": 0, "type": "CREDIT"})
	req := withUser(httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(body)), uuid.New().String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ── GET /transactions ─────────────────────────────────────────────────────────

func TestListTransactions_ReturnsAll(t *testing.T) {
	userID := uuid.New().String()
	want := []domain.Transaction{
		{ID: uuid.New(), Amount: 100.0, Type: domain.Credit},
		{ID: uuid.New(), Amount: 50.0, Type: domain.Debit},
	}

	svc := &mockService{
		listFn: func(uid string, txType string) ([]domain.Transaction, error) {
			if uid != userID {
				t.Errorf("wrong userID: %s", uid)
			}
			if txType != "" {
				t.Errorf("expected empty filter, got %q", txType)
			}
			return want, nil
		},
	}

	req := withUser(httptest.NewRequest(http.MethodGet, "/transactions", nil), userID)
	rec := httptest.NewRecorder()
	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var got []domain.Transaction
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(got))
	}
}

func TestListTransactions_TypeFilter(t *testing.T) {
	svc := &mockService{
		listFn: func(_ string, txType string) ([]domain.Transaction, error) {
			if txType != "CREDIT" {
				t.Errorf("expected CREDIT filter, got %q", txType)
			}
			return []domain.Transaction{}, nil
		},
	}

	req := withUser(httptest.NewRequest(http.MethodGet, "/transactions?type=CREDIT", nil), uuid.New().String())
	rec := httptest.NewRecorder()
	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ── GET /wallets/balance (internal JWT + ?user_id=) ──────────────────────────

func TestGetBalance_ReturnsBalance(t *testing.T) {
	walletID := uuid.New()
	userID := uuid.New()
	svc := &mockService{
		getWalletFn: func(uid string) (domain.Wallet, error) {
			if uid != userID.String() {
				t.Errorf("expected user_id %s, got %s", userID, uid)
			}
			return domain.Wallet{ID: walletID, UserID: userID, Balance: 350.75, Version: 1}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/wallets/balance?user_id="+userID.String(), nil)
	rec := httptest.NewRecorder()
	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — %s", rec.Code, rec.Body.String())
	}
	var got map[string]float64
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if got["balance"] != 350.75 {
		t.Errorf("expected balance 350.75, got %v", got["balance"])
	}
}

func TestGetBalance_ServiceError(t *testing.T) {
	svc := &mockService{
		getWalletFn: func(string) (domain.Wallet, error) {
			return domain.Wallet{}, errors.New("db error")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/wallets/balance?user_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetBalance_MissingUserID(t *testing.T) {
	svc := &mockService{}

	req := httptest.NewRequest(http.MethodGet, "/wallets/balance", nil)
	rec := httptest.NewRecorder()
	newRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
