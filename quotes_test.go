package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_MissingQuote(t *testing.T) {
	r := require.New(t)
	input := `<%= foo("asdf) %>`
	ctx := NewContext()
	ctx.Set("foo", func(string) {})
	_, err := Render(input, ctx)
	r.Error(err)
}

func Test_MissingQuote_Variant(t *testing.T) {
	r := require.New(t)
	input := `<%= foo("test) %>".`
	ctx := NewContext()
	ctx.Set("foo", func(string) {})
	_, err := Render(input, ctx)
	r.Error(err)
}

func Test_MissingQuote_Variant2(t *testing.T) {
	r := require.New(t)
	input := `<%= title("Running Migrations) %>(default "./migrations")`
	ctx := NewContext()
	ctx.Set("foo", func(string) {})
	_, err := Render(input, ctx)
	r.Error(err)
}
