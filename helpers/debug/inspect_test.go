package debug

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Inspect(t *testing.T) {
	r := require.New(t)
	s := struct {
		Name string
	}{"Ringo"}

	o := Inspect(s)
	r.Contains(o, "Ringo")
}
