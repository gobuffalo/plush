package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_LineNumberErrors(t *testing.T) {
	r := require.New(t)
	input := `<p>
	<%= f.Foo %>
</p>`

	_, err := plush.Render(input, plush.NewContext())
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

	_, err := plush.Render(input, plush.NewContext())
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

	_, err := plush.Parse(input)
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
	ctx := plush.NewContext()
	ctx.Set("numbers", []int{1, 2})
	_, err := plush.Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "line 3:")
}

func Test_LineNumberErrors_MissingKeyword(t *testing.T) {
	r := require.New(t)
	input := `
	
	
	
	
	<%=  (n) in numbers { %>
		<%= n %>
	<% } %>
	`
	ctx := plush.NewContext()
	ctx.Set("numbers", []int{1, 2})
	_, err := plush.Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "line 6:")
}
