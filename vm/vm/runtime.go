package vm

import (
	"fmt"
	"sync"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/parser"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/gobuffalo/plush/v5/vm/object"
)

func New(bytecode *compiler.Bytecode) *VM {
	return NewWithContext(bytecode, plush.NewContext())
}

func NewWithContext(bytecode *compiler.Bytecode, ctx hctx.Context) *VM {
	return newWithContext(bytecode, ctx, false)
}

func newPooledWithContext(bytecode *compiler.Bytecode, ctx hctx.Context) *VM {
	return newWithContext(bytecode, ctx, true)
}

func newPooledWithContextDiagnostics(bytecode *compiler.Bytecode, ctx hctx.Context, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder) *VM {
	return newWithContextDiagnostics(bytecode, ctx, true, vmHotspots)
}

func newWithContext(bytecode *compiler.Bytecode, ctx hctx.Context, pooled bool) *VM {
	if ctx == nil {
		ctx = plush.NewContext()
	}
	return newWithContextDiagnosticsState(bytecode, ctx, pooled, plush.RenderVMHotspotDiagnosticsRecorder{}, false)
}

func newWithContextDiagnostics(bytecode *compiler.Bytecode, ctx hctx.Context, pooled bool, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder) *VM {
	return newWithContextDiagnosticsState(bytecode, ctx, pooled, vmHotspots, true)
}

func newWithContextDiagnosticsState(bytecode *compiler.Bytecode, ctx hctx.Context, pooled bool, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder, vmHotspotsCaptured bool) *VM {
	if ctx == nil {
		ctx = plush.NewContext()
	}

	mainFn := &object.CompiledFunction{
		Instructions:              bytecode.Instructions,
		CallNames:                 bytecode.CallNames,
		LocalNames:                bytecode.LocalNames,
		DynamicContextNameIndexes: bytecode.DynamicContextNameIndexes,
		DynamicContextNamesReady:  bytecode.DynamicContextNamesReady,
		LineNumbers:               bytecode.LineNumbers,
		Properties:                bytecode.Properties,
		PropertyCaches:            bytecode.PropertyCaches,
		CallCaches:                bytecode.CallCaches,
		RegexCaches:               bytecode.RegexCaches,
		NumLocals:                 bytecode.NumLocals,
	}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := newFrame(mainClosure, 0, pooled)

	frames := borrowFrames(pooled)
	frames[0] = mainFrame

	holes := borrowHoles(pooled)
	globalSize := globalStoreSize(bytecode)

	machine := &VM{
		constants:          bytecode.Constants,
		stack:              borrowStack(pooled),
		sp:                 mainFn.NumLocals,
		stackMax:           mainFn.NumLocals,
		globals:            borrowGlobals(globalSize, pooled),
		globalNames:        bytecode.GlobalNames,
		frames:             frames,
		framesIndex:        1,
		ctx:                ctx,
		vmHotspots:         vmHotspots,
		vmHotspotsCaptured: vmHotspotsCaptured,
		holes:              holes,
		pooled:             pooled,
		ownGlobals:         pooled,
		ownHoles:           pooled,
	}
	return machine
}

func NewWithGlobalsStore(bytecode *compiler.Bytecode, globals []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = globals
	return vm
}

func Compile(input string) (*Template, error) {
	program, err := parser.Parse(preprocessTrimTags(input))
	if err != nil {
		return nil, err
	}

	return templateFromBytecode(compileProgramBytecode(program))
}

func compileProgramBytecode(program *ast.Program) (*compiler.Bytecode, error) {
	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		return nil, err
	}
	return comp.Bytecode(), nil
}

func templateFromBytecode(bytecode *compiler.Bytecode, err error) (*Template, error) {
	if err != nil {
		return nil, err
	}
	return &Template{bytecode: bytecode}, nil
}

func (t *Template) Render(ctx hctx.Context) (string, error) {
	if t == nil || t.bytecode == nil {
		return "", fmt.Errorf("cannot render nil compiled template")
	}
	if ctx == nil {
		ctx = plush.NewContext()
	}
	start := time.Now()
	plush.UpdateRenderDiagnostics(ctx, func(d *plush.RenderDiagnostics) {
		d.Mode = plush.RenderModeNameVM
		d.VMBytecodeCache = plush.VMBytecodeCacheDirect
		d.FastPath = plush.RenderFastPathGeneric
		d.PunchHoleCache = plush.PunchHoleCacheDisabled
	})
	updateBytecodeDiagnostics(ctx, t.bytecode)
	defer func() {
		plush.UpdateRenderDiagnostics(ctx, func(d *plush.RenderDiagnostics) {
			d.Mode = plush.RenderModeNameVM
			d.EngineDuration = time.Since(start)
		})
	}()
	return renderBytecode(t.bytecode, ctx)
}

