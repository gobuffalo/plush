package plush_test

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"testing"
	"time"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/stretchr/testify/require"
)

func Test_Parity_Root_Plush_Render_Cases(t *testing.T) {
	compareRender(t, `<p>Hi</p>`, emptyContext)
	compareRender(t, `<p><%= "mark" %></p>`, emptyContext)
	compareRender(t, `<p><%= name %></p>`, contextWith(map[string]interface{}{"name": "Mark"}))
	compareBothRenderError(t, `<p><%= name %></p>`, emptyContext)
	compareRender(t, `<% let add = fn(x) { return x + 2; }; %><%= add(2) %>`, emptyContext)
}

func Test_Parity_Root_Variadic_Helper_Cases(t *testing.T) {
	compareRender(t, `<%= foo(1, 2, 3) %>`, contextWith(map[string]interface{}{
		"foo": func(args ...int) int { return len(args) },
	}))
	compareRender(t, `<%= foo("hello") %>`, contextWith(map[string]interface{}{
		"foo": func(s string, args ...interface{}) string { return s },
	}))
	compareRender(t, `<%= foo() %>`, contextWith(map[string]interface{}{
		"foo": func(args ...int) int { return len(args) },
	}))
	compareRender(t, `<%= foo(1) %>`, contextWith(map[string]interface{}{
		"foo": func(a int, args ...int) int { return a + len(args) },
	}))
	compareExactRenderError(t, `<%= foo(1, 2, "test") %>`, contextWith(map[string]interface{}{
		"foo": func(args ...int) int { return len(args) },
	}))
}

func Test_Parity_Root_Hash_Index_Cases(t *testing.T) {
	compareRender(t, `<%= m["first"]%>`, contextWith(map[string]interface{}{
		"m": map[interface{}]bool{"first": true},
	}))
	compareExactRenderError(t, `<%= m["first"]%>`, contextWith(map[string]interface{}{
		"m": map[int]bool{0: true},
	}))
	compareRender(t, `<%= m["first"] + " " + m["last"] %>|<%= a[0+1] %>`, contextWith(map[string]interface{}{
		"m": map[string]string{"first": "Mark", "last": "Bates"},
		"a": []string{"john", "paul"},
	}))
	compareRender(t, `<%= m["a"] %>`, contextWith(map[string]interface{}{
		"m": map[string]string{"a": "A"},
	}))
	compareRender(t, `<%= m.MyMap[key] %>`, contextWith(map[string]interface{}{
		"m": struct {
			MyMap map[string]string
		}{MyMap: map[string]string{"a": "A"}},
		"key": "a",
	}))
	compareRender(t, `<%= debug(m.MyMap[key]) %>`, contextWith(map[string]interface{}{
		"m": struct {
			MyMap map[string]string
		}{MyMap: map[string]string{"a": "A"}},
		"key": "a",
	}))
}

