package handlers

import (
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
	"github.com/lxcfh/lxcfh/internal/models"
)

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.Services.GetSettings(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, settings)
}

func (h *Handler) PatchSettings(w http.ResponseWriter, r *http.Request) {
	var patch models.Settings
	if err := httputil.DecodeJSON(r, &patch); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	settings, err := h.Services.UpdateSettings(r.Context(), patch)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, settings)
}
