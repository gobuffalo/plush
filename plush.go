package plush

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
)

// DefaultTimeFormat is the default way of formatting a time.Time type.
// This a **GLOBAL** variable, so if you change it, it will change for
// templates rendered through the `plush` package. If you want to set a
// specific time format for a particular call to `Render` you can set
// the `TIME_FORMAT` in the context.
//
/*
	ctx.Set("TIME_FORMAT", "2006-02-Jan")
	s, err = Render(input, ctx)
*/
var DefaultTimeFormat = "January 02, 2006 15:04:05 -0700"
var PunchHoleCacheLifetime = 1 * time.Minute
var cacheEnabled bool
var holeTemplateFileKey = "__plush_internal_hole_render_key_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var interpreterPartialRenderKey = "__plush_internal_interpreter_partial_render_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var errClearCache error = errors.New("template recently cached, skipping")
var punchHoleConcurrencyLimit atomic.Int64

var templateCacheBackend TemplateCache

func init() {
	punchHoleConcurrencyLimit.Store(int64(runtime.GOMAXPROCS(0)))
}

func SetPunchHoleConcurrencyLimit(limit int) int {
	previous := int(punchHoleConcurrencyLimit.Swap(int64(limit)))
	return previous
}

func GetPunchHoleConcurrencyLimit() int {
	return int(punchHoleConcurrencyLimit.Load())
}

func PlushCacheSetup(ts TemplateCache) {
	cacheEnabled = true
	templateCacheBackend = ts
}

// BuffaloRenderer implements the render.TemplateEngine interface allowing velvet to be used as a template engine
// for Buffalo
func BuffaloRenderer(input string, data map[string]interface{}, helpers map[string]interface{}) (string, error) {
	return BuffaloRendererWithContext(input, data, helpers, nil)
}

// BuffaloRendererWithContext is BuffaloRenderer with an optional context
// configuration hook. The hook runs after data/helpers are loaded and before
// rendering, so callers can attach per-render options such as VM fast helpers.
func BuffaloRendererWithContext(input string, data map[string]interface{}, helpers map[string]interface{}, configure func(*Context)) (string, error) {
	if data == nil {
		data = make(map[string]interface{})
	}
	for k, v := range helpers {
		data[k] = v
	}
	ctx := NewContextWith(data)
	if configure != nil {
		configure(ctx)
	}
	defer func() {
		if data != nil {
			for k := range ctx.data.localInterner.stringToID {
				data[k] = ctx.Value(k)
			}
		}
	}()
	return Render(input, ctx)
}

// Parse an input string and return a Template, and caches the parsed template.
func Parse(input ...string) (*Template, error) {
	if templateCacheBackend == nil || !cacheEnabled || len(input) == 1 || len(input) > 2 {
		return NewTemplate(input[0])
	}

	filename := input[1]
	sourceHash := templateSourceHash(preprocessTrimTags(input[0]))
	var astKey string
	isPlushFile := isFilePlush(filename)
	if isPlushFile {

		astKey = GenerateASTKey(filename)
	}
	if filename != "" && templateCacheBackend != nil && isPlushFile {
		t, ok := templateCacheBackend.Get(astKey)
		if ok && templateSourceMatches(t, sourceHash) {
			if t.Program != nil {
				return cachedParseTemplate(t), nil
			}
			if t.Input != "" {
				parsed, err := NewTemplate(t.Input)
				if err != nil {
					return parsed, err
				}
				astTemplate := &Template{
					Program:    parsed.Program,
					VMBytecode: t.VMBytecode,
					SourceHash: sourceHash,
					IsCache:    false,
				}
				templateCacheBackend.Set(astKey, astTemplate)
				return cachedParseTemplate(astTemplate), nil
			}
		}
	}

	t, err := NewTemplate(input[0])
	if err != nil {
		return t, err
	}
	// Cache the AST
	if cacheEnabled && templateCacheBackend != nil && filename != "" && isPlushFile {
		astTemplate := &Template{
			Program:    t.Program,
			SourceHash: sourceHash,
			IsCache:    false,
		}
		if cached, ok := templateCacheBackend.Get(astKey); ok && cached != nil && templateSourceMatches(cached, sourceHash) {
			astTemplate.VMBytecode = cached.VMBytecode
		}
		templateCacheBackend.Set(astKey, astTemplate)
	}
	return t, nil
}

