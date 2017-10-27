package plush

import (
	"fmt"
	"html/template"
	"sync"

	"github.com/pkg/errors"
)

var cache = map[string]*Template{}
var moot = &sync.Mutex{}

// BuffaloRenderer implements the render.TemplateEngine interface allowing velvet to be used as a template engine
// for Buffalo
func BuffaloRenderer(input string, data map[string]interface{}, helpers map[string]interface{}) (string, error) {
	t, err := Parse(input)
	if err != nil {
		return "", err
	}
	if helpers != nil {
		for k, v := range helpers {
			data[k] = v
		}
	}
	return t.Exec(NewContextWith(data))
}

// Parse an input string and return a Template, and caches the parsed template.
func Parse(input string) (*Template, error) {
	moot.Lock()
	defer moot.Unlock()
	if t, ok := cache[input]; ok {
		return t, nil
	}
	t, err := NewTemplate(input)

	if err == nil {
		cache[input] = t
	}

	if err != nil {
		return t, errors.WithStack(err)
	}

	return t, nil
}

// Render a string using the given the context.
func Render(input string, ctx *Context) (string, error) {
	t, err := Parse(input)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return t.Exec(ctx)
}

// RunScript allows for "pure" plush scripts to be executed.
func RunScript(input string, ctx *Context) error {
	input = "<% " + input + "%>"

	ctx = ctx.New()
	ctx.Set("print", func(i interface{}) {
		fmt.Print(i)
	})
	ctx.Set("println", func(i interface{}) {
		fmt.Println(i)
	})

	_, err := Render(input, ctx)
	return err
}

type interfaceable interface {
	Interface() interface{}
}

// HTMLer generates HTML source
type HTMLer interface {
	HTML() template.HTML
}
