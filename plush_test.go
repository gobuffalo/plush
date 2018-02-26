package plush

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func Test_Render_Simple_HTML(t *testing.T) {
	r := require.New(t)

	input := `<p>Hi</p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal(input, s)
}

func Test_Render_Keeps_Spacing(t *testing.T) {
	r := require.New(t)
	input := `<%= greet %> <%= name %>`

	ctx := NewContext()
	ctx.Set("greet", "hi")
	ctx.Set("name", "mark")

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("hi mark", s)
}

// support identifiers containing digits, but not starting with a digits
func Test_Identifiers_With_Digits(t *testing.T) {
	r := require.New(t)
	input := `<%= my123greet %> <%= name3 %>`

	ctx := NewContext()
	ctx.Set("my123greet", "hi")
	ctx.Set("name3", "mark")

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("hi mark", s)
}

func Test_Render_HTML_InjectedString(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "mark" %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>mark</p>", s)
}

func Test_Render_EscapedString(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "<script>alert('pwned')</script>" %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>&lt;script&gt;alert(&#39;pwned&#39;)&lt;/script&gt;</p>", s)
}

func Test_Render_Injected_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"name": "Mark",
	}))
	r.NoError(err)
	r.Equal("<p>Mark</p>", s)
}

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

func Test_Render_Missing_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	_, err := Render(input, NewContext())
	r.Error(err)
}

func Test_Render_HTML_Escape(t *testing.T) {
	r := require.New(t)

	input := `<%= escapedHTML() %>|<%= unescapedHTML() %>|<%= raw("<b>unsafe</b>") %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"escapedHTML": func() string {
			return "<b>unsafe</b>"
		},
		"unescapedHTML": func() template.HTML {
			return "<b>unsafe</b>"
		},
	}))
	r.NoError(err)
	r.Equal("&lt;b&gt;unsafe&lt;/b&gt;|<b>unsafe</b>|<b>unsafe</b>", s)
}

func Test_Render_ShowNoShow(t *testing.T) {
	r := require.New(t)
	input := `<%= "shown" %><% "notshown" %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("shown", s)
}

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

func Test_Render_ScriptFunction(t *testing.T) {
	r := require.New(t)

	input := `<% let add = fn(x) { return x + 2; }; %><%= add(2) %>`

	s, err := Render(input, NewContext())
	if err != nil {
		r.NoError(err)
	}
	r.Equal("4", s)
}

func Test_Render_HasBlock(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
	ctx.Set("blockCheck", func(help HelperContext) string {
		if help.HasBlock() {
			s, _ := help.Block()
			return s
		}
		return "no block"
	})
	input := `<%= blockCheck() {return "block"} %>|<%= blockCheck() %>`
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("block|no block", s)
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
}

type Avatar string

func (a Avatar) URL() string {
	return strings.ToUpper(string(a))
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

func Test_Render_Var_ends_in_Number(t *testing.T) {
	r := require.New(t)
	ctx := NewContextWith(map[string]interface{}{
		"myvar1": []string{"john", "paul"},
	})
	s, err := Render(`<%= for (n) in myvar1 {return n}`, ctx)
	r.NoError(err)
	r.Equal("johnpaul", s)
}

func Test_Render_Dash_in_Helper(t *testing.T) {
	r := require.New(t)
	ctx := NewContextWith(map[string]interface{}{
		"my-helper": func() string {
			return "hello"
		},
	})
	s, err := Render(`<%= my-helper() %>`, ctx)
	r.NoError(err)
	r.Equal("hello", s)
}

func Test_(t *testing.T) {
	r := require.New(t)
	input := `<%= foo() %><%= name %>`
	data := map[string]interface{}{
		"name": "Ringo",
	}
	helpers := map[string]interface{}{
		"foo": func() string {
			return "George"
		},
	}
	s, err := BuffaloRenderer(input, data, helpers)
	r.NoError(err)
	r.Equal("GeorgeRingo", s)
}

func Test_Helper_Nil_Arg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(nil, "k") %><%= foo(one, "k") %>`
	ctx := NewContextWith(map[string]interface{}{
		"one": map[string]string{
			"k": "test",
		},
		"foo": func(a map[string]string, b string) string {
			if a != nil {
				return a[b]
			}
			return ""
		},
	})
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("test", s)
}

func Test_UndefinedArg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(bar) %>`
	ctx := NewContext()
	ctx.Set("foo", func(string) {})

	_, err := Render(input, ctx)
	r.Error(err)
	r.Equal(ErrUnknownIdentifier, errors.Cause(err))
}

func Test_VariadicHelper(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(1, 2, 3) %>`
	ctx := NewContext()
	ctx.Set("foo", func(args ...int) int {
		return len(args)
	})

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("3", s)
}

