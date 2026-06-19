package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
	"github.com/lxcfh/lxcfh/internal/models"
)

func (h *Handler) ListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.Services.ListNodes(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, nodes)
}

func (h *Handler) CreateNode(w http.ResponseWriter, r *http.Request) {
	var n models.Node
	if err := httputil.DecodeJSON(r, &n); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	created, err := h.Services.CreateNode(r.Context(), n)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) GetNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "nodeID")
	node, err := h.Services.GetNode(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, node)
}

func (h *Handler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "nodeID")
	var n models.Node
	if err := httputil.DecodeJSON(r, &n); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	updated, err := h.Services.UpdateNode(r.Context(), id, n)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "nodeID")
	if err := h.Services.DeleteNode(r.Context(), id); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PingNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "nodeID")
	result, err := h.Services.PingNode(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) TestNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "nodeID")
	result, err := h.Services.TestNode(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

type acceptHostKeyRequest struct {
	Fingerprint string `json:"fingerprint"`
}

func (h *Handler) AcceptHostKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "nodeID")
	var req acceptHostKeyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	node, err := h.Services.AcceptHostKey(r.Context(), id, req.Fingerprint)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, node)
}
