package user

import (
	"errors"
	"regexp"

	"github.com/google/uuid"
)

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type User struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	FirstName string    `db:"first_name" json:"first_name"`
	LastName  string    `db:"last_name"  json:"last_name"`
	Email     string    `db:"email"      json:"email"`
	Password  string    `db:"password"   json:"-"`
}

func (u *User) Validate() error {
	if u.FirstName == "" {
		return errors.New("first_name is required")
	}
	if u.LastName == "" {
		return errors.New("last_name is required")
	}
	if u.Email == "" {
		return errors.New("email is required")
	}
	if !emailRegex.MatchString(u.Email) {
		return errors.New("email is invalid")
	}
	if u.Password == "" {
		return errors.New("password is required")
	}
	return nil
}
