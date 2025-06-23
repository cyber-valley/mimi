package markdownv2

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gomarkdown/markdown/ast"
)

// RendererOptions is a collection of supplementary parameters tweaking
// the behavior of various parts of Telegram MarkdownV2 renderer.
type RendererOptions struct {
}

// Renderer implements Renderer interface for Telegram MarkdownV2 output.
type Renderer struct {
	Opts RendererOptions
}

// NewRenderer creates and configures an Renderer object, which
// satisfies the Renderer interface.
func NewRenderer(opts RendererOptions) *Renderer {
	return &Renderer{
		Opts: opts,
	}
}

var telegramEscaper = map[byte][]byte{
	'_':  []byte("\\_"),
	'*':  []byte("\\*"),
	'[':  []byte("\\["),
	']':  []byte("\\]"),
	'(':  []byte("\\("),
	')':  []byte("\\)"),
	'~':  []byte("\\~"),
	'`':  []byte("\\`"),
	'>':  []byte("\\>"),
	'#':  []byte("\\#"),
	'+':  []byte("\\+"),
	'-':  []byte("\\-"),
	'=':  []byte("\\="),
	'|':  []byte("\\|"),
	'{':  []byte("\\{"),
	'}':  []byte("\\}"),
	'.':  []byte("\\."),
	'!':  []byte("\\!"),
	'\\': []byte("\\\\"),
}

func EscapeTelegram(w io.Writer, d []byte) {
	var start, end int
	n := len(d)
	for end < n {
		escSeq, found := telegramEscaper[d[end]]
		if found {
			w.Write(d[start:end])
			w.Write(escSeq)
			start = end + 1
		}
		end++
	}
	if start < n && end <= n {
		w.Write(d[start:end])
	}
}

func (r *Renderer) Out(w io.Writer, d []byte) {
	w.Write(d)
}

func (r *Renderer) Outs(w io.Writer, s string) {
	w.Write([]byte(s))
}

func (r *Renderer) CR(w io.Writer) {
	io.WriteString(w, "\n")
}

// RenderNode renders a markdown node to Telegram MarkdownV2
func (r *Renderer) RenderNode(w io.Writer, node ast.Node, entering bool) ast.WalkStatus {
	switch node := node.(type) {
	case *ast.Text:
		EscapeTelegram(w, node.Literal)
	case *ast.Emph:
		r.Outs(w, "_")
	case *ast.Strong:
		r.Outs(w, "*")
	case *ast.Del:
		r.Outs(w, "~")
	case *ast.BlockQuote:
		if entering {
			r.Outs(w, ">")
		} else {
			r.CR(w)
		}
	case *ast.Link:
		if entering {
			r.Outs(w, "[")
		} else {
			r.Outs(w, "](")
			r.Out(w, node.Destination)
			r.Outs(w, ")")
		}
	case *ast.Image:
		if entering {
			r.Outs(w, "!")
			r.Outs(w, "[")
		} else {
			r.Outs(w, "](")
			r.Out(w, node.Destination)
			r.Outs(w, ")")
		}

	case *ast.Code:
		r.Outs(w, "`")
		code := bytes.ReplaceAll(node.Literal, []byte("`"), []byte("\\`"))
		code = bytes.ReplaceAll(code, []byte("\\"), []byte("\\\\"))

		r.Out(w, code)
		r.Outs(w, "`")
	case *ast.CodeBlock:
		var language string
		if len(node.Info) > 0 {
			language = string(node.Info)
		}
		r.Outs(w, "```")
		r.Outs(w, language)
		r.CR(w)
		code := bytes.ReplaceAll(node.Literal, []byte("`"), []byte("\\`"))
		code = bytes.ReplaceAll(code, []byte("\\"), []byte("\\\\"))
		r.Out(w, code)
		r.CR(w)
		r.Outs(w, "```")

	case *ast.List:
		// do nothing
	case *ast.ListItem:
		if entering {
			r.Outs(w, "- ") // Telegram doesn't support ordered lists
		}
	case *ast.HorizontalRule:
		r.CR(w)
		r.Outs(w, "---")
		r.CR(w)

	case *ast.Paragraph:
		if !entering {
			r.CR(w)
			r.CR(w)
		}
	case *ast.Heading:
		r.Outs(w, "*")
		if !entering {
			r.CR(w)
			r.CR(w)
		}
	case *ast.HTMLSpan:
		// Ignore HTML
	case *ast.HTMLBlock:
		// Ignore HTML
	case *ast.Softbreak:
		r.CR(w)
	case *ast.Hardbreak:
		r.CR(w)
		r.CR(w)
	case *ast.Document:
		// do nothing
	case *ast.Table:
		r.CR(w)
		r.Outs(w, "Tables are not supported in Telegram MarkdownV2")
		r.CR(w)
	case *ast.TableCell:
	case *ast.TableHeader:
	case *ast.TableBody:
	case *ast.TableRow:
	case *ast.TableFooter:
	case *ast.Math:
		r.Outs(w, "`")
		code := bytes.ReplaceAll(node.Literal, []byte("`"), []byte("\\`"))
		code = bytes.ReplaceAll(code, []byte("\\"), []byte("\\\\"))
		r.Out(w, code)
		r.Outs(w, "`")
	case *ast.MathBlock:
		r.CR(w)
		r.Outs(w, "```")
		r.CR(w)
		code := bytes.ReplaceAll(node.Literal, []byte("`"), []byte("\\`"))
		code = bytes.ReplaceAll(code, []byte("\\"), []byte("\\\\"))
		r.Out(w, code)
		r.CR(w)
		r.Outs(w, "```")
		r.CR(w)
	case *ast.Subscript:
		r.Outs(w, "__")
		if entering {
			EscapeTelegram(w, node.Literal)
		}
		r.Outs(w, "__")
	case *ast.Superscript:
		// Ignore
	case *ast.DocumentMatter:
		// Ignore
	case *ast.Callout:
		// Ignore
	case *ast.Index:
		// Ignore
	case *ast.NonBlockingSpace:
		r.Outs(w, " ")
	default:
		panic(fmt.Sprintf("Unknown node %T", node))
	}
	return ast.GoToNext
}

// RenderHeader writes nothing for Telegram MarkdownV2.
func (r *Renderer) RenderHeader(w io.Writer, ast ast.Node) {}

// RenderFooter writes nothing for Telegram MarkdownV2.
func (r *Renderer) RenderFooter(w io.Writer, node ast.Node) {}
