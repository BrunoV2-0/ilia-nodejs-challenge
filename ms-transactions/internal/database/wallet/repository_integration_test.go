package walletdb_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	walletdb "github.com/ilia/ms-transactions/internal/database/wallet"
	domain "github.com/ilia/ms-transactions/internal/domain/wallet"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	require.NoError(t, err)
	t.Cleanup(func() { container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sqlx.Connect("postgres", connStr)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS transactions (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id    UUID NOT NULL,
		amount     DECIMAL(15,2) NOT NULL,
		type       VARCHAR(10) NOT NULL CHECK (type IN ('CREDIT', 'DEBIT')),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS wallets (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id    UUID NOT NULL UNIQUE,
		balance    DECIMAL(15,2) NOT NULL DEFAULT 0,
		version    BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	require.NoError(t, err)

	return db
}

func TestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()
	tx := domain.Transaction{
		UserID: userID,
		Amount: 100.50,
		Type:   domain.Credit,
	}

	got, err := repo.CreateTransaction(tx)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.UUID{}, got.ID, "ID should be assigned by the database")
	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, 100.50, got.Amount)
	assert.Equal(t, domain.Credit, got.Type)
}

func TestRepository_FindAll(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()
	otherUserID := uuid.New()

	_, err := repo.CreateTransaction(domain.Transaction{UserID: userID, Amount: 100.00, Type: domain.Credit})
	require.NoError(t, err)
	_, err = repo.CreateTransaction(domain.Transaction{UserID: userID, Amount: 40.00, Type: domain.Debit})
	require.NoError(t, err)
	_, err = repo.CreateTransaction(domain.Transaction{UserID: otherUserID, Amount: 200.00, Type: domain.Credit})
	require.NoError(t, err)

	t.Run("returns all transactions for user with no type filter", func(t *testing.T) {
		txs, err := repo.FindAll(userID.String(), "")
		require.NoError(t, err)
		assert.Len(t, txs, 2)
	})

	t.Run("filters by CREDIT type", func(t *testing.T) {
		txs, err := repo.FindAll(userID.String(), "CREDIT")
		require.NoError(t, err)
		assert.Len(t, txs, 1)
		assert.Equal(t, domain.Credit, txs[0].Type)
	})

	t.Run("filters by DEBIT type", func(t *testing.T) {
		txs, err := repo.FindAll(userID.String(), "DEBIT")
		require.NoError(t, err)
		assert.Len(t, txs, 1)
		assert.Equal(t, domain.Debit, txs[0].Type)
	})

	t.Run("does not return other user transactions", func(t *testing.T) {
		txs, err := repo.FindAll(otherUserID.String(), "")
		require.NoError(t, err)
		assert.Len(t, txs, 1)
	})
}

func TestRepository_FindOrCreateWallet(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()

	t.Run("creates wallet on first call", func(t *testing.T) {
		w, err := repo.FindOrCreateWallet(userID)
		require.NoError(t, err)

		assert.NotEqual(t, uuid.UUID{}, w.ID)
		assert.Equal(t, userID, w.UserID)
		assert.Equal(t, 0.0, w.Balance)
		assert.Equal(t, int64(0), w.Version)
	})

	t.Run("returns same wallet on subsequent calls", func(t *testing.T) {
		w1, err := repo.FindOrCreateWallet(userID)
		require.NoError(t, err)

		w2, err := repo.FindOrCreateWallet(userID)
		require.NoError(t, err)

		assert.Equal(t, w1.ID, w2.ID)
		assert.Equal(t, w1.Version, w2.Version)
	})
}

