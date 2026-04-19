package wallet_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ilia/ms-transactions/internal/domain/wallet"
)

func TestTransaction_Validate(t *testing.T) {
	validUserID := uuid.New()

	tests := []struct {
		name    string
		tx      wallet.Transaction
		wantErr bool
	}{
		{
			name: "valid credit transaction",
			tx: wallet.Transaction{
				UserID: validUserID,
				Amount: 100.50,
				Type:   wallet.Credit,
			},
			wantErr: false,
		},
		{
			name: "valid debit transaction",
			tx: wallet.Transaction{
				UserID: validUserID,
				Amount: 49.99,
				Type:   wallet.Debit,
			},
			wantErr: false,
		},
		{
			name: "empty UserID",
			tx: wallet.Transaction{
				UserID: uuid.UUID{},
				Amount: 100.00,
				Type:   wallet.Credit,
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			tx: wallet.Transaction{
				UserID: validUserID,
				Amount: 0.0,
				Type:   wallet.Credit,
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			tx: wallet.Transaction{
				UserID: validUserID,
				Amount: -10.00,
				Type:   wallet.Credit,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			tx: wallet.Transaction{
				UserID: validUserID,
				Amount: 100,
				Type:   "TRANSFER",
			},
			wantErr: true,
		},
		{
			name: "empty type",
			tx: wallet.Transaction{
				UserID: validUserID,
				Amount: 100,
				Type:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tx.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}
