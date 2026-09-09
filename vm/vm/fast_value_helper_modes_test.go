package vm

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Fast_Value_Helper_Registration_Mode_Replaces_Previous_Mode(t *testing.T) {
	ctx := plush.NewContext()

	SetFastValueHelper(ctx, "helper", func(hctx.Context, FastArgs) (interface{}, error) {
		return "read-write", nil
	})
	registration, ok := fastValueHelperRegistrationForContext(ctx, "helper")
	require.True(t, ok)
	require.Equal(t, fastValueHelperReadWrite, registration.mode)
	require.NotNil(t, registration.readWrite)

	SetFastNoContextValueHelper(ctx, "helper", func(FastArgs) (interface{}, error) {
		return "no-context", nil
	})
	registration, ok = fastValueHelperRegistrationForContext(ctx, "helper")
	require.True(t, ok)
	require.Equal(t, fastValueHelperNoContext, registration.mode)
	require.NotNil(t, registration.noContext)
	_, ok = fastValueHelperForContext(ctx, "helper")
	require.False(t, ok)

	SetFastReadOnlyValueHelper(ctx, "helper", func(FastReadOnlyContext, FastArgs) (interface{}, error) {
		return "read-only", nil
	})
	registration, ok = fastValueHelperRegistrationForContext(ctx, "helper")
	require.True(t, ok)
	require.Equal(t, fastValueHelperReadOnly, registration.mode)
	require.NotNil(t, registration.readOnly)

	ClearFastReadOnlyValueHelper(ctx, "helper")
	_, ok = fastValueHelperRegistrationForContext(ctx, "helper")
	require.False(t, ok)
}

func Test_VM_Fast_Value_Helper_Mode_Setters_Ignore_Invalid_Registrations(t *testing.T) {
	require.NotPanics(t, func() {
		SetFastNoContextValueHelper(nil, "helper", func(FastArgs) (interface{}, error) { return nil, nil })
		SetFastNoContextValueHelper(plush.NewContext(), "", func(FastArgs) (interface{}, error) { return nil, nil })
		SetFastReadOnlyValueHelper(nil, "helper", func(FastReadOnlyContext, FastArgs) (interface{}, error) { return nil, nil })
		SetFastReadOnlyValueHelper(plush.NewContext(), "", func(FastReadOnlyContext, FastArgs) (interface{}, error) { return nil, nil })
		ClearFastNoContextValueHelper(nil, "helper")
		ClearFastReadOnlyValueHelper(nil, "helper")
	})
}

func Test_VM_Registered_No_Context_Value_Helper_Skips_Scoped_Context(t *testing.T) {
	ctx := newPartialOverlayContext(plush.NewContext())
	machine := newRuntimeHelperTestVM(ctx)
	frame := machine.currentFrame()
	frame.cl.Fn.LocalNames = map[int]string{0: "local"}
	machine.stack[0] = &object.String{Value: "unchanged"}
	machine.stack[1] = &object.String{Value: "value"}
	machine.sp = 2

	called := false
	SetFastNoContextValueHelper(ctx, "upper", func(args FastArgs) (interface{}, error) {
		called = true
		value, ok := args.String(0)
		require.True(t, ok)
		return strings.ToUpper(value), nil
	})

	handled, err := machine.tryCallRegisteredFastValueHelper("upper", 1, false)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, called)
	require.Equal(t, "unchanged", object.ToGo(machine.stack[0]))
	require.Equal(t, "VALUE", object.ToGo(machine.stack[1]))
}

func Test_VM_Registered_Read_Only_Value_Helper_Reads_Frame_And_Root_Bindings(t *testing.T) {
	root := plush.NewContextWith(map[string]interface{}{"root": "root-value"})
	ctx := newPartialOverlayContext(root)
	machine := newRuntimeHelperTestVM(ctx)
	frame := machine.currentFrame()
	frame.cl.Fn.LocalNames = map[int]string{0: "local"}
	machine.stack[0] = &object.String{Value: "local-value"}
	machine.stack[1] = &object.String{Value: "argument"}
	machine.sp = 2

	SetFastReadOnlyValueHelper(ctx, "inspect", func(callCtx FastReadOnlyContext, args FastArgs) (interface{}, error) {
		require.Equal(t, "root-value", callCtx.Value("root"))
		require.Equal(t, "local-value", callCtx.Value("local"))
		require.True(t, callCtx.Has("root"))
		require.True(t, callCtx.Has("local"))
		_, writable := interface{}(callCtx).(hctx.Context)
		require.False(t, writable)
		value, ok := args.String(0)
		require.True(t, ok)
		return value + "-result", nil
	})

	handled, err := machine.tryCallRegisteredFastValueHelper("inspect", 1, false)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "local-value", object.ToGo(machine.stack[0]))
	require.Equal(t, "argument-result", object.ToGo(machine.stack[1]))
}

