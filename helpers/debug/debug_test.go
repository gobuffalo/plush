package debug

import (
	"fmt"
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Debug(t *testing.T) {
	table := []struct {
		in  interface{}
		out string
	}{
		{"foo", "foo"},
	}

	for _, tt := range table {
		t.Run(fmt.Sprint(tt.in), func(st *testing.T) {
			r := require.New(st)
			out := fmt.Sprintf("<pre>%s</pre>", tt.out)
			r.Equal(template.HTML(out), Debug(tt.in))
		})
	}
}
