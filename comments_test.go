package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_Comment(t *testing.T) {
	r := require.New(t)
	input := map[string]string{
		"test1": `
		<%# this is a comment %>
		Hi
		`,
		"test2": `
		<% <%# this is a comment %> %>
		Hi
		`,
	}

	for _, test := range input {
		s, err := plush.Render(test, plush.NewContext())
		r.NoError(err)
		r.Contains(s, "Hi")
		r.NotContains(s, "this is a comment")
	}
}

func Test_BlockComment(t *testing.T) {
	r := require.New(t)
	input := map[string]string{
		"test1": `
		<%# this is 
		a block comment %>
		Hi
		`,
		"test2": `
		<% <%# this is 
		a block comment %> %>
		Hi`,
	}

	for _, test := range input {
		s, err := plush.Render(test, plush.NewContext())
		r.NoError(err)
		r.Contains(s, "Hi")
		r.NotContains(s, []string{"this is", "a block comment"})
	}
}
