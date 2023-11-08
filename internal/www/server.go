package www

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
	"github.com/jclem/jclem.me/internal/dispatches"
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
	miss  *dispatches.Service
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

	missSvc, err := dispatches.New()
	if err != nil {
		return nil, fmt.Errorf("error creating dispatches service: %w", err)
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
		miss:  missSvc,
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
	router.Get("/dispatches", s.listDispatches())
	router.Get("/sitemap.xml", s.sitemap())
	router.Get("/rss.xml", s.rss())
	router.Route("/api/dispatches", s.dispatchesRouter())
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

func (s *Server) listDispatches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dispatches, err := s.miss.ListDispatches(r.Context())
		if err != nil {
			returnError(w, err, "error listing dispatches")

			return
		}

		type dispatchItem struct {
			URL        string
			Alt        string
			Content    template.HTML
			InsertedAt time.Time
		}

		dispItems := make([]dispatchItem, 0, len(dispatches))

		for _, disp := range dispatches {
			var buf bytes.Buffer
			if err := s.md.Convert([]byte(disp.Data["content"]), &buf); err != nil {
				returnError(w, err, "error converting markdown")
			}

			dispItems = append(dispItems, dispatchItem{
				URL:        disp.Data["url"],
				Alt:        disp.Data["alt"],
				Content:    template.HTML(buf.String()), //nolint:gosec
				InsertedAt: disp.InsertedAt,
			})
		}

		if err := s.view.RenderHTML(w, "dispatches/index", dispItems,
			view.WithTitle("Dispatches"),
			view.WithDescription("A collection of dispatches by Jonathan Clem"),
			view.WithLayout("dispatches/layout/index"),
		); err != nil {
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

func (s *Server) dispatchesRouter() func(r chi.Router) {
	return func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(ensureAuthorized)
			r.Post("/", s.apiCreateDispatch())
			r.Get("/", s.apiListDispatches())
		})
	}
}

func (s *Server) apiCreateDispatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(8 << 20) // 8 MB
		if err != nil {
			returnCodeError(w, http.StatusBadRequest, fmt.Sprintf("error parsing form: %s", err))

			return
		}

		// Get the image file from the form
		file, header, err := r.FormFile("image")
		if err != nil {
			returnCodeError(w, http.StatusBadRequest, fmt.Sprintf("error getting image file: %s", err))

			return
		}
		defer file.Close()

		// Get the additional fields from the form
		typeField := r.FormValue("type")
		altField := r.FormValue("alt")
		contentField := r.FormValue("content")

		if typeField != "image" {
			returnCodeError(w, http.StatusBadRequest, fmt.Sprintf("invalid type: %s", typeField))

			return
		}

		ext := filepath.Ext(header.Filename)
		filename := time.Now().UTC().Format("20060102T150405Z") + strings.ToLower(ext)

		// Upload the image file to S3
		disp, err := s.miss.CreateDispatch(r.Context(), contentField, altField, filename, file)
		if err != nil {
			returnError(w, err, "error creating dispatch")

			return
		}

		dispJSON, err := json.Marshal(disp)
		if err != nil {
			returnError(w, err, "error marshaling dispatch")

			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(dispJSON)
	}
}

func (s *Server) apiListDispatches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dispatches, err := s.miss.ListDispatches(r.Context())
		if err != nil {
			returnError(w, err, "error listing dispatches")

			return
		}

		dispJSON, err := json.Marshal(dispatches)
		if err != nil {
			returnError(w, err, "error marshaling dispatches")

			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(dispJSON)
	}
}

type apiError struct {
	Code    int    `json:"code"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func ensureAuthorized(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if subtle.ConstantTimeCompare([]byte(token), []byte(config.APIKey())) != 1 {
			returnCodeError(w, http.StatusUnauthorized, "invalid or missing API key")

			return
		}

		next.ServeHTTP(w, r)
	})
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