func cachedParseTemplate(t *Template) *Template {
	return &Template{
		Input:      t.Input,
		Program:    t.Program,
		VMBytecode: t.VMBytecode,
		SourceHash: t.SourceHash,
		IsCache:    true,
	}
}

// RenderWithBudget renders a template and enforces a work-unit limit.
// Returns ErrBudgetExceeded if the template exhausts the budget.
// Existing Render() is completely unchanged.
func RenderWithBudget(input string, limit int64, ctx *Context) (string, error) {
	b := NewBudget(limit)
	ctx.WithBudget(b)
	return Render(input, ctx)
}

// RenderWithBudgetConfig renders with a fully custom cost configuration.
func RenderWithBudgetConfig(input string, limit int64, costs BudgetCosts, ctx *Context) (string, error) {
	b := NewBudgetWithCosts(limit, costs)
	ctx.WithBudget(b)
	return Render(input, ctx)
}

func isHole(ctx hctx.Context) bool {
	if c, ok := ctx.(*Context); ok {
		return isHoleContext(c)
	}
	if ctx.Value(holeTemplateFileKey) == nil {
		return false
	}
	if ctx.Value(meta.TemplateFileKey) == nil {
		return false
	}

	ss, _ := ctx.Value(holeTemplateFileKey).(string)
	ss2, _ := ctx.Value(meta.TemplateFileKey).(string)
	return ss2 == ss

}

func IsHoleRender(ctx hctx.Context) bool {
	return isHole(ctx)
}

func PunchHoleTemplateFilename(ctx hctx.Context) string {
	if c, ok := ctx.(*Context); ok {
		return punchHoleTemplateFilenameContext(c)
	}
	if isHole(ctx) || ctx.Value(meta.TemplateFileKey) == nil {
		return ""
	}
	rawFilename, ok := ctx.Value(meta.TemplateFileKey).(string)
	if !ok {
		return ""
	}
	return cleanFilePath(rawFilename)
}

func isHoleContext(ctx *Context) bool {
	if ctx == nil {
		return false
	}
	ctx.moot.RLock()
	defer ctx.moot.RUnlock()

	holeFile, ok := ctx.data.Resolve(holeTemplateFileKey)
	if !ok || holeFile == nil {
		return false
	}
	templateFile, ok := ctx.data.Resolve(meta.TemplateFileKey)
	if !ok || templateFile == nil {
		return false
	}

	holeName, _ := holeFile.(string)
	templateName, _ := templateFile.(string)
	return templateName == holeName
}

func punchHoleTemplateFilenameContext(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	ctx.moot.RLock()
	defer ctx.moot.RUnlock()

	templateFile, templateOK := ctx.data.Resolve(meta.TemplateFileKey)
	if !templateOK || templateFile == nil {
		return ""
	}
	holeFile, holeOK := ctx.data.Resolve(holeTemplateFileKey)
	if holeOK && holeFile != nil {
		holeName, _ := holeFile.(string)
		templateName, _ := templateFile.(string)
		if templateName == holeName {
			return ""
		}
	}

	rawFilename, ok := templateFile.(string)
	if !ok {
		return ""
	}
	return cleanFilePath(rawFilename)
}

func IsPlushTemplateFile(filename string) bool {
	return isFilePlush(filename)
}

func IsVMBytecodeCacheableTemplateFile(filename string) bool {
	return isVMBytecodeCacheableFile(filename)
}

func RenderFromPunchHoleCache(filename string, ctx hctx.Context) (string, error) {
	return RenderFromPunchHoleCacheWithSource(filename, "", ctx)
}

func RenderFromPunchHoleCacheWithSource(filename string, source string, ctx hctx.Context) (string, error) {
	return renderFromCache(filename, source, ctx)
}

func IsPunchHoleCacheExpired(err error) bool {
	return errors.Is(err, errClearCache)
}

func CachePunchHoleSkeleton(filename string, ctx hctx.Context, skeleton string, holes []HoleMarker, force bool) {
	CachePunchHoleSkeletonWithSource(filename, ctx, skeleton, holes, force, "")
}

