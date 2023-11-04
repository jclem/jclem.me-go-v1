package markdown

import (
	"bytes"
	"fmt"
	"html/template"
	"log"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var codeblockTemplate *template.Template

func init() {
	tmpl, err := template.ParseGlob("internal/markdown/codeblock.html.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	codeblockTemplate = tmpl
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

	var codebuf bytes.Buffer

	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		codebuf.Write(line.Value(source))
	}

	if err := codeblockTemplate.Execute(w, struct {
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
