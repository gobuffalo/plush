package plush_test

import (
	"fmt"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/stretchr/testify/require"
)

func Test_Compiled_Template_Render(t *testing.T) {
	tmpl, err := vmplush.Compile(`<p><%= greet(name) %></p>`)
	require.NoError(t, err)

	out, err := tmpl.Render(rootplush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"greet": func(name string) string {
			return "hi " + name
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "<p>hi Mido</p>", out)
}

func Test_Compiled_Template_Render_Reuses_Bytecode_With_Fresh_Contexts(t *testing.T) {
	tmpl, err := vmplush.Compile(`<%= for (i, item) in items { %><%= prefix %>-<%= i %>:<%= item %>;<% } %>`)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		out, err := tmpl.Render(rootplush.NewContextWith(map[string]interface{}{
			"prefix": fmt.Sprintf("run%d", i),
			"items":  []string{"a", "b"},
		}))
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("run%d-0:a;run%d-1:b;", i, i), out)
	}
}
