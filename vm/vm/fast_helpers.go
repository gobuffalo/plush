package vm

import (
	"context"
	"errors"
	"html/template"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/object"
)

var ErrFastUnsupported = errors.New("fast write unsupported")

var errFastWriteUnsupported = ErrFastUnsupported

type writeFastInvoker func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error
type writeFastBuilderInvoker func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error
type valueFastInvoker func(name string, raw interface{}, args *fastCallArgs) (interface{}, error)
type contextualValueFastInvoker func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error)
type fastStructLoopDirectCallWriter func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error)
type FastHelperFunc func(FastWriter, FastArgs) error

// FastNoContextValueHelperFunc is a fast value helper that cannot access the
// render context. Use it for helpers whose result depends only on their
// arguments (or on state explicitly captured by the helper closure).
//
// Unlike FastValueHelperFunc, the VM does not create a scoped helper context,
// copy frame locals into that context, or synchronize bindings after the call.
// The normal helper must still be registered for ErrFastUnsupported fallback.
type FastNoContextValueHelperFunc func(FastArgs) (interface{}, error)

// FastReadOnlyContext exposes binding lookup without Set, Update, or New.
// It is valid only for the duration of a FastReadOnlyValueHelperFunc call and
// must not be retained.
//
// Read-only applies to context bindings, not recursively to the values stored
// in them. Maps, slices, pointers (including pointers to arrays), arrays that
// contain reference values, and objects returned by Value may still expose
// mutable data. Read-only helpers must treat all reachable values as immutable.
// A helper that needs to mutate a reachable value must use FastValueHelperFunc
// or make and mutate its own defensive copy.
type FastReadOnlyContext interface {
	context.Context
	Has(key string) bool
}

// FastReadOnlyValueHelperFunc is a fast value helper that may inspect context
// bindings but cannot create, replace, or update them through its context.
// The VM makes frame locals visible for the call but deliberately skips binding
// synchronization afterward.
//
// This is shallow read-only access. The helper must not mutate maps, slices,
// pointers, arrays containing references, or objects obtained from the context;
// see FastReadOnlyContext.
type FastReadOnlyValueHelperFunc func(FastReadOnlyContext, FastArgs) (interface{}, error)

// FastValueHelperFunc receives a context that is valid only for the duration
// of the helper call. Helpers must not retain the context after returning. It
// is the existing full read/write mode: binding changes are synchronized back
// into the VM after a successful helper call.
type FastValueHelperFunc func(hctx.Context, FastArgs) (interface{}, error)

type fastValueHelperMode uint8

const (
	fastValueHelperReadWrite fastValueHelperMode = iota
	fastValueHelperNoContext
	fastValueHelperReadOnly
)

type fastValueHelperRegistration struct {
	mode      fastValueHelperMode
	readWrite FastValueHelperFunc
	noContext FastNoContextValueHelperFunc
	readOnly  FastReadOnlyValueHelperFunc
}

type fastHelperRegistry struct {
	mu           sync.RWMutex
	helpers      map[string]FastHelperFunc
	valueHelpers map[string]fastValueHelperRegistration
}

type FastWriter struct {
	out *strings.Builder
	ctx hctx.Context
}

// FastArgs is valid only for the duration of its helper call. Helpers that need
// an argument afterward must retain the value returned by Raw, not FastArgs.
type FastArgs struct {
	args *fastCallArgs
}

func SetFastHelper(ctx hctx.Context, name string, helper FastHelperFunc) {
	if ctx == nil || name == "" {
		return
	}
	registry := fastHelperRegistryForContext(ctx, true)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if helper == nil {
		delete(registry.helpers, name)
		return
	}
	registry.helpers[name] = helper
}

func ClearFastHelper(ctx hctx.Context, name string) {
	SetFastHelper(ctx, name, nil)
}

func SetFastValueHelper(ctx hctx.Context, name string, helper FastValueHelperFunc) {
	setFastValueHelperRegistration(ctx, name, fastValueHelperRegistration{
		mode:      fastValueHelperReadWrite,
		readWrite: helper,
	}, helper == nil)
}

// SetFastNoContextValueHelper registers a helper that receives arguments but
// no render context. Registering any value-helper mode for the same name
// replaces the previous mode. The normal helper should remain registered for
// correctness and ErrFastUnsupported fallback.
func SetFastNoContextValueHelper(ctx hctx.Context, name string, helper FastNoContextValueHelperFunc) {
	setFastValueHelperRegistration(ctx, name, fastValueHelperRegistration{
		mode:      fastValueHelperNoContext,
		noContext: helper,
	}, helper == nil)
}

