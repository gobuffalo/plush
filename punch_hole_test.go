package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func Test_Render_HolePunching_IntermediateOutput(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("myArray", []string{"a", "b"})

	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %>`

	// Simulate first pass: get output with markers
	tmpl, err := plush.Parse(input)
	r.NoError(err)
	s, holes, err := tmpl.Exec(ctx)
	r.NoError(err)

	// Check that the output contains the expected markers
	r.Len(holes, 2)
	r.Contains(s, "<PLUSH_HOLE_0>")
	r.Contains(s, "<PLUSH_HOLE_1>")
	r.Contains(s, "ab1<PLUSH_HOLE_0>ab1<PLUSH_HOLE_1>")
}

func Test_Render_HolePunching_ErrorInHole(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("myArray", []string{"a", "b"})

	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H hole_punch_first_error" %><%= a %><%H "sssss" %>`
	ss, err := plush.Render(input, ctx)
	r.ErrorContains(err, `line 1: "hole_punch_first_error": unknown identifier`)
	r.Empty(ss)
}
func Test_Render_HolePunching_SecondPass_NoCache(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	ctx.Set("myArray", []string{"a", "b"})
	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab1testingab1sssss`, ss)
}

func Test_Render_HolePunching_MultipleHolesAtEnd(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	ctx.Set("myArray", []string{"a", "b"})
	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %><%H "dddd" %><%H "eeee" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab1testingab1sssssddddeeee`, ss)
}

func Test_Render_HolePunching_HolesAtStart(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	ctx.Set("myArray", []string{"a", "b"})
	input := `<%H "testing" %><% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %><%H "dddd" %><%H "eeee" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`testingab1testingab1sssssddddeeee`, ss)
}
func Test_Render_HolePunching_SecondPass_WithCache(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	cacheFileName := "myfile"

	ctx.Set("myArray", []string{"a", "b"})
	ctx.Set(plush.TemplateFileKey, cacheFileName)

	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)

	r.Equal(`ab1testingab1sssss`, ss, "Free call")

	templ, ok := ff.Get(cacheFileName)
	r.True(ok)
	r.NotEmpty(templ)

	//This should still work and use the cached template even if we update the input
	input = `<% let a = myArray %><% a = a + "2" %><%=a %><%H "testing" %><%= a %><%H "sssss" %>`
	ss, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab1testingab1sssss`, ss, "The output should be the same as the first render since it is cached as the first template")

	ff.Delete(cacheFileName)
	ss, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab2testingab2sssss`, ss)
}

func Test_Render_HolePunching_HoleAtStartAndEnd(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	input := `<%H "start" %><%H "end" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("startend", ss)
}

func Test_Render_HolePunching_EmptyHoleContent(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	input := `<%H "" %>foo<%H  %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("foo", ss)
}

func Test_Render_HolePunching_ManyHoles(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	input := ""
	expected := ""
	for i := 0; i < 100; i++ {
		input += `<%H "x" %>`
		expected += "x"
	}
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(expected, ss)
}

func Test_Render_HolePunching_TemplateContainsHolePunch(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	input := `<PLUSH_HOLE_0><%H "start" %><%H "end" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<PLUSH_HOLE_0>startend", ss)
}
