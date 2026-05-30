// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — MITM proxy module.

// MITM TLS Listener (skeleton).

package mitm

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/flowork-os/flowork_Router/internal/mitm/handlers"
)

// Server is an HTTPS listener bound to a local port, using cm for per-SNI
// certs. Pass a handler that knows how to rewrite each known IDE's traffic.
type Server struct {
	addr    string
	cm      *CertManager
	handler http.Handler
	httpSrv *http.Server
}

// NewServer constructs a Server. addr is "host:port" (typically
// "127.0.0.1:8443" for development; ":443" needs CAP_NET_BIND_SERVICE or
// elevated privileges on most OS).
func NewServer(addr string, cm *CertManager, handler http.Handler) *Server {
	if handler == nil {
		handler = defaultHandler()
	}
	return &Server{addr: addr, cm: cm, handler: handler}
}

// Start blocks until the listener stops. Use Shutdown(ctx) on signal.
func (s *Server) Start() error {
	if s.cm == nil {
		return errors.New("Server.cm is nil")
	}
	cfg := &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: s.cm.GetCertificate,
	}
	s.httpSrv = &http.Server{
		Addr:              s.addr,
		Handler:           s.handler,
		TLSConfig:         cfg,
		ReadHeaderTimeout: 30 * time.Second,
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}
	tlsLn := tls.NewListener(ln, cfg)
	return s.httpSrv.Serve(tlsLn)
}

// Shutdown gracefully drains the server with the given context timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

func defaultHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Look up the per-IDE rewriter by intercepted host.
		tool := GetToolForHost(r.Host)
		if tool == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":"flow_router MITM: host not in TargetHosts","host":"` + r.Host + `"}`))
			return
		}
		h := handlers.Get(tool)
		if h == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":"flow_router MITM: no handler registered for tool","tool":"` + tool + `","host":"` + r.Host + `"}`))
			return
		}
		h.Handle(w, r)
	})
}
