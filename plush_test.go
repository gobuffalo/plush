package plush

import (
	"testing"

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

func Test_Render_HTML_InjectedString(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "mark" %></p>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("<p>mark</p>", s)
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

func Test_Render_Missing_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	_, err := Render(input, NewContext())
	r.Error(err)
}

func Test_Render_ShowNoShow(t *testing.T) {
	r := require.New(t)
	input := `<%= "shown" %><% "notshown" %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("shown", s)
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

func Test_BuffaloRenderer(t *testing.T) {
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

// func Test_FizzParses(t *testing.T) {
// 	r := require.New(t)
// 	_, err := Parse(createTable)
// 	r.NoError(err)
// }

// func Test_FizzWorks(t *testing.T) {
// 	r := require.New(t)
//
// 	table := &table{
// 		data: map[string]column{},
// 	}
//
// 	ctx := NewContext()
// 	ctx.Set("create_table", func(tn string, help HelperContext) error {
// 		c := ctx.New()
// 		c.Set("t", table)
// 		_, err := help.BlockWith(c)
// 		return err
// 	})
//
// 	err := RunScript(createTable, ctx)
// 	r.NoError(err)
// 	r.Len(table.data, 4)
//
// 	c := table.data["id"]
// 	r.Equal("id", c.name)
// 	r.Equal("uuid", c.tiep)
// 	r.True(c.opts["primary"].(bool))
//
// 	c = table.data["name"]
// 	r.Equal("name", c.name)
// 	r.Equal("string", c.tiep)
// 	r.Empty(c.opts)
// }

type table struct {
	data map[string]column
}

type column struct {
	name string
	tiep string
	opts map[string]interface{}
}

func (table *table) Column(n string, t string, opts map[string]interface{}) {
	table.data[n] = column{
		name: n,
		tiep: t,
		opts: opts,
	}
}

const createTable = `create_table("toys") {
	t.Column("id", "uuid", {primary: true})
	t.Column("name", "string")
	t.Column("description", "text", {null: true})
	t.Column("size", "integer")
}`
