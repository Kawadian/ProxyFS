package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
	"github.com/lxcfh/lxcfh/internal/models"
)

type createUserRequest struct {
	Username    string      `json:"username"`
	Password    string      `json:"password"`
	DisplayName string      `json:"display_name"`
	Email       string      `json:"email"`
	Role        models.Role `json:"role"`
}

type updateUserRequest struct {
	DisplayName *string      `json:"display_name"`
	Email       *string      `json:"email"`
	Role        *models.Role `json:"role"`
	Enabled     *bool        `json:"enabled"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

type sshKeyRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Services.ListUsers(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, users)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if req.Role == "" {
		req.Role = models.RoleViewer
	}
	user, err := h.Services.CreateUser(r.Context(), req.Username, req.Password, req.DisplayName, req.Email, req.Role)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, user)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	user, err := h.Services.GetUser(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	var req updateUserRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	user, err := h.Services.UpdateUser(r.Context(), id, req.DisplayName, req.Email, req.Role, req.Enabled)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	if err := h.Services.DeleteUser(r.Context(), id); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	var req passwordRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.ChangePassword(r.Context(), id, req.Password); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListUserSSHKeys(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	keys, err := h.Services.ListUserSSHKeys(r.Context(), userID)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *Handler) AddUserSSHKey(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var req sshKeyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	key, err := h.Services.AddUserSSHKey(r.Context(), userID, req.Name, req.PublicKey)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, key)
}

func (h *Handler) DeleteUserSSHKey(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	keyID := chi.URLParam(r, "keyID")
	if err := h.Services.DeleteUserSSHKey(r.Context(), userID, keyID); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
