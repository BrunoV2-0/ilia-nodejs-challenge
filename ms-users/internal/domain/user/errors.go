package user

import "errors"

var (
	ErrNotFound          = errors.New("user not found")
	ErrEmailTaken        = errors.New("email already in use")
	ErrEmailNotAvailable = errors.New("email not available")
	ErrUnauthorized      = errors.New("invalid credentials")
	ErrWalletNotEmpty    = errors.New("cannot delete user with remaining wallet balance")
)
