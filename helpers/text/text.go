package text

import "github.com/gobuffalo/plush/v5/helpers/hctx"

// Keys to be used in templates for the functions in this package.
const (
	TruncateKey = "truncate"
)

// New returns a map of the helpers within this package.
func New() hctx.Map {
	return hctx.Map{
		TruncateKey: Truncate,
	}
}
