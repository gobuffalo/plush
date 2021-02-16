package plush

//Test issue  https://github.com/gobuffalo/plush/issues/53

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Return_Exit_With_InfixExpression(t *testing.T) {
	r := require.New(t)
	input := `
	<%
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
		}
	%>
	<%= numberify("one") %>
	`
	s, err := Render(input, NewContext())
	///fmt.Printf("Stack Trace  => %+v \n\n", errors.Cause(err))
	r.NoError(err)
	r.Equal("2", strings.TrimSpace(s))
}

func Test_Simple_Return_Exit(t *testing.T) {
	r := require.New(t)
	input := `
	<%
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
		}
	%>
	<%= numberify("two") %>
	`
	s, err := Render(input, NewContext())
	///fmt.Printf("Stack Trace  => %+v \n\n", errors.Cause(err))
	r.NoError(err)
	r.Equal("445", strings.TrimSpace(s))
}

func Test_Simple_Return_Default(t *testing.T) {
	r := require.New(t)
	input := `
	<%
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
		}
	%>
	<%= numberify("six") %>
	`
	s, err := Render(input, NewContext())
	///fmt.Printf("Stack Trace  => %+v \n\n", errors.Cause(err))
	r.NoError(err)
	r.Equal("unsupported", strings.TrimSpace(s))
}


func Test_User_Function_Return(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
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
	out, err := Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: **********.`, out)

	ctx.Set("data", obj{Secret: true, GiveHint: true, String: "your royal highness"})
	out, err = Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: your roy****.`, out)

	ctx.Set("data", obj{Secret: false, String: "your royal highness"})
	out, err = Render(in, ctx)
	r.NoError(err, "Render")
	r.Equal(`You are: your royal highness.`, out)
}
