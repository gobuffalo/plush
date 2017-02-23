package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Context_Set(t *testing.T) {
	r := require.New(t)
	c := NewContext()
	r.Nil(c.Get("foo"))
	c.Set("foo", "bar")
	r.NotNil(c.Get("foo"))
}

func Test_Context_Get(t *testing.T) {
	r := require.New(t)
	c := NewContext()
	r.Nil(c.Get("foo"))
	c.Set("foo", "bar")
	r.Equal("bar", c.Get("foo"))
}

func Test_NewSubContext_Set(t *testing.T) {
	r := require.New(t)

	c := NewContext()
	r.Nil(c.Get("foo"))

	sc := c.New()
	r.Nil(sc.Get("foo"))
	sc.Set("foo", "bar")
	r.Equal("bar", sc.Get("foo"))

	r.Nil(c.Get("foo"))
}

func Test_NewSubContext_Get(t *testing.T) {
	r := require.New(t)

	c := NewContext()
	c.Set("foo", "bar")

	sc := c.New()
	r.Equal("bar", sc.Get("foo"))
}