func Test_Parity_Root_If_Condition_Table(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		success bool
	}{
		{"if_else_true", `<%= if (true) { return "good"} else { return "bad"} %>`, true},
		{"if_else_false", `<%= if (false) { return "good"} else { return "bad"} %>`, true},
		{"missing_output_marker", `<%  if (true) { return "good"} else { return "bad"} %>`, true},
		{"value_from_template_html", `<%= if (true) { %>good<% } %>`, true},
		{"if_false", `<%= if (false) { return "good"} %>`, true},
		{"if_bang_false", `<%= if (!false) { return "good"} %>`, true},
		{"bool_true_is_true", `<%= if (true == true) { return "good"} else { return "bad"} %>`, true},
		{"bool_true_is_not_true", `<%= if (true != true) { return "good"} else { return "bad"} %>`, true},
		{"let_var_is_false", `<% let test = true %><%= if (test == false) { return "good"} %>`, true},
		{"let_var_is_true", `<% let test = true %><%= if (test == true) { return "good"} %>`, true},
		{"let_var_is_not_true", `<% let test = true %><%= if (test != true) { return "good"} %>`, true},
		{"let_var_is_not_bool", `<% let test = 1 %><%= if (test != true) { return "good"} %>`, false},
		{"let_var_is_1", `<% let test = 1 %><%= if (test == 1) { return "good"} %>`, true},
		{"logical_false_and_true", `<%= if (false && true) { %>good<% } %>`, true},
		{"logical_true_and_true", `<%= if (2 == 2 && 1 == 1) { %>good<% } %>`, true},
		{"logical_false_or_true", `<%= if (false || true) { %>good<% } %>`, true},
		{"logical_false_or_false", `<%= if (1 == 2 || 2 == 1) { %>good<% } %>`, true},
		{"nil_and", `<%= if (names && len(names) >= 1) { %>good<% } %>`, true},
		{"nil_and_else", `<%= if (names && len(names) >= 1) { %>good<% } else { %>else<% } %>`, true},
		{"nil", `<%= if (names) { %>good<% } %>`, true},
		{"not_nil", `<%= if (!names) { %>good<% } %>`, true},
		{"compare_equal_to", `<%= if (1 == 2) { %>good<% } %>`, true},
		{"compare_not_equal_to", `<%= if (1 != 2) { %>good<% } %>`, true},
		{"compare_less_than", `<%= if (1 < 2) { %>good<% } %>`, true},
		{"compare_less_than_equal_to", `<%= if (1 <= 2) { %>good<% } %>`, true},
		{"compare_greater_than", `<%= if (1 > 2) { %>good<% } %>`, true},
		{"compare_greater_than_equal_to", `<%= if (1 >= 2) { %>good<% } %>`, true},
		{"if_match_foo_bar", `<%= if ("foo" ~= "bar") { %>good<% } %>`, true},
		{"if_match_foo_prefix", `<%= if ("foo" ~= "^fo") { %>good<% } %>`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.success {
				compareRender(t, tt.input, emptyContext)
				return
			}
			compareExactRenderError(t, tt.input, emptyContext)
		})
	}
}

func Test_Parity_Root_Condition_Only_Table(t *testing.T) {
	ctxEmpty := emptyContext
	ctxWithPaths := contextWith(map[string]interface{}{"paths": "cart"})

	tests := []struct {
		name    string
		input   string
		factory contextFactory
	}{
		{"unknown_equal_to_nil", `<%= paths == nil %>`, ctxEmpty},
		{"nil_equal_to_unknown", `<%= nil == paths %>`, ctxEmpty},
		{"not_set_identifier", `<%= !paths %>`, ctxWithPaths},
		{"not_unknown_identifier", `<%= !pages %>`, ctxWithPaths},
		{"set_or_unknown", `<%= paths || pages %>`, ctxWithPaths},
		{"unknown_or_set", `<%= pages || paths %>`, ctxWithPaths},
		{"set_or_unknown_equal", `<%= paths || pages == "cart" %>`, ctxWithPaths},
		{"unknown_equal_or_set", `<%= pages == "cart" || paths %>`, ctxWithPaths},
		{"equal_or_unknown", `<%= paths == "cart" || pages %>`, ctxWithPaths},
		{"unknown_or_equal", `<%= pages || paths == "cart" %>`, ctxWithPaths},
		{"equal_or_unknown_equal", `<%= paths == "cart" || pages == "cart" %>`, ctxWithPaths},
		{"unknown_equal_or_equal", `<%= pages == "cart" || paths == "cart" %>`, ctxWithPaths},
		{"set_and_unknown", `<%= paths && pages %>`, ctxWithPaths},
		{"unknown_and_set", `<%= pages && paths %>`, ctxWithPaths},
		{"set_and_unknown_equal", `<%= paths && pages == "cart" %>`, ctxWithPaths},
		{"unknown_equal_and_set", `<%= pages == "cart" && paths %>`, ctxWithPaths},
		{"equal_and_unknown", `<%= paths == "cart" && pages %>`, ctxWithPaths},
		{"unknown_and_equal", `<%= pages && paths == "cart" %>`, ctxWithPaths},
		{"equal_and_unknown_equal", `<%= paths == "cart" && pages == "cart" %>`, ctxWithPaths},
		{"unknown_equal_and_equal", `<%= pages == "cart" && paths == "cart" %>`, ctxWithPaths},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compareRender(t, tt.input, tt.factory)
		})
	}
}