func globalStoreSize(bytecode *compiler.Bytecode) int {
	size := bytecode.NumGlobals
	for index := range bytecode.GlobalNames {
		if index+1 > size {
			size = index + 1
		}
	}
	if size < 0 {
		return 0
	}
	return size
}

func borrowStack(pooled bool) []object.Object {
	if !pooled {
		return make([]object.Object, StackSize)
	}
	stack := stackPool.Get().(*[]object.Object)
	return (*stack)[:StackSize]
}

func releaseStack(stack []object.Object, used int) {
	if cap(stack) < StackSize {
		return
	}
	stack = stack[:StackSize]
	if used > len(stack) {
		used = len(stack)
	}
	if used > 0 {
		clearObjectSlice(stack[:used])
	}
	stackPool.Put(&stack)
}

func borrowFrames(pooled bool) []*Frame {
	if !pooled {
		return make([]*Frame, MaxFrames)
	}
	frames := framesPool.Get().(*[]*Frame)
	return (*frames)[:MaxFrames]
}

func releaseFrames(frames []*Frame) {
	if cap(frames) < MaxFrames {
		return
	}
	frames = frames[:MaxFrames]
	clear(frames)
	framesPool.Put(&frames)
}

func newFrame(cl *object.Closure, basePointer int, pooled bool) *Frame {
	if !pooled {
		return NewFrame(cl, basePointer)
	}
	frame := framePool.Get().(*Frame)
	frame.pooled = true
	frame.reset(cl, basePointer)
	return frame
}

func releaseFrame(frame *Frame) {
	if frame == nil || !frame.pooled {
		return
	}
	frame.reset(nil, 0)
	framePool.Put(frame)
}

func borrowHoles(pooled bool) *[]plush.HoleMarker {
	if !pooled {
		holes := []plush.HoleMarker{}
		return &holes
	}
	holes := holesPool.Get().(*[]plush.HoleMarker)
	clear(*holes)
	*holes = (*holes)[:0]
	return holes
}

func releaseHoles(holes *[]plush.HoleMarker) {
	if holes == nil {
		return
	}
	clear(*holes)
	*holes = (*holes)[:0]
	holesPool.Put(holes)
}

func borrowGlobals(size int, pooled bool) []object.Object {
	if size <= 0 {
		return nil
	}
	if !pooled {
		return make([]object.Object, size)
	}
	pool := globalsPool(size)
	if globalsPtr, ok := pool.Get().(*[]object.Object); ok && cap(*globalsPtr) >= size {
		globals := (*globalsPtr)[:size]
		clearObjectSlice(globals)
		return globals
	}
	return make([]object.Object, size)
}

func releaseGlobals(globals []object.Object) {
	if cap(globals) == 0 {
		return
	}
	size := cap(globals)
	globals = globals[:size]
	clearObjectSlice(globals)
	globalsPool(size).Put(&globals)
}

func globalsPool(size int) *sync.Pool {
	pool, _ := globalsPools.LoadOrStore(size, &sync.Pool{})
	return pool.(*sync.Pool)
}

func clearObjectSlice(objects []object.Object) {
	for i := range objects {
		objects[i] = nil
	}
}

func (vm *VM) Release() {
	if vm == nil {
		return
	}
	vm.discardLastHelperContext()
	if !vm.pooled {
		return
	}
	if vm.frames != nil {
		for i := 0; i < vm.framesIndex && i < len(vm.frames); i++ {
			releaseFrame(vm.frames[i])
			vm.frames[i] = nil
		}
		releaseFrames(vm.frames)
	}
	if vm.stack != nil {
		releaseStack(vm.stack, vm.stackMax)
	}
	if vm.ownGlobals {
		releaseGlobals(vm.globals)
	}
	if vm.ownHoles {
		releaseHoles(vm.holes)
	}
	*vm = VM{}
}

