package vm

import (
	"errors"
	"html/template"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

type vmCoveragePointerMethod struct {
	Name string
}

func (v *vmCoveragePointerMethod) Echo() string {
	return v.Name
}

type vmCoveragePointerField struct {
	Value *string
}

func vmCoverageBadProgram() *ast.Program {
	return &ast.Program{Statements: []ast.Statement{
		&ast.ExpressionStatement{Expression: &ast.PrefixExpression{
			Operator: "??",
			Right:    &ast.Boolean{Value: true},
		}},
	}}
}

func Test_VM_Runtime_And_Property_Remaining_Edge_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	machine.halted = true
	machine.lastPopped = &object.String{Value: "<done>"}
	require.Equal(t, "&lt;done&gt;", machine.Rendered())

	values := []string{"a"}
	handled, err := machine.executeNativeSliceAppend(&object.Native{Value: &values}, &object.String{Value: "b"})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, []string{"a", "b"}, object.ToGo(machine.pop()))

	ordered, err := compareOrdered(code.OpGreaterEqual, &object.String{Value: "b"}, &object.String{Value: "b"})
	require.NoError(t, err)
	require.True(t, ordered)

	ctx := plush.NewContextWith(map[string]interface{}{"name": struct{ Value string }{Value: "slow"}})
	machine = newRuntimeHelperTestVM(ctx)
	var frame Frame
	require.NoError(t, machine.writeName(&frame, 0, false))
	require.True(t, frame.hasOutput)

	raw := &vmCoveragePointerMethod{Name: "method"}
	value, err := machine.propertyValue(raw, "Echo", object.PropertyAccess{Method: true, Receiver: "raw", Full: "raw.Echo()"}, nil)
	require.NoError(t, err)
	method, ok := value.(func() string)
	require.True(t, ok)
	require.Equal(t, "method", method())

	rv, rawValue, ok, err := fastPropertyReflectValue(raw)
	require.NoError(t, err)
	require.True(t, ok)
	require.Same(t, raw, rawValue)
	require.Equal(t, reflect.Struct, rv.Kind())

	rv, rawValue, ok, err = fastPropertyReflectValue(nil)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, rv.IsValid())
	require.Nil(t, rawValue)

	pointerLookup := cachedPropertyLookup(reflect.TypeOf(vmCoveragePointerMethod{}), "Echo")
	pointerEntry := &propertyInlineCacheEntry{lookup: pointerLookup}
	addressable := reflect.ValueOf(raw).Elem()
	value, err = fastPropertyValueFromReflect(addressable, raw, "Echo", object.PropertyAccess{Method: true}, pointerEntry)
	require.NoError(t, err)
	method, ok = value.(func() string)
	require.True(t, ok)
	require.Equal(t, "method", method())

	reader := buildFastPropertyReader(pointerLookup)
	value, err = reader(addressable, object.PropertyAccess{Method: true}, "Echo")
	require.NoError(t, err)
	method, ok = value.(func() string)
	require.True(t, ok)
	require.Equal(t, "method", method())

	fieldValue := "field"
	field := reflect.ValueOf(vmCoveragePointerField{Value: &fieldValue}).FieldByName("Value")
	var out strings.Builder
	written, err := writeFastField(&out, plush.NewContext(), field, object.PropertyAccess{}, "Value", field.Type())
	require.NoError(t, err)
	require.True(t, written)
	require.Equal(t, "field", out.String())
}

func Test_VM_Execute_For_Remaining_Output_And_Stop_Branches(t *testing.T) {
	outputOnlyBlock := executeForClosure(code.Make(code.OpWriteLocal, 1), 2)
	result, err := newExecuteForTestVM().executeFor(&object.Native{Value: []string{"out"}}, outputOnlyBlock, "i", "item")
	require.NoError(t, err)
	outputArray := result.(*object.Array)
	require.Len(t, outputArray.Elements, 1)
	require.Equal(t, template.HTML("out"), object.ToGo(outputArray.Elements[0]))

	breakBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpWriteLocal, 1),
		code.Make(code.OpBreak),
	), 2)

	result, err = newExecuteForTestVM().executeFor(
		&object.Native{Value: []object.Object{&object.String{Value: "obj"}, &object.String{Value: "skip"}}},
		breakBlock,
		"i",
		"item",
	)
	require.NoError(t, err)
	objectArray := result.(*object.Array)
	require.Len(t, objectArray.Elements, 1)
	require.Equal(t, template.HTML("obj"), object.ToGo(objectArray.Elements[0]))

	result, err = newExecuteForTestVM().executeFor(&object.Native{Value: []int{7, 8}}, breakBlock, "i", "item")
	require.NoError(t, err)
	reflectArray := result.(*object.Array)
	require.Len(t, reflectArray.Elements, 1)
	require.Equal(t, template.HTML("7"), object.ToGo(reflectArray.Elements[0]))

	result, err = newExecuteForTestVM().executeFor(
		&object.Native{Value: &vmForIterator{values: []interface{}{"first", "second"}}},
		breakBlock,
		"i",
		"item",
	)
	require.NoError(t, err)
	iteratorArray := result.(*object.Array)
	require.Len(t, iteratorArray.Elements, 1)
	require.Equal(t, template.HTML("first"), object.ToGo(iteratorArray.Elements[0]))
}

