package plush

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func Test_Template_Exec_Concurrency(t *testing.T) {
	r := require.New(t)
	tmpl, err := NewTemplate(``)
	r.NoError(err)
	exec := func() error {
		_, e := tmpl.Exec(NewContext())
		return e
	}
	wg := errgroup.Group{}
	wg.Go(exec)
	wg.Go(exec)
	wg.Go(exec)
	err = wg.Wait()
	r.NoError(err)
}
