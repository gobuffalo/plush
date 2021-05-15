package plush

import (
	"database/sql"

	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorType(t *testing.T) {
	r := require.New(t)

	ctx := NewContext()
	ctx.Set("sqlError", func() error {
		return sql.ErrNoRows
	})

	_, err := Render(`<%= sqlError() %>`, ctx)
	r.True(errors.Is(err, sql.ErrNoRows))
}
