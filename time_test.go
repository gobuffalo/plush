package plush

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Default_Time_Format(t *testing.T) {
	r := require.New(t)

	shortForm := "2006-Jan-02"
	tm, err := time.Parse(shortForm, "2013-Feb-03")
	r.NoError(err)
	ctx := NewContext()
	ctx.Set("tm", tm)

	input := `<%= tm %>`

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("February 03, 2013 00:00:00 +0000", s)

	ctx.Set("TIME_FORMAT", "2006-02-Jan")
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("2013-03-Feb", s)

	ctx.Set("tm", &tm)
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("2013-03-Feb", s)
}
