package plush

import (
	"testing"

	"github.com/markbates/going/randx"
	"github.com/stretchr/testify/require"
)

func Test_truncateHelper(t *testing.T) {
	r := require.New(t)
	x := randx.String(51)
	s := truncateHelper(x, map[string]interface{}{})
	r.Len(s, 50)
	r.Equal("...", s[47:])

	s = truncateHelper(x, map[string]interface{}{
		"size": 10,
	})
	r.Len(s, 10)
	r.Equal("...", s[7:])

	s = truncateHelper(x, map[string]interface{}{
		"size":  10,
		"trail": "more",
	})
	r.Len(s, 10)
	r.Equal("more", s[6:])

	// Case size < len(trail)
	s = truncateHelper(x, map[string]interface{}{
		"size":  3,
		"trail": "more",
	})
	r.Len(s, 4)
	r.Equal("more", s)
}

func Test_inspectHelper(t *testing.T) {
	r := require.New(t)
	s := struct {
		Name string
	}{"Ringo"}

	o := inspectHelper(s)
	r.Contains(o, "Ringo")
}
