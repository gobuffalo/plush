package text

import (
	"testing"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/stretchr/testify/require"
)

func Test_Truncate(t *testing.T) {
	var runesS []rune
	var runesX []rune

	r := require.New(t)
	x := "世界uFHyyImKUMhSkSolLqgqevKQNZUjpSZokrGbZqnUrUnWrTDwi"
	s := Truncate(x, hctx.Map{})
	runesS = []rune(s)
	r.Equal(len(runesS), 50)
	r.Equal("...", string(runesS[47:]))

	s = Truncate(x, hctx.Map{
		"size": 10,
	})
	runesS = []rune(s)
	r.Equal(len(runesS), 10)
	r.Equal("...", string(runesS[7:]))

	s = Truncate(x, hctx.Map{
		"size":  10,
		"trail": "世界re",
	})
	runesS = []rune(s)
	r.Equal(len(runesS), 10)
	r.Equal("世界re", string(runesS[6:]))

	// Case size < len(trail)
	s = Truncate(x, hctx.Map{
		"size":  3,
		"trail": "世界re",
	})
	runesS = []rune(s)
	r.Equal(len(runesS), 4)
	r.Equal("世界re", s)

	// Case size >= len(string)
	s = Truncate(x, hctx.Map{
		"size": len(x),
	})
	runesS = []rune(s)
	runesX = []rune(x)
	r.Equal(len(runesS), len(runesX))
	r.Equal(string(runesX[48:]), string(runesS[48:]))
}
