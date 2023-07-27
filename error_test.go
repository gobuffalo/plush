package plush_test

import (
	"database/sql"

	"errors"
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

func TestErrorType(t *testing.T) {
	r := require.New(t)

	ctx := plush.NewContext()
	ctx.Set("sqlError", func() error {
		return sql.ErrNoRows
	})

	_, err := plush.Render(`<%= sqlError() %>`, ctx)
	r.True(errors.Is(err, sql.ErrNoRows))
}
