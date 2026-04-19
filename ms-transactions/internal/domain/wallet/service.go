package wallet

import "github.com/google/uuid"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTransaction(userID string, amount float64, txType TransactionType) (Transaction, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return Transaction{}, err
	}

	tx := Transaction{
		UserID: uid,
		Amount: amount,
		Type:   txType,
	}
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}

	return s.repo.Create(tx)
}

func (s *Service) ListTransactions(userID string, txType string) ([]Transaction, error) {
	return s.repo.FindAll(userID, txType)
}

func (s *Service) GetBalance(userID string) (float64, error) {
	return s.repo.GetBalance(userID)
}
