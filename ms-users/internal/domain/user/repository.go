package user

import "github.com/google/uuid"

type UpdateFields struct {
	FirstName *string
	LastName  *string
	Email     *string
	Password  *string
}

type Repository interface {
	Create(u User) (User, error)
	FindAll() ([]User, error)
	FindByID(id uuid.UUID) (User, error)
	FindByEmail(email string) (User, error)
	Update(id uuid.UUID, fields UpdateFields) (User, error)
	Delete(id uuid.UUID) error
}
