package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_MarkdownHelper(t *testing.T) {
	r := require.New(t)
	input := `<%= markdown(m) %>`
	ctx := NewContext()
	ctx.Set("m", "# H1")
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Contains(s, "H1</h1>")
}
