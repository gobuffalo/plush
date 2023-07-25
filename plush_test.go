package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_Render_Simple_HTML(t *testing.T) {
	r := require.New(t)

	input := `<p>Hi</p>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal(input, s)
}

func Test_Render_Keeps_Spacing(t *testing.T) {
	r := require.New(t)
	input := `<%= greet %> <%= name %>`

	ctx := plush.NewContext()
	ctx.Set("greet", "hi")
	ctx.Set("name", "mark")

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("hi mark", s)
}

func Test_Render_HTML_InjectedString(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "mark" %></p>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("<p>mark</p>", s)
}

func Test_Render_Injected_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	s, err := plush.Render(input, plush.NewContextWith(map[string]interface{}{
		"name": "Mark",
	}))
	r.NoError(err)
	r.Equal("<p>Mark</p>", s)
}

func Test_Render_Missing_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	_, err := plush.Render(input, plush.NewContext())
	r.Error(err)
}

func Test_Render_ShowNoShow(t *testing.T) {
	r := require.New(t)
	input := `<%= "shown" %><% "notshown" %>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("shown", s)
}

func Test_Render_ScriptFunction(t *testing.T) {
	r := require.New(t)

	input := `<% let add = fn(x) { return x + 2; }; %><%= add(2) %>`

	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("4", s)
}

func Test_Render_HasBlock(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("blockCheck", func(help plush.HelperContext) string {
		if help.HasBlock() {
			s, _ := help.Block()
			return s
		}
		return "no block"
	})
	input := `<%= blockCheck() {return "block"} %>|<%= blockCheck() %>`
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("block|no block", s)
}

func Test_Render_Dash_in_Helper(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContextWith(map[string]interface{}{
		"my-helper": func() string {
			return "hello"
		},
	})
	s, err := plush.Render(`<%= my-helper() %>`, ctx)
	r.NoError(err)
	r.Equal("hello", s)
}

func Test_BuffaloRenderer(t *testing.T) {
	r := require.New(t)
	input := `<%= foo() %><%= name %>`
	data := map[string]interface{}{
		"name": "Ringo",
	}
	helpers := map[string]interface{}{
		"foo": func() string {
			return "George"
		},
	}
	s, err := plush.BuffaloRenderer(input, data, helpers)
	r.NoError(err)
	r.Equal("GeorgeRingo", s)
}

func Test_Helper_Nil_Arg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(nil, "k") %><%= foo(one, "k") %>`
	ctx := plush.NewContextWith(map[string]interface{}{
		"one": map[string]string{
			"k": "test",
		},
		"foo": func(a map[string]string, b string) string {
			if a != nil {
				return a[b]
			}
			return ""
		},
	})
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("test", s)
}

func Test_UndefinedArg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(bar) %>`
	ctx := plush.NewContext()
	ctx.Set("foo", func(string) {})

	_, err := plush.Render(input, ctx)
	r.Error(err)
	r.Equal(`line 1: "bar": unknown identifier`, err.Error())
}

func Test_Caching(t *testing.T) {
	r := require.New(t)

	template, err := plush.NewTemplate("<%= \"AA\" %>")
	r.NoError(err)

	plush.CacheSet("<%= a %>", template)
	plush.CacheEnabled = true

	tc, err := plush.Parse("<%= a %>")
	r.NoError(err)
	r.Equal(tc, template)

	plush.CacheEnabled = false
	tc, err = plush.Parse("<%= a %>")
	r.NoError(err)
	r.NotEqual(tc, template)
}
