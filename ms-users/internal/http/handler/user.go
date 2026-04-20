package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domain "github.com/ilia/ms-users/internal/domain/user"
)

type userService interface {
	CreateUser(firstName, lastName, email, password string) (domain.User, error)
	ListUsers() ([]domain.User, error)
	GetUser(id uuid.UUID) (domain.User, error)
	UpdateUser(id uuid.UUID, fields domain.UpdateFields) (domain.User, error)
	DeleteUser(id uuid.UUID) error
}

type UserHandler struct {
	svc userService
}

func NewUserHandler(svc userService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) PublicRoutes(r chi.Router) {
	r.Post("/users", h.createUser)
}

func (h *UserHandler) ProtectedRoutes(r chi.Router) {
	r.Get("/users", h.listUsers)
	r.Get("/users/{id}", h.getUser)
	r.Patch("/users/{id}", h.updateUser)
	r.Delete("/users/{id}", h.deleteUser)
}

type createUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (h *UserHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	u, err := h.svc.CreateUser(req.FirstName, req.LastName, req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmailTaken):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, u)
}

func (h *UserHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, users)
}

func (h *UserHandler) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	u, err := h.svc.GetUser(id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, u)
}

type updateUserRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Email     *string `json:"email"`
	Password  *string `json:"password"`
}

func (h *UserHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fields := domain.UpdateFields{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  req.Password,
	}

	u, err := h.svc.UpdateUser(id, fields)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrEmailTaken):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, u)
}

func (h *UserHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := h.svc.DeleteUser(id); err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrWalletNotEmpty):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
