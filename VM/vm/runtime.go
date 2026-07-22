package vm

import (
	"fmt"
	"sync"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/parser"
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

func newWithContext(bytecode *compiler.Bytecode, ctx hctx.Context, pooled bool) *VM {
	if ctx == nil {
		ctx = plush.NewContext()
	}

	mainFn := &object.CompiledFunction{
		Instructions:   bytecode.Instructions,
		CallNames:      bytecode.CallNames,
		LocalNames:     bytecode.LocalNames,
		LineNumbers:    bytecode.LineNumbers,
		Properties:     bytecode.Properties,
		PropertyCaches: bytecode.PropertyCaches,
		CallCaches:     bytecode.CallCaches,
		NumLocals:      bytecode.NumLocals,
	}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := newFrame(mainClosure, 0, pooled)

	frames := borrowFrames(pooled)
	frames[0] = mainFrame

	holes := borrowHoles(pooled)
	globalSize := globalStoreSize(bytecode)

	machine := &VM{
		constants:   bytecode.Constants,
		stack:       borrowStack(pooled),
		sp:          mainFn.NumLocals,
		stackMax:    mainFn.NumLocals,
		globals:     borrowGlobals(globalSize, pooled),
		globalNames: bytecode.GlobalNames,
		frames:      frames,
		framesIndex: 1,
		ctx:         ctx,
		holes:       holes,
		pooled:      pooled,
		ownGlobals:  pooled,
		ownHoles:    pooled,
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
	if vm == nil || !vm.pooled {
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
				return bytecode.StaticOutput, nil
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
	if rendered, ok, err := tryRenderFastBytecode(cachedBytecode, ctx); ok || err != nil {
		if ok {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.FastPath = plush.RenderFastPathFast
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
		if cachedBytecode.HasHoles {
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

		return renderBytecodeVMWithState(cachedBytecode, ctx, filename, forceCacheClear, cacheSource)
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
				return bytecode.StaticOutput, nil
			}
			if rendered, ok, err := tryRenderFastBytecode(bytecode, ctx); ok || err != nil {
				if ok {
					plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
						d.VMBytecodeCache = plush.VMBytecodeCacheHit
						d.FastPath = plush.RenderFastPathFast
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
			return renderBytecodeVMWithState(bytecode, ctx, filename, forceCacheClear, cacheSource)
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

func renderSourceCachedBytecode(source string, ctx hctx.Context, bytecode *compiler.Bytecode) (string, error) {
	updateBytecodeDiagnostics(ctx, bytecode)
	if bytecode.Static {
		plush.UpdateRenderDiagnosticsForTemplate(ctx, "", func(d *plush.RenderDiagnostics) {
			d.VMBytecodeCache = plush.VMBytecodeCacheHitSource
			d.FastPath = plush.RenderFastPathStatic
		})
		return bytecode.StaticOutput, nil
	}
	plush.UpdateRenderDiagnosticsForTemplate(ctx, "", func(d *plush.RenderDiagnostics) {
		d.VMBytecodeCache = plush.VMBytecodeCacheHitSource
	})
	if shouldFallbackGenericBytecode(bytecode) {
		return renderInterpreterFallback(source, ctx, "")
	}
	if rendered, ok, err := tryRenderFastBytecode(bytecode, ctx); ok || err != nil {
		if ok {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, "", func(d *plush.RenderDiagnostics) {
				d.FastPath = plush.RenderFastPathFast
			})
		}
		return rendered, err
	}
	if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
		defer restorePartial()
	}
	return renderBytecodeVMWithState(bytecode, ctx, "", false, source)
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
		return bytecode.StaticOutput, nil
	}
	if rendered, ok, err := tryRenderFastBytecode(bytecode, ctx); ok || err != nil {
		if ok {
			plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
				d.FastPath = plush.RenderFastPathFast
			})
		}
		return rendered, err
	}
	if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
		defer restorePartial()
	}

	forceCacheClear := false
	if bytecode == nil || bytecode.HasHoles {
		var cached string
		var ok bool
		filename, forceCacheClear, cached, ok = punchHoleCacheState(ctx)
		if ok {
			return cached, nil
		}
	}
	return renderBytecodeVMWithState(bytecode, ctx, filename, forceCacheClear, "")
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
		return bytecode.StaticOutput, nil
	}
	if rendered, ok, err := tryRenderFastBytecode(bytecode, ctx); ok || err != nil {
		return rendered, err
	}
	return renderBytecodeVMWithState(bytecode, ctx, filename, forceCacheClear, source)
}

func renderBytecodeVMWithState(bytecode *compiler.Bytecode, ctx hctx.Context, filename string, forceCacheClear bool, source string) (string, error) {
	machine := newPooledWithContext(bytecode, ctx)
	if err := machine.Run(); err != nil {
		defer machine.Release()
		return "", machine.wrapRuntimeError(err)
	}

	rendered := machine.Rendered()
	if bytecode == nil || !bytecode.HasHoles {
		machine.Release()
		return rendered, nil
	}
	holes := machine.PunchHoles()
	machine.Release()
	if !plush.IsPlushTemplateFile(filename) || len(holes) == 0 {
		return rendered, nil
	}

	holes = plush.FinalizePunchHolePositions(rendered, holes)
	plush.CachePunchHoleSkeletonWithSource(filename, ctx, rendered, holes, forceCacheClear, source)
	if plush.IsHoleRender(ctx) {
		return rendered, nil
	}

	holes = plush.RenderPunchHolesConcurrentlyWith(holes, ctx, Render)
	return plush.FillPunchHoles(rendered, holes)
}
