package tgmd

import (
	"github.com/ai-shift/tgmd/markdownv2"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
)

func Telegramify(s string) string {
	md := []byte(s)
	p := parser.New()
	doc := p.Parse(md)
	opts := markdownv2.RendererOptions{}
	renderer := markdownv2.NewRenderer(opts)
	escaped := markdown.Render(doc, renderer)
	return string(escaped)
}
