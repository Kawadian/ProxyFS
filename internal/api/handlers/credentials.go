package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
)

type credentialRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Username string `json:"username"`
	Secret   string `json:"secret"`
}

func (h *Handler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := h.Services.ListCredentials(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, creds)
}

func (h *Handler) CreateCredential(w http.ResponseWriter, r *http.Request) {
	var req credentialRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	cred, err := h.Services.CreateCredential(r.Context(), req.Name, req.Type, req.Username, req.Secret)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, cred)
}

func (h *Handler) GetCredential(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "credentialID")
	cred, err := h.Services.GetCredential(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, cred)
}

func (h *Handler) UpdateCredential(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "credentialID")
	var req credentialRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	cred, err := h.Services.UpdateCredential(r.Context(), id, req.Name, req.Type, req.Username, req.Secret)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, cred)
}

func (h *Handler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "credentialID")
	if err := h.Services.DeleteCredential(r.Context(), id); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestCredential(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "credentialID")
	result, err := h.Services.TestCredential(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}
