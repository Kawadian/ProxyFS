package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lxcfh/lxcfh/internal/httputil"
)

func (h *Handler) CreateUpload(w http.ResponseWriter, r *http.Request) {
	nodeID := r.Header.Get("Upload-Node-ID")
	if nodeID == "" {
		nodeID = r.Header.Get("X-Node-ID")
	}
	path := r.Header.Get("Upload-Path")
	if path == "" {
		path = r.Header.Get("X-Path")
	}
	length, _ := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if length <= 0 {
		length, _ = strconv.ParseInt(r.Header.Get("X-Length"), 10, 64)
	}
	upload, err := h.Services.CreateUpload(r.Context(), nodeID, path, length)
	if h.mapError(w, r, err) {
		return
	}
	w.Header().Set("Location", "/api/v1/uploads/"+upload.ID)
	w.Header().Set("Tus-Resumable", "1.0.0")
	w.Header().Set("Upload-Offset", "0")
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) HeadUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "uploadID")
	upload, err := h.Services.GetUpload(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	w.Header().Set("Tus-Resumable", "1.0.0")
	w.Header().Set("Upload-Offset", strconv.FormatInt(upload.Offset, 10))
	w.Header().Set("Upload-Length", strconv.FormatInt(upload.Size, 10))
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) PatchUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "uploadID")
	offset, _ := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	upload, err := h.Services.PatchUpload(r.Context(), id, offset, body)
	if h.mapError(w, r, err) {
		return
	}
	w.Header().Set("Tus-Resumable", "1.0.0")
	w.Header().Set("Upload-Offset", strconv.FormatInt(upload.Offset, 10))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "uploadID")
	upload, err := h.Services.GetUpload(r.Context(), id)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, upload)
}

func (h *Handler) DeleteUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "uploadID")
	if err := h.Services.DeleteUpload(r.Context(), id); h.mapError(w, r, err) {
		return
	}
	w.Header().Set("Tus-Resumable", "1.0.0")
	w.WriteHeader(http.StatusNoContent)
}