func CachePunchHoleSkeletonWithSource(filename string, ctx hctx.Context, skeleton string, holes []HoleMarker, force bool, source string) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isFilePlush(filename) || len(holes) == 0 {
		return
	}
	sourceHash := templateSourceCacheHash(source)
	if !force {
		if cached, ok := templateCacheBackend.Get(generateFullKeyFromCleanFilename(filename, ctx)); ok && templateSourceMatches(cached, sourceHash) {
			return
		}
	}
	astKey := GenerateASTKeyFromCleanFilename(filename)
	if _, ok := templateCacheBackend.Get(astKey); !ok {
		templateCacheBackend.Set(astKey, &Template{IsCache: false})
	}
	templateCacheBackend.Set(generateFullKeyFromCleanFilename(filename, ctx), &Template{
		Skeleton:   skeleton,
		PunchHole:  holesCopy(holes),
		SourceHash: sourceHash,
		IsCache:    false,
		LastCached: time.Now(),
	})
}

func FinalizePunchHolePositions(rendered string, holes []HoleMarker) []HoleMarker {
	out := holesCopy(holes)
	for i := range out {
		hole := &out[i]
		if hole.start == -1 && hole.end == -1 {
			pos := strings.Index(rendered, hole.marker_name)
			if pos != -1 {
				hole.start = pos
				hole.end = pos + len(hole.marker_name)
			}
		}
	}
	return out
}

func RenderPunchHolesConcurrently(holes []HoleMarker, ctx hctx.Context) []HoleMarker {
	return renderHolesConcurrently(holesCopy(holes), ctx)
}

func RenderPunchHolesConcurrentlyWith(holes []HoleMarker, ctx hctx.Context, renderer func(string, hctx.Context) (string, error)) []HoleMarker {
	return renderHolesConcurrentlyWith(holesCopy(holes), ctx, renderer)
}

func FillPunchHoles(rendered string, holes []HoleMarker) (string, error) {
	return fillHoles(rendered, holes)
}

// Render a string using the given context.
func Render(input string, ctx hctx.Context) (string, error) {
	if ctx == nil {
		ctx = NewContext()
	}
	restoreDiagnosticsRoot := SetRenderDiagnosticsRootActive(ctx, true)
	if restoreDiagnosticsRoot != nil {
		defer restoreDiagnosticsRoot()
	}
	start := time.Now()
	mode := RenderModeNameInterpreter
	if GetRenderMode() == RenderModeVM {
		mode = RenderModeNameVM
	}
	filename := PunchHoleTemplateFilename(ctx)
	UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *RenderDiagnostics) {
		d.Mode = mode
		d.TemplateFilename = filename
		if mode == RenderModeNameInterpreter {
			d.VMBytecodeCache = VMBytecodeCacheDisabled
			if d.FastPath == "" {
				d.FastPath = RenderFastPathGeneric
			}
		}
		if d.PunchHoleCache == "" {
			d.PunchHoleCache = PunchHoleCacheDisabled
		}
	})
	defer func() {
		UpdateRenderDiagnostics(ctx, func(d *RenderDiagnostics) {
			d.Mode = mode
			if d.TemplateFilename == "" {
				d.TemplateFilename = filename
			}
			if mode == RenderModeNameInterpreter {
				d.VMBytecodeCache = VMBytecodeCacheDisabled
			}
			d.EngineDuration += time.Since(start)
		})
	}()

	if GetRenderMode() == RenderModeVM {
		renderer, ok := registeredVMRenderer()
		if !ok {
			return "", fmt.Errorf("%w: import github.com/gobuffalo/plush/v5/VM/plush to register it", ErrVMRendererNotRegistered)
		}
		return renderer(input, ctx)
	}
	return renderInterpreter(input, ctx)
}

