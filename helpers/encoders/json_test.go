package encoders

import (
	"fmt"
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ToJSON(t *testing.T) {
	x := struct {
		A string
		B int
		C bool
	}{"A", 42, true}

	f := func() {}

	table := []struct {
		in  interface{}
		out string
		err bool
	}{
		{"foo", `"foo"`, false},
		{[]string{"foo", "bar"}, `["foo","bar"]`, false},
		{x, `{"A":"A","B":42,"C":true}`, false},
		{nil, "null", false},
		{f, "", true},
	}

	for _, tt := range table {
		t.Run(fmt.Sprint(tt.in), func(st *testing.T) {
			r := require.New(st)
			h, err := ToJSON(tt.in)
			if tt.err {
				r.Error(err)
				return
			}
			r.NoError(err)
			r.Equal(template.HTML(tt.out), h)
		})
	}
}
