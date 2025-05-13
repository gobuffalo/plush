package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func TestInternTable(t *testing.T) {
	r := require.New(t)

	it := plush.NewInternTable()

	r.NotNil(it)
	// Intern a new string
	id1 := it.Intern("alpha")

	r.Equal(0, id1)

	// Intern another string
	id2 := it.Intern("beta")

	r.Equal(1, id2)

	// Re-intern the first string
	id1Again := it.Intern("alpha")

	r.Equal(id1, id1Again)

	// Lookup existing string
	lookupID, found := it.Lookup("beta")
	r.True(found)
	r.NotEqual(id1, lookupID)
	r.Equal(id2, lookupID)

	// Lookup non-existent string
	_, found = it.Lookup("gamma")
	r.False(found)

	// SymbolName for known ID
	name := it.SymbolName(id1)
	r.Equal("alpha", name)

	// SymbolName for unknown ID
	unknown := it.SymbolName(999)
	r.Equal("<unknown>", unknown)
}

func TestNewInternTable(t *testing.T) {
	r := require.New(t)

	rs := plush.NewInternTable()
	r.NotNil(rs)
}
