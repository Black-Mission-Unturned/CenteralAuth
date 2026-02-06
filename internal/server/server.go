package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/internal/client"
	"github.com/BlackMission/centralauth/internal/exchange"
	"github.com/BlackMission/centralauth/internal/handler"
	"github.com/BlackMission/centralauth/internal/state"
)

// Config holds the server configuration.
type Config struct {
	Host string
	Port int
}

// Deps holds the service dependencies.
type Deps struct {
	Clients   *client.Registry
	Providers *auth.Registry
	State     *state.Service
	Exchange  *exchange.Codec
}

// Server wraps the HTTP server and router.
type Server struct {
	httpServer *http.Server
	handler    http.Handler
}

// New creates a new Server with all routes wired.
func New(cfg Config, deps Deps) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.Health())
	mux.HandleFunc("GET /auth/{provider}", handler.Authorize(deps.Clients, deps.Providers, deps.State))
	mux.HandleFunc("GET /callback/{provider}", handler.Callback(deps.Providers, deps.State, deps.Exchange))
	mux.HandleFunc("GET /exchange", handler.Exchange(deps.Clients, deps.Exchange))
	mux.HandleFunc("GET /providers", handler.Providers(deps.Providers))

	logged := loggingMiddleware(mux)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return &Server{
		handler: logged,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      logged,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Handler returns the server's HTTP handler (for testing).
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Start begins listening and serving.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.httpServer.Addr, err)
	}
	log.Printf("CentralAuth listening on %s", s.httpServer.Addr)
	return s.httpServer.Serve(ln)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
