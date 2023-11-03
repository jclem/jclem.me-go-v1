package view

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"

	"github.com/jclem/jclem.me/internal/pages"
	"github.com/jclem/jclem.me/internal/posts"
)

//go:embed templates
var fs embed.FS

type Service struct {
	pages     *pages.Service
	posts     *posts.Service
	templates *template.Template
}

type renderOpts struct {
	title       string
	description string
	layout      string
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

type renderedPage struct {
	Title       string
	Description string
	Content     template.HTML
}

func (s *Service) RenderTemplate(w io.Writer, name string, data any, opts ...RenderOpt) error {
	ropts := &renderOpts{}
	for _, opt := range opts {
		opt(ropts)
	}

	var tbuf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&tbuf, name, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	if ropts.layout != "" {
		var lbuf bytes.Buffer
		if err := s.templates.ExecuteTemplate(&lbuf, ropts.layout, template.HTML(tbuf.String())); err != nil { //nolint:gosec
			return fmt.Errorf("error executing template: %w", err)
		}

		return s.renderRoot(w, ropts.title, ropts.description, template.HTML(lbuf.String())) //nolint:gosec
	}

	return s.renderRoot(w, ropts.title, ropts.description, template.HTML(tbuf.String())) //nolint:gosec
}

func (s *Service) renderRoot(w io.Writer, title, description string, content template.HTML) error {
	if err := s.templates.ExecuteTemplate(w, "root", renderedPage{
		Title:       title,
		Description: description,
		Content:     content,
	}); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	return nil
}

func New(pages *pages.Service, posts *posts.Service) (*Service, error) {
	templates, err := template.New("").ParseFS(fs, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	subdirs, err := fs.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("error reading templates directory: %w", err)
	}

	for _, subdir := range subdirs {
		if !subdir.IsDir() {
			continue
		}

		_, err := templates.ParseFS(fs, "templates/"+subdir.Name()+"/*.tmpl")
		if err != nil {
			return nil, fmt.Errorf("error parsing templates: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	return &Service{
		pages:     pages,
		posts:     posts,
		templates: templates,
	}, nil
}
