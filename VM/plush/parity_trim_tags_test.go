package plush_test

import (
	"html/template"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
)

func Test_Parity_Phase_8_Trim_Tags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		factory  contextFactory
	}{
		{
			name:     "issue newline example",
			input:    "<pre>\n<%- \"Hello\" %>\n</pre>",
			expected: "<pre>Hello</pre>",
			factory:  emptyContext,
		},
		{
			name:     "issue inline spaces example",
			input:    `<pre> <%- "Hello" %> </pre>`,
			expected: "<pre>Hello</pre>",
			factory:  emptyContext,
		},
		{
			name:     "middle of document",
			input:    "<p>A</p>\n<%- \"B\" %>\n<p>C</p>",
			expected: "<p>A</p>B<p>C</p>",
			factory:  emptyContext,
		},
		{
			name:     "raw html safety unchanged",
			input:    `<%- raw("<b>x</b>") %>`,
			expected: "<b>x</b>",
			factory:  emptyContext,
		},
		{
			name:     "inside if",
			input:    "<%= if (true) { %>\n<%- \"yes\" %>\n<% } %>",
			expected: "yes",
			factory:  emptyContext,
		},
		{
			name:     "inside for",
			input:    "<%= for (i,v) in items { %>\n<%- v %>\n<% } %>",
			expected: "ab",
			factory: contextWith(map[string]interface{}{
				"items": []string{"a", "b"},
			}),
		},
		{
			name:     "inside helper block",
			input:    "<%= wrap() { %>\n<%- \"x\" %>\n<% } %>",
			expected: "<span>x</span>",
			factory: contextWith(map[string]interface{}{
				"wrap": func(help rootplush.HelperContext) (template.HTML, error) {
					body, err := help.Block()
					return template.HTML("<span>" + body + "</span>"), err
				},
			}),
		},
		{
			name:     "inside partial",
			input:    `<%= partial("row.plush") %>`,
			expected: "<span>x</span>",
			factory: contextWith(map[string]interface{}{
				"partialFeeder": func(string) (string, error) {
					return "<span>\n<%- \"x\" %>\n</span>", nil
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, tt.factory)
		})
	}
}

func Test_Phase_8_Old_Eval_Whitespace_Behavior_Unchanged(t *testing.T) {
	requireBothRender(t, "<pre>\n<%= \"Hello\" %>\n</pre>", "<pre>\nHello\n</pre>", emptyContext)
}
