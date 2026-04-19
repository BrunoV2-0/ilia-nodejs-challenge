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
			return domain.User{}, domain.ErrEmailTaken
		}
		return domain.User{}, fmt.Errorf("creating user: %w", err)
	}
	return row, nil
}

func (r *PostgresRepository) FindAll() ([]domain.User, error) {
	const query = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at
		FROM users ORDER BY created_at DESC`

	var rows []domain.User
	if err := r.db.Select(&rows, query); err != nil {
		return nil, fmt.Errorf("finding users: %w", err)
	}
	return rows, nil
}

func (r *PostgresRepository) FindByID(id uuid.UUID) (domain.User, error) {
	const query = `
		SELECT id, first_name, last_name, email, password, created_at, updated_at
		FROM users WHERE id = $1`

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
		FROM users WHERE email = $1`

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
	args := []interface{}{}
	idx := 1

	if fields.FirstName != nil {
		setClauses = append(setClauses, fmt.Sprintf("first_name = $%d", idx))
		args = append(args, *fields.FirstName)
		idx++
	}
	if fields.LastName != nil {
		setClauses = append(setClauses, fmt.Sprintf("last_name = $%d", idx))
		args = append(args, *fields.LastName)
		idx++
	}
	if fields.Email != nil {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", idx))
		args = append(args, *fields.Email)
		idx++
	}
	if fields.Password != nil {
		setClauses = append(setClauses, fmt.Sprintf("password = $%d", idx))
		args = append(args, *fields.Password)
		idx++
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE users SET %s WHERE id = $%d
		RETURNING id, first_name, last_name, email, password, created_at, updated_at`,
		strings.Join(setClauses, ", "), idx)

	var row domain.User
	if err := r.db.QueryRowx(query, args...).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("updating user: %w", err)
	}
	return row, nil
}

func (r *PostgresRepository) Delete(id uuid.UUID) error {
	const query = `DELETE FROM users WHERE id = $1`

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
