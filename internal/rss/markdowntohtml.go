package rss

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	mdp "github.com/gomarkdown/markdown/parser"
)

func MarkdownToHTML(md string) (outputHTML string) {
	p := mdp.NewWithExtensions(mdp.CommonExtensions | mdp.AutoHeadingIDs | mdp.NoEmptyLineBeforeBlock)
	doc := p.Parse([]byte(md))
	renderer := html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
	})
	outputHTML = string(markdown.Render(doc, renderer))
	return
}
