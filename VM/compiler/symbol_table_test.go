package compiler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Symbol_Table_Define_And_Resolve(t *testing.T) {
	global := NewSymbolTable()

	a := global.Define("a")
	require.Equal(t, Symbol{Name: "a", Scope: GlobalScope, Index: 0}, a)
	b := global.Define("b")
	require.Equal(t, Symbol{Name: "b", Scope: GlobalScope, Index: 1}, b)

	firstLocal := NewEnclosedSymbolTable(global)
	c := firstLocal.Define("c")
	require.Equal(t, Symbol{Name: "c", Scope: LocalScope, Index: 0}, c)
	d := firstLocal.Define("d")
	require.Equal(t, Symbol{Name: "d", Scope: LocalScope, Index: 1}, d)

	secondLocal := NewEnclosedSymbolTable(firstLocal)
	e := secondLocal.Define("e")
	require.Equal(t, Symbol{Name: "e", Scope: LocalScope, Index: 0}, e)
	f := secondLocal.Define("f")
	require.Equal(t, Symbol{Name: "f", Scope: LocalScope, Index: 1}, f)

	tests := []struct {
		table    *SymbolTable
		name     string
		expected Symbol
	}{
		{global, "a", Symbol{Name: "a", Scope: GlobalScope, Index: 0}},
		{global, "b", Symbol{Name: "b", Scope: GlobalScope, Index: 1}},
		{firstLocal, "a", Symbol{Name: "a", Scope: GlobalScope, Index: 0}},
		{firstLocal, "b", Symbol{Name: "b", Scope: GlobalScope, Index: 1}},
		{firstLocal, "c", Symbol{Name: "c", Scope: LocalScope, Index: 0}},
		{firstLocal, "d", Symbol{Name: "d", Scope: LocalScope, Index: 1}},
		{secondLocal, "a", Symbol{Name: "a", Scope: GlobalScope, Index: 0}},
		{secondLocal, "b", Symbol{Name: "b", Scope: GlobalScope, Index: 1}},
		{secondLocal, "e", Symbol{Name: "e", Scope: LocalScope, Index: 0}},
		{secondLocal, "f", Symbol{Name: "f", Scope: LocalScope, Index: 1}},
	}

	for _, tt := range tests {
		actual, ok := tt.table.Resolve(tt.name)
		require.Truef(t, ok, "expected to resolve %q", tt.name)
		require.Equal(t, tt.expected, actual)
	}
}

func Test_Symbol_Table_Define_Resolve_Builtins(t *testing.T) {
	global := NewSymbolTable()
	firstLocal := NewEnclosedSymbolTable(global)
	secondLocal := NewEnclosedSymbolTable(firstLocal)

	expected := []Symbol{
		{Name: "a", Scope: BuiltinScope, Index: 0},
		{Name: "c", Scope: BuiltinScope, Index: 1},
		{Name: "e", Scope: BuiltinScope, Index: 2},
		{Name: "f", Scope: BuiltinScope, Index: 3},
	}

	for i, symbol := range expected {
		global.DefineBuiltin(i, symbol.Name)
	}

	for _, table := range []*SymbolTable{global, firstLocal, secondLocal} {
		for _, symbol := range expected {
			actual, ok := table.Resolve(symbol.Name)
			require.Truef(t, ok, "expected to resolve builtin %q", symbol.Name)
			require.Equal(t, symbol, actual)
		}
	}
}

func Test_Symbol_Table_Resolve_Free(t *testing.T) {
	global := NewSymbolTable()
	global.Define("a")
	global.Define("b")

	firstLocal := NewEnclosedSymbolTable(global)
	firstLocal.Define("c")
	firstLocal.Define("d")

	secondLocal := NewEnclosedSymbolTable(firstLocal)
	secondLocal.Define("e")
	secondLocal.Define("f")

	tests := []struct {
		table       *SymbolTable
		expected    []Symbol
		freeSymbols []Symbol
	}{
		{
			table: firstLocal,
			expected: []Symbol{
				{Name: "a", Scope: GlobalScope, Index: 0},
				{Name: "b", Scope: GlobalScope, Index: 1},
				{Name: "c", Scope: LocalScope, Index: 0},
				{Name: "d", Scope: LocalScope, Index: 1},
			},
			freeSymbols: []Symbol{},
		},
		{
			table: secondLocal,
			expected: []Symbol{
				{Name: "a", Scope: GlobalScope, Index: 0},
				{Name: "b", Scope: GlobalScope, Index: 1},
				{Name: "c", Scope: FreeScope, Index: 0},
				{Name: "d", Scope: FreeScope, Index: 1},
				{Name: "e", Scope: LocalScope, Index: 0},
				{Name: "f", Scope: LocalScope, Index: 1},
			},
			freeSymbols: []Symbol{
				{Name: "c", Scope: LocalScope, Index: 0},
				{Name: "d", Scope: LocalScope, Index: 1},
			},
		},
	}

	for _, tt := range tests {
		for _, symbol := range tt.expected {
			actual, ok := tt.table.Resolve(symbol.Name)
			require.Truef(t, ok, "expected to resolve %q", symbol.Name)
			require.Equal(t, symbol, actual)
		}
		require.Equal(t, tt.freeSymbols, tt.table.FreeSymbols)
	}
}

func Test_Symbol_Table_Resolve_Unresolvable_Free(t *testing.T) {
	global := NewSymbolTable()
	global.Define("a")

	firstLocal := NewEnclosedSymbolTable(global)
	firstLocal.Define("c")

	secondLocal := NewEnclosedSymbolTable(firstLocal)
	secondLocal.Define("e")
	secondLocal.Define("f")

	expected := []Symbol{
		{Name: "a", Scope: GlobalScope, Index: 0},
		{Name: "c", Scope: FreeScope, Index: 0},
		{Name: "e", Scope: LocalScope, Index: 0},
		{Name: "f", Scope: LocalScope, Index: 1},
	}

	for _, symbol := range expected {
		actual, ok := secondLocal.Resolve(symbol.Name)
		require.Truef(t, ok, "expected to resolve %q", symbol.Name)
		require.Equal(t, symbol, actual)
	}

	_, ok := secondLocal.Resolve("b")
	require.False(t, ok)
	_, ok = secondLocal.Resolve("d")
	require.False(t, ok)
}

func Test_Symbol_Table_Function_Name_And_Shadowing(t *testing.T) {
	global := NewSymbolTable()
	fn := global.DefineFunctionName("a")
	require.Equal(t, Symbol{Name: "a", Scope: FunctionScope, Index: 0}, fn)

	actual, ok := global.Resolve("a")
	require.True(t, ok)
	require.Equal(t, fn, actual)

	shadow := global.Define("a")
	require.Equal(t, Symbol{Name: "a", Scope: GlobalScope, Index: 0}, shadow)

	actual, ok = global.Resolve("a")
	require.True(t, ok)
	require.Equal(t, shadow, actual)
}

func Test_Inline_Block_Symbol_Table_Uses_Function_Local_Owner(t *testing.T) {
	global := NewSymbolTable()
	fn := NewEnclosedSymbolTable(global)
	param := fn.Define("value")

	inline := NewInlineBlockSymbolTable(fn)
	local := inline.Define("inside")

	require.Equal(t, Symbol{Name: "value", Scope: LocalScope, Index: 0}, param)
	require.Equal(t, Symbol{Name: "inside", Scope: LocalScope, Index: 1}, local)
	require.Equal(t, 2, fn.numDefinitions)

	actual, ok := inline.Resolve("value")
	require.True(t, ok)
	require.Equal(t, param, actual)
	require.Empty(t, inline.FreeSymbols)
}
