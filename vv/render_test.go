package vv

import (
	"errors"
	"fmt"
	"html/template"
	"testing"

	"github.com/gobuffalo/velvet"
	"github.com/stretchr/testify/require"
)

func Test_Render(t *testing.T) {
	r := require.New(t)

	input := `<p>Hi</p>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal(input, s)
}

func Test_Render2(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "mark" %></p>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("<p>mark</p>", s)
}

func Test_Render3(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "<script>alert('pwned')</script>" %></p>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("<p>&lt;script&gt;alert(&#39;pwned&#39;)&lt;/script&gt;</p>", s)
}

func Test_Render4(t *testing.T) {
	r := require.New(t)

	input := `<p><%= 1 + 3 %></p>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("<p>4</p>", s)
}

func Test_Render5(t *testing.T) {
	r := require.New(t)

	input := `<p><%= 1.1 + 3.1 %></p>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("<p>4.2</p>", s)
}

func Test_Render6(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	s, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"name": "Mark",
	}))
	r.NoError(err)
	r.Equal("<p>Mark</p>", s)
}

func Test_Render7(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><%= h["a"] %> </p>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("<p>A</p>", s)
}

func Test_Render8a(t *testing.T) {
	r := require.New(t)

	input := `<%= "a"  + "b" %>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("ab", s)
}

func Test_Render8b(t *testing.T) {
	r := require.New(t)

	input := `<%= "a"  + 1 %>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("a1", s)
}

func Test_Render8c(t *testing.T) {
	r := require.New(t)

	input := `<%= "a" + "b" + "c" %>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render8d(t *testing.T) {
	r := require.New(t)

	input := `<%= "a" + "b" + "c" + "d" %>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("abcd", s)
}

func Test_Render8e(t *testing.T) {
	r := require.New(t)

	input := `<%= true + 1 %>`
	_, err := Render(input, velvet.NewContext())
	r.Error(err)
}

func Test_Render9(t *testing.T) {
	r := require.New(t)

	input := `<%= m["first"] + " " + m["last"] %>|<%= a[0+1] %>`
	s, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"m": map[string]string{"first": "Mark", "last": "Bates"},
		"a": []string{"john", "paul"},
	}))
	r.NoError(err)
	r.Equal("Mark Bates|paul", s)
}

func Test_Render10(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	s, err := Render(input, velvet.NewContext())
	r.NoError(err)
	r.Equal("<p></p>", s)
}

func Test_Render11(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f() %></p>`
	s, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"f": func() string {
			return "hi!"
		},
	}))
	r.NoError(err)
	r.Equal("<p>hi!</p>", s)
}

func Test_Render12(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f("mark") %></p>`
	s, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"f": func(s string) string {
			return fmt.Sprintf("hi %s!", s)
		},
	}))
	r.NoError(err)
	r.Equal("<p>hi mark!</p>", s)
}

func Test_Render13(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f(name) %></p>`
	s, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"f": func(s string) string {
			return fmt.Sprintf("hi %s!", s)
		},
		"name": "mark",
	}))
	r.NoError(err)
	r.Equal("<p>hi mark!</p>", s)
}

func Test_Render14(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f({"name": name}) %></p>`
	s, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"f": func(m map[string]interface{}) string {
			return fmt.Sprintf("hi %s!", m["name"])
		},
		"name": "mark",
	}))
	r.NoError(err)
	r.Equal("<p>hi mark!</p>", s)
}

func Test_Render15(t *testing.T) {
	r := require.New(t)

	input := `<%= safe() %>|<%= unsafe() %>`
	s, err := Render(input, velvet.NewContextWith(map[string]interface{}{
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

func Test_Render16(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f() %></p>`
	_, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"f": func() (string, error) {
			return "hi!", errors.New("oops!")
		},
	}))
	r.Error(err)
}

type greeter struct{}

func (g greeter) Greet(s string) string {
	return fmt.Sprintf("hi %s!", s)
}

func Test_Render17(t *testing.T) {
	r := require.New(t)

	input := `<p><%= g.Greet("mark") %></p>`
	_, err := Render(input, velvet.NewContextWith(map[string]interface{}{
		"g": greeter{},
	}))
	r.Error(err)
}
