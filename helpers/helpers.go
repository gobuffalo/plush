package helpers

import (
	"github.com/gobuffalo/plush/v5/helpers/content"
	"github.com/gobuffalo/plush/v5/helpers/debug"
	"github.com/gobuffalo/plush/v5/helpers/encoders"
	"github.com/gobuffalo/plush/v5/helpers/env"
	"github.com/gobuffalo/plush/v5/helpers/escapes"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/inflections"
	"github.com/gobuffalo/plush/v5/helpers/iterators"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/helpers/paths"
	"github.com/gobuffalo/plush/v5/helpers/text"
)

var Content = content.New()
var Debug = debug.New()
var Encoders = encoders.New()
var Env = env.New()
var Escapes = escapes.New()
var Inflections = inflections.New()
var Iterators = iterators.New()
var Meta = meta.New()
var Paths = paths.New()
var Text = text.New()

var Base = hctx.Merge(
	Content,
	Debug,
	Encoders,
	Env,
	Escapes,
	Inflections,
	Iterators,
	Meta,
	Paths,
	Text,
)
