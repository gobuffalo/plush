package plush

import (
	"github.com/gobuffalo/plush/v5/helpers"
)

// Helpers contains all of the default helpers for
// These will be available to all templates. You should add
// any custom global helpers to this list.
var Helpers = helpers.NewMap(map[string]interface{}{})

func init() {
	Helpers.AddMany(helpers.Base)
	Helpers.Add("partial", PartialHelper)
}
