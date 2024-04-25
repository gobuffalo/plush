package plush_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_Render_Struct_Attribute(t *testing.T) {
	r := require.New(t)
	input := `<%= f.Name %>`
	ctx := plush.NewContext()
	f := struct {
		Name string
	}{"Mark"}
	ctx.Set("f", f)
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Mark", s)
}

func Test_Render_UnknownAttribute_on_Callee(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("m", struct{}{})
	input := `<%= m.Foo %>`
	_, err := plush.Render(input, ctx)
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
	ctx := plush.NewContext()
	bender := Robot{
		Avatar: Avatar("bender.jpg"),
	}
	ctx.Set("robot", bender)
	input := `<%= robot.Avatar.URL() %>`
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("BENDER.JPG", s)
}

func Test_Render_Struct_PointerMethod(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	robot := Robot{name: "robot"}

	t.Run("ByValue", func(t *testing.T) {
		ctx.Set("robot", robot)
		input := `<%= robot.Name() %>`
		s, err := plush.Render(input, ctx)
		r.NoError(err)
		r.Equal("robot", s)
	})
	t.Run("ByPointer", func(t *testing.T) {
		ctx.Set("robot", &robot)
		input := `<%= robot.Name() %>`
		s, err := plush.Render(input, ctx)
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
		ctx := plush.NewContextWith(map[string]interface{}{"p": p})
		res, err := plush.Render(input, ctx)
		r.NoError(err)
		r.Equal(resE[p.N], res)

	}
}

func Test_Render_Struct_PointerValue_Nil(t *testing.T) {
	r := require.New(t)

	type user struct {
		Name  string
		Image *string
	}

	u := user{
		Name:  "Garn Clapstick",
		Image: nil,
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"user": u,
	})
	input := `<%= user.Name %>: <%= user.Image %>`
	res, err := plush.Render(input, ctx)

	r.NoError(err)
	r.Equal(`Garn Clapstick: `, res)
}

func Test_Render_Struct_PointerValue_NonNil(t *testing.T) {
	r := require.New(t)

	type user struct {
		Name  string
		Image *string
	}

	image := "bicep.png"
	u := user{
		Name:  "Scrinch Archipeligo",
		Image: &image,
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"user": u,
	})
	input := `<%= user.Name %>: <%= user.Image %>`
	res, err := plush.Render(input, ctx)

	r.NoError(err)
	r.Equal(`Scrinch Archipeligo: bicep.png`, res)
}

func Test_Render_Struct_Multiple_Access(t *testing.T) {
	r := require.New(t)

	type mylist struct {
		Name string
	}

	input := `<%= myarray[0].Name %> <%= myarray[1].Name %>`

	gg := make([]mylist, 3)
	gg[0].Name = "John"
	gg[1].Name = "Doe"
	ctx := plush.NewContext()
	ctx.Set("myarray", gg)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("John Doe", res)

}

func Test_Render_Nested_Structs_Start_With_Slice(t *testing.T) {
	r := require.New(t)

	type b struct {
		Final string
	}
	type mylist struct {
		Name b
	}

	input := `<%= myarray[0].Name.Final %>`

	gg := make([]mylist, 3)
	gg[0].Name.Final = "Hello World"
	ctx := plush.NewContext()
	ctx.Set("myarray", gg)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Hello World", res)

}

func Test_Render_Nested_Structs_Start_With_Slice_End_With_Slice(t *testing.T) {
	r := require.New(t)
	type b struct {
		A []string
	}
	type mylist struct {
		Name b
	}

	input := `<%= myarray[0].Name.A[0] %>`

	gg := make([]mylist, 3)
	gg[0].Name = b{[]string{"Hello World"}}
	ctx := plush.NewContext()
	ctx.Set("myarray", gg)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Hello World", res)

}
func Test_Render_Nested_Structs_Ends_With_Slice(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	type c struct {
		Final []string
	}

	type b struct {
		B c
	}
	type mylist struct {
		Name b
	}

	bender := mylist{
		Name: b{
			B: c{
				Final: []string{"bendser.jpg"},
			},
		},
	}
	ctx.Set("robot", bender)
	input := `<%= robot.Name.B.Final[0] %>`
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("bendser.jpg", s)
}

func Test_Render_Nested_Structs(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	type c struct {
		Final string
	}

	type b struct {
		B c
	}
	type mylist struct {
		Name b
	}

	bender := mylist{
		Name: b{
			B: c{
				Final: "bendser.jpg",
			},
		},
	}
	ctx.Set("robot", bender)
	input := `<%= robot.Name.B.Final %>`
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("bendser.jpg", s)
}
func Test_Render_Struct_Access_Slice_Field(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	type D struct {
		Final string
	}
	type c struct {
		Final []D
	}

	type b struct {
		B c
	}
	type mylist struct {
		Name b
	}

	bender := mylist{
		Name: b{
			B: c{
				Final: []D{
					{Final: "String"},
				},
			},
		},
	}
	ctx.Set("robot", bender)
	input := `<%= robot.Name.B.Final[0].Final %>`
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("String", s)
}

func Test_Render_Struct_Nested_Slice_Access(t *testing.T) {
	r := require.New(t)
	type d struct {
		Final string
	}
	type c struct {
		He []d
	}
	type b struct {
		A []c
	}
	type mylist struct {
		Name []b
	}

	input := `<%= myarray[0].Name[0].A[1].He[2].Final %>`

	gg := make([]mylist, 3)

	var bc b
	var ca c
	ca.He = make([]d, 3)
	ca.He[2] = d{"Hello World"}
	bc.A = make([]c, 3)
	bc.A[1] = ca
	gg[0].Name = []b{bc}

	ctx := plush.NewContext()
	ctx.Set("myarray", gg)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Hello World", res)

}

