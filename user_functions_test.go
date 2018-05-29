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
		if (obj.Secret) {
			return "**********"
		}
		return obj.String
	}

%>You are: <%= print(data) %>.`
	type obj struct {
		Secret bool
		String string
	}
	ctx.Set("data", obj{Secret: true, String: "your royal highness"})
	out, err := Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: **********.`, out)
}
