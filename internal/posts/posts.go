package posts

import (
	"embed"
	"fmt"
	"html/template"
	"sort"
	"time"

	"github.com/jclem/jclem.me/internal/markdown"
)

type Post struct {
	Title       string `yaml:"title"`
	Slug        string `yaml:"slug"`
	Content     template.HTML
	PublishedAt time.Time `yaml:"published_at"`
	Published   bool      `yaml:"published"`
	HasMath     bool      `yaml:"has_math"`
	Summary     string    `yaml:"summary"`
}

//go:embed *.md
var Content embed.FS

type Service struct {
	md    *markdown.Service
	posts []Post
}

func New() *Service {
	md := markdown.New(Content)

	return &Service{
		md:    md,
		posts: make([]Post, 0, len(md.Data)),
	}
}

func (s *Service) Start() error {
	if err := s.md.Load(); err != nil {
		return fmt.Errorf("error loading posts markdown: %w", err)
	}

	for _, document := range s.md.Data {
		var post Post

		if err := document.Frontmatter.Decode(&post); err != nil {
			return fmt.Errorf("error unmarshaling page frontmatter: %w", err)
		}

		post.Content = template.HTML(document.Content) //nolint:gosec

		s.posts = append(s.posts, post)
	}

	return nil
}

type listOpts struct {
	withDrafts bool
}

type ListOpt func(*listOpts)

func WithDrafts() ListOpt {
	return func(o *listOpts) {
		o.withDrafts = true
	}
}

type PostNotFoundError struct {
	Slug string
}

func (e PostNotFoundError) Error() string {
	return fmt.Sprintf("post not found: %s", e.Slug)
}

func (s *Service) Get(slug string) (Post, error) {
	for _, post := range s.posts {
		if post.Slug == slug {
			return post, nil
		}
	}

	return Post{}, PostNotFoundError{Slug: slug}
}

func (s *Service) List(opts ...ListOpt) ([]Post, error) {
	var o listOpts
	for _, opt := range opts {
		opt(&o)
	}

	posts := make([]Post, 0, len(s.posts))

	for _, post := range s.posts {
		if !o.withDrafts && !post.Published {
			continue
		}

		posts = append(posts, post)
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].PublishedAt.After(posts[j].PublishedAt)
	})

	return posts, nil
}
