package handlers

import (
	"io"
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
)

func (h *Handler) ConfigExport(w http.ResponseWriter, r *http.Request) {
	data, err := h.YAML.Export(r.Context())
	if h.mapError(w, r, err) {
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=\"lxcfh-config.yaml\"")
	_, _ = w.Write(data)
}

func (h *Handler) ConfigValidate(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	result, err := h.YAML.Validate(r.Context(), data)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) ConfigPreview(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	preview, err := h.YAML.Preview(r.Context(), data)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, preview)
}

func (h *Handler) ConfigApply(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.YAML.Apply(r.Context(), data); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
