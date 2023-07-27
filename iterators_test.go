package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_groupByHelper(t *testing.T) {
	r := require.New(t)
	g, err := plush.GroupByHelper(2, []string{"a", "b", "c", "d", "e"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b", "c"}, g1)
	g2 := g.Next()
	r.Equal([]string{"d", "e"}, g2)
	r.Nil(g.Next())
}

func Test_groupByHelper_Exact(t *testing.T) {
	r := require.New(t)
	g, err := plush.GroupByHelper(2, []string{"a", "b"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b"}, g1)
	r.Nil(g.Next())
}

func Test_groupByHelper_Pointer(t *testing.T) {
	r := require.New(t)
	g, err := plush.GroupByHelper(2, &[]string{"a", "b", "c", "d", "e"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b", "c"}, g1)
	g2 := g.Next()
	r.Equal([]string{"d", "e"}, g2)
	r.Nil(g.Next())
}

func Test_groupByHelper_SmallGroup(t *testing.T) {
	r := require.New(t)
	g, err := plush.GroupByHelper(1, []string{"a", "b", "c", "d", "e"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b", "c", "d", "e"}, g1)
	r.Nil(g.Next())
}

func Test_groupByHelper_NonGroupable(t *testing.T) {
	r := require.New(t)
	_, err := plush.GroupByHelper(1, 1)
	r.Error(err)
}

func Test_groupByHelper_ZeroSize(t *testing.T) {
	r := require.New(t)
	_, err := plush.GroupByHelper(0, []string{"a"})
	r.Error(err)
}
