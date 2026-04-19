package wallet

import "github.com/google/uuid"

type Repository interface {
	Create(tx Transaction) (Transaction, error)
	FindAll(userID string, txType string) ([]Transaction, error)
	GetBalance(userID string) (float64, error)

	FindOrCreateWallet(userID uuid.UUID) (Wallet, error)
	UpdateWalletVersion(walletID uuid.UUID, currentVersion int64, balanceDelta float64) (bool, error)

	InTx(fn func(Repository) error) error
}
