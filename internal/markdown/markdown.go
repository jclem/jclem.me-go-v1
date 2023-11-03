package markdown

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/frontmatter"
)

var languageNames = map[string]string{
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

type codeRenderer struct {
	writer html.Writer
}

func (r *codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.render)
}

type WrongNodeError struct {
	Node ast.Node
}

func (e WrongNodeError) Error() string {
	return fmt.Sprintf("node is not a fenced code block: %v", e.Node)
}

func (r *codeRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.FencedCodeBlock)
	if !ok {
		return ast.WalkStop, &WrongNodeError{Node: node}
	}

	if !entering {
		return ast.WalkContinue, nil
	}

	lang := string(n.Language(source))

	langName, ok := languageNames[lang]
	if !ok {
		langName = lang
	}

	if _, err := w.WriteString(`<div class="code-example">`); err != nil {
		return ast.WalkStop, fmt.Errorf("error writing code-example div: %w", err)
	}

	if _, err := w.WriteString(fmt.Sprintf(`
		<div class="flex items-center justify-end gap-2 border-b border-dashed border-border p-1 text-xs">%s
			<button onclick="copyNextCode(this)" aria-label="Copy code to clipboard">
				<svg class="h-4 w-4 text-text-deemphasize hover:text-text" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 7.5V6.108c0-1.135.845-2.098 1.976-2.192.373-.03.748-.057 1.123-.08M15.75 18H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08M15.75 18.75v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5A3.375 3.375 0 006.375 7.5H5.25m11.9-3.664A2.251 2.251 0 0015 2.25h-1.5a2.251 2.251 0 00-2.15 1.586m5.8 0c.065.21.1.433.1.664v.75h-6V4.5c0-.231.035-.454.1-.664M6.75 7.5H4.875c-.621 0-1.125.504-1.125 1.125v12c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V16.5a9 9 0 00-9-9z">
					</path>
				</svg>
			</button>
		</div>`, langName)); err != nil {
		return ast.WalkStop, fmt.Errorf("error writing code-example div: %w", err)
	}

	if _, err := w.WriteString(`<pre class="overflow-x-auto p-2 text-xs">`); err != nil {
		return ast.WalkStop, fmt.Errorf("error writing code-example div: %w", err)
	}

	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		r.writer.RawWrite(w, line.Value(source))
	}

	if _, err := w.WriteString(`</pre></div>`); err != nil {
		return ast.WalkStop, fmt.Errorf("error writing code-example div: %w", err)
	}

	return ast.WalkSkipChildren, nil
}

func (s *Service) Load() error {
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
				util.Prioritized(&codeRenderer{writer: html.DefaultWriter}, 200),
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