// SetFastReadOnlyValueHelper registers a binding-read-only helper. The helper
// can read root bindings and current frame locals, but it cannot write bindings
// through FastReadOnlyContext and the VM performs no post-call write-back.
//
// This API does not provide deep immutability. Helpers must not mutate maps,
// slices, pointers, arrays containing references, or objects obtained from the
// context. Use the existing SetFastValueHelper API when mutation is required.
func SetFastReadOnlyValueHelper(ctx hctx.Context, name string, helper FastReadOnlyValueHelperFunc) {
	setFastValueHelperRegistration(ctx, name, fastValueHelperRegistration{
		mode:     fastValueHelperReadOnly,
		readOnly: helper,
	}, helper == nil)
}

func setFastValueHelperRegistration(ctx hctx.Context, name string, registration fastValueHelperRegistration, clear bool) {
	if ctx == nil || name == "" {
		return
	}
	registry := fastHelperRegistryForContext(ctx, true)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.valueHelpers == nil {
		registry.valueHelpers = map[string]fastValueHelperRegistration{}
	}
	if clear {
		delete(registry.valueHelpers, name)
		return
	}
	registry.valueHelpers[name] = registration
}

func ClearFastValueHelper(ctx hctx.Context, name string) {
	SetFastValueHelper(ctx, name, nil)
}

func ClearFastNoContextValueHelper(ctx hctx.Context, name string) {
	SetFastNoContextValueHelper(ctx, name, nil)
}

func ClearFastReadOnlyValueHelper(ctx hctx.Context, name string) {
	SetFastReadOnlyValueHelper(ctx, name, nil)
}

func fastHelperRegistryForContext(ctx hctx.Context, create bool) *fastHelperRegistry {
	if ctx == nil {
		return nil
	}
	var value interface{}
	if lookup, ok := ctx.(contextLookup); ok {
		value, _ = lookup.Lookup(vmFastHelpersKey)
	} else {
		value = ctx.Value(vmFastHelpersKey)
	}
	if registry, ok := value.(*fastHelperRegistry); ok && registry != nil {
		return registry
	}
	if !create {
		return nil
	}
	registry := &fastHelperRegistry{helpers: map[string]FastHelperFunc{}}
	ctx.Set(vmFastHelpersKey, registry)
	return registry
}

