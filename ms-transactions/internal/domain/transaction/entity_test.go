package transaction_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ilia/ms-transactions/internal/domain/transaction"
)

func TestTransaction_Validate(t *testing.T) {
	validUserID := uuid.New()

	tests := []struct {
		name    string
		tx      transaction.Transaction
		wantErr bool
	}{
		{
			name: "valid credit transaction",
			tx: transaction.Transaction{
				UserID: validUserID,
				Amount: 100.50,
				Type:   transaction.Credit,
			},
			wantErr: false,
		},
		{
			name: "valid debit transaction",
			tx: transaction.Transaction{
				UserID: validUserID,
				Amount: 49.99,
				Type:   transaction.Debit,
			},
			wantErr: false,
		},
		{
			name: "empty UserID",
			tx: transaction.Transaction{
				UserID: uuid.UUID{},
				Amount: 100.00,
				Type:   transaction.Credit,
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			tx: transaction.Transaction{
				UserID: validUserID,
				Amount: 0.0,
				Type:   transaction.Credit,
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			tx: transaction.Transaction{
				UserID: validUserID,
				Amount: -10.00,
				Type:   transaction.Credit,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			tx: transaction.Transaction{
				UserID: validUserID,
				Amount: 100,
				Type:   "TRANSFER",
			},
			wantErr: true,
		},
		{
			name: "empty type",
			tx: transaction.Transaction{
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
