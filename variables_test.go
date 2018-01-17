package plush

import (
	"html/template"
	"strings"
	"testing"

	"github.com/gobuffalo/tags"
	"github.com/pkg/errors"
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

	ctx := NewContext()
	ctx.Set("myArray", []string{"a", "b"})

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("bar\n    \n  \nbaz", strings.TrimSpace(s))
}

func Test_Let_Reassignment_UnknownIdent(t *testing.T) {
	r := require.New(t)
	input := `<% foo = "baz" %>`

	ctx := NewContext()
	ctx.Set("myArray", []string{"a", "b"})

	_, err := Render(input, ctx)
	r.Error(err)
}

func Test_Let_Inside_Helper(t *testing.T) {
	r := require.New(t)
	ctx := NewContextWith(map[string]interface{}{
		"divwrapper": func(opts map[string]interface{}, helper HelperContext) (template.HTML, error) {
			body, err := helper.Block()
			if err != nil {
				return template.HTML(""), errors.WithStack(err)
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

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Contains(s, "<li>0 - 1</li>")
	r.Contains(s, "<li>1 - 2</li>")
	r.Contains(s, "<li>2 - three</li>")
	r.Contains(s, "<li>3 - four</li>")
}

func Test_Render_Let_Hash(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><%= h["a"] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>A</p>", s)
}