func Test_VM_Partial_Remaining_Error_Branches(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	fallbackCtx := newVMFallbackContext(map[string]interface{}{
		vmPartialFeederName:          func(string) (string, error) { return "never", nil },
		meta.TemplateBaseFileNameKey: "index",
		meta.TemplateExtensionKey:    "plush",
		meta.TemplateFileKey:         12,
	})
	handled, err := renderFastDataPartialInto(&strings.Builder{}, &compiler.FastPartialPlan{
		Name: "edge_fallback_meta_error_no_data.plush",
		Line: 3,
	}, fallbackCtx, fastRenderBindings{}, nil)
	require.True(t, handled)
	require.ErrorContains(t, err, "expected fileKey to be a string")

	partial, dataPlan := vmPartialDataPlan("edge_fallback_meta_error.plush", "name")
	handled, err = renderFastDataPartialInto(&strings.Builder{}, partial, fallbackCtx, fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "expected fileKey to be a string")

	badLinkFile := "edge_partial_compile_error.plush"
	plush.CacheVMBytecodeForCleanFilename(badLinkFile, vmCoverageBadProgram(), "not-bytecode")
	_, err = partialBytecodeLinkForInput("ignored", badLinkFile, plush.NewContext())
	require.ErrorContains(t, err, "unknown operator ??")

	_, err = renderLinkedPartial("ignored", partialCompileErrorContext("edge_render_linked_compile_error.plush"))
	require.ErrorContains(t, err, "unknown operator ??")

	var out strings.Builder
	handled, err = renderLinkedPartialInline(&out, "ignored", partialCompileErrorContext("edge_render_linked_inline_compile_error.plush"))
	require.False(t, handled)
	require.ErrorContains(t, err, "unknown operator ??")

	errorDataPartial := &compiler.FastPartialPlan{
		Name: "edge_slow_data_error.plush",
		Data: []compiler.FastPartialDataPair{{
			Key: "name",
			Value: compiler.FastValuePlan{
				Kind: compiler.FastValueCall,
				Call: &compiler.FastCallPlan{Name: "echo", NameIndex: 0, Line: 44},
			},
			Line: 44,
		}},
		Line: 44,
	}
	errorCtx := plush.NewContextWith(map[string]interface{}{"echo": func() string { return "never" }}).WithBudget(plush.NewBudget(0))
	errorBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"echo"}}, errorCtx)
	overlay := borrowPartialOverlayContext(errorCtx)
	err = applyFastPartialDataSlow(overlay, errorDataPartial, errorCtx, errorBindings)
	releasePartialOverlayContext(overlay)
	require.ErrorContains(t, err, "line 44")

	inlineErrorName := "edge_no_data_inline_compile_error.plush"
	plush.CacheVMBytecodeForCleanFilename(inlineErrorName, vmCoverageBadProgram(), "not-bytecode")
	noDataCtx := plush.NewContextWith(map[string]interface{}{
		vmPartialFeederName: func(string) (string, error) { return "ignored", nil },
	})
	handled, err = renderFastNoDataPartialInto(&strings.Builder{}, inlineErrorName, noDataCtx, 55)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 55")
	require.ErrorContains(t, err, "unknown operator ??")

	parseErrorCtx := plush.NewContextWith(map[string]interface{}{
		vmPartialFeederName: func(string) (string, error) { return `<%=`, nil },
	})
	handled, err = renderFastNoDataPartialInto(&strings.Builder{}, "edge_no_data_inline_parse_error.plush", parseErrorCtx, 56)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 56")

	slowInlineErrorName := "edge_no_data_slow_inline_error.plush"
	plush.CacheVMBytecodeForCleanFilename(slowInlineErrorName, nil, &compiler.Bytecode{
		FastRenderPlan: &compiler.FastRenderPlan{
			Bindings: []string{meta.TemplateFileKey, "missing"},
			Segments: []compiler.FastRenderSegment{{
				Kind:      compiler.FastRenderSegmentName,
				NameIndex: 1,
				Value:     "missing",
				Line:      58,
			}},
		},
	})
	slowInlineCtx := plush.NewContextWith(map[string]interface{}{
		vmPartialFeederName: func(string) (string, error) { return "ignored", nil },
	})
	handled, err = renderFastNoDataPartialInto(&strings.Builder{}, slowInlineErrorName, slowInlineCtx, 58)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 58")
	require.ErrorContains(t, err, `"missing": unknown identifier`)

	attachErrorPartial := &compiler.FastPartialPlan{
		Name: "edge_direct_attach_error.plush",
		Data: []compiler.FastPartialDataPair{{
			Key: "name",
			Value: compiler.FastValuePlan{
				Kind: compiler.FastValueCall,
				Call: &compiler.FastCallPlan{Name: "echo", NameIndex: 0, Line: 57},
			},
			Line: 57,
		}},
		Line: 57,
	}
	attachDataPlan := buildFastPartialDataBindingPlan(attachErrorPartial)
	attachBytecode := requireCompiledBytecode(t, `<%= name %>`)
	plush.CacheVMBytecodeForCleanFilename(attachErrorPartial.Name, nil, attachBytecode)
	attachCtx := plush.NewContextWith(map[string]interface{}{
		"echo": func() string { return "never" },
		vmPartialFeederName: func(string) (string, error) {
			return `<%= name %>`, nil
		},
	}).WithBudget(plush.NewBudget(0))
	attachBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"echo"}}, attachCtx)
	handled, err = renderFastDataPartialDirectInto(&strings.Builder{}, attachErrorPartial, attachCtx, attachBindings, attachDataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 57")
}

