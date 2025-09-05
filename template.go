package plush

import (
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/hctx"

	"github.com/gobuffalo/plush/v5/parser"
)

// Template represents an input and helpers to be used
// to evaluate and render the input.
type Template struct {
	Input     string
	Program   *ast.Program
	PunchHole []HoleMarker
	Skeleton  string
	IsCache   bool
}

// NewTemplate from the input string. Adds all of the
// global helper functions from "Helpers", this function does not
// cache the template.
func NewTemplate(input string) (*Template, error) {
	t := &Template{
		Input: input,
	}

	err := t.Parse()
	if err != nil {
		return t, err
	}

	return t, nil
}

// Parse the template this can be called many times
// as a successful result is cached and is used on subsequent
// uses.
func (t *Template) Parse() error {
	if t.Program != nil {
		return nil
	}

	program, err := parser.Parse(t.Input)
	if err != nil {
		return err
	}

	t.Program = program
	return nil
}

// Exec the template using the content and return the results
func (t *Template) Exec(ctx hctx.Context) (string, []HoleMarker, error) {
	err := t.Parse()
	if err != nil {
		return "", nil, err
	}

	ev := compiler{
		ctx:     ctx,
		program: t.Program,
	}

	s, err := ev.compile()
	return s, ev.positionStartEnds, err
}

// Clone a template. This is useful for defining helpers on per "instance" of the template.
func (t *Template) Clone() *Template {
	t2 := &Template{
		Input:   t.Input,
		Program: t.Program,
	}
	return t2
}