func Render(input string, ctx hctx.Context) (string, error) {
	if ctx == nil {
		ctx = plush.NewContext()
	}
	start := time.Now()
	cacheSource := preprocessTrimTags(input)
	filename := plush.PunchHoleTemplateFilename(ctx)
	if trustedFilename := plush.TrustedTopLevelBytecodeCacheFilename(ctx); trustedFilename != "" {
		filename = trustedFilename
		plush.SetTrustedTopLevelBytecodeCacheFilename(ctx, "")
	}
	initialCacheState := plush.VMBytecodeCacheMiss
	if filename == "" || !plush.IsVMBytecodeCacheableTemplateFile(filename) {
		initialCacheState = plush.VMBytecodeCacheDisabled
	}
	plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
		d.Mode = plush.RenderModeNameVM
		d.TemplateFilename = filename
		d.VMBytecodeCache = initialCacheState
		d.FastPath = plush.RenderFastPathGeneric
		d.PunchHoleCache = plush.PunchHoleCacheDisabled
	})
	defer func() {
		if plush.RenderDiagnosticsRootActive(ctx) {
			return
		}
		plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
			d.Mode = plush.RenderModeNameVM
			if d.TemplateFilename == "" {
				d.TemplateFilename = filename
			}
			d.EngineDuration = time.Since(start)
		})
	}()

	if filename == "" {
		if bytecode, ok := cachedSourceBytecode(cacheSource); ok {
			return renderSourceCachedBytecode(cacheSource, ctx, bytecode)
		}
	}

	var cachedBytecode *compiler.Bytecode
	if cached, ok := plush.CachedVMBytecodeForCleanFilenameWithSource(filename, cacheSource); ok {
		if bytecode, ok := cached.(*compiler.Bytecode); ok {
			updateBytecodeDiagnostics(ctx, bytecode)
			if bytecode.Static {
				plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
					d.VMBytecodeCache = plush.VMBytecodeCacheHitStatic
					d.FastPath = plush.RenderFastPathStatic
				})
				return renderStaticOutput(bytecode, ctx, outputSizeOptions{topLevel: true}), nil
			}
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.VMBytecodeCache = plush.VMBytecodeCacheHit
			})
			cachedBytecode = bytecode
		}
	}
	if shouldFallbackGenericBytecode(cachedBytecode) {
		return renderInterpreterFallback(input, ctx, filename)
	}
	if rendered, ok, err := tryRenderFastBytecodeTopLevel(cachedBytecode, ctx); ok || err != nil {
		if ok {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.FastPath = renderFastPathForPlan(cachedBytecode.FastRenderPlan)
			})
		}
		return rendered, err
	}

	if cachedBytecode != nil {
		updateBytecodeDiagnostics(ctx, cachedBytecode)
		if restorePartial := installVMPartialHelperForBytecode(cachedBytecode, ctx); restorePartial != nil {
			defer restorePartial()
		}
		forceCacheClear := false
		if cachedBytecode.HasHoles && bytecodeCanUsePunchHoleCache(cachedBytecode) {
			var cached string
			var ok bool
			filename, forceCacheClear, cached, ok = punchHoleCacheStateForFilename(filename, ctx, cacheSource)
			if ok {
				plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
					d.TemplateFilename = filename
					d.PunchHoleCache = plush.PunchHoleCacheHit
				})
				return cached, nil
			}
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.TemplateFilename = filename
				d.PunchHoleCache = plush.PunchHoleCacheMiss
			})
		}

		return renderBytecodeVMWithStateTopLevel(cachedBytecode, ctx, filename, forceCacheClear, cacheSource)
	}

	filename, forceCacheClear, cached, ok := punchHoleCacheStateForFilename(filename, ctx, cacheSource)
	if ok {
		plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
			d.TemplateFilename = filename
			d.PunchHoleCache = plush.PunchHoleCacheHit
		})
		return cached, nil
	}

	input = cacheSource
	if cached, ok := plush.CachedVMBytecodeForCleanFilenameWithSource(filename, cacheSource); ok {
		if bytecode, ok := cached.(*compiler.Bytecode); ok {
			updateBytecodeDiagnostics(ctx, bytecode)
			if bytecode.Static {
				plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
					d.VMBytecodeCache = plush.VMBytecodeCacheHitStatic
					d.FastPath = plush.RenderFastPathStatic
				})
				return renderStaticOutput(bytecode, ctx, outputSizeOptions{topLevel: true}), nil
			}
			if rendered, ok, err := tryRenderFastBytecodeTopLevel(bytecode, ctx); ok || err != nil {
				if ok {
					plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
						d.VMBytecodeCache = plush.VMBytecodeCacheHit
						d.FastPath = renderFastPathForPlan(bytecode.FastRenderPlan)
					})
				}
				return rendered, err
			}
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.VMBytecodeCache = plush.VMBytecodeCacheHit
			})
			if shouldFallbackGenericBytecode(bytecode) {
				return renderInterpreterFallback(input, ctx, filename)
			}
			if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
				defer restorePartial()
			}
			return renderBytecodeVMWithStateTopLevel(bytecode, ctx, filename, forceCacheClear, cacheSource)
		}
	}

	program, cachedProgram, err := parseProgram(input, filename, ctx)
	if err != nil {
		return "", err
	}

	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		return "", err
	}

	bytecode := comp.Bytecode()
	if filename == "" {
		cacheSourceBytecode(cacheSource, bytecode)
	} else {
		plush.CacheVMBytecodeForCleanFilenameWithSource(filename, cachedProgram, bytecode, cacheSource)
	}
	updateBytecodeDiagnostics(ctx, bytecode)
	plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
		if filename == "" {
			d.VMBytecodeCache = plush.VMBytecodeCacheMissStoreSource
		} else if plush.IsPlushTemplateFile(filename) {
			d.VMBytecodeCache = plush.VMBytecodeCacheMissStore
		}
	})
	if shouldFallbackGenericBytecode(bytecode) {
		return renderInterpreterFallback(input, ctx, filename)
	}
	if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
		defer restorePartial()
	}
	return renderBytecodeWithState(bytecode, ctx, filename, forceCacheClear, cacheSource)
}

