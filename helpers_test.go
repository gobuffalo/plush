package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Helpers_WithoutData(t *testing.T) {
	type data map[string]interface{}
	r := require.New(t)

	table := []struct {
		I string
		E string
	}{
		{I: `<%= foo() {return bar + name} %>`, E: "BARunknown"},
		{I: `<%= foo({name: "mark"}) {return bar + name} %>`, E: "BARmark"},
	}

	for _, tt := range table {
		ctx := NewContext()
		ctx.Set("name", "unknown")
		ctx.Set("bar", "BAR")
		ctx.Set("foo", func(d data, help HelperContext) (string, error) {
			c := help.New()
			if n, ok := d["name"]; ok {
				c.Set("name", n)
			}
			return help.BlockWith(c)
		})
		s, err := Render(tt.I, ctx)
		r.NoError(err)
		r.Equal(tt.E, s)
	}

}
