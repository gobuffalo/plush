package plush_test

import (
	"html/template"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/gobuffalo/tags/v3"
	"github.com/stretchr/testify/require"
)

func Test_Let_Reassignment(t *testing.T) {
	r := require.New(t)
	input := `<% let foo = "bar" %>
  <%= for (a) in myArray { %>
<%= foo %>
    <% if (foo != "baz") { %>
      <% foo = "baz" %>
    <% } %>
  <% } %>
<% } %>`

	ctx := plush.NewContext()
	ctx.Set("myArray", []string{"a", "b"})

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("bar\n    \n  \nbaz", strings.TrimSpace(s))
}

func Test_Let_SyntaxError_NoEqualSign(t *testing.T) {
	r := require.New(t)
	input := `<% let foo %>`

	ctx := plush.NewContext()

	_, err := plush.Render(input, ctx)
	r.ErrorContains(err, "expected next token to be =")
}

func Test_Let_SyntaxError_NoIdentifier(t *testing.T) {
	r := require.New(t)
	input := `<% let = %>`

	ctx := plush.NewContext()

	_, err := plush.Render(input, ctx)
	r.ErrorContains(err, "expected next token to be IDENT")
}

func Test_Let_Reassignment_UnknownIdent(t *testing.T) {
	r := require.New(t)
	input := `<% foo = "baz" %>`

	ctx := plush.NewContext()
	ctx.Set("myArray", []string{"a", "b"})

	_, err := plush.Render(input, ctx)
	r.ErrorContains(err, "\"foo\": unknown identifier")
}

func Test_Let_Inside_Helper(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContextWith(map[string]interface{}{
		"divwrapper": func(opts map[string]interface{}, helper plush.HelperContext) (template.HTML, error) {
			body, err := helper.Block()
			if err != nil {
				return template.HTML(""), err
			}
			t := tags.New("div", opts)
			t.Append(body)
			return t.HTML(), nil
		},
	})

	input := `<%= divwrapper({"class": "myclass"}) { %>
<ul>
    <% let a = [1, 2, "three", "four"] %>
    <%= for (index, name) in a { %>
        <li><%=index%> - <%=name%></li>
    <% } %>
</ul>
<% } %>`

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Contains(s, "<li>0 - 1</li>")
	r.Contains(s, "<li>1 - 2</li>")
	r.Contains(s, "<li>2 - three</li>")
	r.Contains(s, "<li>3 - four</li>")
}

func Test_Render_Let_Hash(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		input    string
		expected string
	}{
		{"success", true, `<p><% let h = {"a": "A"} %><%= h["a"] %></p>`, "<p>A</p>"},
		{"assign", true, `<p><% let h = {"a": "A"} %><% h["a"] = "C" %><%= h["a"] %></p>`, "<p>C</p>"},
		{"assign", true, `<p><% let h = {"a": "A"} %><% h["b"] = "D" %><%= h["b"] %></p>`, "<p>D</p>"},
		{"intvar", true, `<p><% let h = {"a": "A"} %><% h["b"] = 3 %><%= h["b"] %></p>`, "<p>3</p>"},
		{"invalid", true, `<p><% let h = {"a": "A"} %><% h["b"] = 3 %><%= h["c"] %></p>`, "<p></p>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			s, err := plush.Render(tc.input, plush.NewContext())
			if tc.success {
				r.NoError(err)
			} else {
				r.Error(err)
			}
			r.Equal(tc.expected, s)
		})
	}
}

func Test_Render_Let_Array(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		input    string
		expected string
	}{
		{"success", true, `<p><% let a = [1, 2, "three", "four", 3.75] %><% a[0] = 3 %><%= a[0] %></p>`, "<p>3</p>"},
		{"addition", true, `<p><% let a = [1, 2, "three", "four", 3.75] %><% a[4] = 3 %><%= a[4] + 2 %></p>`, "<p>5</p>"},
		{"invalid_key", false, `<p><% let a = [1, 2, "three", "four", 3.75] %><% a["b"] = 3 %><%= a["c"] %></p>`, ""},
		{"outofbounds_assign", false, `<p><% let a = [1, 2, "three", "four", 3.75] %><% a[5] = 3 %><%= a[4] + 2 %></p>`, ""},
		{"outofbounds_access", false, `<p><% let a = [1, 2, "three", "four", 3.75] %><%= a[5] %></p>`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			s, err := plush.Render(tc.input, plush.NewContext())
			if tc.success {
				r.NoError(err)
			} else {
				r.Error(err)
			}
			r.Equal(tc.expected, s)
		})
	}
}

type Category1 struct {
	Products []Product1
}
type Product1 struct {
	Name []string
}

func Test_Render_Access_CalleeArray(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		expected string
		data     Category1
	}{
		{"success", true, "Buffalo", Category1{
			[]Product1{
				{Name: []string{"Buffalo"}},
			},
		}},
		{"outofbounds", false, "", Category1{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			input := `<% let a = product_listing.Products[0].Name[0] %><%= a  %>`

			ctx := plush.NewContext()
			ctx.Set("product_listing", tc.data)

			s, err := plush.Render(input, ctx)
			if tc.success {
				r.NoError(err)
			} else {
				r.Error(err)
			}
			r.Equal(tc.expected, s)
		})
	}

}