func fastHelperForContext(ctx hctx.Context, name string) (FastHelperFunc, bool) {
	if name == "" {
		return nil, false
	}
	registry := fastHelperRegistryForContext(ctx, false)
	if registry == nil {
		return nil, false
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	helper, ok := registry.helpers[name]
	return helper, ok && helper != nil
}

func fastValueHelperForContext(ctx hctx.Context, name string) (FastValueHelperFunc, bool) {
	registration, ok := fastValueHelperRegistrationForContext(ctx, name)
	if !ok || registration.mode != fastValueHelperReadWrite || registration.readWrite == nil {
		return nil, false
	}
	return registration.readWrite, true
}

func fastValueHelperRegistrationForContext(ctx hctx.Context, name string) (fastValueHelperRegistration, bool) {
	if name == "" {
		return fastValueHelperRegistration{}, false
	}
	registry := fastHelperRegistryForContext(ctx, false)
	if registry == nil {
		return fastValueHelperRegistration{}, false
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	registration, ok := registry.valueHelpers[name]
	return registration, ok
}

func writeRegisteredFastHelper(out *strings.Builder, ctx hctx.Context, helper FastHelperFunc, args *fastCallArgs) (bool, error) {
	if out == nil || helper == nil {
		return false, nil
	}
	err := helper(FastWriter{out: out, ctx: ctx}, FastArgs{args: args})
	if errors.Is(err, ErrFastUnsupported) {
		return false, nil
	}
	return true, err
}

func writeRegisteredFastHelperNamed(out *strings.Builder, ctx hctx.Context, name string, helper FastHelperFunc, args *fastCallArgs, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder) (bool, error) {
	var start time.Time
	if vmHotspots.Enabled() {
		start = time.Now()
	}
	handled, err := writeRegisteredFastHelper(out, ctx, helper, args)
	if handled && vmHotspots.Enabled() {
		vmHotspots.AddHelperCall(name, vmHelperCallSignature(reflect.TypeOf(helper)), plush.RenderVMHelperCallDirect, time.Since(start))
	}
	return handled, err
}

func callRegisteredFastValueHelper(ctx hctx.Context, name string, helper FastValueHelperFunc, args *fastCallArgs, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder) (interface{}, bool, error) {
	return callRegisteredFastValueHelperRegistration(ctx, name, fastValueHelperRegistration{
		mode:      fastValueHelperReadWrite,
		readWrite: helper,
	}, args, vmHotspots)
}

func callRegisteredFastValueHelperRegistration(ctx hctx.Context, name string, registration fastValueHelperRegistration, args *fastCallArgs, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder) (interface{}, bool, error) {
	switch registration.mode {
	case fastValueHelperReadWrite:
		if registration.readWrite == nil {
			return nil, false, nil
		}
	case fastValueHelperNoContext:
		if registration.noContext == nil {
			return nil, false, nil
		}
	case fastValueHelperReadOnly:
		if registration.readOnly == nil {
			return nil, false, nil
		}
	default:
		return nil, false, nil
	}
	var start time.Time
	if vmHotspots.Enabled() {
		start = time.Now()
	}
	fastArgs := FastArgs{args: args}
	var value interface{}
	var err error
	switch registration.mode {
	case fastValueHelperNoContext:
		value, err = registration.noContext(fastArgs)
	case fastValueHelperReadOnly:
		value, err = registration.readOnly(fastReadOnlyContext{ctx: ctx}, fastArgs)
	default:
		value, err = registration.readWrite(ctx, fastArgs)
	}
	if errors.Is(err, ErrFastUnsupported) {
		return nil, false, nil
	}
	if vmHotspots.Enabled() {
		vmHotspots.AddHelperCall(name, vmHelperCallRegistrationSignature(registration), plush.RenderVMHelperCallDirect, time.Since(start))
	}
	return value, true, err
}

func vmHelperCallRegistrationSignature(registration fastValueHelperRegistration) string {
	switch registration.mode {
	case fastValueHelperNoContext:
		return vmHelperCallSignature(reflect.TypeOf(registration.noContext))
	case fastValueHelperReadOnly:
		return vmHelperCallSignature(reflect.TypeOf(registration.readOnly))
	default:
		return vmHelperCallSignature(reflect.TypeOf(registration.readWrite))
	}
}

type fastReadOnlyContext struct {
	ctx hctx.Context
}

func (c fastReadOnlyContext) Deadline() (time.Time, bool) {
	if c.ctx == nil {
		return time.Time{}, false
	}
	return c.ctx.Deadline()
}

func (c fastReadOnlyContext) Done() <-chan struct{} {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Done()
}

func (c fastReadOnlyContext) Err() error {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Err()
}

func (c fastReadOnlyContext) Value(key interface{}) interface{} {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Value(key)
}

func (c fastReadOnlyContext) Has(key string) bool {
	return c.ctx != nil && c.ctx.Has(key)
}

func (w FastWriter) Context() hctx.Context {
	return w.ctx
}

func (w FastWriter) WriteEscapedString(value string) {
	if w.out != nil {
		writeFastEscapedString(w.out, value)
	}
}

func (w FastWriter) WriteHTML(value template.HTML) {
	if w.out != nil {
		w.out.WriteString(string(value))
	}
}

func (w FastWriter) WriteHTMLString(value string) {
	if w.out != nil {
		w.out.WriteString(value)
	}
}

func (w FastWriter) WriteGoValue(value interface{}) bool {
	if w.out == nil {
		return false
	}
	return writeFastGoValue(w.out, w.ctx, value)
}

func (a FastArgs) Len() int {
	if a.args == nil {
		return 0
	}
	return a.args.Len()
}

func (a FastArgs) raw(index int) (interface{}, bool) {
	if a.args == nil || index < 0 || index >= a.args.Len() {
		return nil, false
	}
	return a.args.Raw(index), true
}

func (a FastArgs) Raw(index int) (interface{}, bool) {
	value, ok := a.raw(index)
	if !ok {
		return nil, false
	}
	return fastArgGoValue(value), true
}

func (a FastArgs) String(index int) (string, bool) {
	value, ok := a.raw(index)
	if !ok {
		return "", false
	}
	return fastWriteRawStringArg(value)
}

func (a FastArgs) Bool(index int) (bool, bool) {
	value, ok := a.raw(index)
	if !ok {
		return false, false
	}
	switch value := value.(type) {
	case *object.Boolean:
		return value.Value, true
	case *object.Native:
		v, ok := value.Value.(bool)
		return v, ok
	}
	value = fastArgGoValue(value)
	v, ok := value.(bool)
	return v, ok
}

func (a FastArgs) Int64(index int) (int64, bool) {
	value, ok := a.raw(index)
	if !ok {
		return 0, false
	}
	return fastArgInt64(value)
}

func (a FastArgs) Uint64(index int) (uint64, bool) {
	value, ok := a.raw(index)
	if !ok {
		return 0, false
	}
	return fastArgUint64(value)
}

func (a FastArgs) Float64(index int) (float64, bool) {
	value, ok := a.raw(index)
	if !ok {
		return 0, false
	}
	return fastArgFloat64(value)
}

func (a FastArgs) Object(index int) (object.Object, bool) {
	if a.args == nil || index < 0 || index >= a.args.Len() {
		return nil, false
	}
	obj, ok := a.args.Raw(index).(object.Object)
	return obj, ok
}

func fastArgGoValue(value interface{}) interface{} {
	if obj, ok := value.(object.Object); ok {
		if object.IsNull(obj) {
			return nil
		}
		return object.ToGo(obj)
	}
	return value
}

func fastArgInt64(value interface{}) (int64, bool) {
	switch value := value.(type) {
	case *object.Integer:
		return int64(int(value.Value)), true
	case *object.Float:
		return int64(value.Value), true
	case *object.Native:
		return fastArgInt64Native(value.Value)
	}
	value = fastArgGoValue(value)
	return fastArgInt64Native(value)
}

func fastArgInt64Native(value interface{}) (int64, bool) {
	switch value := value.(type) {
	case int:
		return int64(value), true
	case int8:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	case uint:
		if uint64(value) > math.MaxInt64 {
			return 0, false
		}
		return int64(value), true
	case uint8:
		return int64(value), true
	case uint16:
		return int64(value), true
	case uint32:
		return int64(value), true
	case uint64:
		if value > math.MaxInt64 {
			return 0, false
		}
		return int64(value), true
	case uintptr:
		if uint64(value) > math.MaxInt64 {
			return 0, false
		}
		return int64(value), true
	case float32:
		return int64(value), true
	case float64:
		return int64(value), true
	default:
		rv := reflect.ValueOf(value)
		if !rv.IsValid() {
			return 0, false
		}
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rv.Int(), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			v := rv.Uint()
			if v > math.MaxInt64 {
				return 0, false
			}
			return int64(v), true
		case reflect.Float32, reflect.Float64:
			return int64(rv.Float()), true
		}
		return 0, false
	}
}

func fastArgUint64(value interface{}) (uint64, bool) {
	switch value := value.(type) {
	case *object.Integer:
		return fastArgUint64Native(int(value.Value))
	case *object.Float:
		return fastArgUint64Native(value.Value)
	case *object.Native:
		return fastArgUint64Native(value.Value)
	}
	value = fastArgGoValue(value)
	return fastArgUint64Native(value)
}

func fastArgUint64Native(value interface{}) (uint64, bool) {
	switch value := value.(type) {
	case int:
		if value < 0 {
			return 0, false
		}
		return uint64(value), true
	case int8:
		if value < 0 {
			return 0, false
		}
		return uint64(value), true
	case int16:
		if value < 0 {
			return 0, false
		}
		return uint64(value), true
	case int32:
		if value < 0 {
			return 0, false
		}
		return uint64(value), true
	case int64:
		if value < 0 {
			return 0, false
		}
		return uint64(value), true
	case uint:
		return uint64(value), true
	case uint8:
		return uint64(value), true
	case uint16:
		return uint64(value), true
	case uint32:
		return uint64(value), true
	case uint64:
		return value, true
	case uintptr:
		return uint64(value), true
	case float32:
		if value < 0 {
			return 0, false
		}
		return uint64(value), true
	case float64:
		if value < 0 {
			return 0, false
		}
		return uint64(value), true
	default:
		rv := reflect.ValueOf(value)
		if !rv.IsValid() {
			return 0, false
		}
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v := rv.Int()
			if v < 0 {
				return 0, false
			}
			return uint64(v), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return rv.Uint(), true
		case reflect.Float32, reflect.Float64:
			v := rv.Float()
			if v < 0 {
				return 0, false
			}
			return uint64(v), true
		}
		return 0, false
	}
}

func fastArgFloat64(value interface{}) (float64, bool) {
	switch value := value.(type) {
	case *object.Integer:
		return float64(int(value.Value)), true
	case *object.Float:
		return value.Value, true
	case *object.Native:
		return fastArgFloat64Native(value.Value)
	}
	value = fastArgGoValue(value)
	return fastArgFloat64Native(value)
}

func fastArgFloat64Native(value interface{}) (float64, bool) {
	switch value := value.(type) {
	case int:
		return float64(value), true
	case int8:
		return float64(value), true
	case int16:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint8:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	case uintptr:
		return float64(value), true
	case float32:
		return float64(value), true
	case float64:
		return value, true
	default:
		rv := reflect.ValueOf(value)
		if !rv.IsValid() {
			return 0, false
		}
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int()), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return float64(rv.Uint()), true
		case reflect.Float32, reflect.Float64:
			return rv.Float(), true
		}
		return 0, false
	}
}