func Test_Parity_Root_If_Scope_And_Else_If_Cases(t *testing.T) {
	input := `<p><%= if (state == "foo") { %>hi foo<% } else if (state == "bar") { %>hi bar<% } else if (state == "fizz") { %>hi fizz<% } else { %>hi buzz<% } %></p>`
	for _, state := range []string{"foo", "bar", "fizz", "buzz"} {
		t.Run("state_"+state, func(t *testing.T) {
			compareRender(t, input, contextWith(map[string]interface{}{"state": state}))
		})
	}

	stringTruthy := `<p><%= if (username && username != "") { return "hi" } else { return "bye" } %></p>`
	compareRender(t, stringTruthy, contextWith(map[string]interface{}{"username": ""}))
	compareRender(t, stringTruthy, contextWith(map[string]interface{}{"username": "foo"}))

	updateInput := `<p><%= if (username && username != "") { username = "hi" } else { username= "bye" } %><%= username %></p>`
	compareRender(t, updateInput, contextWith(map[string]interface{}{"username": ""}))
	compareRender(t, updateInput, contextWith(map[string]interface{}{"username": "foo"}))
	compareExactRenderError(t, updateInput, emptyContext)

	compareRender(t, `<p><% let username = "Hello World" %><%= if (username && username != "") {
		let username = "hi"
		username = "hi"
	} else {
		let username = "bye"
		username = "1"
	 }%><%= username %></p>`, emptyContext)

	compareRender(t, `<p><% let username = "Hello World" %><%= if (username && username != "") {
		let username = "hi"
		username = "hi"
		if (username == "hi"){
			username = "hi2"
		}
	} else {
		let username = "bye"
		username = "1"
	 }%><%= username %></p>`, emptyContext)

	compareRender(t, `<p><% let username = "Hello World" %><%= if (username && username != "") {
		username = "hi"
		if (username == "hi"){
			username = "hi2"
		}
	} else {
		let username = "bye"
		username = "1"
	 }%><%= username %></p>`, emptyContext)

	compareRender(t, `<%= if ( paths == "cart" || (page && page.PageTitle != "cafe") || paths == "cart") { %>hi<%} %>`, contextWith(map[string]interface{}{
		"path":  "cart",
		"paths": "cart",
	}))
}

func Test_Parity_Root_Variable_Assignment_Index_And_Append_Cases(t *testing.T) {
	compareRender(t, `<% let foo = "bar" %>
  <%= for (a) in myArray { %>
<%= foo %>
    <% if (foo != "baz") { %>
      <% foo = "baz" %>
    <% } %>
  <% } %>
<% } %>`, contextWith(map[string]interface{}{
		"myArray": []string{"a", "b"},
	}))

	successes := []string{
		`<p><% let h = {"a": "A"} %><%= h["a"] %></p>`,
		`<p><% let h = {"a": "A"} %><% h["a"] = "C" %><%= h["a"] %></p>`,
		`<p><% let h = {"a": "A"} %><% h["b"] = "D" %><%= h["b"] %></p>`,
		`<p><% let h = {"a": "A"} %><% h["b"] = 3 %><%= h["b"] %></p>`,
		`<p><% let h = {"a": "A"} %><% h["b"] = 3 %><%= h["c"] %></p>`,
		`<p><% let a = [1, 2, "three", "four", 3.75] %><% a[0] = 3 %><%= a[0] %></p>`,
		`<p><% let a = [1, 2, "three", "four", 3.75] %><% a[4] = 3 %><%= a[4] + 2 %></p>`,
		`<% let a = [1,"22",33] %><% a = a + 1 %><%= a %>`,
		`<% let a = [1,2,"HelloWorld"] %><% a = a + 2.2 %><%= a %>`,
	}
	for _, input := range successes {
		t.Run(input, func(t *testing.T) {
			compareRender(t, input, emptyContext)
		})
	}

	errors := []string{
		`<p><% let a = [1, 2, "three", "four", 3.75] %><% a["b"] = 3 %><%= a["c"] %></p>`,
		`<p><% let a = [1, 2, "three", "four", 3.75] %><% a[5] = 3 %><%= a[4] + 2 %></p>`,
		`<p><% let a = [1, 2, "three", "four", 3.75] %><%= a[5] %></p>`,
	}
	for _, input := range errors {
		t.Run(input, func(t *testing.T) {
			compareBothRenderError(t, input, emptyContext)
		})
	}

	type item struct {
		P string
	}
	compareBothRenderError(t, `<p><% let a = myArray %></p><% a[0] = "HELLO WORLD" %>`, contextWith(map[string]interface{}{
		"myArray": []item{{P: "t"}},
	}))
	compareRender(t, `<p><% let a = myArray %></p><% a[0] = "HELLO WORLD" %>`, contextWith(map[string]interface{}{
		"myArray": []interface{}{item{P: "t"}, item{P: "g"}},
	}))
	compareBothRenderError(t, `<% let a = myArray %><% a = a + 1 %><%= a %>`, contextWith(map[string]interface{}{
		"myArray": []string{"a", "b"},
	}))
	compareRender(t, `<% let a = myArray %><% a = a + 1 %><%= a %>`, contextWith(map[string]interface{}{
		"myArray": []interface{}{"a", "b"},
	}))
}

