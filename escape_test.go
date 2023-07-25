package plush_test

import (
	"html/template"
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_Render_EscapedString(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "<script>alert('pwned')</script>" %></p>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("<p>&lt;script&gt;alert(&#39;pwned&#39;)&lt;/script&gt;</p>", s)
}

func Test_Render_HTML_Escape(t *testing.T) {
	r := require.New(t)

	input := `<%= escapedHTML() %>|<%= unescapedHTML() %>|<%= raw("<b>unsafe</b>") %>`
	s, err := plush.Render(input, plush.NewContextWith(map[string]interface{}{
		"escapedHTML": func() string {
			return "<b>unsafe</b>"
		},
		"unescapedHTML": func() template.HTML {
			return "<b>unsafe</b>"
		},
	}))
	r.NoError(err)
	r.Equal("&lt;b&gt;unsafe&lt;/b&gt;|<b>unsafe</b>|<b>unsafe</b>", s)
}

func Test_Escaping_EscapeExpression(t *testing.T) {
	r := require.New(t)
	input := `C:\\<%= "temp" %>`

	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal(`C:\temp`, s)
}
