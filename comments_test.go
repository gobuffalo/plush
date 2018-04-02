package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Comment(t *testing.T) {
	r := require.New(t)
	input := `
	<%# This is a comment %>
	Hi
	`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Contains(s, "Hi")
	r.NotContains(s, "This is a comment")
}

func Test_BlockComment(t *testing.T) {
	r := require.New(t)
	input := `
	<%# This is a 
	block comment %>
	Hi
	`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Contains(s, "Hi")
	r.NotContains(s, "This is a")
	r.NotContains(s, "block comment")
}