func partialCompileErrorContext(filename string) hctx.Context {
	plush.CacheVMBytecodeForCleanFilename(filename, vmCoverageBadProgram(), "not-bytecode")
	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)
	return ctx
}

func Test_VM_Native_Call_Remaining_Edge_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	require.ErrorContains(t, machine.writeNativeValueCall("bad", 7, 0, nil, false), "invalid function")

	require.NoError(t, machine.push(&object.Native{Value: func() (string, error) {
		return "", errors.New("boom")
	}}))
	require.ErrorContains(t, machine.writeNativeValueCall("boom", machine.stack[0].(*object.Native).Value, 0, nil, true), "boom")

	var frame Frame
	machine.writeNativeReturnValue(&frame, reflect.ValueOf("raw-object-fallback"), &callPlan{returnKind: callReturnObject})
	require.Equal(t, "raw-object-fallback", frame.output.String())

	_, ok := fastOptionalArg(optionalArgMap, reflect.TypeOf(namedVMCoverageMap{}), plush.NewContext())
	require.True(t, ok)

	_, ok = machine.optionalArg(optionalArgMap, reflect.TypeOf(namedVMCoverageMap{}), nil)
	require.True(t, ok)

	arg, err := fastReflectArgForCall("nilarg", 0, nil, stringType)
	require.NoError(t, err)
	require.Equal(t, "", arg.String())

	arg, err = machine.reflectArg("nilnative", 0, &object.Native{Value: nil}, stringType)
	require.NoError(t, err)
	require.Equal(t, "", arg.String())

	rawArgs := fastCallArgs{}
	rawArgs.Append("extra")
	_, err = fastReflectArgsInto("tooMany", &callPlan{numIn: 0}, &rawArgs, plush.NewContext(), nil)
	require.ErrorContains(t, err, "too many arguments")

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.String{Value: "extra"}))
	_, err = machine.reflectArgs("tooMany", &callPlan{numIn: 0}, 1, nil, nil)
	require.ErrorContains(t, err, "too many arguments")

	badValue := 7
	require.ErrorContains(t, machine.writeNativeValueCall("badPointer", &badValue, 0, nil, false), "invalid function")

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.Native{Value: &badValue}))
	require.ErrorContains(t, machine.executeWriteCall("badPointer", 0, nil), "invalid function")

	errorFunc := func() (string, error) {
		return "", errors.New("slow boom")
	}
	require.ErrorContains(t, machine.writeNativeValueCall("slowBoom", &errorFunc, 0, nil, false), "slow boom")

	entry := &fastOutputCacheEntry{kind: fastOutputKind(255)}
	require.False(t, entry.matches("anything"))
}

