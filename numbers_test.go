package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

// support identifiers containing digits, but not starting with a digits
func Test_Identifiers_With_Digits(t *testing.T) {
	r := require.New(t)
	input := `<%= my123greet %> <%= name3 %>`

	ctx := plush.NewContext()
	ctx.Set("my123greet", "hi")
	ctx.Set("name3", "mark")

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("hi mark", s)
}

func Test_Render_Var_ends_in_Number(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContextWith(map[string]interface{}{
		"myvar1": []string{"john", "paul"},
	})
	s, err := plush.Render(`<%= for (n) in myvar1 {return n}`, ctx)
	r.NoError(err)
	r.Equal("johnpaul", s)
}

func Test_Render_AllowsManyNumericTypes(t *testing.T) {
	r := require.New(t)
	input := `<%= i32 %> <%= u32 %> <%= i8 %>`

	ctx := plush.NewContext()
	ctx.Set("i32", int32(1))
	ctx.Set("u32", uint32(2))
	ctx.Set("i8", int8(3))

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("1 2 3", s)
}