func renderInterpreter(input string, ctx hctx.Context) (string, error) {
	// Extract filename from context if we're not in a hole rendering pass.
	// The filename is used for template caching - only main templates (not holes) should use cache.
	filename := PunchHoleTemplateFilename(ctx)
	forceCacheClear := false
	// Try to render from cache if conditions are met:
	// - Not in hole rendering pass (prevents infinite recursion)
	// - Cache is enabled and backend is available
	// - Template has a filename for cache key
	if filename != "" {
		cacheT, cacheErr := renderFromCache(filename, input, ctx)
		if cacheErr == nil {
			return cacheT, nil
		} else if cacheErr == errClearCache {
			forceCacheClear = true
		}
	}

	t, err := Parse(input, filename)
	if err != nil {
		return "", err
	}
	isPlushFile := isFilePlush(filename)

	// Execute template to get skeleton with hole markers
	s, holeMarkers, err := t.Exec(ctx)
	if err != nil {
		return "", err
	}
	if !isPlushFile {
		return s, err
	}
	//Don't bloat the cache with Skeletons that have no holes
	// If there are holes, store skeleton and hole markers in the template for future use
	// This is used when caching the fully rendered template with holes filled in
	if len(holeMarkers) > 0 {
		t.Skeleton = s
		t.PunchHole = holeMarkers
	}

	if (!t.IsCache || forceCacheClear) && cacheEnabled {
		defer func() {
			if templateCacheBackend != nil && filename != "" && isPlushFile && len(holeMarkers) > 0 {
				fullKey := generateFullKeyFromCleanFilename(filename, ctx)
				cacheableTemplate := &Template{
					Skeleton:   t.Skeleton,
					PunchHole:  holesCopy(t.PunchHole),
					SourceHash: templateSourceHash(preprocessTrimTags(input)),
					IsCache:    false,
					LastCached: time.Now(),
				}
				templateCacheBackend.Set(fullKey, cacheableTemplate)
			}
		}()
	}

	// If we have holes and this is the main render pass (not hole rendering),
	// render holes concurrently and fill them into the skeleton
	if !isHole(ctx) && len(t.PunchHole) > 0 {
		hc := renderHolesConcurrently(t.PunchHole, ctx)
		return fillHoles(s, hc)
	}

	// Return skeleton as-is (either no holes or we're in hole rendering pass)
	return s, nil
}

func RenderInterpreter(input string, ctx hctx.Context) (string, error) {
	if ctx == nil {
		ctx = NewContext()
	}
	restore := forceInterpreterPartialRender(ctx)
	defer restore()
	return renderInterpreter(input, ctx)
}

func forceInterpreterPartialRender(ctx hctx.Context) func() {
	if ctx == nil {
		return func() {}
	}
	previous := ctx.Value(interpreterPartialRenderKey)
	ctx.Set(interpreterPartialRenderKey, true)
	return func() {
		if previous == nil {
			ctx.Set(interpreterPartialRenderKey, false)
			return
		}
		ctx.Set(interpreterPartialRenderKey, previous)
	}
}

func useInterpreterPartialRender(ctx hctx.Context) bool {
	if ctx == nil {
		return false
	}
	useInterpreter, _ := ctx.Value(interpreterPartialRenderKey).(bool)
	return useInterpreter
}

// InterpreterPartialRenderEnabled reports whether partials should stay on the
// interpreter path for the current render.
func InterpreterPartialRenderEnabled(ctx hctx.Context) bool {
	return useInterpreterPartialRender(ctx)
}

// fillHoles replaces all markers in the rendered string with their rendered content using stored positions.
func fillHoles(rendered string, holes []HoleMarker) (string, error) {

	var sb strings.Builder
	last := 0
	for _, pos := range holes {
		if pos.err != nil {
			return "", pos.err
		}
		sb.WriteString(rendered[last:pos.start])
		sb.WriteString(pos.content)
		last = pos.end
	}
	sb.WriteString(rendered[last:])
	return sb.String(), nil
}

func renderHolesConcurrently(holes []HoleMarker, ctx hctx.Context) []HoleMarker {
	if useInterpreterPartialRender(ctx) {
		return renderHolesConcurrentlyWith(holes, ctx, RenderInterpreter)
	}
	return renderHolesConcurrentlyWith(holes, ctx, Render)
}

