package vm

import (
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/stretchr/testify/require"
)

func Test_VM_Render_Linked_Partial_Bytecode_Inline_Edges(t *testing.T) {
	var out strings.Builder
	ok, err := renderLinkedPartialBytecodeInline(nil, nil, nil)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderLinkedPartialBytecodeInline(&out, nil, nil)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderLinkedPartialBytecodeInline(&out, &partialBytecodeLink{}, nil)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderLinkedPartialBytecodeInline(&out, &partialBytecodeLink{bytecode: &compiler.Bytecode{
		Static:       true,
		StaticOutput: "<static>",
	}}, plush.NewContext())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "<static>", out.String())

	out.Reset()
	ok, err = renderLinkedPartialBytecodeInline(&out, &partialBytecodeLink{bytecode: &compiler.Bytecode{
		HasHoles: true,
	}}, plush.NewContext())
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, out.String())

	ok, err = renderLinkedPartialBytecodeInline(&out, &partialBytecodeLink{bytecode: &compiler.Bytecode{}}, plush.NewContext())
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, out.String())
}
