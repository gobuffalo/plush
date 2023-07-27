package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_Render_Hash_Key_Interface(t *testing.T) {
	r := require.New(t)

	input := `<%= m["first"]%>`
	s, err := plush.Render(input, plush.NewContextWith(map[string]interface{}{

		"m": map[interface{}]bool{"first": true},
	}))
	r.NoError(err)
	r.Equal("true", s)
}

func Test_Render_Hash_Key_Int_With_String_Index(t *testing.T) {
	r := require.New(t)

	input := `<%= m["first"]%>`
	_, err := plush.Render(input, plush.NewContextWith(map[string]interface{}{

		"m": map[int]bool{0: true},
	}))

	errStr := "line 1: cannot use first (string constant) as int value in map index"
	r.Error(err)
	r.Equal(errStr, err.Error())

}

func Test_Render_Hash_Array_Index(t *testing.T) {
	r := require.New(t)

	input := `<%= m["first"] + " " + m["last"] %>|<%= a[0+1] %>`
	s, err := plush.Render(input, plush.NewContextWith(map[string]interface{}{
		"m": map[string]string{"first": "Mark", "last": "Bates"},
		"a": []string{"john", "paul"},
	}))
	r.NoError(err)
	r.Equal("Mark Bates|paul", s)
}

func Test_Render_HashCall(t *testing.T) {
	r := require.New(t)
	input := `<%= m["a"] %>`
	ctx := plush.NewContext()
	ctx.Set("m", map[string]string{
		"a": "A",
	})
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("A", s)
}

func Test_Render_HashCall_OnAttribute(t *testing.T) {
	r := require.New(t)
	input := `<%= m.MyMap[key] %>`
	ctx := plush.NewContext()
	ctx.Set("m", struct {
		MyMap map[string]string
	}{
		MyMap: map[string]string{"a": "A"},
	})
	ctx.Set("key", "a")
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("A", s)
}

func Test_Render_HashCall_OnAttribute_IntoFunction(t *testing.T) {
	r := require.New(t)
	input := `<%= debug(m.MyMap[key]) %>`
	ctx := plush.NewContext()
	ctx.Set("m", struct {
		MyMap map[string]string
	}{
		MyMap: map[string]string{"a": "A"},
	})
	ctx.Set("key", "a")
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<pre>A</pre>", s)
}