func renderHolesConcurrentlyWith(holes []HoleMarker, ctx hctx.Context, renderer func(string, hctx.Context) (string, error)) []HoleMarker {
	if len(holes) == 0 {
		return holes
	}
	if renderer == nil {
		renderer = Render
	}

	holeCtx := ctx.New()
	var currentfileName string
	if holeCtx.Value(meta.TemplateFileKey) != nil {

		tempF, _ := holeCtx.Value(meta.TemplateFileKey).(string)
		if tempF != "" {
			currentfileName = filepath.Base(tempF)
		}
	}
	holeCtx.Set(holeTemplateFileKey, holeCtx.Value(meta.TemplateFileKey))

	workerCount := GetPunchHoleConcurrencyLimit()
	if workerCount <= 0 || workerCount > len(holes) {
		workerCount = len(holes)
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer wg.Done()
			for k := range jobs {
				childCtx := holeCtx.New()
				content, err := renderer(holes[k].input, childCtx)
				if err != nil {
					content = err.Error() + " in " + currentfileName
				}
				holes[k].content = content
			}
		}()
	}
	for k := range holes {
		jobs <- k
	}
	close(jobs)
	wg.Wait()
	return holes
}

func RenderR(input io.Reader, ctx hctx.Context) (string, error) {
	b, err := io.ReadAll(input)
	if err != nil {
		return "", err
	}
	return Render(string(b), ctx)
}

// RunScript allows for "pure" plush scripts to be executed.
func RunScript(input string, ctx hctx.Context) error {
	input = "<% " + input + "%>"

	ctx = ctx.New()
	ctx.Set("print", func(i interface{}) {
		fmt.Print(i)
	})
	ctx.Set("println", func(i interface{}) {
		fmt.Println(i)
	})

	_, err := Render(input, ctx)
	return err
}

type interfaceable interface {
	Interface() interface{}
}

// HTMLer generates HTML source
type HTMLer interface {
	HTML() template.HTML
}

func holesCopy(holes []HoleMarker) []HoleMarker {
	holesCopy := make([]HoleMarker, len(holes))
	for i := range holes {
		holesCopy[i] = holes[i]
		holesCopy[i].content = ""
		holesCopy[i].err = nil
	}
	return holesCopy
}

// This function tries to get the template from the cache if it exists
// It only works if we are not in a hole rendering pass, and if we have a filename
// and if cache is enabled and if we have a cache backend
// If we are in a hole rendering pass, we should not use the cache.
// If we are in the first pass, and there is a filename, we should use the cache.
// If there is no filename, we should not use the cache.
// If cache is disabled, we should not use the cache.
// If there is no templateCacheBackend, we should not use the cache.
func renderFromCache(filename string, source string, ctx hctx.Context) (string, error) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || isHole(ctx) {
		return "", errors.New("cache not available")
	}

	sourceHash := templateSourceCacheHash(preprocessTrimTags(source))
	astKey := GenerateASTKeyFromCleanFilename(filename)
	astTemplate, astExists := templateCacheBackend.Get(astKey)
	if !astExists || !templateSourceMatches(astTemplate, sourceHash) {
		return "", errors.New("AST not cached")
	}

	fullKey := generateFullKeyFromCleanFilename(filename, ctx)
	inCacheTemplate, inCache := templateCacheBackend.Get(fullKey)
	if inCache &&
		inCacheTemplate != nil &&
		templateSourceMatches(inCacheTemplate, sourceHash) &&
		inCacheTemplate.Skeleton != "" &&
		len(inCacheTemplate.PunchHole) > 0 {
		if time.Since(inCacheTemplate.LastCached) > PunchHoleCacheLifetime {
			return "", errClearCache
		}
		hc := holesCopy(inCacheTemplate.PunchHole)
		hc = renderHolesConcurrently(hc, ctx)
		return fillHoles(inCacheTemplate.Skeleton, hc)
	}
	return "", errors.New("no cached template found")
}

func isFilePlush(filename string) bool {
	if len(filename) < 6 {
		return false
	}
	// Check for .plush.html first (longer suffix)
	if len(filename) >= 11 && filename[len(filename)-11:] == ".plush.html" {
		return true
	}
	// Check for .plush
	return filename[len(filename)-6:] == ".plush"
}

func isVMBytecodeCacheableFile(filename string) bool {
	if isFilePlush(filename) {
		return true
	}
	return strings.HasSuffix(filename, ".html")
}