// RenderTrustedBytecodeCache renders a filename-keyed bytecode entry without
// source validation. File-backed callers must invalidate that filename when
// its source changes.
func RenderTrustedBytecodeCache(filename string, ctx hctx.Context) (string, bool, error) {
	if ctx == nil || filename == "" || !plush.TrustedPartialBytecodeCacheEnabled(ctx) {
		return "", false, nil
	}
	trustedFilename := plush.TrustedTopLevelBytecodeCacheFilename(ctx)
	if trustedFilename == "" {
		plush.SetTrustedTopLevelBytecodeCacheFilename(ctx, filename)
		trustedFilename = plush.TrustedTopLevelBytecodeCacheFilename(ctx)
		if trustedFilename == "" {
			return "", false, nil
		}
	}
	plush.SetTrustedTopLevelBytecodeCacheFilename(ctx, "")
	filename = trustedFilename
	cached, ok := plush.CachedVMBytecodeForCleanFilename(filename)
	if !ok {
		return "", false, nil
	}
	bytecode, ok := cached.(*compiler.Bytecode)
	if !ok || bytecode == nil || shouldFallbackGenericBytecode(bytecode) {
		return "", false, nil
	}

	start := time.Now()
	plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
		d.Mode = plush.RenderModeNameVM
		d.TemplateFilename = filename
		d.VMBytecodeCache = plush.VMBytecodeCacheHit
		d.FastPath = plush.RenderFastPathGeneric
		d.PunchHoleCache = plush.PunchHoleCacheDisabled
	})
	defer func() {
		plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
			d.EngineDuration += time.Since(start)
		})
	}()

	updateBytecodeDiagnostics(ctx, bytecode)
	if bytecode.Static {
		plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
			d.VMBytecodeCache = plush.VMBytecodeCacheHitStatic
			d.FastPath = plush.RenderFastPathStatic
		})
		return renderStaticOutput(bytecode, ctx, outputSizeOptions{topLevel: true}), true, nil
	}
	if rendered, handled, err := tryRenderFastBytecodeTopLevel(bytecode, ctx); handled || err != nil {
		if handled {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.FastPath = renderFastPathForPlan(bytecode.FastRenderPlan)
			})
		}
		return rendered, true, err
	}
	if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
		defer restorePartial()
	}

	forceCacheClear := false
	if bytecode.HasHoles && bytecodeCanUsePunchHoleCache(bytecode) {
		var rendered string
		var hit bool
		filename, forceCacheClear, rendered, hit = punchHoleCacheStateForFilename(filename, ctx, "")
		if hit {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.PunchHoleCache = plush.PunchHoleCacheHit
			})
			return rendered, true, nil
		}
	}

	rendered, err := renderBytecodeVMWithStateTopLevel(bytecode, ctx, filename, forceCacheClear, "")
	return rendered, true, err
}

