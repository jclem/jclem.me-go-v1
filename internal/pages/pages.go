package pages

import (
	"embed"
	"fmt"
	"html/template"

	"github.com/jclem/jclem.me/internal/markdown"
)

type Page struct {
	Slug        string `yaml:"slug"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Content     template.HTML
}

//go:embed *.md
var Content embed.FS

type Service struct {
	md    *markdown.Service
	pages []Page
}

func (s *Service) Start() error {
	if err := s.md.Load(); err != nil {
		return fmt.Errorf("error loading pages markdown: %w", err)
	}

	for _, document := range s.md.Data {
		var page Page

		if err := document.Frontmatter.Decode(&page); err != nil {
			return fmt.Errorf("error unmarshaling page frontmatter: %w", err)
		}

		page.Content = template.HTML(document.Content) //nolint:gosec

		s.pages = append(s.pages, page)
	}

	return nil
}

type PageNotFoundError struct {
	Path string
}

func (e PageNotFoundError) Error() string {
	return fmt.Sprintf("page not found: %s", e.Path)
}

func (s *Service) Get(slug string) (Page, error) {
	for _, page := range s.pages {
		if page.Slug == slug {
			return page, nil
		}
	}

	return Page{}, PageNotFoundError{}
}

func New() *Service {
	md := markdown.New(Content)

	return &Service{
		md:    md,
		pages: make([]Page, 0, len(md.Data)),
	}
}