func Test_Parity_Root_Function_Call_And_Block_Cases(t *testing.T) {
	compareRender(t, `<p><%= f() %></p>`, contextWith(map[string]interface{}{
		"f": func() string { return "hi!" },
	}))
	compareRender(t, `<p><%= f("mark") %></p>`, contextWith(map[string]interface{}{
		"f": func(s string) string { return fmt.Sprintf("hi %s!", s) },
	}))
	compareRender(t, `<p><%= f(name) %></p>`, contextWith(map[string]interface{}{
		"f":    func(s string) string { return fmt.Sprintf("hi %s!", s) },
		"name": "mark",
	}))
	compareRender(t, `<p><%= f({name: name}) %></p>`, contextWith(map[string]interface{}{
		"f":    func(m map[string]interface{}) string { return fmt.Sprintf("hi %s!", m["name"]) },
		"name": "mark",
	}))
	compareBothRenderError(t, `<p><%= f({name: name) %></p>`, contextWith(map[string]interface{}{
		"f":    func(m map[string]interface{}) string { return fmt.Sprintf("hi %s!", m["name"]) },
		"name": "mark",
	}))
	compareExactRenderError(t, `<p><%= f() %></p>`, contextWith(map[string]interface{}{
		"f": func() (string, error) { return "hi!", errors.New("oops") },
	}))
	compareRender(t, `<p><%= f() { %>hello<% } %></p>`, contextWith(map[string]interface{}{
		"f": func(h rootplush.HelperContext) string {
			s, _ := h.Block()
			return s
		},
	}))
	compareRender(t, `<p><% let i = "hello-world" %><%= f() { i = "bye" } %><%= i %></p>`, contextWith(map[string]interface{}{
		"f": func(h rootplush.HelperContext) string {
			s, _ := h.Block()
			return s
		},
	}))
	compareBothRenderError(t, `<p><%= f() { let i = "hello-world" } %><%= i %></p>`, contextWith(map[string]interface{}{
		"f": func(h rootplush.HelperContext) string {
			s, _ := h.Block()
			return s
		},
	}))
	compareRender(t, `<%= foo() %>|<%= bar({a: "A"}) %>`, contextWith(map[string]interface{}{
		"foo": func(opts map[string]interface{}, help rootplush.HelperContext) string { return "foo" },
		"bar": func(opts map[string]interface{}) string { return opts["a"].(string) },
	}))
}

