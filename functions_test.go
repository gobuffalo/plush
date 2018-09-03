package plush

import (
	"errors"
	"fmt"
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"
)

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

func Test_Render_Unknown_Function_Call(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f() %></p>`
	_, err := Render(input, NewContext())
	r.Error(err)
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

func Test_Render_Function_Call_With_Error(t *testing.T) {
	r := require.New(t)

	input := `<p><%= f() %></p>`
	_, err := Render(input, NewContextWith(map[string]interface{}{
		"f": func() (string, error) {
			return "hi!", errors.New("oops")
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

func Test_Render_Function_Optional_Map(t *testing.T) {
	r := require.New(t)
	input := `<%= foo() %>|<%= bar({a: "A"}) %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"foo": func(opts map[string]interface{}, help HelperContext) string {
			return "foo"
		},
		"bar": func(opts map[string]interface{}) string {
			return opts["a"].(string)
		},
	}))
	r.NoError(err)
	r.Equal("foo|A", s)
}

func Test_Render_Function_With_Backticks_And_Quotes(t *testing.T) {
	// From https://github.com/gobuffalo/pop/issues/168
	r := require.New(t)
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

	output := `CREATE MATERIALIZED VIEW view_papers AS
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
   WITH DATA`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"raw": func(arg string) template.HTML {
			return template.HTML(arg)
		},
	}))
	r.NoError(err)
	r.Equal(output, s)
}
