package api

import (
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/services"
)

func AuthMiddleware(svc *services.Services) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(httputil.SessionCookie)
			if err != nil || cookie.Value == "" {
				httputil.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing or invalid session")
				return
			}
			sess, user, err := svc.GetSession(r.Context(), cookie.Value)
			if err != nil {
				httputil.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "session expired or invalid")
				return
			}
			ctx := httputil.WithSession(r.Context(), sess)
			ctx = httputil.WithUser(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			sess := httputil.SessionFromContext(r.Context())
			token := r.Header.Get(httputil.CSRFHeader)
			if token == "" {
				token = r.FormValue("csrf_token")
			}
			if token == "" || token != sess.CSRFToken {
				httputil.WriteProblem(w, r, http.StatusForbidden, "Forbidden", "invalid CSRF token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRoles(roles ...models.Role) func(http.Handler) http.Handler {
	allowed := map[models.Role]bool{}
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := httputil.UserFromContext(r.Context())
			if !allowed[user.Role] {
				httputil.WriteProblem(w, r, http.StatusForbidden, "Forbidden", "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func SetupMiddleware(svc *services.Services) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setup, err := svc.IsSetup(r.Context())
			if err != nil {
				httputil.WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", err.Error())
				return
			}
			if setup {
				httputil.WriteProblem(w, r, http.StatusConflict, "Conflict", "system already configured")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
