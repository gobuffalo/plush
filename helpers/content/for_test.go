package content

import (
	"errors"
	"testing"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/helptest"
	"github.com/stretchr/testify/require"
)

func Test_ContentFor(t *testing.T) {
	r := require.New(t)

	in := "<button>hi</button>"
	hc := helptest.NewContext()
	hc.BlockContextFn = func(c hctx.Context) (string, error) {
		return in, nil
	}

	cf := hc.New().(*helptest.HelperContext)
	ContentFor("buttons", hc)
	s, err := ContentOf("buttons", hctx.Map{}, cf)
	r.NoError(err)
	r.Contains(s, in)
}

func Test_ContentFor_Fail(t *testing.T) {
	r := require.New(t)

	hc := helptest.NewContext()
	hc.BlockContextFn = func(c hctx.Context) (string, error) {
		return "", errors.New("nope")
	}

	cf := hc.New().(*helptest.HelperContext)
	ContentFor("buttons", hc)
	s, err := ContentOf("buttons", hctx.Map{}, cf)
	r.Error(err)
	r.Empty(s)
}
