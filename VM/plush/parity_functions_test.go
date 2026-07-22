package plush_test

import (
	"errors"
	"html/template"
	"testing"
)

type parityPointerArgSection struct {
	Name string
}

func Test_Parity_Functions_Helper_Call(t *testing.T) {
	compareRender(t, `<%= greet(name) %>`, contextWith(map[string]interface{}{
		"name": "mark",
		"greet": func(name string) string {
			return "hello " + name
		},
	}))
}

func Test_Parity_Functions_Helper_Call_With_Value_For_Pointer_Arg(t *testing.T) {
	compareRender(t, `<%= render_section(section) %>`, contextWith(map[string]interface{}{
		"section": parityPointerArgSection{Name: "hero"},
		"render_section": func(section *parityPointerArgSection) string {
			return section.Name
		},
	}))
}

func Test_Parity_Functions_User_Function(t *testing.T) {
	compareRender(t, `<% let add = fn(x) { return x + 2 } %><%= add(3) %>`, emptyContext)
}

func Test_Parity_Functions_Unknown_Function_Errors(t *testing.T) {
	compareBothRenderError(t, `<p><%= f() %></p>`, emptyContext)
}

func Test_Parity_Functions_Helper_Error_Return(t *testing.T) {
	compareBothRenderError(t, `<p><%= fail() %></p>`, contextWith(map[string]interface{}{
		"fail": func() (string, error) {
			return "hi", errors.New("oops")
		},
	}))
}

func Test_Parity_Functions_Nil_Arg(t *testing.T) {
	compareRender(t, `<%= lookup(nil, "k") %><%= lookup(one, "k") %>`, contextWith(map[string]interface{}{
		"one": map[string]string{
			"k": "test",
		},
		"lookup": func(values map[string]string, key string) string {
			if values == nil {
				return ""
			}
			return values[key]
		},
	}))
}

func Test_Parity_Functions_Undefined_Arg_Errors(t *testing.T) {
	compareBothRenderError(t, `<%= foo(bar) %>`, contextWith(map[string]interface{}{
		"foo": func(string) {},
	}))
}

func Test_Parity_Functions_Early_Return(t *testing.T) {
	compareRender(t, `<%
let numberify = fn(arg) {
	if (arg == "one") {
		return 1 + 1
	}
	if (arg == "two") {
		return 44
	}
	return "unsupported"
} %><%= numberify("one") %>`, emptyContext)
}

type paritySecretData struct {
	Secret   bool
	GiveHint bool
	String   string
}

type parityGreeter struct{}

func (parityGreeter) Greet(name string) string {
	return "hi " + name + "!"
}

func Test_Parity_Functions_Nested_Return_With_Default_Helper(t *testing.T) {
	input := `<%
let print = fn(obj) {
	if (obj.Secret) {
		if (obj.GiveHint) {
			return truncate(obj.String, {size: 12, trail: "****"})
		}
		return "**********"
	}
	return obj.String
}
%>You are: <%= print(data) %>.`

	cases := []map[string]interface{}{
		{"data": paritySecretData{Secret: true, String: "your royal highness"}},
		{"data": paritySecretData{Secret: true, GiveHint: true, String: "your royal highness"}},
		{"data": paritySecretData{Secret: false, String: "your royal highness"}},
	}

	for _, data := range cases {
		compareRender(t, input, contextWith(data))
	}
}

func Test_Parity_Functions_Method_Call_On_Callee(t *testing.T) {
	compareRender(t, `<p><%= g.Greet("mark") %></p>`, contextWith(map[string]interface{}{
		"g": parityGreeter{},
	}))
}

func Test_Parity_Functions_Optional_Map_Arg(t *testing.T) {
	compareRender(t, `<%= foo() %>|<%= bar({a: "A"}) %>`, contextWith(map[string]interface{}{
		"foo": func(opts map[string]interface{}) string {
			return "foo"
		},
		"bar": func(opts map[string]interface{}) string {
			return opts["a"].(string)
		},
	}))
}

func Test_Parity_Functions_Dash_In_Helper(t *testing.T) {
	compareRender(t, `<%= my-helper() %>`, contextWith(map[string]interface{}{
		"my-helper": func() string {
			return "hello"
		},
	}))
}

func Test_Parity_Functions_Closure(t *testing.T) {
	input := `<% let newAdder = fn(x) { return fn(y) { return x + y } } %><% let addTwo = newAdder(2) %><%= addTwo(3) %>`
	out, err := renderVM(input, emptyContext)
	if err != nil {
		t.Fatal(err)
	}
	if out != "5" {
		t.Fatalf("expected VM closure output %q, got %q", "5", out)
	}
}

func Test_Parity_Functions_Recursion(t *testing.T) {
	compareRender(t, `<% let countdown = fn(x) { if (x == 0) { return 0 } return countdown(x - 1) } %><%= countdown(3) %>`, emptyContext)
}

func Test_Parity_Functions_Backtick_String(t *testing.T) {
	compareRender(t, "<%= raw(`CREATE VIEW x AS SELECT \"name\" FROM things`) %>", contextWith(map[string]interface{}{
		"raw": func(value string) template.HTML {
			return template.HTML(value)
		},
	}))
}
