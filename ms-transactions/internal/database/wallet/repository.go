package walletdb

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	domain "github.com/ilia/ms-transactions/internal/domain/wallet"
	"github.com/jmoiron/sqlx"
)

// querier is satisfied by both *sqlx.DB and *sqlx.Tx, allowing InTx to swap
// the underlying connection without duplicating query logic.
type querier interface {
	QueryRowx(query string, args ...interface{}) *sqlx.Row
	Select(dest interface{}, query string, args ...interface{}) error
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type PostgresRepository struct {
	db *sqlx.DB // kept for Beginx; nil only in tests that bypass InTx
	q  querier  // active connection (db or tx)
}

func New(db *sqlx.DB) domain.Repository {
	return &PostgresRepository{db: db, q: db}
}

func (r *PostgresRepository) CreateTransaction(tx domain.Transaction) (domain.Transaction, error) {
	const query = `
		INSERT INTO transactions (user_id, amount, type)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, amount, type, created_at`

	var row domain.Transaction
	err := r.q.QueryRowx(query, tx.UserID, tx.Amount, tx.Type).StructScan(&row)
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
		const query = `SELECT id, user_id, amount, type, created_at FROM transactions WHERE user_id = $1 ORDER BY created_at DESC`
		err = r.q.Select(&rows, query, userID)
	} else {
		const query = `SELECT id, user_id, amount, type, created_at FROM transactions WHERE user_id = $1 AND type = $2 ORDER BY created_at DESC`
		err = r.q.Select(&rows, query, userID, txType)
	}

	if err != nil {
		return nil, fmt.Errorf("finding transactions: %w", err)
	}
	return rows, nil
}


func (r *PostgresRepository) FindOrCreateWallet(userID uuid.UUID) (domain.Wallet, error) {
	const query = `
		INSERT INTO wallets (user_id) VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET user_id = wallets.user_id
		RETURNING id, user_id, balance, version`

	var w domain.Wallet
	err := r.q.QueryRowx(query, userID).StructScan(&w)
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("finding or creating wallet: %w", err)
	}
	return w, nil
}

func (r *PostgresRepository) UpdateWalletVersion(walletID uuid.UUID, currentVersion int64, balanceDelta float64) (bool, error) {
	const query = `
		UPDATE wallets
		SET version = version + 1, balance = balance + $1
		WHERE id = $2 AND version = $3`

	result, err := r.q.Exec(query, balanceDelta, walletID, currentVersion)
	if err != nil {
		return false, fmt.Errorf("updating wallet version: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking rows affected: %w", err)
	}

	return rows == 1, nil
}

func (r *PostgresRepository) InTx(fn func(domain.Repository) error) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	txRepo := &PostgresRepository{db: r.db, q: tx}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
