package wallet

type Repository interface {
	Create(tx Transaction) (Transaction, error)
	FindAll(userID string, txType string) ([]Transaction, error)
	GetBalance(userID string) (float64, error)
}
