package markdown

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"
)

type Document struct {
	Frontmatter *frontmatter.Data
	Content     string
}

type Service struct {
	fs   embed.FS
	Data map[string]Document
}

func New(content embed.FS) *Service {
	return &Service{
		fs:   content,
		Data: make(map[string]Document),
	}
}

type DocumentNotFoundError struct {
	Path string
}

func (e DocumentNotFoundError) Error() string {
	return fmt.Sprintf("document not found: %s", e.Path)
}

func (s *Service) Get(path string) (Document, error) {
	doc, ok := s.Data[path]
	if !ok {
		return Document{}, DocumentNotFoundError{Path: path}
	}

	return doc, nil
}

func (s *Service) Load() error {
	gm := goldmark.New(
		goldmark.WithExtensions(
			&frontmatter.Extender{},
		),
	)

	m, err := fs.Glob(s.fs, "*.md")
	if err != nil {
		return fmt.Errorf("error globbing markdown files: %w", err)
	}

	for _, path := range m {
		pctx := parser.NewContext()

		b, err := fs.ReadFile(s.fs, path)
		if err != nil {
			return fmt.Errorf("error reading markdown file: %w", err)
		}

		var buf bytes.Buffer
		if err := gm.Convert(b, &buf, parser.WithContext(pctx)); err != nil {
			return fmt.Errorf("error converting markdown: %w", err)
		}

		fm := frontmatter.Get(pctx)

		s.Data[path] = Document{
			Frontmatter: fm,
			Content:     buf.String(),
		}
	}

	return nil
}
