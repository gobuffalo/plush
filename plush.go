package plush

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
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
var errClearCache error = errors.New("template recently cached, skipping")

var templateCacheBackend TemplateCache

func PlushCacheSetup(ts TemplateCache) {
	cacheEnabled = true
	templateCacheBackend = ts
}

// BuffaloRenderer implements the render.TemplateEngine interface allowing velvet to be used as a template engine
// for Buffalo
func BuffaloRenderer(input string, data map[string]interface{}, helpers map[string]interface{}) (string, error) {
	if data == nil {
		data = make(map[string]interface{})
	}
	for k, v := range helpers {
		data[k] = v
	}
	ctx := NewContextWith(data)
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
	var astKey string
	isPlushFile := isFilePlush(filename)
	if isPlushFile {

		astKey = GenerateASTKey(filename)
	}
	if filename != "" && templateCacheBackend != nil && isPlushFile {
		t, ok := templateCacheBackend.Get(astKey)
		if ok {
			cloned := &Template{
				Program: t.Program,
				IsCache: true,
			}
			return cloned, nil
		}
	}

	t, err := NewTemplate(input[0])
	if err != nil {
		return t, err
	}
	// Cache the AST
	if cacheEnabled && templateCacheBackend != nil && filename != "" && isPlushFile {
		astTemplate := &Template{
			Program: t.Program,
			IsCache: false,
		}
		templateCacheBackend.Set(astKey, astTemplate)
	}
	return t, nil
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

// Render a string using the given context.
func Render(input string, ctx hctx.Context) (string, error) {
	var filename string

	// Extract filename from context if we're not in a hole rendering pass.
	// The filename is used for template caching - only main templates (not holes) should use cache.
	if !isHole(ctx) && ctx.Value(meta.TemplateFileKey) != nil {
		if rawFilename, ok := ctx.Value(meta.TemplateFileKey).(string); ok {
			filename = cleanFilePath(rawFilename) // ✅ Clean once here
		}
	}
	forceCacheClear := false
	// Try to render from cache if conditions are met:
	// - Not in hole rendering pass (prevents infinite recursion)
	// - Cache is enabled and backend is available
	// - Template has a filename for cache key
	if !isHole(ctx) && filename != "" {
		cacheT, cacheErr := renderFromCache(filename, ctx)
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
				fullKey := generateFullKey(filename, ctx)
				cacheableTemplate := &Template{
					Skeleton:   t.Skeleton,
					PunchHole:  holesCopy(t.PunchHole),
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
	if len(holes) == 0 {
		return holes
	}

	var wg sync.WaitGroup
	wg.Add(len(holes))

	holeCtx := ctx.New()

	octx := holeCtx.(*Context)
	defer func() {
		ctx = octx
	}()
	var currentfileName string
	if holeCtx.Value(meta.TemplateFileKey) != nil {

		tempF, _ := holeCtx.Value(meta.TemplateFileKey).(string)
		if tempF != "" {
			currentfileName = filepath.Base(tempF)
		}
	}
	holeCtx.Set(holeTemplateFileKey, holeCtx.Value(meta.TemplateFileKey))
	for k, hole := range holes {
		go func(k int, childCtx hctx.Context, h HoleMarker) {
			defer wg.Done()

			content, err := Render(h.input, childCtx)
			if err != nil {
				content = err.Error() + " in " + currentfileName

			}
			holes[k].content = content
		}(k, holeCtx.New(), hole)
	}
	wg.Wait()
	return holes
}

func RenderR(input io.Reader, ctx hctx.Context) (string, error) {
	b, err := ioutil.ReadAll(input)
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
func renderFromCache(filename string, ctx hctx.Context) (string, error) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || isHole(ctx) {
		return "", errors.New("cache not available")
	}

	astKey := GenerateASTKey(filename)
	_, astExists := templateCacheBackend.Get(astKey)
	if !astExists {
		return "", errors.New("AST not cached")
	}

	fullKey := generateFullKey(filename, ctx)
	inCacheTemplate, inCache := templateCacheBackend.Get(fullKey)
	if inCache &&
		inCacheTemplate != nil &&
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