func Test_VM_No_Context_And_Read_Only_Value_Helpers_Write_Bytecode_Output(t *testing.T) {
	for _, test := range []struct {
		name     string
		register func(hctx.Context)
	}{
		{
			name: "no context",
			register: func(ctx hctx.Context) {
				SetFastNoContextValueHelper(ctx, "decorate", func(args FastArgs) (interface{}, error) {
					value, _ := args.String(0)
					return "<" + value + ">", nil
				})
			},
		},
		{
			name: "read only",
			register: func(ctx hctx.Context) {
				SetFastReadOnlyValueHelper(ctx, "decorate", func(callCtx FastReadOnlyContext, args FastArgs) (interface{}, error) {
					prefix, _ := callCtx.Value("prefix").(string)
					value, _ := args.String(0)
					return "<" + prefix + value + ">", nil
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := newPartialOverlayContext(plush.NewContextWith(map[string]interface{}{"prefix": ""}))
			test.register(ctx)
			machine := newRuntimeHelperTestVM(ctx)
			machine.stack[0] = &object.String{Value: "value"}
			machine.sp = 1

			handled, err := machine.tryWriteRegisteredFastValueHelper("decorate", 1, false)
			require.NoError(t, err)
			require.True(t, handled)
			require.Equal(t, "&lt;value&gt;", machine.currentFrame().output.String())
		})
	}
}

func Test_VM_Fast_Read_Only_Context_Has_No_Binding_Mutation_Methods(t *testing.T) {
	readOnlyType := reflect.TypeOf((*FastReadOnlyContext)(nil)).Elem()
	for _, method := range []string{"Set", "Update", "New"} {
		_, ok := readOnlyType.MethodByName(method)
		require.Falsef(t, ok, "FastReadOnlyContext unexpectedly exposes %s", method)
	}
}

func Test_VM_Fast_Read_Only_Context_Is_Shallow(t *testing.T) {
	mutable := map[string]string{"status": "before"}
	ctx := plush.NewContextWith(map[string]interface{}{"mutable": mutable})

	SetFastReadOnlyValueHelper(ctx, "inspect", func(callCtx FastReadOnlyContext, _ FastArgs) (interface{}, error) {
		value := callCtx.Value("mutable").(map[string]string)
		value["status"] = "after"
		return "ok", nil
	})

	machine := newRuntimeHelperTestVM(ctx)
	handled, err := machine.tryCallRegisteredFastValueHelper("inspect", 0, false)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "after", mutable["status"], "read-only bindings do not deep-freeze reachable values")
}

func Test_VM_Fast_Value_Helper_Context_Modes_Reduce_Allocations(t *testing.T) {
	readWriteAllocs := fastValueHelperModeAllocs(t, fastValueHelperReadWrite)
	readOnlyAllocs := fastValueHelperModeAllocs(t, fastValueHelperReadOnly)
	noContextAllocs := fastValueHelperModeAllocs(t, fastValueHelperNoContext)

	require.Less(t, readOnlyAllocs, readWriteAllocs)
	require.Less(t, noContextAllocs, readOnlyAllocs)
}

func fastValueHelperModeAllocs(t *testing.T, mode fastValueHelperMode) float64 {
	t.Helper()

	ctx := newPartialOverlayContext(plush.NewContextWith(map[string]interface{}{"root": "value"}))
	machine := newRuntimeHelperTestVM(ctx)
	frame := machine.currentFrame()
	frame.cl.Fn.LocalNames = make(map[int]string, 12)
	for index := 0; index < 12; index++ {
		name := "local" + string(rune('a'+index))
		frame.cl.Fn.LocalNames[index] = name
		machine.stack[index] = &object.String{Value: name + "-value"}
		ctx.InternID(name)
	}
	argIndex := 12
	machine.stack[argIndex] = &object.String{Value: "argument"}
	machine.sp = argIndex + 1

	switch mode {
	case fastValueHelperNoContext:
		SetFastNoContextValueHelper(ctx, "identity", func(args FastArgs) (interface{}, error) {
			value, _ := args.Object(0)
			return value, nil
		})
	case fastValueHelperReadOnly:
		SetFastReadOnlyValueHelper(ctx, "identity", func(callCtx FastReadOnlyContext, args FastArgs) (interface{}, error) {
			_ = callCtx.Value("locala")
			value, _ := args.Object(0)
			return value, nil
		})
	default:
		SetFastValueHelper(ctx, "identity", func(_ hctx.Context, args FastArgs) (interface{}, error) {
			value, _ := args.Object(0)
			return value, nil
		})
	}

	return testing.AllocsPerRun(100, func() {
		handled, err := machine.tryCallRegisteredFastValueHelper("identity", 1, false)
		if err != nil || !handled {
			panic("fast value helper call failed")
		}
		registeredFastValueHelperBenchmarkSink = machine.stack[argIndex]
	})
}

func Test_VM_Fast_Value_Helper_Modes_Preserve_Unsupported_Fallback(t *testing.T) {
	for _, test := range []struct {
		name     string
		register func(hctx.Context)
	}{
		{
			name: "no context",
			register: func(ctx hctx.Context) {
				SetFastNoContextValueHelper(ctx, "decorate", func(FastArgs) (interface{}, error) {
					return nil, ErrFastUnsupported
				})
			},
		},
		{
			name: "read only",
			register: func(ctx hctx.Context) {
				SetFastReadOnlyValueHelper(ctx, "decorate", func(FastReadOnlyContext, FastArgs) (interface{}, error) {
					return nil, ErrFastUnsupported
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := plush.NewContextWith(map[string]interface{}{
				"decorate": func(value string) string { return "fallback:" + value },
			})
			test.register(ctx)

			out, err := Render(`<% let value = decorate("x") %><%= value %>`, ctx)
			require.NoError(t, err)
			require.Equal(t, "fallback:x", out)
		})
	}
}

func Benchmark_VM_Registered_Fast_Value_Helper_Context_Modes(b *testing.B) {
	for _, mode := range []string{"read-write", "read-only", "no-context"} {
		b.Run(mode, func(b *testing.B) {
			root := plush.NewContextWith(map[string]interface{}{"root": "value"})
			ctx := newPartialOverlayContext(root)
			machine := newRuntimeHelperTestVM(ctx)
			frame := machine.currentFrame()
			frame.cl.Fn.LocalNames = make(map[int]string, 12)
			for index := 0; index < 12; index++ {
				name := "local" + string(rune('a'+index))
				frame.cl.Fn.LocalNames[index] = name
				machine.stack[index] = &object.String{Value: name + "-value"}
				ctx.InternID(name)
			}
			argIndex := 12
			machine.stack[argIndex] = &object.String{Value: "argument"}
			machine.sp = argIndex + 1

			switch mode {
			case "read-write":
				SetFastValueHelper(ctx, "identity", func(_ hctx.Context, args FastArgs) (interface{}, error) {
					value, _ := args.Object(0)
					return value, nil
				})
			case "read-only":
				SetFastReadOnlyValueHelper(ctx, "identity", func(callCtx FastReadOnlyContext, args FastArgs) (interface{}, error) {
					_ = callCtx.Value("locala")
					value, _ := args.Object(0)
					return value, nil
				})
			case "no-context":
				SetFastNoContextValueHelper(ctx, "identity", func(args FastArgs) (interface{}, error) {
					value, _ := args.Object(0)
					return value, nil
				})
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				handled, err := machine.tryCallRegisteredFastValueHelper("identity", 1, false)
				if err != nil || !handled {
					b.Fatalf("handled=%v err=%v", handled, err)
				}
			}
			registeredFastValueHelperBenchmarkSink = machine.stack[argIndex]
		})
	}
}
