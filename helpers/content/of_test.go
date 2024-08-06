package content

import (
	"html/template"
	"testing"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/helptest"
	"github.com/stretchr/testify/require"
)

func Test_ContentOf_MissingBlock(t *testing.T) {
	r := require.New(t)

	cf := helptest.NewContext()
	s, err := ContentOf("buttons", hctx.Map{}, cf)
	r.Error(err)
	r.Empty(s)
}

func Test_ContentOf_MissingBlock_DefaultBlock(t *testing.T) {
	r := require.New(t)

	cf := helptest.NewContext()
	cf.BlockContextFn = func(hctx.Context) (string, error) {
		return "default", nil
	}

	s, err := ContentOf("buttons", hctx.Map{}, cf)
	r.NoError(err)
	r.Equal(s, template.HTML("default"))
}

func Test_ContentOf(t *testing.T) {
	r := require.New(t)

	cf := helptest.NewContext()
	cf.BlockContextFn = func(hctx.Context) (string, error) {
		return "default", nil
	}

	name := "testing"
	cf.Set("contentFor:"+name, func(data hctx.Map) (template.HTML, error) {
		return template.HTML("body"), nil
	})

	s, err := ContentOf(name, hctx.Map{}, cf)
	r.NoError(err)
	r.Equal(s, template.HTML("body"))
}