type namedVMCoverageMap map[string]interface{}

func Test_VM_Fast_Render_And_Value_Remaining_Edge_Branches(t *testing.T) {
	handled, err := renderFastSimplePlanInlineSafe(nil, plush.NewContext(), fastRenderBindings{}, nil)
	require.NoError(t, err)
	require.False(t, handled)

	ctx := plush.NewContextWith(map[string]interface{}{
		"label": func(value string) string { return value },
		"user":  struct{}{},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label", "user"}}, ctx)
	_, ok, err := evalFastCallValuePlan(&compiler.FastCallPlan{
		Name:      "label",
		NameIndex: 0,
		Args: []compiler.FastValuePlan{{
			Kind:      compiler.FastValuePath,
			NameIndex: 1,
			Value:     "user",
			Path: []compiler.FastPathStep{{
				Kind:     compiler.FastPathStepProperty,
				Value:    "Missing",
				Receiver: "user",
				Full:     "user.Missing",
				Line:     66,
			}},
			Line: 66,
		}},
		Line: 66,
	}, nil, ctx, bindings, nil, nil, nil)
	require.True(t, ok)
	require.ErrorContains(t, err, "line 66")

	var nilProduct *vmStructLoopProduct
	value, ok, err := evalFastFieldChainValue(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{{
			Kind:  compiler.FastPathStepProperty,
			Value: "Name",
			Line:  67,
		}},
	}, nilProduct, plush.NewContext())
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	emptyChainPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{{
			Kind:  compiler.FastPathStepProperty,
			Value: "Name",
			Line:  69,
		}},
	}
	emptyChainKey := fastFieldChainPlanKey{plan: emptyChainPlan, typ: reflect.TypeOf(vmStructLoopProduct{})}
	fastFieldChainPlanCache.Store(emptyChainKey, &fastFieldChainPlan{})
	defer fastFieldChainPlanCache.Delete(emptyChainKey)
	rawProduct := vmStructLoopProduct{Name: "cached"}
	value, ok, err = evalFastFieldChainValue(emptyChainPlan, rawProduct, plush.NewContext())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, rawProduct, value)

	indexed, found, handled := fastAccessDirectMapIndex(reflect.ValueOf(map[string]int{"name": 1}), &fastAccessChainStep{
		mapDirect: fastMapDirectStringString,
		mapString: "name",
	})
	require.False(t, indexed.IsValid())
	require.False(t, found)
	require.False(t, handled)

	var nilPointer *int
	var iface interface{} = nilPointer
	_, numericOK := numericValueFromReflectValue(reflect.ValueOf(&iface).Elem())
	require.False(t, numericOK)

	boolValue, boolOK := fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&iface).Elem()}.boolValue()
	require.False(t, boolValue)
	require.False(t, boolOK)
	_, stringOK := fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&iface).Elem()}.stringValue()
	require.False(t, stringOK)
}

func Test_VM_Fast_Call_Segment_General_Argument_Error_Branch(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"format": func() string { return "never" },
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"format"}}, ctx)
	err := writeFastCallSegment(&strings.Builder{}, ctx, bindings, &compiler.FastCallPlan{
		Name:      "format",
		NameIndex: 0,
		Args: []compiler.FastValuePlan{
			{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 68},
			{Kind: compiler.FastValueString, Value: "suffix", Line: 68},
		},
		Line: 68,
	})
	require.ErrorContains(t, err, `line 68`)
	require.ErrorContains(t, err, `"missing": unknown identifier`)
}

func Test_VM_Fast_Mixed_Render_Remaining_Error_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"user": vmFastValueHiddenChild{child: vmFastPropertyChild{Name: "hidden"}},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"user"}}, ctx)

	handled, err := renderFastMixedPlan(&strings.Builder{}, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind:      fastMixedOpAccessChain,
		nameIndex: 0,
		line:      81,
		valuePlan: compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: 0,
			Value:     "user",
			Path: []compiler.FastPathStep{{
				Kind:     compiler.FastPathStepProperty,
				Value:    "child",
				Receiver: "user",
				Full:     "user.child",
				Line:     81,
			}},
			Line: 81,
		},
	}}})
	require.True(t, handled)
	require.ErrorContains(t, err, "line 81")

	handled, err = renderFastMixedPlan(&strings.Builder{}, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpAccessChain,
		line: 82,
		valuePlan: compiler.FastValuePlan{
			Kind:     compiler.FastValueInfix,
			Operator: "??",
			Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1},
			Right:    &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 2},
			Line:     82,
		},
	}}})
	require.True(t, handled)
	require.ErrorContains(t, err, "line 82")

	handled, err = renderFastMixedPlan(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpConditional,
		conditional: &compiler.FastConditionalPlan{Branches: []compiler.FastConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true, Line: 83},
			Segments:  []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "never"}},
			Line:      83,
		}}},
	}}})
	require.True(t, handled)
	require.ErrorContains(t, err, "line 83")
}

