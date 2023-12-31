package www

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/hostrouter"
	"github.com/go-chi/httplog/v2"
	ap "github.com/jclem/jclem.me/internal/activitypub"
	"github.com/jclem/jclem.me/internal/www/config"
)

type Server struct {
	*chi.Mux
	port string
}

const domain = "www.jclem.me"

func New() (*Server, error) {
	webRouter, err := newWebRouter()
	if err != nil {
		return nil, fmt.Errorf("error creating web router: %w", err)
	}

	pubRouter, err := newPubRouter()
	if err != nil {
		return nil, fmt.Errorf("error creating pub router: %w", err)
	}

	middleware.RequestIDHeader = "fly-request-id"

	r := chi.NewRouter()
	s := &Server{Mux: r, port: config.Port()}
	r.Use(httplog.RequestLogger(newLogger("server", config.IsProd())))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Get("/meta/healthcheck", s.healthcheck)

	if config.IsProd() {
		hr := hostrouter.New()
		hr.Map(ap.Domain, pubRouter)
		hr.Map(domain, webRouter)
		r.Mount("/", hr)
	} else {
		r.Mount("/pub", pubRouter)
		r.Mount("/", webRouter)
	}

	return s, nil
}

func (s *Server) Start() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", s.port),
		Handler:           s,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 500 * time.Millisecond,
		WriteTimeout:      5 * time.Second,
	}

	slog.Info("listening on", slog.String("port", s.port))

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

func (*Server) healthcheck(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type apiError struct {
	Code    int    `json:"code"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func returnCodeError(ctx context.Context, w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(apiError{
		Code:    code,
		Reason:  http.StatusText(code),
		Message: message,
	}); err != nil {
		oplog := httplog.LogEntry(ctx)
		oplog.ErrorContext(ctx, "error encoding error response", "error", err)
	}
}

func returnError(ctx context.Context, w http.ResponseWriter, err error, message string) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")

	oplog := httplog.LogEntry(ctx)
	oplog.ErrorContext(ctx, fmt.Sprintf("unexpected error in request handler: %s", message), "error", err)

	if err := json.NewEncoder(w).Encode(apiError{
		Code:    http.StatusInternalServerError,
		Reason:  http.StatusText(http.StatusInternalServerError),
		Message: "Internal server error",
	}); err != nil {
		oplog.ErrorContext(ctx, "error encoding error response", "error", err)
	}
}

func newLogger(name string, prodLogger bool) *httplog.Logger {
	return httplog.NewLogger(name, httplog.Options{
		JSON:            prodLogger,
		LogLevel:        slog.LevelInfo,
		Concise:         prodLogger,
		RequestHeaders:  prodLogger,
		ResponseHeaders: prodLogger,
	})
}
