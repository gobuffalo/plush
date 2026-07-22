package plush

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

const (
	RenderModeNameInterpreter = "interpreter"
	RenderModeNameVM          = "vm"

	VMBytecodeCacheDisabled        = "disabled"
	VMBytecodeCacheHit             = "hit"
	VMBytecodeCacheHitStatic       = "hit-static"
	VMBytecodeCacheHitSource       = "hit-source"
	VMBytecodeCacheMiss            = "miss"
	VMBytecodeCacheMissStore       = "miss-store"
	VMBytecodeCacheMissStoreSource = "miss-store-source"
	VMBytecodeCacheDirect          = "compiled-template"

	RenderFastPathStatic              = "static"
	RenderFastPathFast                = "fast"
	RenderFastPathGeneric             = "generic"
	RenderFastPathInterpreterFallback = "interpreter-fallback"

	PunchHoleCacheDisabled = "disabled"
	PunchHoleCacheHit      = "hit"
	PunchHoleCacheMiss     = "miss"
)

var renderDiagnosticsKey = "__plush_internal_render_diagnostics_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var renderVMHotspotDiagnosticsKey = "__plush_internal_render_vm_hotspot_diagnostics_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var renderDiagnosticsRootActiveKey = "__plush_internal_render_diagnostics_root_active_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"

type renderDiagnosticsState struct {
	mu          sync.Mutex
	diagnostics RenderDiagnostics
}

type RenderDiagnostics struct {
	Mode             string
	TemplateFilename string
	VMBytecodeCache  string
	FastPath         string
	FastRejectLine   int
	FastReject       string
	PunchHoleCache   string
	EngineDuration   time.Duration
	FastPlan         RenderFastPlanDiagnostics
	VMHotspots       RenderVMHotspotDiagnostics
}

type RenderFastPlanDiagnostics struct {
	Bindings       int
	Segments       int
	StaticSegments int
	NameSegments   int
	PropertyReads  int
	ValueWrites    int
	HelperCalls    int
	Conditionals   int
	Loops          int
	LoopParts      int
	Partials       int
	MaxDepth       int
	HelperNames    []string
	PartialNames   []string
}

type RenderVMHotspotDiagnostics struct {
	HelperCalls     int
	HelperDuration  time.Duration
	PartialCalls    int
	PartialDuration time.Duration
	Helpers         []RenderVMHotspot
	Partials        []RenderVMHotspot
}

type RenderVMHotspot struct {
	Name     string
	Calls    int
	Duration time.Duration
}

func (d RenderDiagnostics) EngineDurationMilliseconds() float64 {
	return float64(d.EngineDuration) / float64(time.Millisecond)
}

func (d RenderDiagnostics) VMHelperDurationMilliseconds() float64 {
	return float64(d.VMHotspots.HelperDuration) / float64(time.Millisecond)
}

func (d RenderDiagnostics) VMPartialDurationMilliseconds() float64 {
	return float64(d.VMHotspots.PartialDuration) / float64(time.Millisecond)
}

func (d RenderDiagnostics) FastPlanHelperNamesHeader() string {
	return strings.Join(d.FastPlan.HelperNames, ";")
}

func (d RenderDiagnostics) FastPlanPartialNamesHeader() string {
	return strings.Join(d.FastPlan.PartialNames, ";")
}

func (d RenderDiagnostics) VMHelperHotspotsHeader() string {
	return renderVMHotspotsHeader(d.VMHotspots.Helpers)
}

func (d RenderDiagnostics) VMPartialHotspotsHeader() string {
	return renderVMHotspotsHeader(d.VMHotspots.Partials)
}

func AddRenderDiagnosticVMHelperTiming(ctx hctx.Context, name string, duration time.Duration) {
	addRenderDiagnosticVMHotspot(ctx, name, duration, true)
}

func AddRenderDiagnosticVMPartialTiming(ctx hctx.Context, name string, duration time.Duration) {
	addRenderDiagnosticVMHotspot(ctx, name, duration, false)
}

func EnableRenderVMHotspotDiagnostics(ctx hctx.Context) {
	SetRenderVMHotspotDiagnostics(ctx, true)
}

func DisableRenderVMHotspotDiagnostics(ctx hctx.Context) {
	SetRenderVMHotspotDiagnostics(ctx, false)
}

func SetRenderVMHotspotDiagnostics(ctx hctx.Context, enabled bool) {
	if ctx == nil {
		return
	}
	ctx.Set(renderVMHotspotDiagnosticsKey, enabled)
	if enabled {
		renderDiagnosticsStateFromContext(ctx, true)
	}
}

func RenderVMHotspotDiagnosticsEnabled(ctx hctx.Context) bool {
	if ctx == nil {
		return false
	}
	enabled, _ := ctx.Value(renderVMHotspotDiagnosticsKey).(bool)
	return enabled
}

func RenderDiagnosticsFromContext(ctx hctx.Context) (RenderDiagnostics, bool) {
	if ctx == nil {
		return RenderDiagnostics{}, false
	}
	return renderDiagnosticsFromValue(ctx.Value(renderDiagnosticsKey))
}

func RenderDiagnosticsFromData(data map[string]interface{}) (RenderDiagnostics, bool) {
	if data == nil {
		return RenderDiagnostics{}, false
	}
	return renderDiagnosticsFromValue(data[renderDiagnosticsKey])
}

