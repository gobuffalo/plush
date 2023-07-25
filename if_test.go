package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_If_Condition(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		name     string
		success  bool
	}{
		{`<%= if (true) { return "good"} else { return "bad"} %>`, "good", "if_else_true", true},
		{`<%= if (false) { return "good"} else { return "bad"} %>`, "bad", "if_else_false", true},
		{`<%  if (true) { return "good"} else { return "bad"} %>`, "", "missing=", true},
		{`<%= if (true) { %>good<% } %>`, "good", "value_from_template_html", true},
		{`<%= if (false) { return "good"} %>`, "", "if_false", true},
		{`<%= if (!false) { return "good"} %>`, "good", "if_bang_false", true},
		{`<%= if (true == true) { return "good"} else { return "bad"} %>`, "good", "bool_true_is_true", true},
		{`<%= if (true != true) { return "good"} else { return "bad"} %>`, "bad", "bool_true_is_not_true", true},
		{`<% let test = true %><%= if (test == false) { return "good"} %>`, "", "let_var_is_false", true},
		{`<% let test = true %><%= if (test == true) { return "good"} %>`, "good", "let_var_is_true", true},
		{`<% let test = true %><%= if (test != true) { return "good"} %>`, "", "let_var_is_not_true", true},
		{`<% let test = 1 %><%= if (test != true) { return "good"} %>`, "line 1: unable to operate (!=) on int and bool", "let_var_is_not_bool", false},
		{`<% let test = 1 %><%= if (test == 1) { return "good"} %>`, "good", "let_var_is_1", true},
		{`<%= if (false && true) { %>good<% } %>`, "", "logical_false_and_true", true},
		{`<%= if (2 == 2 && 1 == 1) { %>good<% } %>`, "good", "logical_true_and_true", true},
		{`<%= if (false || true) { %>good<% } %>`, "good", "logical_false_or_true", true},
		{`<%= if (1 == 2 || 2 == 1) { %>good<% } %>`, "", "logical_false_or_false", true},
		{`<%= if (names && len(names) >= 1) { %>good<% } %>`, "", "nil_and", true},
		{`<%= if (names && len(names) >= 1) { %>good<% } else { %>else<% } %>`, "else", "nil_and_else", true},
		{`<%= if (names) { %>good<% } %>`, "", "nil", true},
		{`<%= if (!names) { %>good<% } %>`, "good", "not_nil", true},
		{`<%= if (1 == 2) { %>good<% } %>`, "", "compare_equal_to", true},
		{`<%= if (1 != 2) { %>good<% } %>`, "good", "compare_not_equal_to", true},
		{`<%= if (1 < 2) { %>good<% } %>`, "good", "compare_less_than", true},
		{`<%= if (1 <= 2) { %>good<% } %>`, "good", "compare_less_than_equal_to", true},
		{`<%= if (1 > 2) { %>good<% } %>`, "", "compare_greater_than", true},
		{`<%= if (1 >= 2) { %>good<% } %>`, "", "compare_greater_than_equal_to", true},
		{`<%= if ("foo" ~= "bar") { %>good<% } %>`, "", "if_match_foo_bar", true},
		{`<%= if ("foo" ~= "^fo") { %>good<% } %>`, "good", "if_match_foo_^fo", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			s, err := plush.Render(tc.input, plush.NewContext())
			if tc.success {
				r.NoError(err)
				r.Equal(tc.expected, s)
			} else {
				r.Error(err, tc.expected)
			}
		})
	}
}

