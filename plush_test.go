package plush

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Render_Simple_HTML(t *testing.T) {
	r := require.New(t)

	input := `<p>Hi</p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal(input, s)
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

func Test_Render_Int_Math(t *testing.T) {
	r := require.New(t)

	input := `<p><%= 1 + 3 %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>4</p>", s)
}

func Test_Render_Float_Math(t *testing.T) {
	r := require.New(t)

	input := `<p><%= 1.1 + 3.1 %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>4.2</p>", s)
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

func Test_Render_Let_Hash(t *testing.T) {
	r := require.New(t)

	input := `<p><% let h = {"a": "A"} %><%= h["a"] %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>A</p>", s)
}

func Test_Render_String_Concat(t *testing.T) {
	r := require.New(t)

	input := `<%= "a"  + "b" %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("ab", s)
}

func Test_Render_String_Concat_Multiple(t *testing.T) {
	r := require.New(t)

	input := `<%= "a" + "b" + "c" %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render_String_Int_Concat(t *testing.T) {
	r := require.New(t)

	input := `<%= "a"  + 1 %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("a1", s)
}

func Test_Render_Bool_Concat(t *testing.T) {
	r := require.New(t)

	input := `<%= true + 1 %>`
	s, err := Render(input, NewContext())
	r.Equal("", s)
	r.NoError(err)
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
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p></p>", s)
}

func Test_Render_Function_Call(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f() %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"f": func() string {
			return "hi!"
		},
	}))
	r.NoError(err)
	r.Equal("<p>hi!</p>", s)
}

func Test_Render_Function_Call_With_Arg(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f("mark") %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"f": func(s string) string {
			return fmt.Sprintf("hi %s!", s)
		},
	}))
	r.NoError(err)
	r.Equal("<p>hi mark!</p>", s)
}

func Test_Render_Function_Call_With_Variable_Arg(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f(name) %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"f": func(s string) string {
			return fmt.Sprintf("hi %s!", s)
		},
		"name": "mark",
	}))
	r.NoError(err)
	r.Equal("<p>hi mark!</p>", s)
}

func Test_Render_Function_Call_With_Hash(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f({name: name}) %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"f": func(m map[string]interface{}) string {
			return fmt.Sprintf("hi %s!", m["name"])
		},
		"name": "mark",
	}))
	r.NoError(err)
	r.Equal("<p>hi mark!</p>", s)
}

func Test_Render_HTML_Escape(t *testing.T) {
	r := require.New(t)

	input := `<%= safe() %>|<%= unsafe() %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"safe": func() string {
			return "<b>unsafe</b>"
		},
		"unsafe": func() template.HTML {
			return "<b>unsafe</b>"
		},
	}))
	r.NoError(err)
	r.Equal("&lt;b&gt;unsafe&lt;/b&gt;|<b>unsafe</b>", s)
}

func Test_Render_Function_Call_With_Error(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f() %></p>`
	_, err := Render(input, NewContextWith(map[string]interface{}{
		"f": func() (string, error) {
			return "hi!", errors.New("oops!")
		},
	}))
	r.Error(err)
}

func Test_Render_Function_Call_With_Block(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f() { %>hello<% } %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"f": func(h HelperContext) string {
			s, _ := h.Block()
			return s
		},
	}))
	r.NoError(err)
	r.Equal("<p>hello</p>", s)
}

type greeter struct{}

func (g greeter) Greet(s string) string {
	return fmt.Sprintf("hi %s!", s)
}

func Test_Render_Function_Call_On_Callee(t *testing.T) {
	r := require.New(t)

	input := `<p><%= g.Greet("mark") %></p>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"g": greeter{},
	}))
	r.NoError(err)
	r.Equal(`<p>hi mark!</p>`, s)
}

