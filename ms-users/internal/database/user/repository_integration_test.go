package userdb_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	userdb "github.com/ilia/ms-users/internal/database/user"
	"github.com/ilia/ms-users/internal/domain/user"
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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		first_name VARCHAR(100) NOT NULL,
		last_name  VARCHAR(100) NOT NULL,
		email      VARCHAR(255) NOT NULL UNIQUE,
		password   VARCHAR(255) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	)`)
	require.NoError(t, err)

	return db
}

func seedUser(t *testing.T, repo user.Repository, email string) user.User {
	t.Helper()
	u, err := repo.Create(user.User{
		FirstName: "John",
		LastName:  "Doe",
		Email:     email,
		Password:  "hashed-password",
	})
	require.NoError(t, err)
	return u
}

func TestRepository_Create(t *testing.T) {
	repo := userdb.New(setupTestDB(t))

	u, err := repo.Create(user.User{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane@example.com",
		Password:  "hashed",
	})
	require.NoError(t, err)

	assert.NotEqual(t, uuid.UUID{}, u.ID)
	assert.Equal(t, "jane@example.com", u.Email)
	assert.False(t, u.CreatedAt.IsZero())
	assert.False(t, u.UpdatedAt.IsZero())
}

func TestRepository_Create_DuplicateEmail(t *testing.T) {
	repo := userdb.New(setupTestDB(t))
	seedUser(t, repo, "dup@example.com")

	_, err := repo.Create(user.User{FirstName: "A", LastName: "B", Email: "dup@example.com", Password: "x"})
	assert.ErrorIs(t, err, user.ErrEmailTaken)
}

func TestRepository_FindAll(t *testing.T) {
	repo := userdb.New(setupTestDB(t))
	seedUser(t, repo, "a@example.com")
	seedUser(t, repo, "b@example.com")

	users, err := repo.FindAll()
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestRepository_FindByID(t *testing.T) {
	repo := userdb.New(setupTestDB(t))
	created := seedUser(t, repo, "find@example.com")

	got, err := repo.FindByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "find@example.com", got.Email)
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	repo := userdb.New(setupTestDB(t))

	_, err := repo.FindByID(uuid.New())
	assert.ErrorIs(t, err, user.ErrNotFound)
}

func TestRepository_FindByEmail(t *testing.T) {
	repo := userdb.New(setupTestDB(t))
	seedUser(t, repo, "email@example.com")

	got, err := repo.FindByEmail("email@example.com")
	require.NoError(t, err)
	assert.Equal(t, "email@example.com", got.Email)
}

func TestRepository_FindByEmail_NotFound(t *testing.T) {
	repo := userdb.New(setupTestDB(t))

	_, err := repo.FindByEmail("ghost@example.com")
	assert.ErrorIs(t, err, user.ErrNotFound)
}

func TestRepository_Update_PartialFields(t *testing.T) {
	repo := userdb.New(setupTestDB(t))
	created := seedUser(t, repo, "update@example.com")

	newFirst := "Updated"
	got, err := repo.Update(created.ID, user.UpdateFields{FirstName: &newFirst})
	require.NoError(t, err)

	assert.Equal(t, "Updated", got.FirstName)
	assert.Equal(t, "Doe", got.LastName)
	assert.Equal(t, "update@example.com", got.Email)
	assert.True(t, got.UpdatedAt.After(created.UpdatedAt) || got.UpdatedAt.Equal(created.UpdatedAt))
}

func TestRepository_Update_AllFields(t *testing.T) {
	repo := userdb.New(setupTestDB(t))
	created := seedUser(t, repo, "allfields@example.com")

	newFirst, newLast, newEmail, newPass := "New", "Name", "new@example.com", "newhash"
	got, err := repo.Update(created.ID, user.UpdateFields{
		FirstName: &newFirst,
		LastName:  &newLast,
		Email:     &newEmail,
		Password:  &newPass,
	})
	require.NoError(t, err)

	assert.Equal(t, "New", got.FirstName)
	assert.Equal(t, "Name", got.LastName)
	assert.Equal(t, "new@example.com", got.Email)
}

func TestRepository_Update_NotFound(t *testing.T) {
	repo := userdb.New(setupTestDB(t))

	name := "X"
	_, err := repo.Update(uuid.New(), user.UpdateFields{FirstName: &name})
	assert.ErrorIs(t, err, user.ErrNotFound)
}

func TestRepository_Delete_SoftDeletes(t *testing.T) {
	db := setupTestDB(t)
	repo := userdb.New(db)
	created := seedUser(t, repo, "delete@example.com")

	require.NoError(t, repo.Delete(created.ID))

	// domain queries must not see the soft-deleted user
	_, err := repo.FindByID(created.ID)
	assert.ErrorIs(t, err, user.ErrNotFound)

	// but the row must still exist in the database with deleted_at set
	var deletedAt *string
	err = db.QueryRow(`SELECT deleted_at::text FROM users WHERE id = $1`, created.ID).Scan(&deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt, "deleted_at must be set after soft delete")
}

func TestRepository_Delete_NotFound(t *testing.T) {
	repo := userdb.New(setupTestDB(t))

	err := repo.Delete(uuid.New())
	assert.ErrorIs(t, err, user.ErrNotFound)
}