func Test_Condition_Only(t *testing.T) {
	ctx_empty := plush.NewContext()

	ctx_with_paths := plush.NewContext()
	ctx_with_paths.Set("paths", "cart")

	tests := []struct {
		input    string
		expected string
		name     string
		success  bool
		context  *plush.Context
	}{
		{`<%= paths == nil %>`, "true", "unknown_equal_to_nil", true, ctx_empty},
		{`<%= nil == paths %>`, "true", "nil_equal_to_unknown", true, ctx_empty},

		{`<%= !paths %>`, "false", "NOT SET", true, ctx_with_paths},
		{`<%= !pages %>`, "true", "NOT UNKNOWN", true, ctx_with_paths},

		{`<%= paths || pages %>`, "true", "SET_or_unknown", true, ctx_with_paths}, // fast return
		{`<%= pages || paths %>`, "true", "unknown_or_SET", true, ctx_with_paths},
		{`<%= paths || pages == "cart" %>`, "true", "SET_or_unknown_equal", true, ctx_with_paths}, // fast return
		{`<%= pages == "cart" || paths %>`, "true", "unknown_equal_or_SET", true, ctx_with_paths},
		{`<%= paths == "cart" || pages %>`, "true", "EQUAL_or_unknown", true, ctx_with_paths}, // fast return
		{`<%= pages || paths == "cart" %>`, "true", "unknown_or_EQUAL", true, ctx_with_paths},
		{`<%= paths == "cart" || pages == "cart" %>`, "true", "EQUAL_or_unknown_equal", true, ctx_with_paths}, // fast return
		{`<%= pages == "cart" || paths == "cart" %>`, "true", "unknown_equal_or_EQUAL", true, ctx_with_paths},

		{`<%= paths && pages %>`, "false", "set_and_UNKNOWN==false", true, ctx_with_paths},
		{`<%= pages && paths %>`, "false", "UNKNOWN_and_set==false", true, ctx_with_paths}, // fast return
		{`<%= paths && pages == "cart" %>`, "false", "set_and_UNKNOWN_equal==false", true, ctx_with_paths},
		{`<%= pages == "cart" && paths %>`, "false", "UNKNOWN_equal_and_set==false", true, ctx_with_paths}, // fast return
		{`<%= paths == "cart" && pages %>`, "false", "equal_and_UNKNOWN", true, ctx_with_paths},
		{`<%= pages && paths == "cart" %>`, "false", "UNKNOWN_and_equal", true, ctx_with_paths}, // fast return
		{`<%= paths == "cart" && pages == "cart" %>`, "false", "equal_and_UNKNOWN_equal", true, ctx_with_paths},
		{`<%= pages == "cart" && paths == "cart" %>`, "false", "UNKNOWN_equal_and_equal", true, ctx_with_paths}, // fast return
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			s, err := plush.Render(tc.input, tc.context)
			if tc.success {
				r.NoError(err)
				r.Equal(tc.expected, s)
			} else {
				r.Error(err, tc.expected)
			}
		})
	}
}

func Test_If_Else_If_Else_True(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	input := `<p><%= if (state == "foo") { %>hi foo<% } else if (state == "bar") { %>hi bar<% } else if (state == "fizz") { %>hi fizz<% } else { %>hi buzz<% } %></p>`

	ctx.Set("state", "foo")
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi foo</p>", s)

	ctx.Set("state", "bar")
	s, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi bar</p>", s)

	ctx.Set("state", "fizz")
	s, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi fizz</p>", s)

	ctx.Set("state", "buzz")
	s, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi buzz</p>", s)
}

func Test_If_String_Truthy(t *testing.T) {
	r := require.New(t)

	ctx := plush.NewContext()
	ctx.Set("username", "")

	input := `<p><%= if (username && username != "") { return "hi" } else { return "bye" } %></p>`
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>bye</p>", s)

	ctx.Set("username", "foo")
	s, err = plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("<p>hi</p>", s)
}

func Test_If_Variable_Not_Set_But_Or_Condition_Is_True_Complex(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("path", "cart")
	ctx.Set("paths", "cart")
	input := `<%= if ( paths == "cart" || (page && page.PageTitle != "cafe") || paths == "cart") { %>hi<%} %>`

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("hi", s)
}

func Test_If_Syntax_Error_On_Last_Node(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("paths", "cart")
	input := `<%= if ( paths == "cart" ||  pages ^^^ ) { %>hi<%} %>`

	_, err := plush.Render(input, ctx)
	r.Error(err)
}

func Test_If_Syntax_Error_On_First_Node(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("paths", "cart")
	input := `<%= if ( paths @#@# "cart" ||  pages) { %>hi<%} %>`

	_, err := plush.Render(input, ctx)
	r.Error(err)
}