func Test_VariadicHelper_SecondArg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo("hello") %>`
	ctx := NewContext()
	ctx.Set("foo", func(s string, args ...interface{}) string {
		return s
	})

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("hello", s)
}

func Test_VariadicHelperNoParam(t *testing.T) {
	r := require.New(t)
	input := `<%= foo() %>`
	ctx := NewContext()
	ctx.Set("foo", func(args ...int) int {
		return len(args)
	})

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("0", s)
}

func Test_VariadicHelperNoVariadicParam(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(1) %>`
	ctx := NewContext()
	ctx.Set("foo", func(a int, args ...int) int {
		return a + len(args)
	})

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("1", s)
}

func Test_VariadicHelperWithWrongParam(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(1, 2, "test") %>`
	ctx := NewContext()
	ctx.Set("foo", func(args ...int) int {
		return len(args)
	})

	_, err := Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "test (string) is an invalid argument for foo at pos 2: expected (int)")
}

func Test_LineNumberErrors(t *testing.T) {
	r := require.New(t)
	input := `<p>
	<%= f.Foo %>
</p>`

	_, err := Render(input, NewContext())
	r.Error(err)
	r.Contains(err.Error(), "line 2:")
}

func Test_LineNumberErrors_ForLoop(t *testing.T) {
	r := require.New(t)
	input := `
	<%= for (n) in numbers.Foo { %>
		<%= n %>
	<% } %>
	`

	_, err := Render(input, NewContext())
	r.Error(err)
	r.Contains(err.Error(), "line 2:")
}

func Test_LineNumberErrors_ForLoop2(t *testing.T) {
	r := require.New(t)
	input := `
	<%= for (n in numbers.Foo { %>
		<%= if (n == 3) { %>
			<%= n %>
		<% } %>
	<% } %>
	`

	_, err := Parse(input)
	r.Error(err)
	r.Contains(err.Error(), "line 2:")
}

func Test_LineNumberErrors_InsideForLoop(t *testing.T) {
	r := require.New(t)
	input := `
	<%= for (n) in numbers { %>
		<%= n.Foo %>
	<% } %>
	`
	ctx := NewContext()
	ctx.Set("numbers", []int{1, 2})
	_, err := Render(input, ctx)
	r.Error(err)
	r.Contains(err.Error(), "line 3:")
}

func Test_MissingQuote(t *testing.T) {
	r := require.New(t)
	input := `<%= foo("asdf) %>`
	ctx := NewContext()
	ctx.Set("foo", func(string) {})
	_, err := Render(input, ctx)
	r.Error(err)
}

func Test_MissingQuote_Variant(t *testing.T) {
	r := require.New(t)
	input := `<%= foo("test) %>".`
	ctx := NewContext()
	ctx.Set("foo", func(string) {})
	_, err := Render(input, ctx)
	r.Error(err)
}

func Test_MissingQuote_Variant2(t *testing.T) {
	r := require.New(t)
	input := `<%= title("Running Migrations) %>(default "./migrations")`
	ctx := NewContext()
	ctx.Set("foo", func(string) {})
	_, err := Render(input, ctx)
	r.Error(err)
}

func Test_RunScript(t *testing.T) {
	r := require.New(t)
	bb := &bytes.Buffer{}
	ctx := NewContextWith(map[string]interface{}{
		"out": func(i interface{}) {
			bb.WriteString(fmt.Sprint(i))
		},
	})
	err := RunScript(script, ctx)
	r.NoError(err)
	r.Equal("3hiasdfasdf", bb.String())
}

func Test_Render_AllowsManyNumericTypes(t *testing.T) {
	r := require.New(t)
	input := `<%= i32 %> <%= u32 %> <%= i8 %>`

	ctx := NewContext()
	ctx.Set("i32", int32(1))
	ctx.Set("u32", uint32(2))
	ctx.Set("i8", int8(3))

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("1 2 3", s)
}

func Test_Default_Time_Format(t *testing.T) {
	r := require.New(t)

	shortForm := "2006-Jan-02"
	tm, err := time.Parse(shortForm, "2013-Feb-03")
	r.NoError(err)
	ctx := NewContext()
	ctx.Set("tm", tm)

	input := `<%= tm %>`

	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("February 03, 2013 00:00:00 +0000", s)

	ctx.Set("TIME_FORMAT", "2006-02-Jan")
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("2013-03-Feb", s)

	ctx.Set("tm", &tm)
	s, err = Render(input, ctx)
	r.NoError(err)
	r.Equal("2013-03-Feb", s)

}

const script = `let x = "foo"

let a = 1
let b = 2
let c = a + b

out(c)

if (c == 3) {
  out("hi")
}

let x = fn(f) {
  f()
}

x(fn() {
  out("asdfasdf")
})`
