package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserFunctions(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
	in := `<%

	let print = fn(obj) {
		if (obj.AllCaps) {
			return capitalize(obj.String)
		}
		return obj.String
	}

%>You are: <%= print(data) %>.`
	type obj struct {
		AllCaps bool
		String  string
	}
	ctx.Set("data", obj{AllCaps: true, String: "your royal highness"})
	out, err := Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: YOUR ROYAL HIGHNESS.`, out)
}