func Test_VM_Fast_Call_And_Loop_Call_Remaining_Error_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"label": func(int) string { return "never" },
		"user":  struct{}{},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label", "user"}}, ctx)
	call := &compiler.FastCallPlan{
		Name:      "label",
		NameIndex: 0,
		Args: []compiler.FastValuePlan{{
			Kind:      compiler.FastValuePath,
			NameIndex: 1,
			Value:     "user",
			Path: []compiler.FastPathStep{{
				Kind:     compiler.FastPathStepProperty,
				Value:    "Missing",
				Receiver: "user",
				Full:     "user.Missing",
				Line:     84,
			}},
			Line: 84,
		}},
		Line: 84,
	}
	err := writeFastCallSegment(&strings.Builder{}, ctx, bindings, call)
	require.ErrorContains(t, err, `line 84`)

	badCtx := plush.NewContextWith(map[string]interface{}{"label": 12})
	badBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, badCtx)
	err = writeFastLoopCallPart(&strings.Builder{}, badCtx, badBindings, &compiler.FastCallPlan{
		Name:      "label",
		NameIndex: 0,
		Line:      85,
	}, 0, "value")
	require.ErrorContains(t, err, "line 85")
	require.ErrorContains(t, err, "invalid function")
}

func Test_VM_Fast_Struct_Loop_Call_Remaining_State_Branches(t *testing.T) {
	item := reflect.ValueOf(vmStructLoopProduct{Name: "bot"})

	floatArg := buildFastStructLoopCallArgPlan(&compiler.FastValuePlan{Kind: compiler.FastValueFloat, FloatValue: 1.25, Line: 70}, reflect.TypeOf(vmStructLoopProduct{}))
	require.Equal(t, fastStructLoopCallArgFloat, floatArg.kind)
	require.Equal(t, 1.25, floatArg.floatVal)

	helperCtx := plush.NewContextWith(map[string]interface{}{
		"label": func(string) string { return "slow" },
		vmFastHelpersKey: &fastHelperRegistry{helpers: map[string]FastHelperFunc{
			"label": func(FastWriter, FastArgs) error {
				return nil
			},
		}},
	})
	_, helperOK := fastHelperForContext(helperCtx, "label")
	require.True(t, helperOK)
	helperBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, helperCtx)
	_, helperBindingOK := helperBindings.value(0)
	require.True(t, helperBindingOK)
	helperPlan := structLoopCallPlan(t, compiler.FastValuePlan{
		Kind:      compiler.FastValueName,
		NameIndex: 99,
		Value:     "missing",
		Line:      71,
	})
	require.Equal(t, "label", helperPlan.call.Name)
	require.Len(t, helperPlan.args, 1)
	err := writeFastStructLoopCallPart(&strings.Builder{}, helperCtx, helperBindings, &fastStructLoopRenderState{}, helperPlan, 0, item)
	require.ErrorContains(t, err, `line 71`)

	validHelperPlan := structLoopCallPlan(t, structLoopNameArg())
	var helperOut strings.Builder
	require.NoError(t, writeFastStructLoopCallPart(&helperOut, helperCtx, helperBindings, &fastStructLoopRenderState{}, validHelperPlan, 0, item))

	entryArgPlan := structLoopCallPlan(t, compiler.FastValuePlan{
		Kind:      compiler.FastValueName,
		NameIndex: 99,
		Value:     "missing",
		Line:      75,
	})
	entryArgState := &fastStructLoopRenderState{
		singleCall: entryArgPlan,
		singleResolvedCall: &fastStructLoopResolvedCall{
			raw: func(int) string { return "never" },
			entry: &fastBuilderCallCacheEntry{
				invoker: func(*strings.Builder, hctx.Context, string, interface{}, *fastCallArgs) error {
					return nil
				},
			},
		},
	}
	err = writeFastStructLoopCallPart(&strings.Builder{}, plush.NewContext(), fastRenderBindings{}, entryArgState, entryArgPlan, 0, item)
	require.ErrorContains(t, err, `line 75`)

	entryErrorPlan := &fastStructLoopCallPlan{
		call: &compiler.FastCallPlan{Name: "label", NameIndex: 0, Line: 76},
	}
	entryErrorState := &fastStructLoopRenderState{
		singleCall: entryErrorPlan,
		singleResolvedCall: &fastStructLoopResolvedCall{
			raw: func() string { return "never" },
			entry: &fastBuilderCallCacheEntry{
				invoker: func(*strings.Builder, hctx.Context, string, interface{}, *fastCallArgs) error {
					return errors.New("entry direct boom")
				},
			},
		},
	}
	err = writeFastStructLoopCallPart(&strings.Builder{}, plush.NewContext(), fastRenderBindings{}, entryErrorState, entryErrorPlan, 0, item)
	require.ErrorContains(t, err, `line 76`)
	require.ErrorContains(t, err, "entry direct boom")

	finalErrorPlan := &fastStructLoopCallPlan{
		call: &compiler.FastCallPlan{Name: "label", NameIndex: 0, Line: 77},
	}
	finalErrorState := &fastStructLoopRenderState{
		singleCall: finalErrorPlan,
		singleResolvedCall: &fastStructLoopResolvedCall{
			raw: func() string { return "never" },
		},
	}
	err = writeFastStructLoopCallPart(&strings.Builder{}, plush.NewContext(), fastRenderBindings{}, finalErrorState, finalErrorPlan, 0, item)
	require.ErrorContains(t, err, `line 77`)
	require.ErrorContains(t, err, "invalid function")

	hiddenPlan, ok := fastAccessChainPlanFor(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{{
			Kind:     compiler.FastPathStepProperty,
			Value:    "child",
			Receiver: "user",
			Full:     "user.child",
			Line:     78,
		}},
	}, reflect.TypeOf(vmFastValueHiddenChild{}))
	require.True(t, ok)
	err = evalFastStructLoopCallPlanArgs(&fastStructLoopCallPlan{
		args: []fastStructLoopCallArgPlan{{
			kind:       fastStructLoopCallArgAccessChain,
			accessPlan: hiddenPlan,
			line:       78,
		}},
	}, plush.NewContext(), fastRenderBindings{}, 0, reflect.ValueOf(vmFastValueHiddenChild{child: vmFastPropertyChild{Name: "hidden"}}), &fastCallArgs{})
	require.ErrorContains(t, err, `line 78`)

	hiddenHelperCtx := plush.NewContextWith(map[string]interface{}{
		"label": func(interface{}) string { return "never" },
		vmFastHelpersKey: &fastHelperRegistry{helpers: map[string]FastHelperFunc{
			"label": func(FastWriter, FastArgs) error {
				return nil
			},
		}},
	})
	hiddenHelperBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, hiddenHelperCtx)
	hiddenHelperPlan := &fastStructLoopCallPlan{
		call: &compiler.FastCallPlan{Name: "label", NameIndex: 0, Line: 79},
		args: []fastStructLoopCallArgPlan{{
			kind:       fastStructLoopCallArgAccessChain,
			accessPlan: hiddenPlan,
			line:       79,
		}},
	}
	err = writeFastStructLoopCallPart(&strings.Builder{}, hiddenHelperCtx, hiddenHelperBindings, &fastStructLoopRenderState{}, hiddenHelperPlan, 0, reflect.ValueOf(vmFastValueHiddenChild{child: vmFastPropertyChild{Name: "hidden"}}))
	require.ErrorContains(t, err, `line 78`)

	unsupportedPlan := &fastStructLoopCallPlan{
		call: &compiler.FastCallPlan{Name: "label", NameIndex: 0, Line: 72},
	}
	unsupportedState := &fastStructLoopRenderState{
		singleCall: unsupportedPlan,
		singleResolvedCall: &fastStructLoopResolvedCall{
			raw: func() string { return "fallback" },
			entry: &fastBuilderCallCacheEntry{
				rt:   reflect.TypeOf(func() string { return "" }),
				plan: cachedCallPlan(reflect.TypeOf(func() string { return "" })),
				invoker: func(*strings.Builder, hctx.Context, string, interface{}, *fastCallArgs) error {
					return errFastWriteUnsupported
				},
			},
		},
	}
	var out strings.Builder
	err = writeFastStructLoopCallPart(&out, plush.NewContext(), fastRenderBindings{}, unsupportedState, unsupportedPlan, 0, item)
	require.NoError(t, err)
	require.Equal(t, "fallback", out.String())

	finalArgPlan := structLoopCallPlan(t, compiler.FastValuePlan{
		Kind:      compiler.FastValueName,
		NameIndex: 99,
		Value:     "missing",
		Line:      73,
	})
	finalArgState := &fastStructLoopRenderState{
		singleCall: finalArgPlan,
		singleResolvedCall: &fastStructLoopResolvedCall{
			raw: func(string) string { return "never" },
		},
	}
	err = writeFastStructLoopCallPart(&strings.Builder{}, plush.NewContext(), fastRenderBindings{}, finalArgState, finalArgPlan, 0, item)
	require.ErrorContains(t, err, `line 73`)

	require.ErrorIs(t, writeFastStructLoopReflectCall(&strings.Builder{}, plush.NewContext(), fastRenderBindings{}, nil, nil, 0, item), errFastWriteUnsupported)

	reflectPlan := structLoopCallPlan(t, structLoopNameArg())
	reflectResolved := &fastStructLoopResolvedCall{
		fn: reflect.ValueOf(func(string) string { return "never" }),
		entry: &fastBuilderCallCacheEntry{plan: &callPlan{
			numIn:    1,
			argTypes: []reflect.Type{stringType},
		}},
	}
	err = writeFastStructLoopReflectCall(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), fastRenderBindings{}, reflectPlan, reflectResolved, 0, item)
	require.ErrorContains(t, err, "line 1")

	staticResolved := &fastStructLoopResolvedCall{
		entry: &fastBuilderCallCacheEntry{plan: &callPlan{
			numIn:    1,
			argTypes: []reflect.Type{stringType},
		}},
	}
	err = staticResolved.prepareStaticReflectArgs(&fastStructLoopCallPlan{
		call: &compiler.FastCallPlan{Name: "label"},
		args: []fastStructLoopCallArgPlan{{
			kind:      fastStructLoopCallArgBinding,
			nameIndex: 99,
			line:      74,
			value:     compiler.FastValuePlan{Value: "missing"},
		}},
	}, fastRenderBindings{})
	require.ErrorContains(t, err, `line 74`)
}

