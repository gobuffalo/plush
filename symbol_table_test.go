package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func TestSymbolTable_NewSymbolTable(t *testing.T) {
	r := require.New(t)

	scope := plush.NewScope(nil)

	r.NotNil(scope)
}

func TestSymbolTable_Declare_And_Resolve(t *testing.T) {
	r := require.New(t)

	scope := plush.NewScope(nil)
	r.NotNil(scope)
	scope.Declare("x", 42)

	val, ok := scope.Resolve("x")

	r.True(ok)
	r.Equal(42, val)
}

func TestSymbolTable_Declare_And_Has(t *testing.T) {
	r := require.New(t)

	scope := plush.NewScope(nil)
	r.NotNil(scope)
	scope.Declare("x", 42)

	ok := scope.Has("x")

	r.True(ok)
}

func TestSymbolTable_Declare_And_Has_Child(t *testing.T) {
	r := require.New(t)

	scope := plush.NewScope(nil)
	r.NotNil(scope)
	scope.Declare("x", 42)
	childA := plush.NewScope(scope)
	r.NotNil(scope)
	childA.Declare("y", 42)
	ok := childA.Has("x")

	r.True(ok)

	ok = childA.Has("y")
	r.True(ok)

	ok = childA.Has("d")
	r.False(ok)
}
func TestSymbolTable_Resolve_From_Parent_Scope(t *testing.T) {
	r := require.New(t)

	parent := plush.NewScope(nil)

	r.NotNil(parent)

	parent.Declare("y", "hello")

	child := plush.NewScope(parent)

	r.NotNil(child)
	val, ok := child.Resolve("y")

	r.Equal("hello", val)
	r.True(ok)
}

func TestSymbolTable_Assign_To_ParentScope(t *testing.T) {
	r := require.New(t)
	parent := plush.NewScope(nil)

	r.NotNil(parent)

	parent.Declare("z", 100)

	child := plush.NewScope(parent)

	r.NotNil(child)

	assigned := child.Assign("z", 200)

	r.True(assigned)

	valC, okC := child.Resolve("z")

	r.True(okC)
	r.Equal(200, valC)

	val, ok := parent.Resolve("z")

	r.True(ok)
	r.Equal(200, val)
}

func TestSymbolTable_Assign_Non_Existent_Fails(t *testing.T) {
	r := require.New(t)

	scope := plush.NewScope(nil)

	r.NotNil(scope)

	assigned := scope.Assign("nonexistent", 123)

	r.False(assigned)
}

func TestSymbolTable_Declare_Nil_Ignored(t *testing.T) {
	r := require.New(t)
	scope := plush.NewScope(nil)

	r.NotNil(scope)

	scope.Declare("a", nil)

	_, ok := scope.Resolve("a")
	r.False(ok)
}

func TestSymbolTable_Shadowing(t *testing.T) {

	r := require.New(t)

	root := plush.NewScope(nil)

	r.NotNil(root)

	root.Declare("v", 1)

	child := plush.NewScope(root)

	r.NotNil(child)
	child.Declare("v", 2)

	valRoot, okRoot := root.Resolve("v")
	valChild, okChild := child.Resolve("v")

	r.True(okRoot)
	r.Equal(1, valRoot)

	r.True(okChild)
	r.Equal(2, valChild)
}
