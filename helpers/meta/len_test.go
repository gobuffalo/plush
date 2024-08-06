package meta

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Len(t *testing.T) {
	table := []struct {
		in  interface{}
		out int
	}{
		{"foo", 3},
		{[]string{"a", "b"}, 2},
		{nil, 0},
		{map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}, 4},
	}

	for _, tt := range table {
		t.Run(fmt.Sprint(tt.in), func(st *testing.T) {
			r := require.New(st)
			r.Equal(Len(tt.in), tt.out)
		})
	}
}
