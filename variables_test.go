package plush

import (
	"html/template"
	"strings"
	"testing"

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

func Test_Render_Let_HashAssign(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><% h["a"] = "C"%><%= h["a"] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>C</p>", s)
}

func Test_Render_Let_HashAssign_NewKey(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><% h["b"] = "d" %><%= h["b"] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>d</p>", s)
}

func Test_Render_Let_HashAssign_Int(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><% h["b"] = 3 %><%= h["b"] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>3</p>", s)
}

func Test_Render_Let_HashAssign_InvalidKey(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><% h["b"] = 3 %><%= h["c"] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p></p>", s)
}

func Test_Render_Let_ArrayAssign_InvalidKey(t *testing.T) {
	r := require.New(t)

	input := `<p><% let a = [1, 2, "three", "four", 3.75] %><% a["b"] = 3 %><%= a["c"] %></p>`
	_, err := Render(input, NewContext())
	r.Error(err)
}

func Test_Render_Let_ArrayAssign_ValidIndex(t *testing.T) {
	r := require.New(t)

	input := `<p><% let a = [1, 2, "three", "four", 3.75] %><% a[0] = 3 %><%= a[0] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>3</p>", s)
}

func Test_Render_Let_ArrayAssign_Resultaddition(t *testing.T) {
	r := require.New(t)

	input := `<p><% let a = [1, 2, "three", "four", 3.75] %><% a[4] = 3 %><%= a[4] + 2 %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>5</p>", s)
}

func Test_Render_Let_ArrayAssign_OutofBoundsIndex(t *testing.T) {
	r := require.New(t)

	input := `<p><% let a = [1, 2, "three", "four", 3.75] %><% a[5] = 3 %><%= a[4] + 2 %></p>`
	_, err := Render(input, NewContext())
	r.Error(err)
}

func Test_Render_Access_Array_OutofBoundsIndex(t *testing.T) {
	r := require.New(t)

	input := `<% let a = [1, 2, "three", "four", 3.75] %><%= a[5]  %>`
	_, err := Render(input, NewContext())
	r.Error(err)
}
