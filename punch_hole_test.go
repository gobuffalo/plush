package plush_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func Test_Render_Hole_Punching_Intermediate_Output(t *testing.T) {
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

func Test_Render_Hole_Punching_Concurrency_Limit(t *testing.T) {
	oldLimit := plush.SetPunchHoleConcurrencyLimit(2)
	defer plush.SetPunchHoleConcurrencyLimit(oldLimit)

	holes := make([]plush.HoleMarker, 8)
	for i := range holes {
		holes[i] = plush.NewHoleMarker(plush.PunchHoleMarkerName(i), "hole", -1, -1)
	}

	var active int64
	var maxActive int64
	renderer := func(input string, ctx hctx.Context) (string, error) {
		current := atomic.AddInt64(&active, 1)
		for {
			maximum := atomic.LoadInt64(&maxActive)
			if current <= maximum || atomic.CompareAndSwapInt64(&maxActive, maximum, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt64(&active, -1)
		return input, nil
	}

	rendered := plush.RenderPunchHolesConcurrentlyWith(holes, plush.NewContext(), renderer)
	require.Len(t, rendered, len(holes))
	require.LessOrEqual(t, atomic.LoadInt64(&maxActive), int64(2))
}

func Test_Render_Hole_Punching_Error_In_Hole(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("myArray", []string{"a", "b"})

	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H hole_punch_first_error" %><%= a %><%H "sssss" %>`
	ss, err := plush.Render(input, ctx)
	r.Nil(err)
	r.Contains(ss, `line 1: "hole_punch_first_error": unknown identifier`)
}
func Test_Render_Hole_Punching_Second_Pass_No_Cache(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("myArray", []string{"a", "b"})
	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab1testingab1sssss`, ss)
}

func Test_Render_Hole_Punching_Skeleton_Cache_Invalidates_When_Source_Changes(t *testing.T) {
	r := require.New(t)
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer plush.ClearTemplateCache()

	previous := plush.SetRenderMode(plush.RenderModeInterpreter)
	defer plush.SetRenderMode(previous)

	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, "holes-source-change.plush")

	ss, err := plush.Render(`A<%H "hole" %>B`, ctx)
	r.NoError(err)
	r.Equal(`AholeB`, ss)

	ss, err = plush.Render(`C<%H "hole" %>D`, ctx)
	r.NoError(err)
	r.Equal(`CholeD`, ss)
}

func Test_Render_Hole_Punching_Multiple_Holes_At_End(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)

	ctx.Set("myArray", []string{"a", "b"})
	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %><%H "dddd" %><%H "eeee" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab1testingab1sssssddddeeee`, ss)
}

func Test_Render_Hole_Punching_Holes_At_Start(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)

	ctx.Set("myArray", []string{"a", "b"})
	input := `<%H "testing" %><% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %><%H "dddd" %><%H "eeee" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`testingab1testingab1sssssddddeeee`, ss)
}
func Test_Render_Hole_Punching_Second_Pass_With_Cache(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	cacheFileName := "myfile.plush"

	ctx.Set("myArray", []string{"a", "b"})
	ctx.Set(meta.TemplateFileKey, cacheFileName)

	input := `<% let a = myArray %><% a = a + "1" %><%=a %><%H "testing" %><%= a %><%H "sssss" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)

	r.Equal(`ab1testingab1sssss`, ss, "Free call")
	astKey := "ast:" + cacheFileName
	templ, ok := ff.Get(astKey)
	r.True(ok)
	r.NotEmpty(templ)

	// Source changes under the same filename must invalidate the cached skeleton.
	input = `<% let a = myArray %><% a = a + "2" %><%=a %><%H "testing" %><%= a %><%H "sssss" %>`
	ss, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab2testingab2sssss`, ss)

	ff.Delete(astKey)
	ss, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`ab2testingab2sssss`, ss)
}

func Test_Render_Hole_Punching_Hole_At_Start_And_End(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	input := `<%H "start" %><%H "end" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("startend", ss)
}

func Test_Render_Hole_Punching_Empty_Hole_Content(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	input := `<%H "" %>foo<%H  %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("foo", ss)
}

func Test_Render_Hole_Punching_Many_Holes(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
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

func Test_Render_Hole_Punching_Template_Contains_Hole_Punch(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	input := `<PLUSH_HOLE_0><%H "start" %><%H "end" %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<PLUSH_HOLE_0>startend", ss)
}
func Test_Render_Hole_Punching_In_Block_Statment(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("a", "22")
	input := `<%= if (a == "22") { %><%H "testing" %><% } else { %><%H "dddd" %><% } %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`testing`, ss)
}

func Test_Render_Hole_Punching_In_For_Loop(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("myArray", []string{"a", "b", "c"})
	input := `<%= for (i,v) in myArray { %><%H "testing" %><%= v %><% } %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`testingatestingbtestingc`, ss)
}

func Test_Render_Hole_Punching_For_Loop_As_Hole(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("myArray", []string{"a", "b", "c"})
	input := `<%H for (i,v) in myArray { %><%= v %><% } %>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`abc`, ss)
}

func Test_Render_Hole_Punching_If_Else(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("number", 3)
	input := `<%H if (number == 0){ %><%= "NUMBER" %><% } else { %><%= number %><%  }%>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`3`, ss)
}

func Test_Render_Hole_Punching_If_Truthy_A(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("number", 3)
	input := `<%H if (number > 0){ %><%= "NUMBER" %><% } else { %><%= number %><%  }%>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`NUMBER`, ss)
}
func Test_Render_Hole_Punching_If_Truthy(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	cacheFileName := "myfile.plush"
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	ctx.Set(meta.TemplateFileKey, cacheFileName)
	ctx.Set("number", 3)
	input := `<%H if (number > 0){ %><%= "NUMBER" %><% } else { %><%= number %><%  }%>`
	ss, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal(`NUMBER`, ss)
}
func Test_Partial_Helper_With_Recursion_Hole(t *testing.T) {
	r := require.New(t)

	ctx := plush.NewContext()
	ctx.Set("number", 3)
	ff := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(ff)
	name := "index.plush"
	data := map[string]interface{}{}
	help := plush.HelperContext{Context: ctx}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<%=
		if (number > 0) { %><%
			let number = number - 1 %><%=
			partial("index.plush") %><%H number %>, <%
		} %>`, nil
	})

	html, err := plush.PartialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`1, 2, 3, `, string(html))
	r.Equal(3, help.Value("number"))
}
