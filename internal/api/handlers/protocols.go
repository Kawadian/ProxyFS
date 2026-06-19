package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
	"github.com/lxcfh/lxcfh/internal/models"
)

type protocolPatchRequest struct {
	Enabled *bool `json:"enabled"`
}

func (h *Handler) GetProtocols(w http.ResponseWriter, r *http.Request) {
	settings, err := h.Services.GetSettings(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, h.Protocols.Status(settings))
}

func (h *Handler) PatchProtocol(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req protocolPatchRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if req.Enabled == nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "enabled is required")
		return
	}

	settings, err := h.Services.GetSettings(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	if settings.Protocols == (models.ProtocolSettings{}) {
		settings.Protocols = models.DefaultProtocolSettings()
	}

	switch name {
	case "sftp":
		settings.Protocols.SFTPEnabled = *req.Enabled
	case "webdav":
		settings.Protocols.WebDAVEnabled = *req.Enabled
	case "smb":
		settings.Protocols.SMBEnabled = *req.Enabled
	default:
		httputil.WriteProblem(w, r, http.StatusNotFound, "Not Found", "unknown protocol")
		return
	}

	updated, err := h.Services.UpdateSettings(r.Context(), settings)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, h.Protocols.Status(updated))
}
