package webdav

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lxcfh/lxcfh/internal/auth"
	"github.com/lxcfh/lxcfh/internal/hub"
	"github.com/lxcfh/lxcfh/internal/rbac"
	"github.com/lxcfh/lxcfh/internal/vfs"
)

const MountPath = "/dav/"

// IsWebDAVRequest reports whether the HTTP request targets the WebDAV service.
func IsWebDAVRequest(r *http.Request) bool {
	switch r.Method {
	case "PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK":
		return true
	}
	if r.Header.Get("Depth") != "" {
		return true
	}
	if r.Method == http.MethodOptions && r.Header.Get("DAV") != "" {
		return true
	}
	return false
}

// NormalizeRequestPath maps the public /dav URL space to the virtual filesystem root.
func NormalizeRequestPath(p string) string {
	if p == MountPath || p == strings.TrimSuffix(MountPath, "/") {
		return "/"
	}
	if strings.HasPrefix(p, MountPath) {
		rest := strings.TrimPrefix(p, strings.TrimSuffix(MountPath, "/"))
		if rest == "" {
			return "/"
		}
		return rest
	}
	if p == "" {
		return "/"
	}
	return p
}

// Config configures the WebDAV HTTP handler.
type Config struct {
	Prefix     string
	AllowGuest bool
}

// Server serves WebDAV over HTTP.
type Server struct {
	cfg     Config
	auth    *auth.Service
	fs      *vfs.VirtualFS
	rbac    *rbac.Engine
	locks   *LockStore
	logger  *slog.Logger
	enabled atomic.Bool
}

// New creates a WebDAV server.
func New(cfg Config, authSvc *auth.Service, fs *vfs.VirtualFS, checker *rbac.Engine, locks *LockStore, logger *slog.Logger) *Server {
	if cfg.Prefix == "" {
		cfg.Prefix = MountPath
	}
	if !strings.HasSuffix(cfg.Prefix, "/") {
		cfg.Prefix += "/"
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		cfg:    cfg,
		auth:   authSvc,
		fs:     fs,
		rbac:   checker,
		locks:  locks,
		logger: logger,
	}
	s.enabled.Store(true)
	return s
}

// SetEnabled toggles WebDAV request handling without stopping HTTP.
func (s *Server) SetEnabled(enabled bool) {
	s.enabled.Store(enabled)
}

// IsEnabled reports whether WebDAV accepts requests.
func (s *Server) IsEnabled() bool {
	return s.enabled.Load()
}

// Handler returns an http.Handler mounted at the configured prefix.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.enabled.Load() {
		http.Error(w, "webdav disabled", http.StatusServiceUnavailable)
		return
	}
	reqPath := NormalizeRequestPath(r.URL.Path)
	if !strings.HasPrefix(reqPath, "/") {
		http.NotFound(w, r)
		return
	}
	user, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rel := strings.TrimPrefix(reqPath, strings.TrimSuffix(s.cfg.Prefix, "/"))
	if rel == "" {
		rel = "/"
	}
	vfsPath, err := vfs.ResolveVirtualPath(rel)
	if err != nil {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	lockToken := parseLockToken(r.Header.Get("If"))

	switch r.Method {
	case http.MethodOptions:
		s.handleOptions(w)
	case "PROPFIND":
		s.handlePropFind(w, r, user, vfsPath)
	case http.MethodGet, http.MethodHead:
		s.handleGet(w, r, user, vfsPath)
	case http.MethodPut:
		s.handlePut(w, r, user, vfsPath, lockToken)
	case http.MethodDelete:
		s.handleDelete(w, r, user, vfsPath, lockToken)
	case "MKCOL":
		s.handleMkcol(w, r, user, vfsPath, lockToken)
	case "COPY":
		s.handleCopy(w, r, user, vfsPath, lockToken)
	case "MOVE":
		s.handleMove(w, r, user, vfsPath, lockToken)
	case "LOCK":
		s.handleLock(w, r, user, vfsPath)
	case "UNLOCK":
		s.handleUnlock(w, r, user, vfsPath)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) authenticate(r *http.Request) (*auth.User, error) {
	user, pass, ok := r.BasicAuth()
	if ok && user != "" {
		return s.auth.AuthenticatePassword(r.Context(), user, pass)
	}
	token := r.Header.Get("Authorization")
	if len(token) > 7 && strings.EqualFold(token[:7], "Bearer ") {
		return s.auth.ValidateSession(r.Context(), token[7:])
	}
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	if idx := strings.Index(ip, ","); idx >= 0 {
		ip = strings.TrimSpace(ip[:idx])
	}
	if host, _, err := splitHostPort(ip); err == nil {
		ip = host
	}
	if s.cfg.AllowGuest {
		return s.auth.AuthenticateGuest(ip)
	}
	return nil, auth.ErrUnauthorized
}

func splitHostPort(addr string) (string, string, error) {
	if strings.Contains(addr, ":") {
		return splitHost(addr)
	}
	return addr, "", nil
}

func splitHost(addr string) (string, string, error) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return addr, "", nil
	}
	return addr[:i], addr[i+1:], nil
}