func TestRepository_UpdateWalletVersion(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()

	t.Run("succeeds when version matches — updates balance and increments version", func(t *testing.T) {
		w, err := repo.FindOrCreateWallet(userID)
		require.NoError(t, err)

		ok, err := repo.UpdateWalletVersion(w.ID, w.Version, 200.00)
		require.NoError(t, err)
		assert.True(t, ok)

		updated, err := repo.FindOrCreateWallet(userID)
		require.NoError(t, err)
		assert.Equal(t, 200.0, updated.Balance)
		assert.Equal(t, w.Version+1, updated.Version)
	})

	t.Run("returns false when version is stale", func(t *testing.T) {
		uid := uuid.New()
		w, err := repo.FindOrCreateWallet(uid)
		require.NoError(t, err)

		// first update succeeds, bumps version to 1
		ok, err := repo.UpdateWalletVersion(w.ID, w.Version, 100.00)
		require.NoError(t, err)
		require.True(t, ok)

		// second update with the old version (0) must fail
		ok, err = repo.UpdateWalletVersion(w.ID, w.Version, 50.00)
		require.NoError(t, err)
		assert.False(t, ok, "stale version should not update")

		// balance must remain 100 (second update was rejected)
		final, err := repo.FindOrCreateWallet(uid)
		require.NoError(t, err)
		assert.Equal(t, 100.0, final.Balance)
	})
}


func TestRepository_InTx_Commit(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()
	var createdID uuid.UUID

	err := repo.InTx(func(tx domain.Repository) error {
		got, err := tx.CreateTransaction(domain.Transaction{UserID: userID, Amount: 100.00, Type: domain.Credit})
		if err != nil {
			return err
		}
		createdID = got.ID
		return nil
	})
	require.NoError(t, err)

	txs, err := repo.FindAll(userID.String(), "")
	require.NoError(t, err)
	require.Len(t, txs, 1)
	assert.Equal(t, createdID, txs[0].ID)
}

func TestRepository_InTx_Rollback(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()
	sentinel := errors.New("abort")

	err := repo.InTx(func(tx domain.Repository) error {
		_, err := tx.CreateTransaction(domain.Transaction{UserID: userID, Amount: 100.00, Type: domain.Credit})
		if err != nil {
			return err
		}
		return sentinel // trigger rollback
	})
	assert.ErrorIs(t, err, sentinel)

	txs, err := repo.FindAll(userID.String(), "")
	require.NoError(t, err)
	assert.Empty(t, txs, "transaction must be rolled back")
}

func TestRepository_OCC_ConcurrentDebit(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()

	// seed wallet with 200.00
	w, err := repo.FindOrCreateWallet(userID)
	require.NoError(t, err)
	ok, err := repo.UpdateWalletVersion(w.ID, w.Version, 200.00)
	require.NoError(t, err)
	require.True(t, ok)

	// two goroutines each try to debit 150 — only one can succeed
	type result struct {
		ok  bool
		err error
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			err := repo.InTx(func(tx domain.Repository) error {
				wallet, err := tx.FindOrCreateWallet(userID)
				if err != nil {
					return err
				}
				if !wallet.CanDebit(150.00) {
					results[i] = result{ok: false, err: domain.ErrInsufficientFunds}
					return domain.ErrInsufficientFunds
				}
				_, err = tx.CreateTransaction(domain.Transaction{UserID: userID, Amount: 150.00, Type: domain.Debit})
				if err != nil {
					return err
				}
				updated, err := tx.UpdateWalletVersion(wallet.ID, wallet.Version, -150.00)
				if err != nil {
					return err
				}
				if !updated {
					return domain.ErrConflict
				}
				results[i] = result{ok: true}
				return nil
			})
			if err != nil && !errors.Is(err, domain.ErrInsufficientFunds) && !errors.Is(err, domain.ErrConflict) {
				results[i] = result{err: err}
			}
		}()
	}
	wg.Wait()

	successes := 0
	for _, r := range results {
		if r.ok {
			successes++
		}
	}
	assert.Equal(t, 1, successes, "exactly one debit must succeed")

	finalWallet, err := repo.FindOrCreateWallet(userID)
	require.NoError(t, err)
	assert.Equal(t, 50.0, finalWallet.Balance, "balance must be 200 - 150 = 50")
}
