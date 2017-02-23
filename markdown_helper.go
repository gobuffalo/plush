package plush

import (
	"html/template"

	"github.com/shurcooL/github_flavored_markdown"
)

// Markdown converts the string into HTML using GitHub flavored markdown.
func markdownHelper(body string) template.HTML {
	b := github_flavored_markdown.Markdown([]byte(body))
	return template.HTML(b)
}
