package plush

import (
	"html/template"

	"github.com/pkg/errors"
)

// ContentFor stores a block of templating code to be re-used later in the template
// via the contentOf helper.
// An optional map of values can be passed to contentOf,
// which are made available to the contentFor block.
/*
	<% contentFor("buttons") { %>
		<button>hi</button>
	<% } %>
*/
func contentForHelper(name string, help HelperContext) {
	help.Set("contentFor:"+name, func(data map[string]interface{}) (template.HTML, error) {
		for k, v := range data {
			help.Set(k, v)
		}
		body, err := help.Block()
		if err != nil {
			return "", errors.WithStack(err)
		}
		return template.HTML(body), nil
	})
}

// ContentOf retrieves a stored block for templating and renders it.
// You can pass an optional map of fields that will be set.
/*
	<%= contentOf("buttons") %>
	<%= contentOf("buttons", {"label": "Click me"}) %>
*/
func contentOfHelper(name string, data map[string]interface{}, help HelperContext) (template.HTML, error) {
	fn := help.Value("contentFor:" + name).(func(data map[string]interface{}) (template.HTML, error))
	return fn(data)
}
