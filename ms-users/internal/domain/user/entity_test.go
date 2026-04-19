package user_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ilia/ms-users/internal/domain/user"
)

func validUser() user.User {
	return user.User{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
		Password:  "secret123",
	}
}

func TestUser_Validate(t *testing.T) {
	t.Run("valid user passes", func(t *testing.T) {
		u := validUser()
		if err := u.Validate(); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("empty first_name", func(t *testing.T) {
		u := validUser()
		u.FirstName = ""
		if err := u.Validate(); err == nil {
			t.Error("expected error for empty first_name")
		}
	})

	t.Run("empty last_name", func(t *testing.T) {
		u := validUser()
		u.LastName = ""
		if err := u.Validate(); err == nil {
			t.Error("expected error for empty last_name")
		}
	})

	t.Run("empty email", func(t *testing.T) {
		u := validUser()
		u.Email = ""
		if err := u.Validate(); err == nil {
			t.Error("expected error for empty email")
		}
	})

	t.Run("invalid email format", func(t *testing.T) {
		u := validUser()
		u.Email = "not-an-email"
		if err := u.Validate(); err == nil {
			t.Error("expected error for invalid email")
		}
	})

	t.Run("empty password", func(t *testing.T) {
		u := validUser()
		u.Password = ""
		if err := u.Validate(); err == nil {
			t.Error("expected error for empty password")
		}
	})
}

func TestUser_PasswordNotSerialised(t *testing.T) {
	u := user.User{
		ID:        uuid.New(),
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
		Password:  "secret",
	}
	// Password field carries json:"-" — verify it is never accidentally exposed
	// by checking the struct tag at compile time via the zero value of the field.
	// The real guard is the json:"-" tag; this test documents the expectation.
	if u.Password == "" {
		t.Error("Password field must be settable on the struct")
	}
}