func SetRenderDiagnostics(ctx hctx.Context, diagnostics RenderDiagnostics) {
	if ctx == nil {
		return
	}
	if state := renderDiagnosticsStateFromContext(ctx, true); state != nil {
		state.mu.Lock()
		state.diagnostics = diagnostics
		state.mu.Unlock()
	}
}

func UpdateRenderDiagnostics(ctx hctx.Context, update func(*RenderDiagnostics)) {
	if ctx == nil || update == nil {
		return
	}
	state := renderDiagnosticsStateFromContext(ctx, true)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	update(&state.diagnostics)
}

func UpdateRenderDiagnosticsForTemplate(ctx hctx.Context, filename string, update func(*RenderDiagnostics)) {
	if update == nil {
		return
	}
	UpdateRenderDiagnostics(ctx, func(d *RenderDiagnostics) {
		if !renderDiagnosticsCanUpdateTemplate(d, filename) {
			return
		}
		update(d)
	})
}

func renderDiagnosticsCanUpdateTemplate(d *RenderDiagnostics, filename string) bool {
	if d == nil || d.TemplateFilename == "" {
		return true
	}
	return filename != "" && d.TemplateFilename == filename
}

func SetRenderDiagnosticsRootActive(ctx hctx.Context, active bool) func() {
	if ctx == nil {
		return nil
	}
	previous, _ := ctx.Value(renderDiagnosticsRootActiveKey).(bool)
	ctx.Set(renderDiagnosticsRootActiveKey, active)
	return func() {
		ctx.Set(renderDiagnosticsRootActiveKey, previous)
	}
}

func RenderDiagnosticsRootActive(ctx hctx.Context) bool {
	if ctx == nil {
		return false
	}
	active, _ := ctx.Value(renderDiagnosticsRootActiveKey).(bool)
	return active
}

func renderDiagnosticsStateFromContext(ctx hctx.Context, create bool) *renderDiagnosticsState {
	if ctx == nil {
		return nil
	}
	switch value := ctx.Value(renderDiagnosticsKey).(type) {
	case *renderDiagnosticsState:
		return value
	case RenderDiagnostics:
		state := &renderDiagnosticsState{diagnostics: value}
		if create {
			ctx.Set(renderDiagnosticsKey, state)
		}
		return state
	case *RenderDiagnostics:
		if value == nil {
			break
		}
		state := &renderDiagnosticsState{diagnostics: *value}
		if create {
			ctx.Set(renderDiagnosticsKey, state)
		}
		return state
	}
	if !create {
		return nil
	}
	state := &renderDiagnosticsState{}
	ctx.Set(renderDiagnosticsKey, state)
	return state
}

func renderDiagnosticsFromValue(value interface{}) (RenderDiagnostics, bool) {
	switch value := value.(type) {
	case *renderDiagnosticsState:
		if value == nil {
			return RenderDiagnostics{}, false
		}
		value.mu.Lock()
		defer value.mu.Unlock()
		return value.diagnostics, true
	case RenderDiagnostics:
		return value, true
	case *RenderDiagnostics:
		if value == nil {
			return RenderDiagnostics{}, false
		}
		return *value, true
	default:
		return RenderDiagnostics{}, false
	}
}

func addRenderDiagnosticVMHotspot(ctx hctx.Context, name string, duration time.Duration, helper bool) {
	if ctx == nil || duration < 0 || !RenderVMHotspotDiagnosticsEnabled(ctx) {
		return
	}
	if name == "" {
		name = "<anonymous>"
	}
	UpdateRenderDiagnostics(ctx, func(d *RenderDiagnostics) {
		if helper {
			d.VMHotspots.HelperCalls++
			d.VMHotspots.HelperDuration += duration
			addRenderVMHotspot(&d.VMHotspots.Helpers, name, duration)
			return
		}
		d.VMHotspots.PartialCalls++
		d.VMHotspots.PartialDuration += duration
		addRenderVMHotspot(&d.VMHotspots.Partials, name, duration)
	})
}

func addRenderVMHotspot(stats *[]RenderVMHotspot, name string, duration time.Duration) {
	for i := range *stats {
		if (*stats)[i].Name == name {
			(*stats)[i].Calls++
			(*stats)[i].Duration += duration
			return
		}
	}
	*stats = append(*stats, RenderVMHotspot{Name: name, Calls: 1, Duration: duration})
}

func renderVMHotspotsHeader(stats []RenderVMHotspot) string {
	if len(stats) == 0 {
		return ""
	}
	copyStats := append([]RenderVMHotspot(nil), stats...)
	sort.Slice(copyStats, func(i, j int) bool {
		if copyStats[i].Duration == copyStats[j].Duration {
			return copyStats[i].Name < copyStats[j].Name
		}
		return copyStats[i].Duration > copyStats[j].Duration
	})
	if len(copyStats) > 8 {
		copyStats = copyStats[:8]
	}
	parts := make([]string, 0, len(copyStats))
	for _, stat := range copyStats {
		parts = append(parts, fmt.Sprintf("%s:%d:%.3f", renderVMHotspotHeaderName(stat.Name), stat.Calls, float64(stat.Duration)/float64(time.Millisecond)))
	}
	return strings.Join(parts, ";")
}

func renderVMHotspotHeaderName(name string) string {
	name = strings.ReplaceAll(name, ",", "_")
	name = strings.ReplaceAll(name, ";", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "<anonymous>"
	}
	return name
}
