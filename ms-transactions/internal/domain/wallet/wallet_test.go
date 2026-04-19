package wallet_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ilia/ms-transactions/internal/domain/wallet"
)

func TestWallet_CanDebit(t *testing.T) {
	w := wallet.Wallet{ID: uuid.New(), UserID: uuid.New(), Balance: 100.00}

	t.Run("sufficient balance returns true", func(t *testing.T) {
		if !w.CanDebit(50.00) {
			t.Error("expected CanDebit(50) = true")
		}
	})

	t.Run("exact balance returns true", func(t *testing.T) {
		if !w.CanDebit(100.00) {
			t.Error("expected CanDebit(100) = true for exact amount")
		}
	})

	t.Run("exceeds balance returns false", func(t *testing.T) {
		if w.CanDebit(100.01) {
			t.Error("expected CanDebit(100.01) = false")
		}
	})

	t.Run("zero balance returns false", func(t *testing.T) {
		empty := wallet.Wallet{Balance: 0}
		if empty.CanDebit(0.01) {
			t.Error("expected CanDebit on zero balance = false")
		}
	})
}
