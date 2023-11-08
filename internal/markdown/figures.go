package markdown

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

type figureRenderer struct {
	writer html.Writer
}

func (r *figureRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderFigure)
}

func (r *figureRenderer) renderFigure(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.Image)
	if !ok {
		return ast.WalkStop, &WrongNodeError{Expected: "image", Node: node}
	}

	if entering {
		_, _ = w.WriteString(`<figure class="img-figure">`)
		_, _ = w.WriteString(`<img src="`)

		if !html.IsDangerousURL(n.Destination) {
			_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
		}

		_, _ = w.WriteString(`" alt="`)
		_, _ = w.Write(util.EscapeHTML(n.Text(source)))

		if n.Title != nil {
			_, _ = w.WriteString(`" title="`)
			_, _ = w.Write(util.EscapeHTML(n.Title))
		}

		if n.Attributes() != nil {
			html.RenderAttributes(w, n, html.ImageAttributeFilter)
		}

		_, _ = w.WriteString(`" />`)

		_, _ = w.WriteString("<figcaption>")
		_, _ = w.Write(util.EscapeHTML(n.Text(source)))
		_, _ = w.WriteString("</figcaption>")
	} else {
		_, _ = w.WriteString(`</figure>`)
	}

	return ast.WalkSkipChildren, nil
}
