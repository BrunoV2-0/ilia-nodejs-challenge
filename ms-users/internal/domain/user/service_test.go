package user_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/ilia/ms-users/internal/domain/user"
)

type mockRepository struct {
	createFn      func(u user.User) (user.User, error)
	findAllFn     func() ([]user.User, error)
	findByIDFn    func(id uuid.UUID) (user.User, error)
	findByEmailFn func(email string) (user.User, error)
	updateFn      func(id uuid.UUID, fields user.UpdateFields) (user.User, error)
	deleteFn      func(id uuid.UUID) error
}

func (m *mockRepository) Create(u user.User) (user.User, error)  { return m.createFn(u) }
func (m *mockRepository) FindAll() ([]user.User, error)          { return m.findAllFn() }
func (m *mockRepository) FindByID(id uuid.UUID) (user.User, error) { return m.findByIDFn(id) }
func (m *mockRepository) FindByEmail(email string) (user.User, error) {
	return m.findByEmailFn(email)
}
func (m *mockRepository) Update(id uuid.UUID, fields user.UpdateFields) (user.User, error) {
	return m.updateFn(id, fields)
}
func (m *mockRepository) Delete(id uuid.UUID) error { return m.deleteFn(id) }

// ── CreateUser ────────────────────────────────────────────────────────────────

func TestService_CreateUser_HashesPassword(t *testing.T) {
	const plaintext = "secret123"
	var stored string

	repo := &mockRepository{
		createFn: func(u user.User) (user.User, error) {
			stored = u.Password
			u.ID = uuid.New()
			return u, nil
		},
	}
	svc := user.NewService(repo)

	got, err := svc.CreateUser("John", "Doe", "john@example.com", plaintext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored == plaintext {
		t.Error("password must be hashed before storing")
	}
	if got.ID == (uuid.UUID{}) {
		t.Error("expected non-zero ID from repo")
	}
}

func TestService_CreateUser_ValidationError(t *testing.T) {
	repoCalled := false
	repo := &mockRepository{
		createFn: func(user.User) (user.User, error) {
			repoCalled = true
			return user.User{}, nil
		},
	}
	svc := user.NewService(repo)

	_, err := svc.CreateUser("", "Doe", "john@example.com", "secret")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if repoCalled {
		t.Error("repo must not be called when entity is invalid")
	}
}

func TestService_CreateUser_RepoErrorPropagated(t *testing.T) {
	repo := &mockRepository{
		createFn: func(user.User) (user.User, error) {
			return user.User{}, user.ErrEmailTaken
		},
	}
	svc := user.NewService(repo)

	_, err := svc.CreateUser("John", "Doe", "john@example.com", "secret")
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

// ── Authenticate ──────────────────────────────────────────────────────────────

func TestService_Authenticate_Success(t *testing.T) {
	svc := user.NewService(&mockRepository{
		createFn: func(u user.User) (user.User, error) { u.ID = uuid.New(); return u, nil },
		findByEmailFn: func(string) (user.User, error) { return user.User{}, user.ErrNotFound },
	})

	// create a user so we have a real bcrypt hash to compare against
	_, err := svc.CreateUser("Jane", "Doe", "jane@example.com", "password123")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// now wire findByEmailFn to return the stored (hashed) user
	var storedUser user.User
	repo := &mockRepository{
		createFn: func(u user.User) (user.User, error) {
			storedUser = u
			storedUser.ID = uuid.New()
			return storedUser, nil
		},
		findByEmailFn: func(string) (user.User, error) { return storedUser, nil },
	}
	svc2 := user.NewService(repo)
	_, _ = svc2.CreateUser("Jane", "Doe", "jane@example.com", "password123")

	got, err := svc2.Authenticate("jane@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != "jane@example.com" {
		t.Errorf("unexpected user returned: %+v", got)
	}
}

func TestService_Authenticate_WrongPassword(t *testing.T) {
	var storedUser user.User
	repo := &mockRepository{
		createFn: func(u user.User) (user.User, error) {
			storedUser = u
			storedUser.ID = uuid.New()
			return storedUser, nil
		},
		findByEmailFn: func(string) (user.User, error) { return storedUser, nil },
	}
	svc := user.NewService(repo)
	_, _ = svc.CreateUser("Jane", "Doe", "jane@example.com", "correct")

	_, err := svc.Authenticate("jane@example.com", "wrong")
	if !errors.Is(err, user.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestService_Authenticate_UserNotFound(t *testing.T) {
	repo := &mockRepository{
		findByEmailFn: func(string) (user.User, error) { return user.User{}, user.ErrNotFound },
	}
	svc := user.NewService(repo)

	_, err := svc.Authenticate("ghost@example.com", "any")
	if !errors.Is(err, user.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

// ── UpdateUser ────────────────────────────────────────────────────────────────

func TestService_UpdateUser_HashesNewPassword(t *testing.T) {
	const newPlain = "newpassword"
	var storedHash string

	repo := &mockRepository{
		updateFn: func(_ uuid.UUID, fields user.UpdateFields) (user.User, error) {
			if fields.Password != nil {
				storedHash = *fields.Password
			}
			return user.User{}, nil
		},
	}
	svc := user.NewService(repo)

	id := uuid.New()
	_, err := svc.UpdateUser(id, user.UpdateFields{Password: &[]string{newPlain}[0]})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storedHash == newPlain {
		t.Error("new password must be hashed before storing")
	}
	if storedHash == "" {
		t.Error("expected a hashed password to be stored")
	}
}

func TestService_UpdateUser_NoPasswordFieldUnchanged(t *testing.T) {
	name := "Updated"
	repo := &mockRepository{
		updateFn: func(_ uuid.UUID, fields user.UpdateFields) (user.User, error) {
			if fields.Password != nil {
				t.Error("password must not be touched when not in UpdateFields")
			}
			return user.User{FirstName: name}, nil
		},
	}
	svc := user.NewService(repo)

	got, err := svc.UpdateUser(uuid.New(), user.UpdateFields{FirstName: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FirstName != name {
		t.Errorf("expected first_name %q, got %q", name, got.FirstName)
	}
}

// ── DeleteUser ────────────────────────────────────────────────────────────────

func TestService_DeleteUser_DelegatesToRepo(t *testing.T) {
	id := uuid.New()
	var deletedID uuid.UUID

	repo := &mockRepository{
		deleteFn: func(i uuid.UUID) error {
			deletedID = i
			return nil
		},
	}
	svc := user.NewService(repo)

	if err := svc.DeleteUser(id); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != id {
		t.Errorf("expected deleted ID %v, got %v", id, deletedID)
	}
}
