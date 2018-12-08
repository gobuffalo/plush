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
