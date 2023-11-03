package www

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jclem/jclem.me/internal/pages"
	"github.com/jclem/jclem.me/internal/posts"
	"github.com/jclem/jclem.me/internal/www/view"
)

type Server struct {
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

	return &Server{
		posts: postsSvc,
		view:  viewSvc,
	}, nil
}

func (s *Server) Start() error {
	router := chi.NewRouter()
	router.Get("/meta/healthcheck", s.healthcheck())
	router.Get("/", s.renderPage("about"))
	router.Get("/writing", s.listPosts())
	router.Get("/writing/{slug}", s.showPost())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           router,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 500 * time.Millisecond,
		WriteTimeout:      5 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

func (s *Server) renderPage(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if err := s.view.RenderPage(w, name); err != nil {
			http.Error(w, fmt.Sprintf("error rendering page: %s", err), http.StatusInternalServerError)

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
		posts, err := s.posts.List()
		if err != nil {
			http.Error(w, fmt.Sprintf("error listing posts: %s", err), http.StatusInternalServerError)

			return
		}

		if err := s.view.RenderTemplate(w, "writing/index", listPostsData{Posts: posts},
			view.WithTitle("Writing Archive"),
			view.WithDescription("A collection of articles and blog posts by Jonathan Clem"),
			view.WithLayout("writing/layout/index"),
		); err != nil {
			http.Error(w, fmt.Sprintf("error rendering page: %s", err), http.StatusInternalServerError)

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
				http.Error(w, fmt.Sprintf("error getting post: %s", err), http.StatusNotFound)

				return
			}

			http.Error(w, fmt.Sprintf("error getting post: %s", err), http.StatusInternalServerError)

			return
		}

		if err := s.view.RenderTemplate(w, "writing/show", post,
			view.WithTitle(post.Title),
			view.WithDescription(post.Summary),
			view.WithLayout("writing/layout/show")); err != nil {
			http.Error(w, fmt.Sprintf("error rendering page: %s", err), http.StatusInternalServerError)

			return
		}
	}
}

func (*Server) healthcheck() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}
