package handlers

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
)

type uploadKeyRequest struct {
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	Comment    string `json:"comment"`
}

type generateKeyRequest struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

type rotateKeyRequest struct {
	Name string `json:"name"`
}

func (h *Handler) ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.Services.ListSSHKeys(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *Handler) UploadKey(w http.ResponseWriter, r *http.Request) {
	var req uploadKeyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	key, err := h.Services.UploadSSHKey(r.Context(), req.Name, req.PrivateKey, req.Comment)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, key)
}

func (h *Handler) GenerateKey(w http.ResponseWriter, r *http.Request) {
	var req generateKeyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	key, err := h.Services.GenerateSSHKey(r.Context(), req.Name, req.Comment)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, key)
}

func (h *Handler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "keyID")
	if err := h.Services.DeleteSSHKey(r.Context(), id); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DownloadPrivateKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "keyID")
	key, priv, err := h.Services.GetSSHKeyPrivate(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+key.Name+".pem\"")
	_, _ = io.WriteString(w, priv)
}

func (h *Handler) RotateKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "keyID")
	var req rotateKeyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	key, err := h.Services.RotateSSHKey(r.Context(), id, req.Name)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, key)
}
