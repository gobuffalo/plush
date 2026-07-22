package plush_test

import (
	"html/template"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func Test_Render_Whitespace_Trim_Tags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		ctx      *plush.Context
	}{
		{
			name:     "issue newline example",
			input:    "<pre>\n<%- \"Hello\" %>\n</pre>",
			expected: "<pre>Hello</pre>",
			ctx:      plush.NewContext(),
		},
		{
			name:     "issue inline spaces example",
			input:    `<pre> <%- "Hello" %> </pre>`,
			expected: "<pre>Hello</pre>",
			ctx:      plush.NewContext(),
		},
		{
			name:     "middle of document",
			input:    "<p>A</p>\n<%- \"B\" %>\n<p>C</p>",
			expected: "<p>A</p>B<p>C</p>",
			ctx:      plush.NewContext(),
		},
		{
			name:     "raw html safety unchanged",
			input:    `<%- raw("<b>x</b>") %>`,
			expected: "<b>x</b>",
			ctx:      plush.NewContext(),
		},
		{
			name:     "inside if",
			input:    "<%= if true { %>\n<%- \"yes\" %>\n<% } %>",
			expected: "yes",
			ctx:      plush.NewContext(),
		},
		{
			name:     "inside for",
			input:    "<%= for (i,v) in items { %>\n<%- v %>\n<% } %>",
			expected: "ab",
			ctx: plush.NewContextWith(map[string]interface{}{
				"items": []string{"a", "b"},
			}),
		},
		{
			name:     "inside helper block",
			input:    "<%= wrap() { %>\n<%- \"x\" %>\n<% } %>",
			expected: "<span>x</span>",
			ctx: plush.NewContextWith(map[string]interface{}{
				"wrap": func(help plush.HelperContext) (template.HTML, error) {
					body, err := help.Block()
					return template.HTML("<span>" + body + "</span>"), err
				},
			}),
		},
		{
			name:     "inside partial",
			input:    `<%= partial("row.plush") %>`,
			expected: "<span>x</span>",
			ctx: plush.NewContextWith(map[string]interface{}{
				"partialFeeder": func(string) (string, error) {
					return "<span>\n<%- \"x\" %>\n</span>", nil
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := plush.Render(tt.input, tt.ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, out)
		})
	}
}

func Test_Render_Old_Eval_Whitespace_Behavior_Unchanged(t *testing.T) {
	out, err := plush.Render("<pre>\n<%= \"Hello\" %>\n</pre>", plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "<pre>\nHello\n</pre>", out)
}

func Test_Render_Optional_If_Parentheses(t *testing.T) {
	type robot struct {
		Name string
	}

	tests := []struct {
		name     string
		input    string
		expected string
		ctx      *plush.Context
	}{
		{
			name:     "literal true",
			input:    `<%= if true { %>yes<% } %>`,
			expected: "yes",
			ctx:      plush.NewContext(),
		},
		{
			name:     "identifier truthy",
			input:    `<%= if name { %><%= name %><% } %>`,
			expected: "mark",
			ctx: plush.NewContextWith(map[string]interface{}{
				"name": "mark",
			}),
		},
		{
			name:     "equality",
			input:    `<%= if name == "mark" { %>yes<% } %>`,
			expected: "yes",
			ctx: plush.NewContextWith(map[string]interface{}{
				"name": "mark",
			}),
		},
		{
			name:     "else if expression",
			input:    `<%= if false { %>no<% } else if name == "mark" { %>yes<% } %>`,
			expected: "yes",
			ctx: plush.NewContextWith(map[string]interface{}{
				"name": "mark",
			}),
		},
		{
			name:     "indexed struct field",
			input:    `<%= if robots[0].Name == "mark" { %>yes<% } %>`,
			expected: "yes",
			ctx: plush.NewContextWith(map[string]interface{}{
				"robots": []robot{{Name: "mark"}},
			}),
		},
		{
			name:     "old syntax still works",
			input:    `<%= if (true) { %>yes<% } else if (false) { %>no<% } %>`,
			expected: "yes",
			ctx:      plush.NewContext(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := plush.Render(tt.input, tt.ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, out)
		})
	}
}
