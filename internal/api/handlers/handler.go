package handlers

import (
	"errors"
	"net/http"

	"github.com/lxcfh/lxcfh/internal/runtime"
	"github.com/lxcfh/lxcfh/internal/httputil"
	"github.com/lxcfh/lxcfh/internal/services"
	"github.com/lxcfh/lxcfh/internal/store"
	"github.com/lxcfh/lxcfh/internal/yamlconfig"
)

type Handler struct {
	Services  *services.Services
	YAML      *yamlconfig.Manager
	Protocols *runtime.ProtocolManager
}

func (h *Handler) mapError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		httputil.WriteProblem(w, r, http.StatusNotFound, "Not Found", err.Error())
	case errors.Is(err, store.ErrConflict):
		httputil.WriteProblem(w, r, http.StatusConflict, "Conflict", err.Error())
	case errors.Is(err, services.ErrUnauthorized):
		httputil.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", err.Error())
	case errors.Is(err, services.ErrForbidden):
		httputil.WriteProblem(w, r, http.StatusForbidden, "Forbidden", err.Error())
	case errors.Is(err, services.ErrInvalidInput):
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
	default:
		httputil.WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", err.Error())
	}
	return true
}