func Test_Render_For_Array(t *testing.T) {
	r := require.New(t)
	input := `<% for (i,v) in ["a", "b", "c"] {return v} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_For_Hash(t *testing.T) {
	r := require.New(t)
	input := `<%= for (k,v) in myMap {return k + ":" + v} %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"myMap": map[string]string{
			"a": "A",
			"b": "B",
		},
	}))
	r.NoError(err)
	r.Contains(s, "a:A")
	r.Contains(s, "b:B")
}

func Test_Render_For_Array_Return(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in ["a", "b", "c"] {return v} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render_For_Array_Key_Only(t *testing.T) {
	r := require.New(t)
	input := `<%= for (v) in ["a", "b", "c"] {%><%=v%><%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render_For_Array_Key_Value(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("0a1b2c", s)
}

func Test_Render_If(t *testing.T) {
	r := require.New(t)
	input := `<% if (true) { return "hi"} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_If_Return(t *testing.T) {
	r := require.New(t)
	input := `<%= if (true) { return "hi"} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_Return_HTML(t *testing.T) {
	r := require.New(t)
	input := `<%= if (true) { %>hi<%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_And(t *testing.T) {
	r := require.New(t)
	input := `<%= if (false && true) { %> hi <%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_If_Or(t *testing.T) {
	r := require.New(t)
	input := `<%= if (false || true) { %>hi<%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_Render_If_Nil(t *testing.T) {
	r := require.New(t)
	input := `<%= if (names && len(names) >= 1) { %>hi<%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_If_Else_Return(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (false) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>bye</p>", s)
}

func Test_Render_If_LessThan(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (1 < 2) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_Render_If_BangFalse(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (!false) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_Render_If_NotEq(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (1 != 2) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_Render_If_GtEq(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (1 >= 2) { return "hi"} else { return "bye"} %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>bye</p>", s)
}

func Test_Render_If_Else_True(t *testing.T) {
	r := require.New(t)
	input := `<p><%= if (true) { %>hi<% } else { %>bye<% } %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
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

	input := `<% fn(x) { x + 2; }; %>`

	s, err := Render(input, NewContext())
	if err != nil {
		r.NoError(err)
	}
	r.Equal("<h1>hi mark</h1>", s)
}

// ExampleTemplate using `if`, `for`, `else`, functions, etc...
func ExampleTemplate() {
	html := `<html>
<%= if (names && len(names) > 0) { %>
	<ul>
		<%= for (n) in names { %>
			<li><%= capitalize(n) %></li>
		<% } %>
	</ul>
<% } else { %>
	<h1>Sorry, no names. :(</h1>
<% } %>
</html>`

	ctx := NewContext()
	ctx.Set("names", []string{"john", "paul", "george", "ringo"})

	s, err := Render(html, ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(s)
	// output: <html>
	// <ul>
	// 		<li>John</li>
	// 		<li>Paul</li>
	// 		<li>George</li>
	// 		<li>Ringo</li>
	// 		</ul>
	// </html>
}

func ExampleTemplate_scripletTags() {
	html := `<%
let h = {name: "mark"}
let greet = fn(n) {
  return "hi " + n
}
%>
<h1><%= greet(h["name"]) %></h1>`

	s, err := Render(html, NewContext())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(s)
	// output: <h1>hi mark</h1>
}

func ExampleTemplate_customHelperFunctions() {
	html := `<p><%= one() %></p>
<p><%= greet("mark")%></p>
<%= can("update") { %>
<p>i can update</p>
<% } %>
<%= can("destroy") { %>
<p>i can destroy</p>
<% } %>
`

	ctx := NewContext()
	ctx.Set("one", func() int {
		return 1
	})
	ctx.Set("greet", func(s string) string {
		return fmt.Sprintf("Hi %s", s)
	})
	ctx.Set("can", func(s string, help HelperContext) (template.HTML, error) {
		if s == "update" {
			h, err := help.Block()
			return template.HTML(h), err
		}
		return "", nil
	})

	s, err := Render(html, ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(s)
	// output: <p>1</p>
	// <p>Hi mark</p>
	// <p>i can update</p>
}
