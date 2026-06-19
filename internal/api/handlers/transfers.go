package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
	"github.com/lxcfh/lxcfh/internal/models"
)

func (h *Handler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	transfers, err := h.Services.ListTransfers(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, transfers)
}

func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var t models.Transfer
	if err := httputil.DecodeJSON(r, &t); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	created, err := h.Services.CreateTransfer(r.Context(), t)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "transferID")
	t, err := h.Services.GetTransfer(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) DeleteTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "transferID")
	if err := h.Services.DeleteTransfer(r.Context(), id); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PauseTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "transferID")
	t, err := h.Services.PauseTransfer(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) ResumeTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "transferID")
	t, err := h.Services.ResumeTransfer(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) CancelTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "transferID")
	t, err := h.Services.CancelTransfer(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) RetryTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "transferID")
	t, err := h.Services.RetryTransfer(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, t)
}
