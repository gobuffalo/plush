package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Render_Hash_Array_Index(t *testing.T) {
	r := require.New(t)

	input := `<%= m["first"] + " " + m["last"] %>|<%= a[0+1] %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"m": map[string]string{"first": "Mark", "last": "Bates"},
		"a": []string{"john", "paul"},
	}))
	r.NoError(err)
	r.Equal("Mark Bates|paul", s)
}

func Test_Render_HashCall(t *testing.T) {
	r := require.New(t)
	input := `<%= m["a"] %>`
	ctx := NewContext()
	ctx.Set("m", map[string]string{
		"a": "A",
	})
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("A", s)
}

func Test_Render_HashCall_OnAttribute(t *testing.T) {
	r := require.New(t)
	input := `<%= m.MyMap[key] %>`
	ctx := NewContext()
	ctx.Set("m", struct {
		MyMap map[string]string
	}{
		MyMap: map[string]string{"a": "A"},
	})
	ctx.Set("key", "a")
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("A", s)
}

func Test_Render_HashCall_OnAttribute_IntoFunction(t *testing.T) {
	r := require.New(t)
	input := `<%= debug(m.MyMap[key]) %>`
	ctx := NewContext()
	ctx.Set("m", struct {
		MyMap map[string]string
	}{
		MyMap: map[string]string{"a": "A"},
	})
	ctx.Set("key", "a")
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("<pre>A</pre>", s)
}
