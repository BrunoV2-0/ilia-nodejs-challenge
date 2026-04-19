package user

import (
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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
	return s.repo.Delete(id)
}
