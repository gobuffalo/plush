package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Context_Set(t *testing.T) {
	r := require.New(t)
	c := NewContext()
	r.Nil(c.Value("foo"))
	c.Set("foo", "bar")
	r.NotNil(c.Value("foo"))
}

func Test_Context_Get(t *testing.T) {
	r := require.New(t)
	c := NewContext()
	r.Nil(c.Value("foo"))
	c.Set("foo", "bar")
	r.Equal("bar", c.Value("foo"))
}

func Test_NewSubContext_Set(t *testing.T) {
	r := require.New(t)

	c := NewContext()
	r.Nil(c.Value("foo"))

	sc := c.New()
	r.Nil(sc.Value("foo"))
	sc.Set("foo", "bar")
	r.Equal("bar", sc.Value("foo"))

	r.Nil(c.Value("foo"))
}

func Test_NewSubContext_Get(t *testing.T) {
	r := require.New(t)

	c := NewContext()
	c.Set("foo", "bar")

	sc := c.New()
	r.Equal("bar", sc.Value("foo"))
}
