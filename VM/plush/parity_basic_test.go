package plush_test

import (
	"html/template"
	"testing"
)

func Test_Parity_Basic_HTML_And_Interpolation(t *testing.T) {
	compareRender(t, `<p>Hello <%= name %></p>`, contextWith(map[string]interface{}{
		"name": "mark",
	}))
}

func Test_Parity_Basic_Spacing_And_Show_No_Show(t *testing.T) {
	compareRender(t, `<%= greet %> <%= name %>`, contextWith(map[string]interface{}{
		"greet": "hi",
		"name":  "mark",
	}))
	compareRender(t, `<%= "shown" %><% "notshown" %>`, emptyContext)
}

func Test_Parity_Basic_Escaping(t *testing.T) {
	compareRender(t, `<%= html %>`, contextWith(map[string]interface{}{
		"html": `<strong>safe?</strong>`,
	}))
}

func Test_Parity_Basic_HTML_Escape_Helpers(t *testing.T) {
	compareRender(t, `<%= escapedHTML() %>|<%= unescapedHTML() %>|<%= raw("<b>unsafe</b>") %>`, contextWith(map[string]interface{}{
		"escapedHTML": func() string {
			return "<b>unsafe</b>"
		},
		"unescapedHTML": func() template.HTML {
			return "<b>unsafe</b>"
		},
	}))
}

func Test_Parity_Basic_Escape_Expression(t *testing.T) {
	compareRender(t, `C:\\<%= "temp" %>`, emptyContext)
}