func Test_Render_Struct_Nested_Map_Slice_Access(t *testing.T) {
	r := require.New(t)
	type d struct {
		Final string
	}
	type c struct {
		He map[string]d
	}
	type b struct {
		A []c
	}
	type mylist struct {
		Name []b
	}

	input := `<%= myarray[0].Name[0].A[1].He["test"].Final %>`

	gg := make([]mylist, 3)

	var bc b
	var ca c
	ca.He = make(map[string]d)
	ca.He["test"] = d{"Hello World"}
	bc.A = make([]c, 3)
	bc.A[1] = ca
	gg[0].Name = []b{bc}

	ctx := plush.NewContext()
	ctx.Set("myarray", gg)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Hello World", res)

}

func Test_Render_Struct_Nested_With_Unexported_Fields(t *testing.T) {
	r := require.New(t)
	type people struct {
		FirstName string
		LastName  string
	}
	type employee struct {
		employee []people
	}
	departments := make(map[string]employee)
	var joh people

	joh.FirstName = "John"
	joh.LastName = "Doe"

	var jane people

	jane.FirstName = "Jane"
	jane.LastName = "Dolittle"

	var employees employee
	employees.employee = []people{joh, jane}
	departments["HR"] = employees

	input := `<%= departments["HR"].employee[0].FirstName %>`
	ctx := plush.NewContext()
	ctx.Set("departments", departments)
	_, err := plush.Render(input, ctx)
	r.Error(err)
	//r.Equal("John", res)

}

func Test_Render_Struct_Nested_Map_Access(t *testing.T) {
	r := require.New(t)
	type people struct {
		FirstName string
		LastName  string
	}
	type employee struct {
		Employee []people
	}
	departments := make(map[string]employee)
	var joh people

	joh.FirstName = "John"
	joh.LastName = "Doe"

	var jane people

	jane.FirstName = "Jane"
	jane.LastName = "Dolittle"

	var employees employee
	employees.Employee = []people{joh, jane}
	departments["HR"] = employees

	input := `<%= departments["HR"].Employee[0].FirstName %> <%= departments["HR"].Employee[0].LastName %>`
	ctx := plush.NewContext()
	ctx.Set("departments", departments)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("John Doe", res)

	input = `<%= departments["HR"].Employee[1].FirstName %> <%= departments["HR"].Employee[1].LastName %>`
	res, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Jane Dolittle", res)

	input = `<%= departments["HR"].Employee[1].FirstName %> <%= departments["HR"].Employee[0].LastName %>`
	res, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Jane Doe", res)

	input = `<%= departments["HR"].Employee[0].FirstName %> <%= departments["HR"].Employee[1].LastName %>`
	res, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("John Dolittle", res)
}

type person struct {
	likes []string
	hates []string
	born  time.Time
}

func (a person) GetAge() time.Duration {
	return time.Since(a.born)
}

func (a person) GetBorn() time.Time {
	return a.born
}

func (a person) Hates() []string {
	return a.hates
}
func (a person) Likes() []string {
	return a.likes
}

func Test_Render_Struct_With_ChainingFunction_ArrayAccess(t *testing.T) {
	r := require.New(t)

	tt := person{likes: []string{"pringles", "galaxy", "carrot cake", "world pendant", "gold braclet"},
		hates: []string{"boiled eggs", "coconut"}}
	input := `<%= nour.Likes()[0] %>`
	ctx := plush.NewContext()
	ctx.Set("nour", tt)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("pringles", res)
}

func Test_Render_Struct_With_ChainingFunction_ArrayAccess_Outofbound(t *testing.T) {
	r := require.New(t)

	tt := person{likes: []string{"pringles", "galaxy", "carrot cake", "world pendant", "gold bracelet"},
		hates: []string{"boiled eggs", "coconut"}}
	input := `<%= nour.Hates()[30] %>`
	ctx := plush.NewContext()
	ctx.Set("nour", tt)
	_, err := plush.Render(input, ctx)
	r.Error(err)
}

func Test_Render_Struct_With_ChainingFunction_FunctionCall(t *testing.T) {
	r := require.New(t)

	tt := person{born: time.Date(2024, time.January, 11, 0, 0, 0, 0, time.UTC).AddDate(-31, 0, 0)}
	input := `<%= nour.GetBorn().Format("Jan 2, 2006") %>`
	ctx := plush.NewContext()
	ctx.Set("nour", tt)
	res, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("Jan 11, 1993", res)
}

func Test_Render_Struct_With_ChainingFunction_UndefinedStructProperty(t *testing.T) {
	r := require.New(t)

	tt := person{born: time.Now()}
	input := `<%= nour.GetBorn().TEST %>`
	ctx := plush.NewContext()
	ctx.Set("nour", tt)
	_, err := plush.Render(input, ctx)
	r.Error(err)

}

func Test_Render_Struct_With_ChainingFunction_InvalidFunctionCall(t *testing.T) {
	r := require.New(t)

	tt := person{born: time.Now()}
	input := `<%= nour.GetBorn().TEST("Jan 2, 2006") %>`
	ctx := plush.NewContext()
	ctx.Set("nour", tt)
	_, err := plush.Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "'nour.GetBorn' does not have a method named 'TEST' (nour.GetBorn.TEST)")
}

func Test_Render_Function_on_Invalid_Function_Struct(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	bender := Robot{
		Avatar: Avatar("bender.jpg"),
	}
	ctx.Set("robot", bender)
	input := `<%= robot.Avatar.URL2() %>`
	_, err := plush.Render(input, ctx)
	r.Error(err)
}
