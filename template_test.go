package plush

import (
	"testing"

	"golang.org/x/sync/errgroup"

	"github.com/stretchr/testify/require"
)

func Test_Template_Exec_Concurrency(t *testing.T) {
	r := require.New(t)
	tmpl, err := NewTemplate(``)
	r.NoError(err)
	tmpl.Helpers.Add("a", func() {})
	exec := func() error {
		_, err := tmpl.Exec(NewContext())
		return err
	}
	wg := errgroup.Group{}
	wg.Go(exec)
	wg.Go(exec)
	wg.Go(exec)
	err = wg.Wait()
	r.NoError(err)
}
