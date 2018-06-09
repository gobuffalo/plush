package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ContentForOf(t *testing.T) {
	r := require.New(t)
	input := `
	<b0><% contentFor("buttons") { %><button>hi</button><% } %></b0>
	<b1><%= contentOf("buttons") %></b1>
	<b2><%= contentOf("buttons") %></b2>
	`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Contains(s, "<b0></b0>")
	r.Contains(s, "<b1><button>hi</button></b1>")
	r.Contains(s, "<b2><button>hi</button></b2>")
}

func Test_ContentForOfWithData(t *testing.T) {
	r := require.New(t)
	input := `
	<b0><% contentFor("buttons") { %><button><%= label %></button><% } %></b0>
	<b1><%= contentOf("buttons", {"label": "Button One"}) %></b1>
	<b2><%= contentOf("buttons", {"label": "Button Two"}) %></b2>
	<b3><%= label %></b3>
	`
	ctx := NewContext()
	ctx.Set("label", "Outer label")
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Contains(s, "<b0></b0>")
	r.Contains(s, "<b1><button>Button One</button></b1>")
	r.Contains(s, "<b2><button>Button Two</button></b2>")
	r.Contains(s, "<b3>Outer label</b3>")
}