func renderSourceCachedBytecode(source string, ctx hctx.Context, bytecode *compiler.Bytecode) (string, error) {
	updateBytecodeDiagnostics(ctx, bytecode)
	if bytecode.Static {
		plush.UpdateRenderDiagnosticsForTemplate(ctx, "", func(d *plush.RenderDiagnostics) {
			d.VMBytecodeCache = plush.VMBytecodeCacheHitSource
			d.FastPath = plush.RenderFastPathStatic
		})
		return renderStaticOutput(bytecode, ctx, outputSizeOptions{topLevel: true}), nil
	}
	plush.UpdateRenderDiagnosticsForTemplate(ctx, "", func(d *plush.RenderDiagnostics) {
		d.VMBytecodeCache = plush.VMBytecodeCacheHitSource
	})
	if shouldFallbackGenericBytecode(bytecode) {
		return renderInterpreterFallback(source, ctx, "")
	}
	if rendered, ok, err := tryRenderFastBytecodeTopLevel(bytecode, ctx); ok || err != nil {
		if ok {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, "", func(d *plush.RenderDiagnostics) {
				d.FastPath = renderFastPathForPlan(bytecode.FastRenderPlan)
			})
		}
		return rendered, err
	}
	if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
		defer restorePartial()
	}
	return renderBytecodeVMWithStateTopLevel(bytecode, ctx, "", false, source)
}

func shouldFallbackGenericBytecode(bytecode *compiler.Bytecode) bool {
	return plush.VMGenericFallbackEnabled() &&
		bytecode != nil &&
		!bytecode.Static &&
		bytecode.FastRenderPlan == nil
}

func renderInterpreterFallback(input string, ctx hctx.Context, filename string) (string, error) {
	plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
		d.FastPath = plush.RenderFastPathInterpreterFallback
	})
	if restorePartial := useInterpreterPartialHelper(ctx); restorePartial != nil {
		defer restorePartial()
	}
	return plush.RenderInterpreter(input, ctx)
}

func parseProgram(input, filename string, ctx hctx.Context) (*ast.Program, *ast.Program, error) {
	if filename != "" && plush.IsPlushTemplateFile(filename) {
		if program, ok := plush.CachedASTProgramWithSource(filename, ctx, input); ok {
			return program, program, nil
		}
	}
	program, err := parser.Parse(input)
	return program, nil, err
}

func renderBytecode(bytecode *compiler.Bytecode, ctx hctx.Context) (string, error) {
	if ctx == nil {
		ctx = plush.NewContext()
	}
	updateBytecodeDiagnostics(ctx, bytecode)
	filename := plush.PunchHoleTemplateFilename(ctx)
	if bytecode != nil && bytecode.Static {
		plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
			d.FastPath = plush.RenderFastPathStatic
		})
		return renderStaticOutput(bytecode, ctx, outputSizeOptions{topLevel: true}), nil
	}
	if rendered, ok, err := tryRenderFastBytecodeTopLevel(bytecode, ctx); ok || err != nil {
		if ok {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.FastPath = renderFastPathForPlan(bytecode.FastRenderPlan)
			})
		}
		return rendered, err
	}
	if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
		defer restorePartial()
	}

	forceCacheClear := false
	if bytecode == nil || bytecodeCanUsePunchHoleCache(bytecode) && bytecode.HasHoles {
		var cached string
		var ok bool
		filename, forceCacheClear, cached, ok = punchHoleCacheState(ctx)
		if ok {
			return cached, nil
		}
	}
	return renderBytecodeVMWithStateTopLevel(bytecode, ctx, filename, forceCacheClear, "")
}

func renderFastPathForPlan(plan *compiler.FastRenderPlan) string {
	if fastRenderPlanUsesGenericVM(plan) {
		return plush.RenderFastPathGeneric
	}
	return plush.RenderFastPathFast
}

func punchHoleCacheState(ctx hctx.Context) (string, bool, string, bool) {
	filename := plush.PunchHoleTemplateFilename(ctx)
	return punchHoleCacheStateForFilename(filename, ctx, "")
}

