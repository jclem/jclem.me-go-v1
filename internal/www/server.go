package www

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
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
}

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

	return &Server{
		pages: pagesSvc,
		posts: postsSvc,
		view:  viewSvc,
		md:    gm,
	}, nil
}

func (s *Server) Start() error {
	router := chi.NewRouter()
	router.Use(httplog.RequestLogger(httplog.NewLogger("www")))
	router.Get("/meta/healthcheck", s.healthcheck())
	router.Get("/", s.renderHome())
	router.Get("/writing", s.listPosts())
	router.Get("/writing/{slug}", s.showPost())
	router.Get("/sitemap.xml", s.sitemap())
	router.Get("/rss.xml", s.rss())
	router.Handle("/public/*", http.StripPrefix("/public/", http.FileServer(http.Dir("internal/www/public"))))

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", config.Port()),
		Handler:           router,
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

func (s *Server) renderHome() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		page, err := s.pages.Get("about")
		if err != nil {
			returnError(w, err, "error getting page")

			return
		}

		if err := s.view.RenderHTML(w, "home", struct{ Content template.HTML }{Content: page.Content},
			view.WithTitle(page.Title),
			view.WithDescription(page.Description),
		); err != nil {
			returnError(w, err, "error rendering page")

			return
		}
	}
}

type listPostsData struct {
	Title       string
	Description string
	Posts       []posts.Post
}

func (s *Server) listPosts() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		posts := s.posts.List()

		if err := s.view.RenderHTML(w, "writing/index", listPostsData{Posts: posts},
			view.WithTitle("Writing Archive"),
			view.WithDescription("A collection of articles and blog posts by Jonathan Clem"),
			view.WithLayout("writing/layout/index"),
		); err != nil {
			returnError(w, err, "error rendering page")

			return
		}
	}
}

func (s *Server) showPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")

		post, err := s.posts.Get(slug)
		if err != nil {
			if errors.As(err, &posts.PostNotFoundError{}) {
				returnCodeError(w, http.StatusNotFound, fmt.Sprintf("post not found: %s", slug))

				return
			}

			returnError(w, err, "error getting post")

			return
		}

		if err := s.view.RenderHTML(w, "writing/show", post,
			view.WithTitle(post.Title),
			view.WithDescription(post.Summary),
			view.WithLayout("writing/layout/show")); err != nil {
			returnError(w, err, "error rendering page")

			return
		}
	}
}

func (*Server) healthcheck() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) sitemap() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		posts := s.posts.List()

		w.Header().Set("Content-Type", "application/xml")

		if err := s.view.RenderXML(w, "sitemap.xml", posts); err != nil {
			returnError(w, err, "error rendering sitemap")

			return
		}
	}
}

type rssData struct {
	BuildDate     string
	CopyrightYear string
	Posts         []posts.Post
}

func (s *Server) rss() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		posts := s.posts.List()
		now := time.Now()

		w.Header().Set("Content-Type", "application/xml")

		if err := s.view.RenderXML(w, "rss.xml", rssData{
			BuildDate:     now.UTC().Format(http.TimeFormat),
			CopyrightYear: strconv.Itoa(now.Year() - 1),
			Posts:         posts,
		}); err != nil {
			returnError(w, err, "error rendering rss")

			return
		}
	}
}

type apiError struct {
	Code    int    `json:"code"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func returnCodeError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(apiError{ //nolint:errchkjson
		Code:    code,
		Reason:  http.StatusText(code),
		Message: message,
	})
}

func returnError(w http.ResponseWriter, err error, message string) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(apiError{ //nolint:errchkjson
		Code:    http.StatusInternalServerError,
		Reason:  http.StatusText(http.StatusInternalServerError),
		Message: fmt.Errorf("%s: %w", message, err).Error(),
	})
}
