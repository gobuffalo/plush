package plush_test

import (
	"html/template"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
)

func holeContext(data map[string]interface{}) contextFactory {
	return func() hctx.Context {
		ctx := contextWith(data)()
		ctx.Set(meta.TemplateFileKey, "phase10.plush")
		return ctx
	}
}

func Test_Parity_Holes_No_Filename_Returns_Skeleton(t *testing.T) {
	compareRender(t, `<%H "hello" %>`, emptyContext)
}

func Test_Parity_Holes_Simple(t *testing.T) {
	compareRender(t, `<%H "hello" %>`, holeContext(nil))
}

func Test_Parity_Holes_Error_In_Hole(t *testing.T) {
	compareRender(t, `<%H missing_helper" %><%H "ok" %>`, holeContext(nil))
}

func Test_Parity_Holes_Positions_And_Counts(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "multiple holes at end",
			input: `<%= "x" %><%H "a" %><%H "b" %><%H "c" %>`,
		},
		{
			name:  "holes at start",
			input: `<%H "a" %><%H "b" %><%= "x" %>`,
		},
		{
			name:  "hole at start and end",
			input: `<%H "start" %>middle<%H "end" %>`,
		},
		{
			name:  "empty holes",
			input: `<%H "" %>foo<%H  %>`,
		},
		{
			name:  "literal marker preserved",
			input: `<PLUSH_HOLE_0><%H "start" %><%H "end" %>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compareRender(t, tt.input, holeContext(nil))
		})
	}
}

func Test_Parity_Holes_Many(t *testing.T) {
	input := ""
	for i := 0; i < 100; i++ {
		input += `<%H "x" %>`
	}
	compareRender(t, input, holeContext(nil))
}

func Test_Parity_Holes_Inside_If_Blocks(t *testing.T) {
	compareRender(t, `<%= if (a == "22") { %><%H "testing" %><% } else { %><%H "dddd" %><% } %>`, holeContext(map[string]interface{}{
		"a": "22",
	}))
	compareRender(t, `<%H if (number > 0){ %><%= "NUMBER" %><% } else { %><%= number %><%  }%>`, holeContext(map[string]interface{}{
		"number": 3,
	}))
}

func Test_Parity_Holes_Loop_Body(t *testing.T) {
	compareRender(t, `<%= for (i,v) in items { %><%H "testing" %><%= v %><% } %>`, holeContext(map[string]interface{}{
		"items": []string{"a", "b", "c"},
	}))
	compareRender(t, `<%H for (i,v) in items { %><%= v %><% } %>`, holeContext(map[string]interface{}{
		"items": []string{"a", "b", "c"},
	}))
}

func Test_Parity_Holes_Inside_Helper_Block(t *testing.T) {
	compareRender(t, `<%= wrap() { %><%H "inside" %><% } %>`, holeContext(map[string]interface{}{
		"wrap": func(help rootplush.HelperContext) (template.HTML, error) {
			body, err := help.Block()
			return template.HTML("<span>" + body + "</span>"), err
		},
	}))
}

func Test_Parity_Holes_Inside_Partial(t *testing.T) {
	compareRender(t, `<%= partial("hole.plush") %>`, holeContext(map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			return `<span><%H "inside" %></span>`, nil
		},
	}))
}

func Test_Parity_Holes_Recursive_Partial(t *testing.T) {
	compareRender(t, `<%= partial("index.plush") %>`, holeContext(map[string]interface{}{
		"number": 3,
		"partialFeeder": func(string) (string, error) {
			return `<%=
		if (number > 0) { %><%
			let number = number - 1 %><%=
			partial("index.plush") %><%H number %>, <%
		} %>`, nil
		},
	}))
}
