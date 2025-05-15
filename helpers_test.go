package plush_test

import (
	"errors"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func Test_Helpers_WithoutData(t *testing.T) {
	type data map[string]interface{}
	r := require.New(t)

	table := []struct {
		I   string
		E   string
		Err error
	}{
		{I: `<%= foo() {return bar + name} %>`, E: "BARunknown"},
		{I: `<%= foo({name: "mark"}) {return bar + name} %>`, E: "BARmark"},
		{I: `<%= foo({name: "mark", bbb: "hello-world"}) {return bar + name} %><%= bbb %>`, Err: errors.New(`line 1: "bbb": unknown identifier`)},
	}

	for _, tt := range table {
		ctx := plush.NewContext()
		ctx.Set("name", "unknown")
		ctx.Set("bar", "BAR")
		ctx.Set("foo", func(d data, help plush.HelperContext) (string, error) {
			c := help.New()
			if n, ok := d["name"]; ok {
				c.Set("name", n)
			}
			if n, ok := d["bbb"]; ok {
				c.Set("bbb", n)
			}
			return help.BlockWith(c)
		})
		s, err := plush.Render(tt.I, ctx)
		if tt.Err == nil {
			r.NoError(err)
			r.Equal(tt.E, s)
		} else {
			r.Error(err)
			r.EqualError(err, tt.Err.Error())
		}
	}
}
