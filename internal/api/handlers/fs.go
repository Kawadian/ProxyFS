package handlers

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/lxcfh/lxcfh/internal/httputil"
)

type fsPathRequest struct {
	NodeID string `json:"node_id"`
	Path   string `json:"path"`
}

type fsRenameRequest struct {
	NodeID string `json:"node_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type fsCopyMoveRequest struct {
	SourceNodeID string `json:"source_node_id"`
	SourcePath   string `json:"source_path"`
	DestNodeID   string `json:"dest_node_id"`
	DestPath     string `json:"dest_path"`
	Mode         string `json:"mode"`
}

type fsTextWriteRequest struct {
	NodeID  string `json:"node_id"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (h *Handler) FSList(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	path := r.URL.Query().Get("path")
	entries, err := h.Services.ListDir(r.Context(), nodeID, path)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, entries)
}

func (h *Handler) FSStat(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	path := r.URL.Query().Get("path")
	stat, err := h.Services.StatPath(r.Context(), nodeID, path)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, stat)
}

func (h *Handler) FSDownload(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	path := r.URL.Query().Get("path")
	f, stat, err := h.Services.DownloadFile(r.Context(), nodeID, path)
	if h.mapError(w, r, err) {
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(stat.Path)+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size, 10))
	_, _ = io.Copy(w, f)
}

func (h *Handler) FSMkdir(w http.ResponseWriter, r *http.Request) {
	var req fsPathRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.Mkdir(r.Context(), req.NodeID, req.Path); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) FSRename(w http.ResponseWriter, r *http.Request) {
	var req fsRenameRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.Rename(r.Context(), req.NodeID, req.From, req.To); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) FSCopyMove(w http.ResponseWriter, r *http.Request) {
	var req fsCopyMoveRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.CopyMovePath(r.Context(), req.SourceNodeID, req.SourcePath, req.DestNodeID, req.DestPath, req.Mode); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) FSDelete(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	path := r.URL.Query().Get("path")
	if err := h.Services.DeletePath(r.Context(), nodeID, path); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) FSReadText(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	path := r.URL.Query().Get("path")
	content, err := h.Services.ReadText(r.Context(), nodeID, path)
	if h.mapError(w, r, err) {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"content": content})
}

func (h *Handler) FSWriteText(w http.ResponseWriter, r *http.Request) {
	var req fsTextWriteRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.WriteText(r.Context(), req.NodeID, req.Path, req.Content); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