func Test_VM_Fast_Struct_Loop_Direct_Writer_And_String_Arg_Remaining_Branches(t *testing.T) {
	ctx := plush.NewContext()
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{}, ctx)
	item := reflect.ValueOf(vmStructLoopProduct{Name: "bot"})
	firstMissingPlan := structLoopCallPlan(t,
		compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 86},
		structLoopNameArg(),
	)

	for _, tt := range []struct {
		name string
		raw  interface{}
	}{
		{"string_string", func(value, prefix string) string { return prefix + value }},
		{"string_string_error", func(value, prefix string) (string, error) { return prefix + value, nil }},
		{"html_string", func(value, prefix string) template.HTML { return template.HTML(prefix + value) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writer := fastStructLoopDirectCallWriterForRaw(tt.raw, firstMissingPlan)
			require.NotNil(t, writer)
			handled, err := writer(&strings.Builder{}, ctx, bindings, firstMissingPlan, 0, item)
			require.NoError(t, err)
			require.False(t, handled)
		})
	}

	errorPlan := structLoopCallPlan(t, structLoopNameArg(), compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "suffix"})
	writer := fastStructLoopDirectCallWriterForRaw(func(string, string) (string, error) {
		return "", errors.New("two arg boom")
	}, errorPlan)
	require.NotNil(t, writer)
	handled, err := writer(&strings.Builder{}, ctx, bindings, errorPlan, 0, item)
	require.True(t, handled)
	require.ErrorContains(t, err, "two arg boom")

	childOnlyPlan, ok := fastAccessChainPlanFor(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{{
			Kind:     compiler.FastPathStepProperty,
			Value:    "Child",
			Receiver: "user",
			Full:     "user.Child",
			Line:     87,
		}},
	}, reflect.TypeOf(vmFastPropertyUser{}))
	require.True(t, ok)

	arg, ok, err := evalFastStructLoopCallArgString(
		&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: childOnlyPlan},
		ctx,
		bindings,
		0,
		reflect.ValueOf(vmFastPropertyUser{}),
	)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, arg)

	nameOnlyPlan, ok := fastAccessChainPlanFor(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{{
			Kind:     compiler.FastPathStepProperty,
			Value:    "Name",
			Receiver: "user",
			Full:     "user.Name",
			Line:     88,
		}},
	}, reflect.TypeOf(vmFastPropertyChild{}))
	require.True(t, ok)
	arg, ok, err = evalFastStructLoopCallArgString(
		&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: nameOnlyPlan},
		ctx,
		bindings,
		0,
		reflect.ValueOf(vmFastPropertyChild{Name: "kid"}),
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "kid", arg)

	hiddenPlan, ok := fastAccessChainPlanFor(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{{
			Kind:     compiler.FastPathStepProperty,
			Value:    "child",
			Receiver: "user",
			Full:     "user.child",
			Line:     89,
		}},
	}, reflect.TypeOf(vmFastValueHiddenChild{}))
	require.True(t, ok)
	arg, ok, err = evalFastStructLoopCallArgString(
		&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: hiddenPlan},
		ctx,
		bindings,
		0,
		reflect.ValueOf(vmFastValueHiddenChild{child: vmFastPropertyChild{Name: "hidden"}}),
	)
	require.ErrorContains(t, err, "line 89")
	require.Empty(t, arg)

	arg, ok, err = evalFastStructLoopCallArgString(
		&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: &fastAccessChainPlan{}},
		ctx,
		bindings,
		0,
		reflect.ValueOf([]string(nil)),
	)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, arg)

	hiddenField := reflect.ValueOf(vmFastValueHiddenChild{child: vmFastPropertyChild{Name: "hidden"}}).FieldByName("child")
	arg, ok, err = evalFastStructLoopCallArgString(
		&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: &fastAccessChainPlan{}},
		ctx,
		bindings,
		0,
		hiddenField,
	)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, arg)
}

