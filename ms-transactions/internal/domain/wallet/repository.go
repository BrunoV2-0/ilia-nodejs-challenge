package wallet

import "github.com/google/uuid"

type Repository interface {
	CreateTransaction(tx Transaction) (Transaction, error)
	FindAllTransactions(userID string, txType string) ([]Transaction, error)

	FindOrCreateWallet(userID uuid.UUID) (Wallet, error)
	UpdateWalletVersion(walletID uuid.UUID, currentVersion int64, balanceDelta float64) (bool, error)

	InTx(fn func(Repository) error) error
}
