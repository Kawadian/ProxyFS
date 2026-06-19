package handlers

import (
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
)

type setupRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	User      any    `json:"user"`
	CSRFToken string `json:"csrf_token"`
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	user, sess, err := h.Services.Setup(r.Context(), req.Username, req.Password, req.DisplayName)
	if h.mapError(w, r, err) {
		return
	}
	httputil.SetSessionCookie(w, sess.ID)
	httputil.WriteJSON(w, http.StatusCreated, authResponse{User: user, CSRFToken: sess.CSRFToken})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	user, sess, err := h.Services.Login(r.Context(), req.Username, req.Password)
	if h.mapError(w, r, err) {
		return
	}
	httputil.SetSessionCookie(w, sess.ID)
	httputil.WriteJSON(w, http.StatusOK, authResponse{User: user, CSRFToken: sess.CSRFToken})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	_ = h.Services.Logout(r.Context(), httputil.SessionIDFromRequest(r))
	httputil.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user := httputil.UserFromContext(r.Context())
	sess := httputil.SessionFromContext(r.Context())
	httputil.WriteJSON(w, http.StatusOK, authResponse{User: user, CSRFToken: sess.CSRFToken})
}

func (h *Handler) Reauth(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.Services.Reauth(r.Context(), httputil.SessionIDFromRequest(r), req.Password); h.mapError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
