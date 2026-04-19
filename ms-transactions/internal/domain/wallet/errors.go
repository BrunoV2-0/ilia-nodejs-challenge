package wallet

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrConflict          = errors.New("conflict: wallet modified concurrently")
)
