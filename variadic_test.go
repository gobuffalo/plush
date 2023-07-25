package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_VariadicHelper(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(1, 2, 3) %>`
	ctx := plush.NewContext()
	ctx.Set("foo", func(args ...int) int {
		return len(args)
	})

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("3", s)
}

func Test_VariadicHelper_SecondArg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo("hello") %>`
	ctx := plush.NewContext()
	ctx.Set("foo", func(s string, args ...interface{}) string {
		return s
	})

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("hello", s)
}

func Test_VariadicHelperNoParam(t *testing.T) {
	r := require.New(t)
	input := `<%= foo() %>`
	ctx := plush.NewContext()
	ctx.Set("foo", func(args ...int) int {
		return len(args)
	})

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("0", s)
}

func Test_VariadicHelperNoVariadicParam(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(1) %>`
	ctx := plush.NewContext()
	ctx.Set("foo", func(a int, args ...int) int {
		return a + len(args)
	})

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("1", s)
}

func Test_VariadicHelperWithWrongParam(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(1, 2, "test") %>`
	ctx := plush.NewContext()
	ctx.Set("foo", func(args ...int) int {
		return len(args)
	})

	_, err := plush.Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "test (string) is an invalid argument for foo at pos 2: expected (int)")
}
