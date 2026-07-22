package iterators

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Group_By(t *testing.T) {
	r := require.New(t)
	g, err := GroupBy(2, []string{"a", "b", "c", "d", "e"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b", "c"}, g1)
	g2 := g.Next()
	r.Equal([]string{"d", "e"}, g2)
	r.Nil(g.Next())
}

func Test_Group_By_Exact(t *testing.T) {
	r := require.New(t)
	g, err := GroupBy(2, []string{"a", "b"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b"}, g1)
	r.Nil(g.Next())
}

func Test_Group_By_Pointer(t *testing.T) {
	r := require.New(t)
	g, err := GroupBy(2, &[]string{"a", "b", "c", "d", "e"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b", "c"}, g1)
	g2 := g.Next()
	r.Equal([]string{"d", "e"}, g2)
	r.Nil(g.Next())
}

func Test_Group_By_Small_Group(t *testing.T) {
	r := require.New(t)
	g, err := GroupBy(1, []string{"a", "b", "c", "d", "e"})
	r.NoError(err)
	g1 := g.Next()
	r.Equal([]string{"a", "b", "c", "d", "e"}, g1)
	r.Nil(g.Next())
}

func Test_Group_By_Non_Groupable(t *testing.T) {
	r := require.New(t)
	_, err := GroupBy(1, 1)
	r.Error(err)
}

func Test_Group_By_Zero_Size(t *testing.T) {
	r := require.New(t)
	_, err := GroupBy(0, []string{"a"})
	r.Error(err)
}
