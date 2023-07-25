package plush_test

import (
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_Return_Exit_With__InfixExpression(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		expected string
		input    string
	}{
		{"infix_expression", true, "2", `<%
		let numberify = fn(arg) {
			if (arg == "one") {
				return 1+1;
			}
			if (arg == "two") {
				return 44;
			}
			if (arg == "three") {
				return 2;
			}
			return "unsupported"
		} %>
		<%= numberify("one") %>`},
		{"simple_return", true, "445", `<%
		let numberify = fn(arg) {
			if (arg == "one") {
				return 1;
			}
			if (arg == "two") {
				return 445;
			}
			if (arg == "three") {
				return 3;
			}
			return "unsupported"
		} %>
		<%= numberify("two") %>`},
		{"default_return", true, "default value", `<%
		let numberify = fn(arg) {
			if (arg == "one") {
				return 1;
			}
			if (arg == "two") {
				return 445;
			}
			if (arg == "three") {
				return 3;
			}
			return "default value"
		} %>
		<%= numberify("six") %>`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)

			s, err := plush.Render(tc.input, plush.NewContext())
			if tc.success {
				r.NoError(err)
			} else {
				r.Error(err)
			}
			r.Equal(tc.expected, strings.TrimSpace(s))
		})
	}
}

func Test_User_Function_Return(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	in := `<%
 	let print = fn(obj) {
 		if (obj.Secret) {
 			if (obj.GiveHint) {
 				return truncate(obj.String, {size: 12, trail: "****"})
 			}
 			return "**********"
 		}
 		return obj.String
 	}
 	%>You are: <%= print(data) %>.`

	type obj struct {
		Secret   bool
		GiveHint bool
		String   string
	}

	ctx.Set("data", obj{Secret: true, String: "your royal highness"})
	out, err := plush.Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: **********.`, out)

	ctx.Set("data", obj{Secret: true, GiveHint: true, String: "your royal highness"})
	out, err = plush.Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: your roy****.`, out)

	ctx.Set("data", obj{Secret: false, String: "your royal highness"})
	out, err = plush.Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: your royal highness.`, out)
}
