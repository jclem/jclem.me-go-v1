// Package markdown provides a general service for loading Markdown documents
// from an embed.FS.
package markdown

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/frontmatter"
)

var languageNames = map[string]string{ //nolint:gochecknoglobals
	"bash":       "Bash",
	"curl":       "cURL",
	"crystal":    "Crystal",
	"dockerfile": "Dockerfile",
	"elixir":     "Elixir",
	"hcl":        "HCL",
	"json":       "JSON",
	"js":         "JavaScript",
	"javascript": "JavaScript",
	"jsx":        "JavaScript JSX",
	"shell":      "Shell",
	"text":       "Plain Text",
	"ts":         "TypeScript",
	"typescript": "TypeScript",
	"tsx":        "TypeScript JSX",
	"yaml":       "YAML",
}

//go:embed templates
var renderTemplates embed.FS

// A Document represents a Markdown document's content and frontmatter.
type Document struct {
	Frontmatter *frontmatter.Data
	Content     string
}

// A service provides access to Markdown documents.
type Service struct {
	fs   embed.FS
	Data map[string]Document
}

// New creates a new Markdown service with the given embed.FS.
func New(content embed.FS) *Service {
	return &Service{
		fs:   content,
		Data: make(map[string]Document),
	}
}

// DocumentNotFoundError is returned when a document is not found.
type DocumentNotFoundError struct {
	Path string
}

// Error implements the error interface.
func (e DocumentNotFoundError) Error() string {
	return fmt.Sprintf("document not found: %s", e.Path)
}

// Get returns the document at the given path.
//
// If no document is found, a DocumentNotFoundError is returned.
func (s *Service) Get(path string) (Document, error) {
	doc, ok := s.Data[path]
	if !ok {
		return Document{}, DocumentNotFoundError{Path: path}
	}

	return doc, nil
}

func (s *Service) Load() error {
	tmpl, err := template.ParseFS(renderTemplates, "templates/*.html.tmpl")
	if err != nil {
		return fmt.Errorf("error parsing Markdown rendering templates: %w", err)
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
			renderer.WithNodeRenderers(
				util.Prioritized(
					&codeRenderer{
						writer:    html.DefaultWriter,
						templates: tmpl,
					}, 200),
			),
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