func Test_Parity_Root_Helper_Block_With_Data_Cases(t *testing.T) {
	type data map[string]interface{}
	tests := []struct {
		name string
		in   string
	}{
		{name: "default data", in: `<%= foo() {return bar + name} %>`},
		{name: "override name", in: `<%= foo({name: "mark"}) {return bar + name} %>`},
		{name: "local data does not leak", in: `<%= foo({name: "mark", bbb: "hello-world"}) {return bar + name} %><%= bbb %>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := contextWith(map[string]interface{}{
				"name": "unknown",
				"bar":  "BAR",
				"foo": func(d data, help rootplush.HelperContext) (string, error) {
					c := help.New()
					if n, ok := d["name"]; ok {
						c.Set("name", n)
					}
					if n, ok := d["bbb"]; ok {
						c.Set("bbb", n)
					}
					return help.BlockWith(c)
				},
			})
			if strings.Contains(tt.in, `<%= bbb %>`) {
				compareExactRenderError(t, tt.in, factory)
				return
			}
			compareRender(t, tt.in, factory)
		})
	}
}

func Test_Parity_Root_For_Cases(t *testing.T) {
	tests := []struct {
		input     string
		factory   contextFactory
		wantError bool
	}{
		{input: `<% for (i,v) in ["a", "b", "c"] {return v} %>`, factory: emptyContext},
		{input: `<% let varTest = "" %><% for (i,v) in ["a", "b", "c"] {varTest =  v} %><%= varTest %>`, factory: emptyContext},
		{input: `<%= for (k,v) in myMap { %><%= k + ":" + v%><% } %> <%= k %>`, factory: contextWith(map[string]interface{}{"myMap": map[string]string{"a": "A"}}), wantError: true},
		{input: `<%= for (k,v) in myMap { %><%= k + ":" + v%><% } %> <%= v %>`, factory: contextWith(map[string]interface{}{"myMap": map[string]string{"a": "A"}}), wantError: true},
		{input: `<%= for (i,v) in ["a", "b", "c"] {return v} %>`, factory: emptyContext},
		{input: `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		continue
		return v
		} %>`, factory: emptyContext},
		{input: `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		break
		return v
		} %>`, factory: emptyContext},
		{input: `<%= for (v) in ["a", "b", "c"] {%><%=v%><%} %>`, factory: emptyContext},
		{input: `<%= for (v) in range(3,5) { %><%=v%><% } %>`, factory: emptyContext},
		{input: `<%= for (v) in between(3,6) { %><%=v%><% } %>`, factory: emptyContext},
		{input: `<%= for (v) in until(3) { %><%=v%><% } %>`, factory: emptyContext},
		{input: `<%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %>`, factory: emptyContext},
		{input: `<%   let i = 10000 %><%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %><%= i %>`, factory: emptyContext},
		{input: `<%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %><%= i %>`, factory: emptyContext, wantError: true},
		{input: ` <%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %><%= v %>`, factory: emptyContext, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if tt.wantError {
				compareBothRenderError(t, tt.input, tt.factory)
				return
			}
			compareRender(t, tt.input, tt.factory)
		})
	}

	mapLoop := `<%= for (k,v) in myMap { %><%= k + ":" + v%><% } %>`
	factory := contextWith(map[string]interface{}{
		"myMap": map[string]string{"a": "A", "b": "B"},
	})
	interpreterOut, interpreterErr := renderInterpreter(mapLoop, factory)
	vmOut, vmErr := renderVM(mapLoop, factory)
	require.NoError(t, interpreterErr)
	require.NoError(t, vmErr)
	for _, fragment := range []string{"a:A", "b:B"} {
		require.Contains(t, interpreterOut, fragment)
		require.Contains(t, vmOut, fragment)
	}
}

func Test_Parity_Root_Return_Exit_Cases(t *testing.T) {
	inputs := []string{
		`<%
		let numberify = fn(arg) {
			if (arg == "one") {
				return 1+1;
			}
			if (arg == "two") {
				return 44;
			}
			if (arg == "three") {
				return 2;
			}
			return "unsupported"
		} %>
		<%= numberify("one") %>`,
		`<%
		let numberify = fn(arg) {
			if (arg == "one") {
				return 1;
			}
			if (arg == "two") {
				return 445;
			}
			if (arg == "three") {
				return 3;
			}
			return "unsupported"
		} %>
		<%= numberify("two") %>`,
		`<%
		let numberify = fn(arg) {
			if (arg == "one") {
				return 1;
			}
			if (arg == "two") {
				return 445;
			}
			if (arg == "three") {
				return 3;
			}
			return "default value"
		} %>
		<%= numberify("six") %>`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			compareRender(t, input, emptyContext)
		})
	}
}

func Test_Parity_Root_Struct_Edge_Cases(t *testing.T) {
	type list struct {
		N    int
		Next *list
	}
	input := `Current number is <%= p.N %>.<%= if (p.Next) { %>  Next up is <%= p.Next.N %>.<% } %>`
	first := &list{N: 0}
	last := first
	for i := 0; i < 5; i++ {
		last.Next = &list{N: i + 1}
		last = last.Next
	}
	for p := first; p != nil; p = p.Next {
		p := p
		t.Run(fmt.Sprintf("list_%d", p.N), func(t *testing.T) {
			compareRender(t, input, contextWith(map[string]interface{}{"p": p}))
		})
	}

	type imageUser struct {
		Name  string
		Image *string
	}
	compareRender(t, `<%= user.Name %>: <%= user.Image %>`, contextWith(map[string]interface{}{
		"user": imageUser{Name: "Garn Clapstick"},
	}))
	image := "bicep.png"
	compareRender(t, `<%= user.Name %>: <%= user.Image %>`, contextWith(map[string]interface{}{
		"user": imageUser{Name: "Scrinch Archipeligo", Image: &image},
	}))

	type bornPerson struct {
		born time.Time
	}
	funcPerson := bornPerson{born: time.Date(1993, time.January, 11, 0, 0, 0, 0, time.UTC)}
	compareRender(t, `<%= nour.GetBorn().Format("Jan 2, 2006") %>`, contextWith(map[string]interface{}{
		"nour": rootParityBornPerson(funcPerson),
	}))
}

type rootParityBornPerson struct {
	born time.Time
}

func (p rootParityBornPerson) GetBorn() time.Time {
	return p.born
}

func Test_Parity_Root_Quote_And_Error_Type_Cases(t *testing.T) {
	for _, input := range []string{
		`<%= foo("asdf) %>`,
		`<%= foo("test) %>".`,
		`<%= title("Running Migrations) %>(default "./migrations")`,
	} {
		t.Run(input, func(t *testing.T) {
			compareBothRenderError(t, input, contextWith(map[string]interface{}{
				"foo":   func(string) {},
				"title": func(string) string { return "" },
			}))
		})
	}

	ctxFactory := contextWith(map[string]interface{}{
		"sqlError": func() error {
			return sql.ErrNoRows
		},
	})
	_, interpreterErr := renderInterpreter(`<%= sqlError() %>`, ctxFactory)
	_, vmErr := renderVM(`<%= sqlError() %>`, ctxFactory)
	require.True(t, errors.Is(interpreterErr, sql.ErrNoRows))
	require.True(t, errors.Is(vmErr, sql.ErrNoRows))
}

