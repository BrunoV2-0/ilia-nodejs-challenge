package user

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// WalletChecker is satisfied by the ms-transactions HTTP client.
// Defined here so the domain rule (no delete with balance) stays in the domain.
type WalletChecker interface {
	HasBalance(userID uuid.UUID) (bool, error)
}

type Service struct {
	repo    Repository
	wallets WalletChecker
}

func NewService(repo Repository, wallets WalletChecker) *Service {
	return &Service{repo: repo, wallets: wallets}
}

func (s *Service) CreateUser(firstName, lastName, email, password string) (User, error) {
	u := User{FirstName: firstName, LastName: lastName, Email: email, Password: password}
	if err := u.Validate(); err != nil {
		return User{}, err
	}

	hashed, err := HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("hashing password: %w", err)
	}
	u.Password = hashed

	return s.repo.Create(u)
}

func (s *Service) Authenticate(email, password string) (User, error) {
	u, err := s.repo.FindByEmail(email)
	if err != nil {
		return User{}, ErrUnauthorized
	}

	if !VerifyPassword(password, u.Password) {
		return User{}, ErrUnauthorized
	}

	return u, nil
}

func (s *Service) ListUsers() ([]User, error) {
	return s.repo.FindAll()
}

func (s *Service) GetUser(id uuid.UUID) (User, error) {
	return s.repo.FindByID(id)
}

func (s *Service) UpdateUser(id uuid.UUID, fields UpdateFields) (User, error) {
	if fields.Email != nil && !emailRegex.MatchString(*fields.Email) {
		return User{}, errors.New("email is invalid")
	}
	if fields.Password != nil {
		hashed, err := HashPassword(*fields.Password)
		if err != nil {
			return User{}, fmt.Errorf("hashing password: %w", err)
		}
		fields.Password = &hashed
	}
	return s.repo.Update(id, fields)
}

func (s *Service) DeleteUser(id uuid.UUID) error {
	hasBalance, err := s.wallets.HasBalance(id)
	if err != nil {
		return fmt.Errorf("checking wallet balance: %w", err)
	}
	if hasBalance {
		return ErrWalletNotEmpty
	}
	return s.repo.Delete(id)
}