func (s *Server) hubUser(u *auth.User) *hub.User {
	return hub.FromRoles(u.ID, u.Username, []string{string(u.Role)}, u.Role == auth.RoleGuest)
}

func (s *Server) authorize(ctx context.Context, u *auth.User, vfsPath string, perm rbac.Permission) error {
	return s.rbac.Check(ctx, s.hubUser(u), vfsPath, perm)
}

func (s *Server) handleOptions(w http.ResponseWriter) {
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, PUT, DELETE, PROPFIND, MKCOL, COPY, MOVE, LOCK, UNLOCK")
	w.Header().Set("DAV", "1, 2")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handlePropFind(w http.ResponseWriter, r *http.Request, user *auth.User, vfsPath string) {
	if err := s.authorize(r.Context(), user, vfsPath, rbac.List); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := s.fs.Stat(r.Context(), vfsPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	entries := []vfs.FileInfo{info}
	if info.IsDir && depth(r) != "0" {
		children, err := s.fs.ReadDir(r.Context(), vfsPath)
		if err == nil {
			entries = append(entries, children...)
		}
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?><multistatus xmlns="DAV:">`)
	for _, e := range entries {
		href := s.publicPath(e.Path)
		if e.IsDir && !strings.HasSuffix(href, "/") {
			href += "/"
		}
		_, _ = fmt.Fprintf(w, `<response><href>%s</href><propstat><prop>`, xmlEscape(href))
		if e.IsDir {
			_, _ = fmt.Fprint(w, `<resourcetype><collection/></resourcetype>`)
		} else {
			_, _ = fmt.Fprint(w, `<resourcetype/>`)
		}
		_, _ = fmt.Fprintf(w, `<getcontentlength>%d</getcontentlength>`, e.Size)
		_, _ = fmt.Fprintf(w, `<getlastmodified>%s</getlastmodified>`, e.ModTime.UTC().Format(http.TimeFormat))
		_, _ = fmt.Fprint(w, `</prop><status>HTTP/1.1 200 OK</status></propstat></response>`)
	}
	_, _ = fmt.Fprint(w, `</multistatus>`)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, user *auth.User, vfsPath string) {
	if err := s.authorize(r.Context(), user, vfsPath, rbac.Read); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rc, err := s.fs.Open(r.Context(), vfsPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer rc.Close()
	info, _ := s.fs.Stat(r.Context(), vfsPath)
	if info.ModTime.IsZero() {
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	} else {
		w.Header().Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = io.Copy(w, rc)
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request, user *auth.User, vfsPath, lockToken string) {
	if err := s.locks.AssertUnlock(r.Context(), vfsPath, lockToken); err != nil {
		http.Error(w, err.Error(), http.StatusLocked)
		return
	}
	if err := s.authorize(r.Context(), user, vfsPath, rbac.Write); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if _, err := s.fs.Write(r.Context(), vfsPath, 0, r.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, user *auth.User, vfsPath, lockToken string) {
	if err := s.locks.AssertUnlock(r.Context(), vfsPath, lockToken); err != nil {
		http.Error(w, err.Error(), http.StatusLocked)
		return
	}
	if err := s.authorize(r.Context(), user, vfsPath, rbac.Delete); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.fs.Remove(r.Context(), vfsPath); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMkcol(w http.ResponseWriter, r *http.Request, user *auth.User, vfsPath, lockToken string) {
	if err := s.locks.AssertUnlock(r.Context(), vfsPath, lockToken); err != nil {
		http.Error(w, err.Error(), http.StatusLocked)
		return
	}
	if err := s.authorize(r.Context(), user, vfsPath, rbac.Write); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.fs.Mkdir(r.Context(), vfsPath); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request, user *auth.User, srcPath, lockToken string) {
	dstHeader := r.Header.Get("Destination")
	if dstHeader == "" {
		http.Error(w, "destination required", http.StatusBadRequest)
		return
	}
	dstRel := destinationToPath(dstHeader, s.cfg.Prefix)
	dstPath, err := vfs.ResolveVirtualPath(dstRel)
	if err != nil {
		http.Error(w, "bad destination", http.StatusBadRequest)
		return
	}
	if err := s.locks.AssertUnlock(r.Context(), srcPath, lockToken); err != nil {
		http.Error(w, err.Error(), http.StatusLocked)
		return
	}
	if err := s.locks.AssertUnlock(r.Context(), dstPath, ""); err != nil {
		http.Error(w, err.Error(), http.StatusLocked)
		return
	}
	if err := s.authorize(r.Context(), user, srcPath, rbac.Read); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.authorize(r.Context(), user, dstPath, rbac.Write); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := copyPath(r.Context(), s.fs, srcPath, dstPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request, user *auth.User, srcPath, lockToken string) {
	dstHeader := r.Header.Get("Destination")
	if dstHeader == "" {
		http.Error(w, "destination required", http.StatusBadRequest)
		return
	}
	dstRel := destinationToPath(dstHeader, s.cfg.Prefix)
	dstPath, err := vfs.ResolveVirtualPath(dstRel)
	if err != nil {
		http.Error(w, "bad destination", http.StatusBadRequest)
		return
	}
	if err := s.locks.AssertUnlock(r.Context(), srcPath, lockToken); err != nil {
		http.Error(w, err.Error(), http.StatusLocked)
		return
	}
	if err := s.locks.AssertUnlock(r.Context(), dstPath, ""); err != nil {
		http.Error(w, err.Error(), http.StatusLocked)
		return
	}
	if err := s.authorize(r.Context(), user, srcPath, rbac.Write); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.authorize(r.Context(), user, dstPath, rbac.Write); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := copyPath(r.Context(), s.fs, srcPath, dstPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.fs.Remove(r.Context(), srcPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleLock(w http.ResponseWriter, r *http.Request, user *auth.User, vfsPath string) {
	if err := s.authorize(r.Context(), user, vfsPath, rbac.Write); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	timeout := parseTimeout(r.Header.Get("Timeout"))
	depth := DepthZero
	if strings.Contains(strings.ToLower(r.Header.Get("Depth")), "infinity") {
		depth = DepthInf
	}
	token := r.Header.Get("If")
	var lock *Lock
	var err error
	if token != "" {
		lock, err = s.locks.Refresh(r.Context(), parseLockToken(token), timeout)
	} else {
		lock, err = s.locks.Create(r.Context(), vfsPath, user.Username, depth, timeout, true)
	}
	if err != nil {
		if errors.Is(err, ErrLocked) {
			http.Error(w, err.Error(), http.StatusLocked)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Lock-Token", "<"+lock.Token+">")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><prop xmlns="DAV:"><lockdiscovery><activelock>
<type><write/></type><depth>%s</depth><timeout>Second-%d</timeout><owner><href>%s</href></owner>
<locktoken><href>%s</href></locktoken></activelock></lockdiscovery></prop>`,
		depthLabel(depth), lock.TimeoutSecs, xmlEscape(user.Username), xmlEscape(lock.Token))
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request, user *auth.User, vfsPath string) {
	token := r.Header.Get("Lock-Token")
	if token == "" {
		token = r.Header.Get("If")
	}
	token = strings.Trim(token, "<>")
	if token == "" {
		http.Error(w, "lock token required", http.StatusBadRequest)
		return
	}
	lock, err := s.locks.Get(r.Context(), token)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if lock.Owner != user.Username && user.Role != auth.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.locks.Release(r.Context(), token); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func copyPath(ctx context.Context, fs *vfs.VirtualFS, src, dst string) error {
	info, err := fs.Stat(ctx, src)
	if err != nil {
		return err
	}
	if info.IsDir {
		_ = fs.Mkdir(ctx, dst) // ignore if already exists
		entries, err := fs.ReadDir(ctx, src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			childSrc := path.Join(src, e.Name)
			childDst := path.Join(dst, e.Name)
			if err := copyPath(ctx, fs, childSrc, childDst); err != nil {
				return err
			}
		}
		return nil
	}
	rc, err := fs.Open(ctx, src)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = fs.Write(ctx, dst, 0, rc)
	return err
}

func depth(r *http.Request) string {
	if d := r.Header.Get("Depth"); d != "" {
		return d
	}
	return "infinity"
}

func parseTimeout(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 3600
	}
	if strings.HasPrefix(strings.ToLower(v), "second-") {
		n, _ := strconv.Atoi(v[7:])
		if n > 0 {
			return n
		}
	}
	return 3600
}

func parseLockToken(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "(")
	v = strings.TrimSuffix(v, ")")
	if idx := strings.Index(v, "<"); idx >= 0 {
		end := strings.Index(v[idx:], ">")
		if end > 0 {
			return v[idx+1 : idx+end]
		}
	}
	v = strings.Trim(v, "<>")
	return v
}

func destinationToPath(dest, prefix string) string {
	dest = strings.TrimSpace(dest)
	if i := strings.Index(dest, "://"); i >= 0 {
		slash := strings.Index(dest[i+3:], "/")
		if slash >= 0 {
			dest = dest[i+3+slash:]
		} else {
			dest = "/"
		}
	}
	dest = NormalizeRequestPath("/" + strings.TrimPrefix(dest, "/"))
	trimmed := strings.TrimSuffix(prefix, "/")
	if trimmed != "" && trimmed != "/" {
		dest = strings.TrimPrefix(dest, trimmed)
	}
	return "/" + strings.TrimPrefix(dest, "/")
}

func (s *Server) publicPath(vfsPath string) string {
	if vfsPath == "/" {
		return s.cfg.Prefix
	}
	prefix := strings.TrimSuffix(s.cfg.Prefix, "/")
	if prefix == "" || prefix == "/" {
		return vfsPath
	}
	return path.Join(prefix, vfsPath)
}

func depthLabel(d LockDepth) string {
	if d == DepthInf {
		return "infinity"
	}
	return "0"
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
