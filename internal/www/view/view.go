package view

import (
	"bytes"
	"embed"
	"fmt"
	html "html/template"
	"io"
	text "text/template"

	"github.com/jclem/jclem.me/internal/pages"
	"github.com/jclem/jclem.me/internal/posts"
	"github.com/jclem/jclem.me/internal/www/public"
)

//go:embed templates
var fs embed.FS

type Service struct {
	pages    *pages.Service
	posts    *posts.Service
	html     *html.Template
	xml      *text.Template
	useHTTPS bool
	hostname string
}

type renderOpts struct {
	title       string
	description string
	layout      string
	noRoot      bool
}

type RenderOpt func(*renderOpts)

func WithTitle(title string) RenderOpt {
	return func(opts *renderOpts) {
		opts.title = title
	}
}

func WithDescription(description string) RenderOpt {
	return func(opts *renderOpts) {
		opts.description = description
	}
}

func WithLayout(layout string) RenderOpt {
	return func(opts *renderOpts) {
		opts.layout = layout
	}
}

func WithNoRoot() RenderOpt {
	return func(opts *renderOpts) {
		opts.noRoot = true
	}
}

type renderedPage struct {
	Title       string
	Description string
	Content     html.HTML
}

func (s *Service) RenderHTML(w io.Writer, name string, data any, opts ...RenderOpt) error {
	ropts := &renderOpts{}
	for _, opt := range opts {
		opt(ropts)
	}

	var tbuf bytes.Buffer
	if err := s.html.ExecuteTemplate(&tbuf, name, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	if ropts.layout != "" {
		var lbuf bytes.Buffer
		if err := s.html.ExecuteTemplate(&lbuf, ropts.layout, html.HTML(tbuf.String())); err != nil { //nolint:gosec
			return fmt.Errorf("error executing template: %w", err)
		}

		return s.renderRoot(w, ropts.title, ropts.description, html.HTML(lbuf.String())) //nolint:gosec
	}

	if ropts.noRoot {
		if _, err := w.Write(tbuf.Bytes()); err != nil {
			return fmt.Errorf("error writing template: %w", err)
		}

		return nil
	}

	return s.renderRoot(w, ropts.title, ropts.description, html.HTML(tbuf.String())) //nolint:gosec
}

func (s *Service) RenderXML(w io.Writer, name string, data any) error {
	if err := s.xml.ExecuteTemplate(w, name, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	return nil
}

func (s *Service) renderRoot(w io.Writer, title, description string, content html.HTML) error {
	if err := s.html.ExecuteTemplate(w, "root", renderedPage{
		Title:       title,
		Description: description,
		Content:     content,
	}); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	return nil
}

func New(pages *pages.Service, posts *posts.Service, useHTTPS bool, hostname string) (*Service, error) {
	svc := Service{pages: pages, posts: posts, useHTTPS: useHTTPS, hostname: hostname}

	htmltmpl, err := html.New("").Funcs(html.FuncMap{
		"mustGetStyles":  public.MustGetStyles,
		"mustGetScripts": public.MustGetScripts,
		"url":            svc.url(),
	}).ParseFS(fs, "templates/*.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("error parsing html templates: %w", err)
	}

	svc.html = htmltmpl

	subdirs, err := fs.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("error reading html templates directory: %w", err)
	}

	for _, subdir := range subdirs {
		if !subdir.IsDir() {
			continue
		}

		_, err := htmltmpl.ParseFS(fs, "templates/"+subdir.Name()+"/*.tmpl")
		if err != nil {
			return nil, fmt.Errorf("error parsing html templates: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	xmltmpl, err := text.New("").Funcs(text.FuncMap{"url": svc.url()}).ParseFS(fs, "templates/*.xml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("error parsing xml templates: %w", err)
	}

	svc.xml = xmltmpl

	return &svc, nil
}

func (s *Service) url() func(path string) string {
	return func(path string) string {
		proto := "http://"
		if s.useHTTPS {
			proto = "https://"
		}

		hostname := s.hostname

		return proto + hostname + path
	}
}
