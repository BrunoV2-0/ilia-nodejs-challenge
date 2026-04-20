package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domain "github.com/ilia/ms-users/internal/domain/user"
	"github.com/ilia/ms-users/internal/http/handler"
	"github.com/ilia/ms-users/internal/http/middleware"
)

type mockUserService struct {
	createFn func(firstName, lastName, email, password string) (domain.User, error)
	listFn   func() ([]domain.User, error)
	getFn    func(id uuid.UUID) (domain.User, error)
	updateFn func(id uuid.UUID, fields domain.UpdateFields) (domain.User, error)
	deleteFn func(id uuid.UUID) error
}

func (m *mockUserService) CreateUser(fn, ln, email, pw string) (domain.User, error) {
	return m.createFn(fn, ln, email, pw)
}
func (m *mockUserService) ListUsers() ([]domain.User, error) { return m.listFn() }
func (m *mockUserService) GetUser(id uuid.UUID) (domain.User, error) {
	return m.getFn(id)
}
func (m *mockUserService) UpdateUser(id uuid.UUID, f domain.UpdateFields) (domain.User, error) {
	return m.updateFn(id, f)
}
func (m *mockUserService) DeleteUser(id uuid.UUID) error { return m.deleteFn(id) }

func newUserRouter(svc *mockUserService) http.Handler {
	r := chi.NewRouter()
	h := handler.NewUserHandler(svc)
	h.PublicRoutes(r)
	h.ProtectedRoutes(r)
	return r
}

func withCallerCtx(r *http.Request, caller string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.CallerKey, caller)
	return r.WithContext(ctx)
}

// ── POST /users ───────────────────────────────────────────────────────────────

func TestCreateUser_Created(t *testing.T) {
	id := uuid.New()
	svc := &mockUserService{
		createFn: func(fn, ln, email, pw string) (domain.User, error) {
			return domain.User{ID: id, FirstName: fn, LastName: ln, Email: email}, nil
		},
	}

	body, _ := json.Marshal(map[string]string{
		"first_name": "Alice", "last_name": "Smith",
		"email": "alice@example.com", "password": "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — %s", rec.Code, rec.Body.String())
	}
	var got domain.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != id {
		t.Errorf("expected ID %v, got %v", id, got.ID)
	}
	if got.Password != "" {
		t.Error("password must not appear in response")
	}
}

func TestCreateUser_BadJSON(t *testing.T) {
	svc := &mockUserService{
		createFn: func(string, string, string, string) (domain.User, error) {
			t.Error("service must not be called on bad JSON")
			return domain.User{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString("bad"))
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateUser_EmailTaken(t *testing.T) {
	svc := &mockUserService{
		createFn: func(string, string, string, string) (domain.User, error) {
			return domain.User{}, domain.ErrEmailTaken
		},
	}

	body, _ := json.Marshal(map[string]string{
		"first_name": "A", "last_name": "B", "email": "a@b.com", "password": "p",
	})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestCreateUser_ValidationError(t *testing.T) {
	svc := &mockUserService{
		createFn: func(string, string, string, string) (domain.User, error) {
			return domain.User{}, errors.New("email is invalid")
		},
	}

	body, _ := json.Marshal(map[string]string{
		"first_name": "A", "last_name": "B", "email": "not-an-email", "password": "p",
	})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ── GET /users ────────────────────────────────────────────────────────────────

func TestListUsers_ReturnsAll(t *testing.T) {
	want := []domain.User{
		{ID: uuid.New(), FirstName: "A", LastName: "B", Email: "a@b.com"},
		{ID: uuid.New(), FirstName: "C", LastName: "D", Email: "c@d.com"},
	}
	svc := &mockUserService{
		listFn: func() ([]domain.User, error) { return want, nil },
	}

	req := withCallerCtx(httptest.NewRequest(http.MethodGet, "/users", nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body.String())
	}
	var got []domain.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 users, got %d", len(got))
	}
	for _, u := range got {
		if u.Password != "" {
			t.Error("password must not appear in response")
		}
	}
}

func TestListUsers_InternalError(t *testing.T) {
	svc := &mockUserService{
		listFn: func() ([]domain.User, error) { return nil, errors.New("db error") },
	}

	req := withCallerCtx(httptest.NewRequest(http.MethodGet, "/users", nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// ── GET /users/:id ────────────────────────────────────────────────────────────

func TestGetUser_ReturnsUser(t *testing.T) {
	id := uuid.New()
	svc := &mockUserService{
		getFn: func(uid uuid.UUID) (domain.User, error) {
			if uid != id {
				t.Errorf("wrong id: %v", uid)
			}
			return domain.User{ID: id, FirstName: "Alice", Email: "a@b.com"}, nil
		},
	}

	req := withCallerCtx(httptest.NewRequest(http.MethodGet, "/users/"+id.String(), nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body.String())
	}
	var got domain.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != id {
		t.Errorf("expected %v, got %v", id, got.ID)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	svc := &mockUserService{
		getFn: func(uuid.UUID) (domain.User, error) { return domain.User{}, domain.ErrNotFound },
	}

	req := withCallerCtx(httptest.NewRequest(http.MethodGet, "/users/"+uuid.New().String(), nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGetUser_InvalidID(t *testing.T) {
	svc := &mockUserService{}

	req := withCallerCtx(httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ── PATCH /users/:id ─────────────────────────────────────────────────────────

func TestUpdateUser_OK(t *testing.T) {
	id := uuid.New()
	newName := "Bob"
	svc := &mockUserService{
		updateFn: func(uid uuid.UUID, f domain.UpdateFields) (domain.User, error) {
			if uid != id {
				t.Errorf("wrong id: %v", uid)
			}
			if f.FirstName == nil || *f.FirstName != newName {
				t.Errorf("expected first_name %q", newName)
			}
			return domain.User{ID: id, FirstName: newName}, nil
		},
	}

	body, _ := json.Marshal(map[string]string{"first_name": newName})
	req := withCallerCtx(httptest.NewRequest(http.MethodPatch, "/users/"+id.String(), bytes.NewReader(body)), "ms-gateway")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUser_NotFound(t *testing.T) {
	svc := &mockUserService{
		updateFn: func(uuid.UUID, domain.UpdateFields) (domain.User, error) {
			return domain.User{}, domain.ErrNotFound
		},
	}

	body, _ := json.Marshal(map[string]string{"last_name": "X"})
	req := withCallerCtx(httptest.NewRequest(http.MethodPatch, "/users/"+uuid.New().String(), bytes.NewReader(body)), "ms-gateway")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ── DELETE /users/:id ─────────────────────────────────────────────────────────

func TestDeleteUser_NoContent(t *testing.T) {
	id := uuid.New()
	svc := &mockUserService{
		deleteFn: func(uid uuid.UUID) error {
			if uid != id {
				t.Errorf("wrong id: %v", uid)
			}
			return nil
		},
	}

	req := withCallerCtx(httptest.NewRequest(http.MethodDelete, "/users/"+id.String(), nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteUser_WalletNotEmpty(t *testing.T) {
	svc := &mockUserService{
		deleteFn: func(uuid.UUID) error { return domain.ErrWalletNotEmpty },
	}

	req := withCallerCtx(httptest.NewRequest(http.MethodDelete, "/users/"+uuid.New().String(), nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	svc := &mockUserService{
		deleteFn: func(uuid.UUID) error { return domain.ErrNotFound },
	}

	req := withCallerCtx(httptest.NewRequest(http.MethodDelete, "/users/"+uuid.New().String(), nil), "ms-gateway")
	rec := httptest.NewRecorder()
	newUserRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
