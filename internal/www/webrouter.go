package www

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jclem/jclem.me/internal/pages"
	"github.com/jclem/jclem.me/internal/posts"
	"github.com/jclem/jclem.me/internal/www/config"
	"github.com/jclem/jclem.me/internal/www/view"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/frontmatter"
)

type webRouter struct {
	*chi.Mux
	md    goldmark.Markdown
	pages *pages.Service
	posts *posts.Service
	view  *view.Service
}

func newWebRouter() (*webRouter, error) {
	pages := pages.New()
	if err := pages.Start(); err != nil {
		return nil, fmt.Errorf("error starting pages service: %w", err)
	}

	posts := posts.New()
	if err := posts.Start(); err != nil {
		return nil, fmt.Errorf("error starting posts service: %w", err)
	}

	view, err := view.New(pages, posts, config.URLUseHTTPS(), config.URLHostname())
	if err != nil {
		return nil, fmt.Errorf("error creating view service: %w", err)
	}

	md := goldmark.New(
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

	r := chi.NewRouter()
	w := &webRouter{Mux: r, md: md, pages: pages, posts: posts, view: view}
	r.Get("/", w.renderHome)
	r.Get("/writing", w.listPosts)
	r.Get("/writing/{slug}", w.showPost)
	r.Get("/sitemap.xml", w.sitemap)
	r.Get("/rss.xml", w.rss)
	r.Handle("/public/*", http.StripPrefix("/public/", http.FileServer(http.Dir("internal/www/public"))))

	return w, nil
}

func (wr *webRouter) renderHome(w http.ResponseWriter, r *http.Request) {
	page, err := wr.pages.Get("about")
	if err != nil {
		returnError(r.Context(), w, err, "error getting page")

		return
	}

	if err := wr.view.RenderHTML(w, "home", struct{ Content template.HTML }{Content: page.Content},
		view.WithTitle(page.Title),
		view.WithDescription(page.Description),
	); err != nil {
		returnError(r.Context(), w, err, "error rendering page")

		return
	}
}

type listPostsData struct {
	Title       string
	Description string
	Posts       []posts.Post
}

func (wr *webRouter) listPosts(w http.ResponseWriter, r *http.Request) {
	posts := wr.posts.List()

	if err := wr.view.RenderHTML(w, "writing/index", listPostsData{Posts: posts},
		view.WithTitle("Writing Archive"),
		view.WithDescription("A collection of articles and blog posts by Jonathan Clem"),
		view.WithLayout("writing/layout/index"),
	); err != nil {
		returnError(r.Context(), w, err, "error rendering page")

		return
	}
}

func (wr *webRouter) showPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	post, err := wr.posts.Get(slug)
	if err != nil {
		if errors.As(err, &posts.PostNotFoundError{}) {
			returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("post not found: %s", slug))

			return
		}

		returnError(r.Context(), w, err, "error getting post")

		return
	}

	if err := wr.view.RenderHTML(w, "writing/show", post,
		view.WithTitle(post.Title),
		view.WithDescription(post.Summary),
		view.WithLayout("writing/layout/show")); err != nil {
		returnError(r.Context(), w, err, "error rendering page")

		return
	}
}

func (wr *webRouter) sitemap(w http.ResponseWriter, r *http.Request) {
	posts := wr.posts.List()

	w.Header().Set("Content-Type", "application/xml")

	if err := wr.view.RenderXML(w, "sitemap.xml", posts); err != nil {
		returnError(r.Context(), w, err, "error rendering sitemap")

		return
	}
}

type rssData struct {
	BuildDate     string
	CopyrightYear string
	Posts         []posts.Post
}

func (wr *webRouter) rss(w http.ResponseWriter, r *http.Request) {
	posts := wr.posts.List()
	now := time.Now()

	w.Header().Set("Content-Type", "application/xml")

	if err := wr.view.RenderXML(w, "rss.xml", rssData{
		BuildDate:     now.UTC().Format(http.TimeFormat),
		CopyrightYear: strconv.Itoa(now.Year() - 1),
		Posts:         posts,
	}); err != nil {
		returnError(r.Context(), w, err, "error rendering rss")

		return
	}
}
