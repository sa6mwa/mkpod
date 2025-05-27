package parser

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	mdp "github.com/gomarkdown/markdown/parser"
)

// MarkdownToHTML takes md as markdown and returns html.
func MarkdownToHTML(md string) (outputHTML string) {
	// Generate html from all description fields
	p := mdp.NewWithExtensions(mdp.CommonExtensions | mdp.AutoHeadingIDs | mdp.NoEmptyLineBeforeBlock)
	doc := p.Parse([]byte(md))
	renderer := html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
	})
	outputHTML = string(markdown.Render(doc, renderer))
	return
}
