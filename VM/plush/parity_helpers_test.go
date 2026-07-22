package plush_test

import (
	"errors"
	"fmt"
	"html/template"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/stretchr/testify/require"
)

func Test_Parity_Phase_6_Helper_Invocation_Matrix(t *testing.T) {
	t.Run("context and positional args", func(t *testing.T) {
		compareRender(t, `<%= greet(name) %>`, contextWith(map[string]interface{}{
			"name": "mark",
			"greet": func(name string) string {
				return "hi " + name
			},
		}))
	})

	t.Run("variable arg expression", func(t *testing.T) {
		compareRender(t, `<%= greet(first + last) %>`, contextWith(map[string]interface{}{
			"first": "ma",
			"last":  "rk",
			"greet": func(name string) string {
				return "hi " + name
			},
		}))
	})

	t.Run("variadic helpers", func(t *testing.T) {
		compareRender(t, `<%= count(1, 2, 3) %>|<%= prefix("a", "b", "c") %>|<%= empty() %>|<%= fixed(4) %>`, contextWith(map[string]interface{}{
			"count": func(values ...int) int {
				return len(values)
			},
			"prefix": func(first string, rest ...string) string {
				return first + fmt.Sprint(len(rest))
			},
			"empty": func(values ...int) int {
				return len(values)
			},
			"fixed": func(first int, rest ...int) int {
				return first + len(rest)
			},
		}))
	})

	t.Run("variadic wrong type errors", func(t *testing.T) {
		compareBothRenderError(t, `<%= count(1, 2, "bad") %>`, contextWith(map[string]interface{}{
			"count": func(values ...int) int {
				return len(values)
			},
		}))
	})

	t.Run("nil args", func(t *testing.T) {
		compareRender(t, `<%= lookup(nil, "k") %><%= lookup(one, "k") %>`, contextWith(map[string]interface{}{
			"one": map[string]string{"k": "test"},
			"lookup": func(values map[string]string, key string) string {
				if values == nil {
					return ""
				}
				return values[key]
			},
		}))
	})
}

func Test_Parity_Phase_6_Helper_Return_Matrix(t *testing.T) {
	t.Run("no return", func(t *testing.T) {
		compareRender(t, `<%= remember("x") %><%= value %>`, contextWith(map[string]interface{}{
			"value": "",
			"remember": func(value string) {
			},
		}))
	})

	t.Run("single return", func(t *testing.T) {
		compareRender(t, `<%= greet() %>`, contextWith(map[string]interface{}{
			"greet": func() string {
				return "hello"
			},
		}))
	})

	t.Run("value error success", func(t *testing.T) {
		compareRender(t, `<%= greet() %>`, contextWith(map[string]interface{}{
			"greet": func() (string, error) {
				return "hello", nil
			},
		}))
	})

	t.Run("value error failure", func(t *testing.T) {
		compareBothRenderError(t, `<%= fail() %>`, contextWith(map[string]interface{}{
			"fail": func() (string, error) {
				return "hidden", errors.New("phase6 failure")
			},
		}))
	})
}

func Test_Parity_Phase_6_Optional_Injection_Matrix(t *testing.T) {
	t.Run("map injection", func(t *testing.T) {
		compareRender(t, `<%= foo() %>|<%= bar({name: "mark"}) %>`, contextWith(map[string]interface{}{
			"foo": func(opts map[string]interface{}) string {
				return fmt.Sprint(len(opts))
			},
			"bar": func(opts map[string]interface{}) string {
				return opts["name"].(string)
			},
		}))
	})

	t.Run("plush helper context", func(t *testing.T) {
		compareRender(t, `<%= blockCheck() { return "block" } %>|<%= blockCheck() %>`, contextWith(map[string]interface{}{
			"blockCheck": func(help rootplush.HelperContext) string {
				if help.HasBlock() {
					body, _ := help.Block()
					return body
				}
				return "no block"
			},
		}))
	})

	t.Run("hctx helper context", func(t *testing.T) {
		compareRender(t, `<%= blockCheck() { return "block" } %>|<%= blockCheck() %>`, contextWith(map[string]interface{}{
			"blockCheck": func(help hctx.HelperContext) string {
				if help.HasBlock() {
					body, _ := help.Block()
					return body
				}
				return "no block"
			},
		}))
	})
}

func Test_Parity_Phase_6_Helper_Context_Block_With_And_Render(t *testing.T) {
	t.Run("block with child context", func(t *testing.T) {
		compareRender(t, `<%= wrap({name: "mark"}) { return prefix + name } %><%= name %>`, contextWith(map[string]interface{}{
			"name":   "outer",
			"prefix": "hi ",
			"wrap": func(data map[string]interface{}, help rootplush.HelperContext) (template.HTML, error) {
				ctx := help.New()
				for k, v := range data {
					ctx.Set(k, v)
				}
				body, err := help.BlockWith(ctx)
				return template.HTML(body), err
			},
		}))
	})

	t.Run("render with current context", func(t *testing.T) {
		compareRender(t, `<%= renderSnippet() %>`, contextWith(map[string]interface{}{
			"name": "mark",
			"renderSnippet": func(help rootplush.HelperContext) (template.HTML, error) {
				body, err := help.Render(`<strong><%= name %></strong>`)
				return template.HTML(body), err
			},
		}))
	})
}

func Test_Parity_Phase_6_Helper_Block_Scope_Behavior(t *testing.T) {
	t.Run("assignment updates outer scope", func(t *testing.T) {
		compareRender(t, `<% let value = "before" %><%= run() { value = "after" } %><%= value %>`, contextWith(map[string]interface{}{
			"run": func(help rootplush.HelperContext) string {
				body, _ := help.Block()
				return body
			},
		}))
	})

	t.Run("local let does not leak", func(t *testing.T) {
		compareBothRenderError(t, `<%= run() { let inner = "secret" } %><%= inner %>`, contextWith(map[string]interface{}{
			"run": func(help rootplush.HelperContext) string {
				body, _ := help.Block()
				return body
			},
		}))
	})
}

func Test_Parity_Phase_6_Method_Calls_And_Helper_Return_Chains(t *testing.T) {
	compareRender(t, `<%= greeter.Greet("mark") %>|<%= makeGreeter().Greet("mido") %>|<%= makeName().Echo() %>`, contextWith(map[string]interface{}{
		"greeter": phase6Greeter{},
		"makeGreeter": func() phase6Greeter {
			return phase6Greeter{}
		},
		"makeName": func() phase6Name {
			return phase6Name("plush")
		},
	}))
}

type phase6Greeter struct{}

func (phase6Greeter) Greet(name string) string {
	return "hi " + name
}

type phase6Name string

func (n phase6Name) Echo() string {
	return string(n)
}

func Test_Parity_Phase_6_Unknown_Helper_Errors(t *testing.T) {
	compareBothRenderError(t, `<%= missingHelper() %>`, emptyContext)
}

func Test_Parity_Phase_6_Helper_Error_Contains_Returned_Error(t *testing.T) {
	_, err := renderVM(`<%= fail() %>`, contextWith(map[string]interface{}{
		"fail": func() (string, error) {
			return "", errors.New("phase6 returned error")
		},
	}))
	require.Error(t, err)
	require.ErrorContains(t, err, "phase6 returned error")
}
