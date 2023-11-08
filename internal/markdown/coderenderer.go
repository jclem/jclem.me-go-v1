package markdown

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

type codeRenderer struct {
	writer    html.Writer
	templates *template.Template
}

func (r *codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.render)
}

type WrongNodeError struct {
	Expected string
	Node     ast.Node
}

func (e WrongNodeError) Error() string {
	return fmt.Sprintf("node is not %s: %v", e.Expected, e.Node)
}

var ErrTemplateNotFound = fmt.Errorf("template not found")

func (r *codeRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.FencedCodeBlock)
	if !ok {
		return ast.WalkStop, &WrongNodeError{Expected: "fenced code block", Node: node}
	}

	if !entering {
		return ast.WalkContinue, nil
	}

	lang := string(n.Language(source))

	langName, ok := languageNames[lang]
	if !ok {
		langName = lang
	}

	var codebuf bytes.Buffer

	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		codebuf.Write(line.Value(source))
	}

	tmpl := r.templates.Lookup("codeblock.html.tmpl")
	if tmpl == nil {
		return ast.WalkStop, ErrTemplateNotFound
	}

	if err := tmpl.Execute(w, struct {
		LanguageName string
		Code         string
	}{
		LanguageName: langName,
		Code:         codebuf.String(),
	}); err != nil {
		return ast.WalkStop, fmt.Errorf("error writing code-example div: %w", err)
	}

	return ast.WalkSkipChildren, nil
}
