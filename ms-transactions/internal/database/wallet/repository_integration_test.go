package walletdb_test

import (
	"context"
	"path/filepath"
	"runtime"
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

	// resolve migrations path relative to this test file
	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "../../migrations")
	_ = migrationsPath

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS transactions (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id    UUID NOT NULL,
		amount     DECIMAL(15,2) NOT NULL,
		type       VARCHAR(10) NOT NULL CHECK (type IN ('CREDIT', 'DEBIT')),
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

	got, err := repo.Create(tx)
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

	_, err := repo.Create(domain.Transaction{UserID: userID, Amount: 100.00, Type: domain.Credit})
	require.NoError(t, err)
	_, err = repo.Create(domain.Transaction{UserID: userID, Amount: 40.00, Type: domain.Debit})
	require.NoError(t, err)
	_, err = repo.Create(domain.Transaction{UserID: otherUserID, Amount: 200.00, Type: domain.Credit})
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

func TestRepository_GetBalance(t *testing.T) {
	db := setupTestDB(t)
	repo := walletdb.New(db)

	userID := uuid.New()

	t.Run("returns 0 on empty table", func(t *testing.T) {
		balance, err := repo.GetBalance(userID.String())
		require.NoError(t, err)
		assert.Equal(t, 0.0, balance)
	})

	t.Run("computes balance from mixed CREDIT and DEBIT rows", func(t *testing.T) {
		_, err := repo.Create(domain.Transaction{UserID: userID, Amount: 200.00, Type: domain.Credit})
		require.NoError(t, err)
		_, err = repo.Create(domain.Transaction{UserID: userID, Amount: 50.50, Type: domain.Debit})
		require.NoError(t, err)

		balance, err := repo.GetBalance(userID.String())
		require.NoError(t, err)
		assert.Equal(t, 149.50, balance)
	})

	t.Run("only credits gives positive balance", func(t *testing.T) {
		uid := uuid.New()
		_, err := repo.Create(domain.Transaction{UserID: uid, Amount: 75.25, Type: domain.Credit})
		require.NoError(t, err)

		balance, err := repo.GetBalance(uid.String())
		require.NoError(t, err)
		assert.Equal(t, 75.25, balance)
	})
}
