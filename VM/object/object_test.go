package object

import (
	"math"
	"testing"

	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/token"
	"github.com/stretchr/testify/require"
)

func Test_Function_Literal(t *testing.T) {
	fn := &FunctionLiteral{
		Parameters: []*ast.Identifier{
			{
				TokenAble: ast.TokenAble{Token: token.Token{Type: token.IDENT, Literal: "x"}},
				Value:     "x",
			},
		},
		Body: &ast.BlockStatement{},
	}

	require.Equal(t, ObjectType(FUNCTION_OBJ), fn.Type())
	require.Equal(t, "fn(x) {\n\n}", fn.Inspect())
}

func Test_Wrap_Preserves_Large_Uint_64(t *testing.T) {
	obj := Wrap(uint64(math.MaxUint64))

	native, ok := obj.(*Native)
	require.Truef(t, ok, "expected large uint64 to stay native, got %T", obj)
	require.Equal(t, uint64(math.MaxUint64), native.Value)
}

func Test_String_Hash_Key(t *testing.T) {
	hello1 := &String{Value: "Hello World"}
	hello2 := &String{Value: "Hello World"}
	diff1 := &String{Value: "My name is johnny"}
	diff2 := &String{Value: "My name is johnny"}

	require.Equal(t, hello1.HashKey(), hello2.HashKey())
	require.Equal(t, diff1.HashKey(), diff2.HashKey())
	require.NotEqual(t, hello1.HashKey(), diff1.HashKey())
}

func Test_Boolean_Hash_Key(t *testing.T) {
	true1 := &Boolean{Value: true}
	true2 := &Boolean{Value: true}
	false1 := &Boolean{Value: false}
	false2 := &Boolean{Value: false}

	require.Equal(t, true1.HashKey(), true2.HashKey())
	require.Equal(t, false1.HashKey(), false2.HashKey())
	require.NotEqual(t, true1.HashKey(), false1.HashKey())
}

func Test_Integer_Hash_Key(t *testing.T) {
	one1 := &Integer{Value: 1}
	one2 := &Integer{Value: 1}
	two1 := &Integer{Value: 2}
	two2 := &Integer{Value: 2}

	require.Equal(t, one1.HashKey(), one2.HashKey())
	require.Equal(t, two1.HashKey(), two2.HashKey())
	require.NotEqual(t, one1.HashKey(), two1.HashKey())
}

func Test_VM_Builtins(t *testing.T) {
	tests := []struct {
		name     string
		args     []Object
		expected Object
	}{
		{
			name:     "len",
			args:     []Object{&String{Value: "hello"}},
			expected: &Integer{Value: 5},
		},
		{
			name:     "len",
			args:     []Object{&Array{Elements: []Object{&Integer{Value: 1}, &Integer{Value: 2}}}},
			expected: &Integer{Value: 2},
		},
		{
			name:     "first",
			args:     []Object{&Array{Elements: []Object{&Integer{Value: 1}, &Integer{Value: 2}}}},
			expected: &Integer{Value: 1},
		},
		{
			name:     "last",
			args:     []Object{&Array{Elements: []Object{&Integer{Value: 1}, &Integer{Value: 2}}}},
			expected: &Integer{Value: 2},
		},
		{
			name:     "rest",
			args:     []Object{&Array{Elements: []Object{&Integer{Value: 1}, &Integer{Value: 2}}}},
			expected: &Array{Elements: []Object{&Integer{Value: 2}}},
		},
		{
			name:     "push",
			args:     []Object{&Array{Elements: []Object{}}, &Integer{Value: 1}},
			expected: &Array{Elements: []Object{&Integer{Value: 1}}},
		},
	}

	for _, tt := range tests {
		builtin := GetBuiltinByName(tt.name)
		require.NotNil(t, builtin)
		require.Equal(t, tt.expected, builtin.Fn(tt.args...))
	}
}

func Test_VM_Builtin_Errors(t *testing.T) {
	tests := []struct {
		name     string
		args     []Object
		expected string
	}{
		{
			name:     "len",
			args:     []Object{&Integer{Value: 1}},
			expected: "argument to `len` not supported, got INTEGER",
		},
		{
			name:     "len",
			args:     []Object{&String{Value: "one"}, &String{Value: "two"}},
			expected: "wrong number of arguments. got=2, want=1",
		},
		{
			name:     "first",
			args:     []Object{&Integer{Value: 1}},
			expected: "argument to `first` must be ARRAY, got INTEGER",
		},
		{
			name:     "last",
			args:     []Object{&Integer{Value: 1}},
			expected: "argument to `last` must be ARRAY, got INTEGER",
		},
		{
			name:     "push",
			args:     []Object{&Integer{Value: 1}, &Integer{Value: 1}},
			expected: "argument to `push` must be ARRAY, got INTEGER",
		},
	}

	for _, tt := range tests {
		builtin := GetBuiltinByName(tt.name)
		require.NotNil(t, builtin)
		result, ok := builtin.Fn(tt.args...).(*Error)
		require.Truef(t, ok, "expected error object for builtin %s", tt.name)
		require.Equal(t, tt.expected, result.Message)
	}
}