func Test_Parity_Root_Script_Execution(t *testing.T) {
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

	rootBuffer := &bytes.Buffer{}
	rootCtx := rootplush.NewContextWith(map[string]interface{}{
		"out": func(i interface{}) {
			rootBuffer.WriteString(fmt.Sprint(i))
		},
	})
	require.NoError(t, rootplush.RunScript(script, rootCtx))

	vmBuffer := &bytes.Buffer{}
	vmCtx := rootplush.NewContextWith(map[string]interface{}{
		"out": func(i interface{}) {
			vmBuffer.WriteString(fmt.Sprint(i))
		},
	})
	require.NoError(t, vmplush.RunScript(script, vmCtx))
	require.Equal(t, rootBuffer.String(), vmBuffer.String())
	require.Equal(t, "3hiasdfasdf", vmBuffer.String())
}

func Test_Parity_Root_Backtick_And_Quote_Helper_Case(t *testing.T) {
	input := "<%= raw(`" + `CREATE MATERIALIZED VIEW view_papers AS
	SELECT papers.created_at,
	   papers.updated_at,
	   papers.id,
	   papers.name,
	   (   setweight(to_tsvector(papers.name::text), 'A'::"char") ||
		   setweight(to_tsvector(papers.author_name), 'B'::"char")
	   ) || setweight(to_tsvector(papers.description), 'C'::"char")
	   AS paper_vector
	  FROM
	  ( SELECT papers.id, string_agg(categories.code, ',') as categories
	   FROM papers
	   LEFT JOIN paper_categories ON paper_categories.paper_id=papers.id LEFT JOIN (select * from categories order by weight asc) categories ON categories.id=paper_categories.category_id
	   GROUP BY papers.id
	  ) a
	  LEFT JOIN papers on a.id=papers.id
	 WHERE (papers.doc_status = ANY (ARRAY[1, 3])) AND papers.status = 1
   WITH DATA` + "`) %>"

	compareRender(t, input, contextWith(map[string]interface{}{
		"raw": func(arg string) template.HTML {
			return template.HTML(arg)
		},
	}))
}