func punchHoleCacheStateForFilename(filename string, ctx hctx.Context, source string) (string, bool, string, bool) {
	if filename == "" {
		return "", false, "", false
	}

	cached, err := plush.RenderFromPunchHoleCacheWithSource(filename, source, ctx)
	if err == nil {
		return filename, false, cached, true
	}
	return filename, plush.IsPunchHoleCacheExpired(err), "", false
}

func renderBytecodeWithState(bytecode *compiler.Bytecode, ctx hctx.Context, filename string, forceCacheClear bool, source string) (string, error) {
	if bytecode != nil && bytecode.Static {
		return renderStaticOutput(bytecode, ctx, outputSizeOptions{topLevel: true}), nil
	}
	if rendered, ok, err := tryRenderFastBytecodeTopLevel(bytecode, ctx); ok || err != nil {
		return rendered, err
	}
	return renderBytecodeVMWithStateTopLevel(bytecode, ctx, filename, forceCacheClear, source)
}

func renderBytecodeVMWithState(bytecode *compiler.Bytecode, ctx hctx.Context, filename string, forceCacheClear bool, source string) (string, error) {
	return renderBytecodeVMWithStateOptions(bytecode, ctx, filename, forceCacheClear, source, outputSizeOptions{})
}

func renderBytecodeVMWithStateDiagnostics(bytecode *compiler.Bytecode, ctx hctx.Context, filename string, forceCacheClear bool, source string, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder) (string, error) {
	return renderBytecodeVMWithStateOptionsDiagnostics(bytecode, ctx, filename, forceCacheClear, source, outputSizeOptions{}, vmHotspots)
}

func renderBytecodeVMWithStateTopLevel(bytecode *compiler.Bytecode, ctx hctx.Context, filename string, forceCacheClear bool, source string) (string, error) {
	return renderBytecodeVMWithStateOptions(bytecode, ctx, filename, forceCacheClear, source, outputSizeOptions{topLevel: true})
}

func renderBytecodeVMWithStateOptions(bytecode *compiler.Bytecode, ctx hctx.Context, filename string, forceCacheClear bool, source string, options outputSizeOptions) (string, error) {
	return renderBytecodeVMWithStateOptionsDiagnostics(bytecode, ctx, filename, forceCacheClear, source, options, plush.CaptureRenderVMHotspotDiagnostics(ctx))
}

func renderBytecodeVMWithStateOptionsDiagnostics(bytecode *compiler.Bytecode, ctx hctx.Context, filename string, forceCacheClear bool, source string, options outputSizeOptions, vmHotspots plush.RenderVMHotspotDiagnosticsRecorder) (string, error) {
	machine := newPooledWithContextDiagnostics(bytecode, ctx, vmHotspots)
	observation := growVMRootOutput(machine, bytecode, ctx, options)
	if err := machine.Run(); err != nil {
		defer machine.Release()
		return "", machine.wrapRuntimeError(err)
	}

	recordVMRootOutputFinal(machine, &observation)
	rootSize := vmRootOutputLen(machine)
	rendered := machine.Rendered()
	if bytecode == nil || !bytecode.HasHoles {
		machine.Release()
		observeOutputSize(bytecode, ctx, options, rootSize, observation)
		return rendered, nil
	}
	holes := machine.PunchHoles()
	machine.Release()
	if !plush.IsPlushTemplateFile(filename) || len(holes) == 0 {
		observeOutputSize(bytecode, ctx, options, rootSize, observation)
		return rendered, nil
	}

	holes = plush.FinalizePunchHolePositions(rendered, holes)
	if bytecodeCanUsePunchHoleCache(bytecode) {
		plush.CachePunchHoleSkeletonWithSource(filename, ctx, rendered, holes, forceCacheClear, source)
	}
	if plush.IsHoleRender(ctx) {
		observeOutputSize(bytecode, ctx, options, rootSize, observation)
		return rendered, nil
	}

	holes = plush.RenderPunchHolesConcurrentlyWith(holes, ctx, Render)
	filled, err := plush.FillPunchHoles(rendered, holes)
	if err != nil {
		return "", err
	}
	observeOutputSize(bytecode, ctx, options, rootSize, observation)
	return filled, nil
}

func bytecodeCanUsePunchHoleCache(bytecode *compiler.Bytecode) bool {
	return bytecode == nil || !bytecode.HasContextWrites
}
