package transaction

import (
	"errors"

	"github.com/google/uuid"
)

type TransactionType string

const (
	Credit TransactionType = "CREDIT"
	Debit  TransactionType = "DEBIT"
)

type Transaction struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Amount float64
	Type   TransactionType
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
