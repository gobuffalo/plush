package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Render_If(t *testing.T) {
	r := require.New(t)
	input := `<% if (true) { return "hi"} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_If_Return(t *testing.T) {
	r := require.New(t)
	input := `<%= if (true) { return "hi"} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_Return_HTML(t *testing.T) {
	r := require.New(t)
	input := `<%= if (true) { %>hi<%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_And(t *testing.T) {
	r := require.New(t)
	input := `<%= if (false && true) { %> hi <%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_If_And_True_True(t *testing.T) {
	r := require.New(t)
	input := `<%= if (2 == 2 && 1 == 1) { return "hi" } %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_Or(t *testing.T) {
	r := require.New(t)
	input := `<%= if (false || true) { %>hi<%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_Or_False_False(t *testing.T) {
	r := require.New(t)
	input := `<%= if (1 == 2 || 2 == 1) { return "hi" } else { return "bye" } %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("bye", s)
}

func Test_Render_If_Nil(t *testing.T) {
	r := require.New(t)
	input := `<%= if (names && len(names) >= 1) { %>hi<%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_If_NotNil(t *testing.T) {
	r := require.New(t)
	input := `<%= if (!names) { %>hi<%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_Nil_Else(t *testing.T) {
	r := require.New(t)
	input := `<%= if (names && len(names) >= 1) { %>hi<%} else { %>something else<% } %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("something else", s)
}

func Test_Render_If_Else_Return(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (false) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>bye</p>", s)
}

func Test_Render_If_LessThan(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (1 < 2) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_Render_If_BangFalse(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (!false) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_Render_If_NotEq(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (1 != 2) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_Render_If_GtEq(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (1 >= 2) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>bye</p>", s)
}

func Test_Render_If_Else_True(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (true) { %>hi<% } else { %>bye<% } %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_Render_If_Else_If_Else_True(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
	input := `<p><%= if (state == "foo") { %>hi foo<% } else if (state == "bar") { %>hi bar<% } else if (state == "fizz") { %>hi fizz<% } else { %>hi buzz<% } %></p>`

	ctx.Set("state", "foo")
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi foo</p>", s)

	ctx.Set("state", "bar")
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi bar</p>", s)

	ctx.Set("state", "fizz")
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi fizz</p>", s)

	ctx.Set("state", "buzz")
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi buzz</p>", s)
}

func Test_Render_If_Matches(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if ("foo" ~= "bar") { return "hi" } else { return "bye" } %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>bye</p>", s)
}

func Test_If_String_Truthy(t *testing.T) {
	r := require.New(t)

	ctx := NewContext()
	ctx.Set("username", "")

	input := `<p><%= if (username && username != "") { return "hi" } else { return "bye" } %></p>`
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>bye</p>", s)

	ctx.Set("username", "foo")
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}
