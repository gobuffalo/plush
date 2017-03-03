package plush

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Render_Simple_HTML(t *testing.T) {
	r := require.New(t)

	input := `<p>Hi</p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal(input, s)
}

func Test_Render_Keeps_Spacing(t *testing.T) {
	r := require.New(t)
	input := `<%= greet %> <%= name %>`

	ctx := NewContext()
	ctx.Set("greet", "hi")
	ctx.Set("name", "mark")

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("hi mark", s)
}

func Test_Render_HTML_InjectedString(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "mark" %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>mark</p>", s)
}

func Test_Render_EscapedString(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "<script>alert('pwned')</script>" %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>&lt;script&gt;alert(&#39;pwned&#39;)&lt;/script&gt;</p>", s)
}

func Test_Render_Injected_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"name": "Mark",
	}))
	r.NoError(err)
	r.Equal("<p>Mark</p>", s)
}

func Test_Render_Let_Hash(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><%= h["a"] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>A</p>", s)
}

func Test_Render_Hash_Array_Index(t *testing.T) {
	r := require.New(t)

	input := `<%= m["first"] + " " + m["last"] %>|<%= a[0+1] %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"m": map[string]string{"first": "Mark", "last": "Bates"},
		"a": []string{"john", "paul"},
	}))
	r.NoError(err)
	r.Equal("Mark Bates|paul", s)
}

func Test_Render_Missing_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p></p>", s)
}

func Test_Render_HTML_Escape(t *testing.T) {
	r := require.New(t)

	input := `<%= safe() %>|<%= unsafe() %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"safe": func() string {
			return "<b>unsafe</b>"
		},
		"unsafe": func() template.HTML {
			return "<b>unsafe</b>"
		},
	}))
	r.NoError(err)
	r.Equal("&lt;b&gt;unsafe&lt;/b&gt;|<b>unsafe</b>", s)
}

func Test_Render_ShowNoShow(t *testing.T) {
	r := require.New(t)
	input := `<%= "shown" %><% "notshown" %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("shown", s)
}

func Test_Render_Struct_Attribute(t *testing.T) {
	r := require.New(t)
	input := `<%= f.Name %>`
	ctx := NewContext()
	f := struct {
		Name string
	}{"Mark"}
	ctx.Set("f", f)
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("Mark", s)
}

func Test_Render_ScriptFunction(t *testing.T) {
	r := require.New(t)

	input := `<% let add = fn(x) { return x + 2; }; %><%= add(2) %>`

	s, err := Render(input, NewContext())
	if err != nil {
		r.NoError(err)
	}
	r.Equal("4", s)
}
