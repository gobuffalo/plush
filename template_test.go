package plush_test

import (
	"testing"

	"golang.org/x/sync/errgroup"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func Test_Template_Exec_Concurrency(t *testing.T) {
	r := require.New(t)
	tmpl, err := plush.NewTemplate(``)
	r.NoError(err)
	exec := func() error {
		_, e := tmpl.Exec(plush.NewContext())
		return e
	}
	wg := errgroup.Group{}
	wg.Go(exec)
	wg.Go(exec)
	wg.Go(exec)
	err = wg.Wait()
	r.NoError(err)
}
