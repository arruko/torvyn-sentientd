package httpserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/arruko/torvyn-sentientd/internal/config"
	"github.com/arruko/torvyn-sentientd/internal/correlation"
	"github.com/arruko/torvyn-sentientd/internal/logging"
	"github.com/arruko/torvyn-sentientd/internal/queue"
)

type Server struct {
	cfg        config.Config
	correlator correlation.Correlator
	queue      queue.Queue // optional, nil if NATS disabled
	httpServer *http.Server
}

func New(cfg config.Config, correlator correlation.Correlator) *Server {
	return &Server{cfg: cfg, correlator: correlator, queue: nil}
}

// SetQueue sets the optional message queue for async alert processing.
// If set, alerts will be published to the queue in addition to sync processing.
func (s *Server) SetQueue(q queue.Queue) {
	s.queue = q
}

func (s *Server) router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	r.Post(s.cfg.AlertPath, s.handleAlertmanagerWebhook)

	return r
}

func (s *Server) ListenAndServe() error {
	s.httpServer = &http.Server{
		Addr:    s.cfg.ListenAddr,
		Handler: s.router(),
	}
	logging.Info.Printf("Starting HTTP server on %s", s.cfg.ListenAddr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	logging.Info.Println("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}
