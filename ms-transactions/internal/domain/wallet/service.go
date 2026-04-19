package wallet

import (
	"errors"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

const (
	maxRetries = 3
	baseDelay  = 10 * time.Millisecond
)

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

	t := Transaction{UserID: uid, Amount: amount, Type: txType}
	if err := t.Validate(); err != nil {
		return Transaction{}, err
	}

	var result Transaction
	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err = s.tryCreate(uid, amount, txType)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, ErrConflict) {
			return Transaction{}, err
		}
		time.Sleep(jitter(baseDelay * (1 << attempt)))
	}
	return Transaction{}, ErrConflict
}

func (s *Service) tryCreate(uid uuid.UUID, amount float64, txType TransactionType) (Transaction, error) {
	var result Transaction
	err := s.repo.InTx(func(repo Repository) error {
		w, err := repo.FindOrCreateWallet(uid)
		if err != nil {
			return err
		}

		if txType == Debit && !w.CanDebit(amount) {
			return ErrInsufficientFunds
		}

		delta := amount
		if txType == Debit {
			delta = -amount
		}

		result, err = repo.CreateTransaction(Transaction{UserID: uid, Amount: amount, Type: txType})
		if err != nil {
			return err
		}

		ok, err := repo.UpdateWalletVersion(w.ID, w.Version, delta)
		if err != nil {
			return err
		}
		if !ok {
			return ErrConflict
		}
		return nil
	})
	return result, err
}

func (s *Service) ListTransactions(userID string, txType string) ([]Transaction, error) {
	return s.repo.FindAllTransactions(userID, txType)
}

func jitter(d time.Duration) time.Duration {
	return d + time.Duration(rand.Int63n(int64(d)+1))
}
