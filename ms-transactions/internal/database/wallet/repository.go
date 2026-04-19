package walletdb

import (
	"fmt"

	domain "github.com/ilia/ms-transactions/internal/domain/wallet"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) domain.Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(tx domain.Transaction) (domain.Transaction, error) {
	const q = `
		INSERT INTO transactions (user_id, amount, type)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, amount, type, created_at`

	var row domain.Transaction
	err := r.db.QueryRowx(q, tx.UserID, tx.Amount, tx.Type).StructScan(&row)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("creating transaction: %w", err)
	}
	return row, nil
}

func (r *PostgresRepository) FindAll(userID string, txType string) ([]domain.Transaction, error) {
	var (
		rows []domain.Transaction
		err  error
	)

	if txType == "" {
		const q = `SELECT id, user_id, amount, type, created_at FROM transactions WHERE user_id = $1 ORDER BY created_at DESC`
		err = r.db.Select(&rows, q, userID)
	} else {
		const q = `SELECT id, user_id, amount, type, created_at FROM transactions WHERE user_id = $1 AND type = $2 ORDER BY created_at DESC`
		err = r.db.Select(&rows, q, userID, txType)
	}

	if err != nil {
		return nil, fmt.Errorf("finding transactions: %w", err)
	}
	return rows, nil
}

func (r *PostgresRepository) GetBalance(userID string) (float64, error) {
	const q = `
		SELECT COALESCE(
			SUM(CASE WHEN type = 'CREDIT' THEN amount ELSE -amount END),
			0
		) FROM transactions WHERE user_id = $1`

	var balance float64
	if err := r.db.QueryRowx(q, userID).Scan(&balance); err != nil {
		return 0, fmt.Errorf("getting balance: %w", err)
	}
	return balance, nil
}
