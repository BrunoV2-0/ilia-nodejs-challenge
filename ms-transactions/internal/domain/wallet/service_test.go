package wallet_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/ilia/ms-transactions/internal/domain/wallet"
)

type mockRepository struct {
	createTransactionFn   func(tx wallet.Transaction) (wallet.Transaction, error)
	findAllFn             func(userID string, txType string) ([]wallet.Transaction, error)
	findOrCreateWalletFn  func(userID uuid.UUID) (wallet.Wallet, error)
	updateWalletVersionFn func(walletID uuid.UUID, currentVersion int64, delta float64) (bool, error)
	inTxFn                func(fn func(wallet.Repository) error) error
}

func (m *mockRepository) CreateTransaction(tx wallet.Transaction) (wallet.Transaction, error) {
	return m.createTransactionFn(tx)
}

func (m *mockRepository) FindAll(userID string, txType string) ([]wallet.Transaction, error) {
	return m.findAllFn(userID, txType)
}

func (m *mockRepository) FindOrCreateWallet(userID uuid.UUID) (wallet.Wallet, error) {
	if m.findOrCreateWalletFn != nil {
		return m.findOrCreateWalletFn(userID)
	}
	return wallet.Wallet{ID: uuid.New(), UserID: userID, Balance: 10000.00, Version: 0}, nil
}

func (m *mockRepository) UpdateWalletVersion(walletID uuid.UUID, currentVersion int64, delta float64) (bool, error) {
	if m.updateWalletVersionFn != nil {
		return m.updateWalletVersionFn(walletID, currentVersion, delta)
	}
	return true, nil
}

func (m *mockRepository) InTx(fn func(wallet.Repository) error) error {
	if m.inTxFn != nil {
		return m.inTxFn(fn)
	}
	return fn(m)
}

func TestService_CreateTransaction(t *testing.T) {
	userID := uuid.New().String()

	t.Run("success — valid credit transaction persisted", func(t *testing.T) {
		repo := &mockRepository{
			createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
				tx.ID = uuid.New()
				return tx, nil
			},
		}
		svc := wallet.NewService(repo)

		got, err := svc.CreateTransaction(userID, 150.75, wallet.Credit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID == (uuid.UUID{}) {
			t.Error("expected non-zero ID from repo")
		}
		if got.Amount != 150.75 {
			t.Errorf("expected amount 150.75, got %v", got.Amount)
		}
		if got.Type != wallet.Credit {
			t.Errorf("expected type CREDIT, got %v", got.Type)
		}
	})

	t.Run("invalid entity — repo never called", func(t *testing.T) {
		repoCalled := false
		repo := &mockRepository{
			createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
				repoCalled = true
				return tx, nil
			},
		}
		svc := wallet.NewService(repo)

		_, err := svc.CreateTransaction(userID, 0, wallet.Credit)
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
			createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
				repoCalled = true
				return tx, nil
			},
		}
		svc := wallet.NewService(repo)

		_, err := svc.CreateTransaction(uuid.UUID{}.String(), 100, wallet.Credit)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if repoCalled {
			t.Error("repo.Create must not be called when entity is invalid")
		}
	})

	t.Run("repo error propagated", func(t *testing.T) {
		repo := &mockRepository{
			createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
				return wallet.Transaction{}, errors.New("db error")
			},
		}
		svc := wallet.NewService(repo)

		_, err := svc.CreateTransaction(userID, 50.00, wallet.Debit)
		if err == nil {
			t.Fatal("expected error from repo, got nil")
		}
	})
}

func TestService_CreateTransaction_DebitInsufficientFunds(t *testing.T) {
	userID := uuid.New().String()

	repo := &mockRepository{
		findOrCreateWalletFn: func(uid uuid.UUID) (wallet.Wallet, error) {
			return wallet.Wallet{ID: uuid.New(), UserID: uid, Balance: 50.00, Version: 0}, nil
		},
		createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
			t.Error("Create must not be called when balance is insufficient")
			return wallet.Transaction{}, nil
		},
	}
	svc := wallet.NewService(repo)

	_, err := svc.CreateTransaction(userID, 100.00, wallet.Debit)
	if !errors.Is(err, wallet.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestService_CreateTransaction_DebitExactBalance(t *testing.T) {
	userID := uuid.New().String()
	walletID := uuid.New()

	repo := &mockRepository{
		findOrCreateWalletFn: func(uid uuid.UUID) (wallet.Wallet, error) {
			return wallet.Wallet{ID: walletID, UserID: uid, Balance: 100.00, Version: 0}, nil
		},
		createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
			tx.ID = uuid.New()
			return tx, nil
		},
	}
	svc := wallet.NewService(repo)

	got, err := svc.CreateTransaction(userID, 100.00, wallet.Debit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == (uuid.UUID{}) {
		t.Error("expected non-zero ID")
	}
}

func TestService_CreateTransaction_OCC_ConflictThenSuccess(t *testing.T) {
	userID := uuid.New().String()
	calls := 0

	repo := &mockRepository{
		createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
			tx.ID = uuid.New()
			return tx, nil
		},
		updateWalletVersionFn: func(walletID uuid.UUID, currentVersion int64, delta float64) (bool, error) {
			calls++
			if calls == 1 {
				return false, nil // simulate stale version on first attempt
			}
			return true, nil
		},
	}
	svc := wallet.NewService(repo)

	got, err := svc.CreateTransaction(userID, 50.00, wallet.Credit)
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if got.ID == (uuid.UUID{}) {
		t.Error("expected non-zero ID")
	}
	if calls != 2 {
		t.Errorf("expected 2 UpdateWalletVersion calls (1 conflict + 1 success), got %d", calls)
	}
}

func TestService_CreateTransaction_OCC_ConflictExhausted(t *testing.T) {
	userID := uuid.New().String()

	repo := &mockRepository{
		createTransactionFn: func(tx wallet.Transaction) (wallet.Transaction, error) {
			tx.ID = uuid.New()
			return tx, nil
		},
		updateWalletVersionFn: func(walletID uuid.UUID, currentVersion int64, delta float64) (bool, error) {
			return false, nil // always conflict
		},
	}
	svc := wallet.NewService(repo)

	_, err := svc.CreateTransaction(userID, 50.00, wallet.Credit)
	if !errors.Is(err, wallet.ErrConflict) {
		t.Fatalf("expected ErrConflict after retries exhausted, got %v", err)
	}
}

func TestService_ListTransactions(t *testing.T) {
	userID := uuid.New().String()

	t.Run("returns all transactions when no type filter", func(t *testing.T) {
		want := []wallet.Transaction{
			{ID: uuid.New(), Amount: 100.00, Type: wallet.Credit},
			{ID: uuid.New(), Amount: 50.00, Type: wallet.Debit},
		}
		repo := &mockRepository{
			findAllFn: func(uid string, txType string) ([]wallet.Transaction, error) {
				if uid != userID {
					t.Errorf("wrong userID passed to repo: %v", uid)
				}
				if txType != "" {
					t.Errorf("expected empty type filter, got %v", txType)
				}
				return want, nil
			},
		}
		svc := wallet.NewService(repo)

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
			findAllFn: func(uid string, txType string) ([]wallet.Transaction, error) {
				if txType != "CREDIT" {
					t.Errorf("expected CREDIT filter, got %v", txType)
				}
				return []wallet.Transaction{}, nil
			},
		}
		svc := wallet.NewService(repo)

		_, err := svc.ListTransactions(userID, "CREDIT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

