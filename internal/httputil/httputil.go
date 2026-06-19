package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lxcfh/lxcfh/internal/models"
)

const (
	SessionCookie = "lxcfh_session"
	CSRFHeader    = "X-CSRF-Token"
)

type contextKey string

const (
	ctxUserKey    contextKey = "user"
	ctxSessionKey contextKey = "session"
)

const ProblemContentType = "application/problem+json"

type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", ProblemContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Type:     problemType(status),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	})
}

func problemType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "https://lxcfh.dev/problems/bad-request"
	case http.StatusUnauthorized:
		return "https://lxcfh.dev/problems/unauthorized"
	case http.StatusForbidden:
		return "https://lxcfh.dev/problems/forbidden"
	case http.StatusNotFound:
		return "https://lxcfh.dev/problems/not-found"
	case http.StatusConflict:
		return "https://lxcfh.dev/problems/conflict"
	case http.StatusTooManyRequests:
		return "https://lxcfh.dev/problems/rate-limited"
	case http.StatusInternalServerError:
		return "https://lxcfh.dev/problems/internal-error"
	default:
		return "https://lxcfh.dev/problems/error"
	}
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func DecodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func WithUser(ctx context.Context, user models.User) context.Context {
	return context.WithValue(ctx, ctxUserKey, user)
}

func WithSession(ctx context.Context, sess models.Session) context.Context {
	return context.WithValue(ctx, ctxSessionKey, sess)
}

func UserFromContext(ctx context.Context) models.User {
	return ctx.Value(ctxUserKey).(models.User)
}

func SessionFromContext(ctx context.Context) models.Session {
	return ctx.Value(ctxSessionKey).(models.Session)
}

func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func SessionIDFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
