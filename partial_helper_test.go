package plush

import (
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func Test_PartialHelper_Nil_Context(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{}

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "invalid context")
	r.Equal("", string(html))
}

func Test_PartialHelper_Blank_Context(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "could not found")
	r.Equal("", string(html))
}

func Test_PartialHelper_Invalid_Feeder(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", "me-rong")

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "could not found")
	r.Equal("", string(html))
}

func Test_PartialHelper_Invalid_FeederFunction(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) string {
		return "me-rong"
	})

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "could not found")
	r.Equal("", string(html))
}

func Test_PartialHelper_Feeder_Error(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return "", errors.New("me-rong")
	})

	_, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "me-rong")
}

func Test_PartialHelper_Good(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<div class="test">Plush!</div>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<div class="test">Plush!</div>`, string(html))
}

func Test_PartialHelper_With_Data(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{"name": "Yonghwan"}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<div class="test">Hello <%= name %></div>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<div class="test">Hello Yonghwan</div>`, string(html))
}

func Test_PartialHelper_With_InternalChange(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContextWith(map[string]interface{}{
		"number": 3,
	})}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<% let number = number - 1
		%><div class="test">Hello <%= number %></div>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<div class="test">Hello 2</div>`, string(html))
	r.Equal(3, help.Value("number"))
}

func Test_PartialHelper_With_Recursion(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContextWith(map[string]interface{}{
		"number": 3,
	})}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<%=
		if (number > 0) { %><%
			let number = number - 1 %><%=
			partial("index") %><%= number %>, <%
		} %>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`0, 1, 2, `, string(html))
	r.Equal(3, help.Value("number"))
}

func Test_PartialHelper_Render_Error(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<div class="test">Hello <%= name </div>`, nil
	})

	_, err := partialHelper(name, data, help)
	r.Error(err)
}

func Test_PartialHelper_With_Layout(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{
		"name":   "Yonghwan",
		"layout": "container",
	}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(name string) (string, error) {
		if name == "container" {
			return `<html><%= yield %></html>`, nil
		}
		return `<div class="test">Hello <%= name %></div>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<html><div class="test">Hello Yonghwan</div></html>`, string(html))
}

func Test_PartialHelper_JavaScript(t *testing.T) {
	r := require.New(t)

	name := "index.js"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("contentType", "application/javascript")
	help.Set("partialFeeder", func(string) (string, error) {
		return `alert('\'Hello\'');`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`alert('\'Hello\'');`, string(html))
}

func Test_PartialHelper_JavaScript_Without_Extension(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("contentType", "application/javascript")
	help.Set("partialFeeder", func(string) (string, error) {
		return `alert('\'Hello\'');`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`alert('\'Hello\'');`, string(html))
}

func Test_PartialHelper_Javascript_With_HTML(t *testing.T) {
	r := require.New(t)

	name := "index.html"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("contentType", "application/javascript")
	help.Set("partialFeeder", func(string) (string, error) {
		return `alert('\'Hello\'');`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`alert(\'\\\'Hello\\\'\');`, string(html))
}

func Test_PartialHelper_Markdown(t *testing.T) {
	r := require.New(t)

	name := "index.md"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("contentType", "text/markdown")
	help.Set("partialFeeder", func(string) (string, error) {
		return "`test`", nil
	})

	md, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<p><code>test</code></p>`, strings.TrimSpace(string(md)))
}

func Test_PartialHelper_Markdown_With_Layout(t *testing.T) {
	r := require.New(t)

	name := "index.md"
	data := map[string]interface{}{
		"layout": "container.html",
	}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(name string) (string, error) {
		if name == data["layout"] {
			return `<html>This <em>is</em> a <%= yield %></html>`, nil
		}
		return `**test**`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal("<html>This <em>is</em> a <p><strong>test</strong></p></html>", string(html))
}

func Test_PartialHelper_Markdown_With_Layout_Reversed(t *testing.T) {
	r := require.New(t)

	name := "index.html"
	data := map[string]interface{}{
		"layout": "container.md",
	}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(name string) (string, error) {
		if name == data["layout"] {
			return `This *is* a <%= yield %>`, nil
		}
		return `<strong>test</strong>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<p>This <em>is</em> a <strong>test</strong></p>`, strings.TrimSpace(string(html)))
}

func Test_PartialHelpers_With_Indentation(t *testing.T) {
	r := require.New(t)

	main := `<div>
    <div>
        <%= partial("dummy.md") %>
    </div>
</div>`
	partial := "```go\n" +
		"if true {\n" +
		"    fmt.Println()\n" +
		"}\n" +
		"```"

	ctx := NewContext()
	ctx.Set("partialFeeder", func(string) (string, error) {
		return partial, nil
	})

	html, err := Render(main, ctx)
	r.NoError(err)
	r.Equal(`<div>
    <div>
        <div class="highlight highlight-go"><pre>if true {
    fmt.Println()
}
</pre></div>
    </div>
</div>`,
		string(html))
}
