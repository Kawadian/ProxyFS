package handlers

import (
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
)

func (h *Handler) LiveHealth(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *Handler) ReadyHealth(w http.ResponseWriter, r *http.Request) {
	if err := h.Services.Ready(r.Context()); err != nil {
		httputil.WriteProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
