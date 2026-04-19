package transaction_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/ilia/ms-transactions/internal/domain/transaction"
)

// mockRepository is a hand-written mock — no code generation.
type mockRepository struct {
	createFn     func(tx transaction.Transaction) (transaction.Transaction, error)
	findAllFn    func(userID string, txType string) ([]transaction.Transaction, error)
	getBalanceFn func(userID string) (float64, error)
}

func (m *mockRepository) Create(tx transaction.Transaction) (transaction.Transaction, error) {
	return m.createFn(tx)
}

func (m *mockRepository) FindAll(userID string, txType string) ([]transaction.Transaction, error) {
	return m.findAllFn(userID, txType)
}

func (m *mockRepository) GetBalance(userID string) (float64, error) {
	return m.getBalanceFn(userID)
}

func TestService_CreateTransaction(t *testing.T) {
	userID := uuid.New().String()

	t.Run("success — valid credit transaction persisted", func(t *testing.T) {
		repo := &mockRepository{
			createFn: func(tx transaction.Transaction) (transaction.Transaction, error) {
				tx.ID = uuid.New()
				return tx, nil
			},
		}
		svc := transaction.NewService(repo)

		got, err := svc.CreateTransaction(userID, 150.75, transaction.Credit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID == (uuid.UUID{}) {
			t.Error("expected non-zero ID from repo")
		}
		if got.Amount != 150.75 {
			t.Errorf("expected amount 150.75, got %v", got.Amount)
		}
		if got.Type != transaction.Credit {
			t.Errorf("expected type CREDIT, got %v", got.Type)
		}
	})

	t.Run("invalid entity — repo never called", func(t *testing.T) {
		repoCalled := false
		repo := &mockRepository{
			createFn: func(tx transaction.Transaction) (transaction.Transaction, error) {
				repoCalled = true
				return tx, nil
			},
		}
		svc := transaction.NewService(repo)

		// zero amount is invalid
		_, err := svc.CreateTransaction(userID, 0, transaction.Credit)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if repoCalled {
			t.Error("repo.Create must not be called when entity is invalid")
		}
	})

	t.Run("invalid userID — repo never called", func(t *testing.T) {
		repoCalled := false
		repo := &mockRepository{
			createFn: func(tx transaction.Transaction) (transaction.Transaction, error) {
				repoCalled = true
				return tx, nil
			},
		}
		svc := transaction.NewService(repo)

		_, err := svc.CreateTransaction(uuid.UUID{}.String(), 100, transaction.Credit)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if repoCalled {
			t.Error("repo.Create must not be called when entity is invalid")
		}
	})

	t.Run("repo error propagated", func(t *testing.T) {
		repo := &mockRepository{
			createFn: func(tx transaction.Transaction) (transaction.Transaction, error) {
				return transaction.Transaction{}, errors.New("db error")
			},
		}
		svc := transaction.NewService(repo)

		_, err := svc.CreateTransaction(userID, 50.00, transaction.Debit)
		if err == nil {
			t.Fatal("expected error from repo, got nil")
		}
	})
}

func TestService_ListTransactions(t *testing.T) {
	userID := uuid.New().String()

	t.Run("returns all transactions when no type filter", func(t *testing.T) {
		want := []transaction.Transaction{
			{ID: uuid.New(), Amount: 100.00, Type: transaction.Credit},
			{ID: uuid.New(), Amount: 50.00, Type: transaction.Debit},
		}
		repo := &mockRepository{
			findAllFn: func(uid string, txType string) ([]transaction.Transaction, error) {
				if uid != userID {
					t.Errorf("wrong userID passed to repo: %v", uid)
				}
				if txType != "" {
					t.Errorf("expected empty type filter, got %v", txType)
				}
				return want, nil
			},
		}
		svc := transaction.NewService(repo)

		got, err := svc.ListTransactions(userID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 transactions, got %d", len(got))
		}
	})

	t.Run("passes type filter to repo", func(t *testing.T) {
		repo := &mockRepository{
			findAllFn: func(uid string, txType string) ([]transaction.Transaction, error) {
				if txType != "CREDIT" {
					t.Errorf("expected CREDIT filter, got %v", txType)
				}
				return []transaction.Transaction{}, nil
			},
		}
		svc := transaction.NewService(repo)

		_, err := svc.ListTransactions(userID, "CREDIT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestService_GetBalance(t *testing.T) {
	userID := uuid.New().String()

	t.Run("delegates to repo and returns balance", func(t *testing.T) {
		repo := &mockRepository{
			getBalanceFn: func(uid string) (float64, error) {
				if uid != userID {
					t.Errorf("wrong userID: %v", uid)
				}
				return 250.50, nil
			},
		}
		svc := transaction.NewService(repo)

		got, err := svc.GetBalance(userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 250.50 {
			t.Errorf("expected 250.50, got %v", got)
		}
	})

	t.Run("zero balance on empty account", func(t *testing.T) {
		repo := &mockRepository{
			getBalanceFn: func(uid string) (float64, error) {
				return 0, nil
			},
		}
		svc := transaction.NewService(repo)

		got, err := svc.GetBalance(userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Errorf("expected 0, got %v", got)
		}
	})

	t.Run("repo error propagated", func(t *testing.T) {
		repo := &mockRepository{
			getBalanceFn: func(uid string) (float64, error) {
				return 0, errors.New("db error")
			},
		}
		svc := transaction.NewService(repo)

		_, err := svc.GetBalance(userID)
		if err == nil {
			t.Fatal("expected error from repo, got nil")
		}
	})
}
