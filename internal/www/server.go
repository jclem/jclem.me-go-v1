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
	ap "github.com/jclem/jclem.me/internal/activitypub"
	"github.com/jclem/jclem.me/internal/pages"
	"github.com/jclem/jclem.me/internal/posts"
	"github.com/jclem/jclem.me/internal/www/config"
	"github.com/jclem/jclem.me/internal/www/view"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/frontmatter"
)

type Server struct {
	md    goldmark.Markdown
	pages *pages.Service
	posts *posts.Service
	view  *view.Service
	pub   *ap.Service
}

const domain = "www.jclem.me"

func New() (*Server, error) {
	pagesSvc := pages.New()
	if err := pagesSvc.Start(); err != nil {
		return nil, fmt.Errorf("error starting pages service: %w", err)
	}

	postsSvc := posts.New()
	if err := postsSvc.Start(); err != nil {
		return nil, fmt.Errorf("error starting posts service: %w", err)
	}

	viewSvc, err := view.New(pagesSvc, postsSvc)
	if err != nil {
		return nil, fmt.Errorf("error creating view service: %w", err)
	}

	gm := goldmark.New(
		goldmark.WithExtensions(
			extension.NewFootnote(),
			extension.NewTypographer(),
			extension.NewLinkify(),
			&frontmatter.Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	pub, err := ap.NewService(context.Background(), config.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("error creating activitypub service: %w", err)
	}

	return &Server{
		pages: pagesSvc,
		posts: postsSvc,
		view:  viewSvc,
		md:    gm,
		pub:   pub,
	}, nil
}

func (s *Server) Start() error {
	middleware.RequestIDHeader = "fly-request-id"

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Get("/meta/healthcheck", s.healthcheck)

	if config.IsProd() {
		hr := hostrouter.New()
		hr.Map(ap.Domain, newPubRouter(s.pub))
		hr.Map(domain, newWebRouter(s.md, s.pages, s.posts, s.view))
		r.Mount("/", hr)
	} else {
		r.Mount("/pub", newPubRouter(s.pub))
		r.Mount("/", newWebRouter(s.md, s.pages, s.posts, s.view))
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", config.Port()),
		Handler:           r,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 500 * time.Millisecond,
		WriteTimeout:      5 * time.Second,
	}

	slog.Info("listening on", slog.String("port", config.Port()))

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
		slog.ErrorContext(ctx, "error encoding error response", "error", err)
	}
}

func returnError(ctx context.Context, w http.ResponseWriter, err error, message string) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")

	slog.ErrorContext(ctx, fmt.Sprintf("unexpected error in request handler: %s", message), "error", err)

	if err := json.NewEncoder(w).Encode(apiError{
		Code:    http.StatusInternalServerError,
		Reason:  http.StatusText(http.StatusInternalServerError),
		Message: "Internal server error",
	}); err != nil {
		slog.ErrorContext(ctx, "error encoding error response", "error", err)
	}
}
