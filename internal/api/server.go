package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/lxcfh/lxcfh/internal/api/handlers"
	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/runtime"
	"github.com/lxcfh/lxcfh/internal/services"
	"github.com/lxcfh/lxcfh/internal/yamlconfig"
)

type Server struct {
	svc       *services.Services
	h         *handlers.Handler
	protocols *runtime.ProtocolManager
	router    chi.Router
}

func NewServer(svc *services.Services, protocols *runtime.ProtocolManager) *Server {
	h := &handlers.Handler{
		Services:  svc,
		YAML:      yamlconfig.NewManager(svc),
		Protocols: protocols,
	}
	s := &Server{svc: svc, h: h, protocols: protocols}
	s.router = s.buildRouter()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	settings, _ := s.svc.GetSettings(context.Background())
	limit := settings.RateLimitPerMinute
	if limit <= 0 {
		limit = 120
	}
	r.Use(httprate.LimitByIP(limit, 60))

	r.Get("/health/live", s.h.LiveHealth)
	r.Get("/health/ready", s.h.ReadyHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.With(SetupMiddleware(s.svc)).Post("/auth/setup", s.h.Setup)
		r.Post("/auth/login", s.h.Login)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(s.svc))
			r.Use(CSRFMiddleware())

			r.Post("/auth/logout", s.h.Logout)
			r.Get("/auth/me", s.h.Me)
			r.Post("/auth/reauth", s.h.Reauth)

			r.Route("/me", func(r chi.Router) {
				r.Put("/password", s.h.ChangeMyPassword)
				r.Route("/ssh-keys", func(r chi.Router) {
					r.Get("/", s.h.ListMySSHKeys)
					r.Post("/", s.h.AddMySSHKey)
					r.Delete("/{keyID}", s.h.DeleteMySSHKey)
				})
			})

			r.Route("/nodes", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/", s.h.ListNodes)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/", s.h.CreateNode)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/{nodeID}", s.h.GetNode)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Patch("/{nodeID}", s.h.UpdateNode)
				r.With(RequireRoles(models.RoleAdmin)).Delete("/{nodeID}", s.h.DeleteNode)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Post("/{nodeID}/ping", s.h.PingNode)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/{nodeID}/test", s.h.TestNode)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/{nodeID}/accept-host-key", s.h.AcceptHostKey)
			})

			r.Route("/credentials", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Get("/", s.h.ListCredentials)
				r.With(RequireRoles(models.RoleAdmin)).Post("/", s.h.CreateCredential)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Get("/{credentialID}", s.h.GetCredential)
				r.With(RequireRoles(models.RoleAdmin)).Patch("/{credentialID}", s.h.UpdateCredential)
				r.With(RequireRoles(models.RoleAdmin)).Delete("/{credentialID}", s.h.DeleteCredential)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/{credentialID}/test", s.h.TestCredential)
			})

			r.Route("/keys", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Get("/", s.h.ListKeys)
				r.With(RequireRoles(models.RoleAdmin)).Post("/upload", s.h.UploadKey)
				r.With(RequireRoles(models.RoleAdmin)).Post("/generate", s.h.GenerateKey)
				r.With(RequireRoles(models.RoleAdmin)).Delete("/{keyID}", s.h.DeleteKey)
				r.With(RequireRoles(models.RoleAdmin)).Get("/{keyID}/download-private", s.h.DownloadPrivateKey)
				r.With(RequireRoles(models.RoleAdmin)).Post("/{keyID}/rotate", s.h.RotateKey)
			})

			r.Route("/users", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin)).Get("/", s.h.ListUsers)
				r.With(RequireRoles(models.RoleAdmin)).Post("/", s.h.CreateUser)
				r.With(RequireRoles(models.RoleAdmin)).Get("/{userID}", s.h.GetUser)
				r.With(RequireRoles(models.RoleAdmin)).Patch("/{userID}", s.h.UpdateUser)
				r.With(RequireRoles(models.RoleAdmin)).Delete("/{userID}", s.h.DeleteUser)
				r.With(RequireRoles(models.RoleAdmin)).Put("/{userID}/password", s.h.ChangePassword)
				r.Route("/{userID}/ssh-keys", func(r chi.Router) {
					r.With(RequireRoles(models.RoleAdmin)).Get("/", s.h.ListUserSSHKeys)
					r.With(RequireRoles(models.RoleAdmin)).Post("/", s.h.AddUserSSHKey)
					r.With(RequireRoles(models.RoleAdmin)).Delete("/{keyID}", s.h.DeleteUserSSHKey)
				})
			})

			r.Route("/fs", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/list", s.h.FSList)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/stat", s.h.FSStat)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/download", s.h.FSDownload)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/mkdir", s.h.FSMkdir)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/rename", s.h.FSRename)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/copy-move", s.h.FSCopyMove)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Delete("/delete", s.h.FSDelete)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/text", s.h.FSReadText)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Put("/text", s.h.FSWriteText)
			})

			r.Route("/transfers", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/", s.h.ListTransfers)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/", s.h.CreateTransfer)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/{transferID}", s.h.GetTransfer)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Delete("/{transferID}", s.h.DeleteTransfer)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/{transferID}/pause", s.h.PauseTransfer)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/{transferID}/resume", s.h.ResumeTransfer)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/{transferID}/cancel", s.h.CancelTransfer)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/{transferID}/retry", s.h.RetryTransfer)
			})

			r.Route("/uploads", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Post("/", s.h.CreateUpload)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Head("/{uploadID}", s.h.HeadUpload)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Patch("/{uploadID}", s.h.PatchUpload)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Get("/{uploadID}", s.h.GetUpload)
				r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator)).Delete("/{uploadID}", s.h.DeleteUpload)
			})

			r.Route("/config", func(r chi.Router) {
				r.With(RequireRoles(models.RoleAdmin)).Get("/export", s.h.ConfigExport)
				r.With(RequireRoles(models.RoleAdmin)).Post("/validate", s.h.ConfigValidate)
				r.With(RequireRoles(models.RoleAdmin)).Post("/preview", s.h.ConfigPreview)
				r.With(RequireRoles(models.RoleAdmin)).Post("/apply", s.h.ConfigApply)
			})

			r.With(RequireRoles(models.RoleAdmin)).Get("/settings", s.h.GetSettings)
			r.With(RequireRoles(models.RoleAdmin)).Patch("/settings", s.h.PatchSettings)

			r.With(RequireRoles(models.RoleAdmin)).Get("/protocols", s.h.GetProtocols)
			r.With(RequireRoles(models.RoleAdmin)).Patch("/protocols/{name}", s.h.PatchProtocol)

			r.With(RequireRoles(models.RoleAdmin)).Post("/backup", s.h.Backup)
			r.With(RequireRoles(models.RoleAdmin)).Post("/backup/restore", s.h.RestoreBackup)

			r.With(RequireRoles(models.RoleAdmin, models.RoleEditor, models.RoleOperator, models.RoleViewer)).Get("/dashboard", s.h.Dashboard)
		})
	})

	return r
}
