package wallet

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type TransactionType string

const (
	Credit TransactionType = "CREDIT"
	Debit  TransactionType = "DEBIT"
)

type Transaction struct {
	ID        uuid.UUID       `db:"id"         json:"id"`
	UserID    uuid.UUID       `db:"user_id"    json:"user_id"`
	Amount    float64         `db:"amount"     json:"amount"`
	Type      TransactionType `db:"type"       json:"type"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}

func (t *Transaction) Validate() error {
	if t.UserID == (uuid.UUID{}) {
		return errors.New("user_id is required")
	}
	if t.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if t.Type != Credit && t.Type != Debit {
		return errors.New("type must be CREDIT or DEBIT")
	}
	return nil
}
