package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lxcfh/lxcfh/internal/api"
	"github.com/lxcfh/lxcfh/internal/auth"
	"github.com/lxcfh/lxcfh/internal/config"
	"github.com/lxcfh/lxcfh/internal/health"
	"github.com/lxcfh/lxcfh/internal/hub"
	"github.com/lxcfh/lxcfh/internal/protocols/sftpserver"
	"github.com/lxcfh/lxcfh/internal/protocols/webdav"
	"github.com/lxcfh/lxcfh/internal/rbac"
	"github.com/lxcfh/lxcfh/internal/services"
	"github.com/lxcfh/lxcfh/internal/store"
	"github.com/lxcfh/lxcfh/internal/transfer"
	"github.com/lxcfh/lxcfh/internal/vfs"
)

type runtimeApp struct {
	cfg      *config.Config
	store    *store.Store
	services *services.Services
	vfs      *vfs.VirtualFS
	vfsMgr   *hub.VFSManager
	api      *api.Server
	sftp     *sftpserver.Server
	webdav   *webdav.Server
	transfer *transfer.Worker
	health   *health.Checker
	logger   *slog.Logger
}

func newRuntimeApp(cfg *config.Config) (*runtimeApp, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	vfsFS := vfs.New("/", cfg.DirMetadataTTL, cfg.StatTTL)
	vfsMgr := hub.NewVFSManager(vfsFS, st, cfg.MasterKey, logger)

	svc := services.New(st, cfg.DataDir)
	svc.VFS = vfsFS
	svc.VFSManager = vfsMgr
	svc.MasterKey = cfg.MasterKey
	svc.SMBEnabled = cfg.SMBEnabled
	svc.OnNodesChanged = func(ctx context.Context) {
		if err := vfsMgr.Refresh(ctx); err != nil {
			logger.Warn("vfs refresh failed", "error", err)
		}
	}

	if err := vfsMgr.Refresh(context.Background()); err != nil {
		logger.Warn("initial vfs refresh", "error", err)
	}

	authSvc := auth.NewService(st.DB(), cfg.SessionTTL)
	_ = authSvc.LoadGuestIPs(context.Background())

	rbacEngine := rbac.NewEngine()
	rbacEngine.SetRules([]rbac.Rule{
		{PathPrefix: "/", Roles: []string{"admin"}, Permissions: []rbac.Permission{rbac.Read, rbac.Write, rbac.Delete, rbac.Execute, rbac.List}},
		{PathPrefix: "/", Roles: []string{"editor", "operator"}, Permissions: []rbac.Permission{rbac.Read, rbac.Write, rbac.Delete, rbac.List}},
		{PathPrefix: "/", Roles: []string{"viewer"}, Permissions: []rbac.Permission{rbac.Read, rbac.List}},
		{PathPrefix: "/", Roles: []string{"guest"}, Permissions: []rbac.Permission{rbac.Read, rbac.List}},
	})

	apiSrv := api.NewServer(svc)

	sftpPort := cfg.SFTPPort
	if sftpPort == 0 {
		sftpPort = 2022
	}
	sftpSrv, err := sftpserver.New(sftpserver.Config{
		Addr:       fmt.Sprintf(":%d", sftpPort),
		AllowGuest: false,
	}, authSvc, vfsFS, rbacEngine, st.DB(), logger)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("sftp: %w", err)
	}

	lockStore := webdav.NewLockStore(st.DB())
	webdavSrv := webdav.New(webdav.Config{Prefix: "/dav/", AllowGuest: false}, authSvc, vfsFS, rbacEngine, lockStore, logger)

	transferWorker := transfer.NewWorker(st.DB(), vfsFS, cfg.TransferChunkSize, cfg.TransferWorkers, logger)
	healthChecker := health.NewChecker(st.DB(), cfg.HealthTTL)

	return &runtimeApp{
		cfg:      cfg,
		store:    st,
		services: svc,
		vfs:      vfsFS,
		vfsMgr:   vfsMgr,
		api:      apiSrv,
		sftp:     sftpSrv,
		webdav:   webdavSrv,
		transfer: transferWorker,
		health:   healthChecker,
		logger:   logger,
	}, nil
}

func (a *runtimeApp) handler() http.Handler {
	apiHandler := a.api.Handler()
	davHandler := a.webdav.Handler()
	static := staticHandler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("ok"))
		case strings.HasPrefix(r.URL.Path, "/health/"):
			apiHandler.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/"):
			apiHandler.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/dav/"):
			davHandler.ServeHTTP(w, r)
		default:
			static.ServeHTTP(w, r)
		}
	})
}

func (a *runtimeApp) run(ctx context.Context) error {
	if err := a.sftp.Start(); err != nil {
		return fmt.Errorf("start sftp: %w", err)
	}
	a.transfer.Start(ctx)

	addr := a.cfg.WebAddr()
	srv := &http.Server{
		Addr:              addr,
		Handler:           a.handler(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("hub listening", "http", addr, "sftp", a.sftp)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		a.logger.Info("shutdown", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	_ = a.sftp.Stop()
	a.transfer.Stop()
	a.vfsMgr.Close()
	return a.store.Close()
}
