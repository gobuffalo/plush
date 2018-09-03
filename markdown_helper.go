package plush

import (
	"html/template"

	"github.com/gobuffalo/github_flavored_markdown"
)

// Markdown converts the string into HTML using GitHub flavored markdown.
func MarkdownHelper(body string, help HelperContext) (template.HTML, error) {
	var err error
	if help.HasBlock() {
		body, err = help.Block()
		if err != nil {
			return "", err
		}
	}
	b := github_flavored_markdown.Markdown([]byte(body))
	return template.HTML(b), err
}
