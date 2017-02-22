package render

import (
	"io"

	"github.com/gobuffalo/buffalo/render"
)

// Renderer interface that must be satisfied to be used with
// buffalo.Context.Render
type Renderer interface {
	ContentType() string
	Render(io.Writer, render.Data) error
}
