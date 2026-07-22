package vm

import (
	"errors"
	"html/template"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

var ErrFastUnsupported = errors.New("fast write unsupported")

var errFastWriteUnsupported = ErrFastUnsupported

type writeFastInvoker func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error
type writeFastBuilderInvoker func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error
type valueFastInvoker func(name string, raw interface{}, args *fastCallArgs) (interface{}, error)
type contextualValueFastInvoker func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error)
type fastStructLoopDirectCallWriter func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error)
type FastHelperFunc func(FastWriter, FastArgs) error

type fastHelperRegistry struct {
	mu      sync.RWMutex
	helpers map[string]FastHelperFunc
}

type FastWriter struct {
	out *strings.Builder
	ctx hctx.Context
}

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

func fastHelperRegistryForContext(ctx hctx.Context, create bool) *fastHelperRegistry {
	if ctx == nil {
		return nil
	}
	if registry, ok := ctx.Value(vmFastHelpersKey).(*fastHelperRegistry); ok && registry != nil {
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

func writeRegisteredFastHelperNamed(out *strings.Builder, ctx hctx.Context, name string, helper FastHelperFunc, args *fastCallArgs) (bool, error) {
	start := time.Now()
	handled, err := writeRegisteredFastHelper(out, ctx, helper, args)
	if handled {
		plush.AddRenderDiagnosticVMHelperTiming(ctx, name, time.Since(start))
	}
	return handled, err
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

func (a FastArgs) Raw(index int) (interface{}, bool) {
	if a.args == nil || index < 0 || index >= a.args.Len() {
		return nil, false
	}
	return fastArgGoValue(a.args.Raw(index)), true
}

func (a FastArgs) String(index int) (string, bool) {
	if a.args == nil || index < 0 || index >= a.args.Len() {
		return "", false
	}
	return fastWriteRawStringArg(a.args.Raw(index))
}

func (a FastArgs) Bool(index int) (bool, bool) {
	value, ok := a.Raw(index)
	if !ok {
		return false, false
	}
	v, ok := value.(bool)
	return v, ok
}

func (a FastArgs) Int64(index int) (int64, bool) {
	value, ok := a.Raw(index)
	if !ok {
		return 0, false
	}
	return fastArgInt64(value)
}

func (a FastArgs) Uint64(index int) (uint64, bool) {
	value, ok := a.Raw(index)
	if !ok {
		return 0, false
	}
	return fastArgUint64(value)
}

func (a FastArgs) Float64(index int) (float64, bool) {
	value, ok := a.Raw(index)
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
	value = fastArgGoValue(value)
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
	value = fastArgGoValue(value)
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
	value = fastArgGoValue(value)
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
