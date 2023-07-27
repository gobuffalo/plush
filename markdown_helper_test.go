package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_MarkdownHelper(t *testing.T) {
	r := require.New(t)
	input := `<%= markdown(m) %>`
	ctx := plush.NewContext()
	ctx.Set("m", "# H1")
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Contains(s, "H1</h1>")
}

func Test_MarkdownHelper_WithBlock(t *testing.T) {
	r := require.New(t)
	input := `<%= markdown("") { return "# H2" } %>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Contains(s, "H2</h1>")
}
