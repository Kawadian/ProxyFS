package handlers

import (
	"io"
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
)

func (h *Handler) Backup(w http.ResponseWriter, r *http.Request) {
	result, err := h.Services.Backup(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.Restore(r.Context(), data); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