func Test_VM_Fast_Struct_Loop_Value_Remaining_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"fail": func() string { return "never" }})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"fail"}}, ctx)
	item := reflect.ValueOf(vmStructLoopProduct{Name: "bot"})

	truthy, ok, err := isTruthyFastStructLoopValue(&compiler.FastValuePlan{
		Kind:      compiler.FastValueName,
		NameIndex: 99,
		Value:     "missing",
	}, ctx, bindings, 0, item)
	require.NoError(t, err)
	require.False(t, truthy)
	require.False(t, ok)

	var objectIface interface{} = object.TrueObject
	require.True(t, isTruthyFastReflectValue(reflect.ValueOf(&objectIface).Elem()))
	var stringIface interface{} = "hello"
	require.True(t, isTruthyFastReflectValue(reflect.ValueOf(&stringIface).Elem()))

	value, ok, err := evalFastStructLoopPathValue(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{{
			Kind:   compiler.FastPathStepProperty,
			Value:  "Echo",
			Method: true,
			Line:   90,
		}},
	}, ctx, bindings, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, value)

	_, ok, err = evalFastStructLoopLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left: &compiler.FastValuePlan{
			Kind: compiler.FastValueCall,
			Call: &compiler.FastCallPlan{Name: "fail", NameIndex: 0, Line: 91},
		},
		Right: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}, plush.NewContextWith(map[string]interface{}{"fail": func() string { return "never" }}).WithBudget(plush.NewBudget(0)), bindings, 0, item)
	require.ErrorContains(t, err, "line 91")
	require.True(t, ok)

	var nilIface interface{}
	conditionValue := reflect.ValueOf(&nilIface).Elem()
	boolValue, boolOK := fastConditionOperandValue{hasReflect: true, reflect: conditionValue}.boolValue()
	require.False(t, boolValue)
	require.False(t, boolOK)
	stringValue, stringOK := fastConditionOperandValue{hasReflect: true, reflect: conditionValue}.stringValue()
	require.Empty(t, stringValue)
	require.False(t, stringOK)
}

func Test_VM_Fast_Access_Chain_Nil_Intermediate_Remaining_Branches(t *testing.T) {
	ctx := plush.NewContext()
	chain := &fastAccessChainPlan{steps: []fastAccessChainStep{
		{kind: fastAccessStepIndex, index: 0, line: 92},
		{kind: fastAccessStepField, line: 92},
	}}

	value, ok, err := evalFastAccessChainPlanValue(chain, reflect.ValueOf([]*vmFastPropertyChild{nil}), ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	var out strings.Builder
	handled, err := writeFastAccessChainPlanOutput(&out, ctx, chain, reflect.ValueOf([]*vmFastPropertyChild{nil}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())
}
