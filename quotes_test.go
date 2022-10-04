package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Quote_Missing(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"case1", `<%= foo("asdf) %>`},
		{"case2", `<%= foo("test) %>".`},
		{"case3", `<%= title("Running Migrations) %>(default "./migrations")`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx := NewContext()
			ctx.Set("foo", func(string) {})
			_, err := Render(tc.input, ctx)
			r.Error(err)
		})
	}
}
