package plush_test

import (
	"fmt"
	"io"
	"os"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/stretchr/testify/require"
)

func Test_Render_Uses_Compiled_VM_Path(t *testing.T) {
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"name": "mark",
	})

	out, err := vmplush.Render(`<p><%= name %></p>`, ctx)
	if err != nil {
		t.Fatalf("Render returned error: %s", err)
	}
	if out != "<p>mark</p>" {
		t.Fatalf("wrong output: %q", out)
	}
}

func Test_Root_Render_And_VM_Render_Coexist(t *testing.T) {
	input := `<%= name %>`

	rootOut, rootErr := rootplush.Render(input, rootplush.NewContextWith(map[string]interface{}{
		"name": "root",
	}))
	if rootErr != nil {
		t.Fatalf("root Render returned error: %s", rootErr)
	}

	vmOut, vmErr := vmplush.Render(input, rootplush.NewContextWith(map[string]interface{}{
		"name": "vm",
	}))
	if vmErr != nil {
		t.Fatalf("VM Render returned error: %s", vmErr)
	}

	if rootOut != "root" {
		t.Fatalf("root renderer output changed: %q", rootOut)
	}
	if vmOut != "vm" {
		t.Fatalf("VM renderer output wrong: %q", vmOut)
	}
}

func Test_Set_Fast_Helper_Custom_Fast_Render_And_Fallback(t *testing.T) {
	fallbackCalls := 0
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"amount": 12.5,
		"money": func(value interface{}) string {
			fallbackCalls++
			return fmt.Sprintf("fallback:%v", value)
		},
	})
	vmplush.SetFastHelper(ctx, "money", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
		amount, ok := args.Float64(0)
		if !ok {
			return vmplush.ErrFastUnsupported
		}
		w.WriteEscapedString(fmt.Sprintf("$%.2f", amount))
		return nil
	})

	out, err := vmplush.Render(`<%= money(amount) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "$12.50", out)
	require.Zero(t, fallbackCalls)

	ctx.Set("amount", "n/a")
	out, err = vmplush.Render(`<%= money(amount) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fallback:n/a", out)
	require.Equal(t, 1, fallbackCalls)
}

func Test_Set_Fast_Helper_Custom_Bytecode_Path(t *testing.T) {
	fallbackCalls := 0
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"amount": int32(7),
		"money": func(value interface{}) string {
			fallbackCalls++
			return fmt.Sprintf("fallback:%v", value)
		},
	})
	vmplush.SetFastHelper(ctx, "money", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
		amount, ok := args.Int64(0)
		if !ok {
			return vmplush.ErrFastUnsupported
		}
		w.WriteEscapedString(fmt.Sprintf("fast:%d", amount))
		return nil
	})

	out, err := vmplush.Render(`<% let local = amount %><%= money(local) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fast:7", out)
	require.Zero(t, fallbackCalls)
}

func Test_Clear_Fast_Helper_Removes_Custom_Fast_Render(t *testing.T) {
	fastCalls := 0
	fallbackCalls := 0
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"name": "plush",
		"label": func(value interface{}) string {
			fallbackCalls++
			return fmt.Sprintf("fallback:%v", value)
		},
	})
	vmplush.SetFastHelper(ctx, "label", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
		fastCalls++
		value, ok := args.String(0)
		if !ok {
			return vmplush.ErrFastUnsupported
		}
		w.WriteEscapedString("fast:" + value)
		return nil
	})

	out, err := vmplush.Render(`<%= label(name) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fast:plush", out)
	require.Equal(t, 1, fastCalls)
	require.Zero(t, fallbackCalls)

	vmplush.ClearFastHelper(ctx, "label")
	out, err = vmplush.Render(`<%= label(name) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fallback:plush", out)
	require.Equal(t, 1, fastCalls)
	require.Equal(t, 1, fallbackCalls)
}

func Test_Run_Script_Installs_Print_Helpers(t *testing.T) {
	ctx := rootplush.NewContext()
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	err = vmplush.RunScript(`print("hello"); println(" world")`, ctx)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(output))
}
