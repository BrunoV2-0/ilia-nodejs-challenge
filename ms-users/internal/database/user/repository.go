package userdb

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	domain "github.com/ilia/ms-users/internal/domain/user"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const pgUniqueViolation = "23505"

type PostgresRepository struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) domain.Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(u domain.User) (domain.User, error) {
	const query = `
		INSERT INTO users (first_name, last_name, email, password)
		VALUES ($1, $2, $3, $4)
		RETURNING id, first_name, last_name, email, password, created_at, updated_at`

	var row domain.User
	err := r.db.QueryRowx(query, u.FirstName, u.LastName, u.Email, u.Password).StructScan(&row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pgUniqueViolation {
			return domain.User{}, r.emailConflictError(u.Email)
		}
		return domain.User{}, fmt.Errorf("creating user: %w", err)
	}
	return row, nil
}

func (r *PostgresRepository) FindAll() ([]domain.User, error) {
	const query = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC`

	var rows []domain.User
	if err := r.db.Select(&rows, query); err != nil {
		return nil, fmt.Errorf("finding users: %w", err)
	}
	return rows, nil
}

func (r *PostgresRepository) FindByID(id uuid.UUID) (domain.User, error) {
	const query = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	var row domain.User
	if err := r.db.QueryRowx(query, id).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("finding user by id: %w", err)
	}
	return row, nil
}

func (r *PostgresRepository) FindByEmail(email string) (domain.User, error) {
	const query = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`

	var row domain.User
	if err := r.db.QueryRowx(query, email).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("finding user by email: %w", err)
	}
	return row, nil
}

func (r *PostgresRepository) Update(id uuid.UUID, fields domain.UpdateFields) (domain.User, error) {
	setClauses := []string{"updated_at = NOW()"}
	params := map[string]any{"id": id}

	if fields.FirstName != nil {
		setClauses = append(setClauses, "first_name = :first_name")
		params["first_name"] = *fields.FirstName
	}
	if fields.LastName != nil {
		setClauses = append(setClauses, "last_name = :last_name")
		params["last_name"] = *fields.LastName
	}
	if fields.Email != nil {
		setClauses = append(setClauses, "email = :email")
		params["email"] = *fields.Email
	}
	if fields.Password != nil {
		setClauses = append(setClauses, "password = :password")
		params["password"] = *fields.Password
	}

	query := fmt.Sprintf(`
		UPDATE users SET %s
		WHERE id = :id AND deleted_at IS NULL
		RETURNING id, first_name, last_name, email, password, created_at, updated_at`,
		strings.Join(setClauses, ", "))

	nq, args, err := sqlx.Named(query, params)
	if err != nil {
		return domain.User{}, fmt.Errorf("building update query: %w", err)
	}

	var row domain.User
	if err := r.db.QueryRowx(r.db.Rebind(nq), args...).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pgUniqueViolation {
			return domain.User{}, r.emailConflictError(*fields.Email)
		}
		return domain.User{}, fmt.Errorf("updating user: %w", err)
	}
	return row, nil
}

// emailConflictError returns ErrEmailNotAvailable when the conflicting email
// row belongs to a soft-deleted user, and ErrEmailTaken when it is active.
func (r *PostgresRepository) emailConflictError(email string) error {
	var deletedAt sql.NullTime
	err := r.db.QueryRow(`SELECT deleted_at FROM users WHERE email = $1`, email).Scan(&deletedAt)
	if err == nil && deletedAt.Valid {
		return domain.ErrEmailNotAvailable
	}
	return domain.ErrEmailTaken
}

func (r *PostgresRepository) Delete(id uuid.UUID) error {
	const query = `
		UPDATE users SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
