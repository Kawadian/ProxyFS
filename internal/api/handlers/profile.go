package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
)

func (h *Handler) ListMySSHKeys(w http.ResponseWriter, r *http.Request) {
	user := httputil.UserFromContext(r.Context())
	keys, err := h.Services.ListUserSSHKeys(r.Context(), user.ID)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *Handler) AddMySSHKey(w http.ResponseWriter, r *http.Request) {
	user := httputil.UserFromContext(r.Context())
	var req sshKeyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	key, err := h.Services.AddUserSSHKey(r.Context(), user.ID, req.Name, req.PublicKey)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, key)
}

func (h *Handler) DeleteMySSHKey(w http.ResponseWriter, r *http.Request) {
	user := httputil.UserFromContext(r.Context())
	keyID := chi.URLParam(r, "keyID")
	if err := h.Services.DeleteUserSSHKey(r.Context(), user.ID, keyID); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ChangeMyPassword(w http.ResponseWriter, r *http.Request) {
	user := httputil.UserFromContext(r.Context())
	var req passwordRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.ChangePassword(r.Context(), user.ID, req.Password); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
