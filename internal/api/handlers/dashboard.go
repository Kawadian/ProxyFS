package handlers

import (
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
)

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	dash, err := h.Services.Dashboard(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dash)
}
