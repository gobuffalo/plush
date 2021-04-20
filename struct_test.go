package plush

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Render_Struct_Attribute(t *testing.T) {
	r := require.New(t)
	input := `<%= f.Name %>`
	ctx := NewContext()
	f := struct {
		Name string
	}{"Mark"}
	ctx.Set("f", f)
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("Mark", s)
}

func Test_Render_UnknownAttribute_on_Callee(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
	ctx.Set("m", struct{}{})
	input := `<%= m.Foo %>`
	_, err := Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "'m' does not have a field or method named 'Foo' (m.Foo)")
}

type Robot struct {
	Avatar Avatar
	name   string
}

type Avatar string

func (a Avatar) URL() string {
	return strings.ToUpper(string(a))
}

func (r *Robot) Name() string {
	return r.name
}

func Test_Render_Function_on_sub_Struct(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
	bender := Robot{
		Avatar: Avatar("bender.jpg"),
	}
	ctx.Set("robot", bender)
	input := `<%= robot.Avatar.URL() %>`
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("BENDER.JPG", s)
}

func Test_Render_Struct_PointerMethod(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
	robot := Robot{name: "robot"}

	t.Run("ByValue", func(t *testing.T) {
		ctx.Set("robot", robot)
		input := `<%= robot.Name() %>`
		s, err := Render(input, ctx)
		r.NoError(err)
		r.Equal("robot", s)
	})
	t.Run("ByPointer", func(t *testing.T) {
		ctx.Set("robot", &robot)
		input := `<%= robot.Name() %>`
		s, err := Render(input, ctx)
		r.NoError(err)
		r.Equal("robot", s)
	})
}

func Test_Render_Struct_PointerMethod_IsNil(t *testing.T) {
	r := require.New(t)

	type mylist struct {
		N    int
		Next *mylist
	}

	input := `Current number is <%= p.N %>.<%= if (p.Next) { %>  Next up is <%= p.Next.N %>.<% } %>`
	first := &mylist{N: 0}
	last := first

	for i := 0; i < 5; i++ {
		last.Next = &mylist{N: i + 1}
		last = last.Next
	}

	resE := []string{
		"Current number is 0.  Next up is 1.",
		"Current number is 1.  Next up is 2.",
		"Current number is 2.  Next up is 3.",
		"Current number is 3.  Next up is 4.",
		"Current number is 4.  Next up is 5.",
		"Current number is 5.",
	}

	for p := first; p != nil; p = p.Next {

		ctx := NewContextWith(map[string]interface{}{"p": p})
		res, err := Render(input, ctx)
		r.NoError(err)
		r.Equal(resE[p.N], res)

	}
}
