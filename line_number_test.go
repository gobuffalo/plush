package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_LineNumberErrors(t *testing.T) {
	r := require.New(t)
	input := `<p>
	<%= f.Foo %>
</p>`

	_, err := Render(input, NewContext())
	r.Error(err)
	r.Contains(err.Error(), "line 2:")
}

func Test_LineNumberErrors_ForLoop(t *testing.T) {
	r := require.New(t)
	input := `
	<%= for (n) in numbers.Foo { %>
		<%= n %>
	<% } %>
	`

	_, err := Render(input, NewContext())
	r.Error(err)
	r.Contains(err.Error(), "line 2:")
}

func Test_LineNumberErrors_ForLoop2(t *testing.T) {
	r := require.New(t)
	input := `
	<%= for (n in numbers.Foo { %>
		<%= if (n == 3) { %>
			<%= n %>
		<% } %>
	<% } %>
	`

	_, err := Parse(input)
	r.Error(err)
	r.Contains(err.Error(), "line 2:")
}

func Test_LineNumberErrors_InsideForLoop(t *testing.T) {
	r := require.New(t)
	input := `
	<%= for (n) in numbers { %>
		<%= n.Foo %>
	<% } %>
	`
	ctx := NewContext()
	ctx.Set("numbers", []int{1, 2})
	_, err := Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "line 3:")
}
