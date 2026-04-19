package wallet

import "github.com/google/uuid"

type Wallet struct {
	ID      uuid.UUID `db:"id"`
	UserID  uuid.UUID `db:"user_id"`
	Balance float64   `db:"balance"`
	Version int64     `db:"version"`
}

func (w Wallet) CanDebit(amount float64) bool {
	return w.Balance >= amount
}
