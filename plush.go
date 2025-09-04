package plush

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
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

// Internal keys with unique prefixes to prevent collision with user variables
// These keys are used internally by the plush engine and should not be set by users. We ensure this by adding a unique timestamp.
var holeTemplateFileKey = "__plush_internal_hole_render_key_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var TemplateFileKey = "__plush_internal_template_file_key_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var cacheEnabled bool

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
	if filename != "" && templateCacheBackend != nil {
		t, ok := templateCacheBackend.Get(filename)
		if ok {
			return t, nil
		}
	}

	t, err := NewTemplate(input[0])
	if err != nil {
		return t, err
	}
	return t, nil
}

// Render a string using the given context.
func Render(input string, ctx hctx.Context) (string, error) {
	var filename string

	// Extract filename from context if we're not in a hole rendering pass.
	// The filename is used for template caching - only main templates (not holes) should use cache.
	if ctx.Value(holeTemplateFileKey) == nil && ctx.Value(TemplateFileKey) != nil {
		filename, _ = ctx.Value(TemplateFileKey).(string)
	}

	// Try to render from cache if conditions are met:
	// - Not in hole rendering pass (prevents infinite recursion)
	// - Cache is enabled and backend is available
	// - Template has a filename for cache key
	if ctx.Value(holeTemplateFileKey) == nil {
		cacheT, cacheErr := renderFromCache(filename, ctx)
		if cacheErr == nil {
			return cacheT, nil
		}
	}

	// Parse template (with caching if enabled)
	t, err := Parse(input, filename)
	if err != nil {
		return "", err
	}

	// Execute template to get skeleton with hole markers
	s, holeMarkers, err := t.Exec(ctx)
	if err != nil {
		return "", err
	}
	t.skeleton = s
	t.punchHole = holeMarkers
	// Cache the template after successful execution (deferred to ensure caching even if hole rendering fails)
	defer func() {
		if cacheEnabled && templateCacheBackend != nil && filename != "" {
			t.punchHole = holesCopy(t.punchHole)
			templateCacheBackend.Set(filename, t)
		}
	}()

	// If we have holes and this is the main render pass (not hole rendering),
	// render holes concurrently and fill them into the skeleton
	if ctx.Value(holeTemplateFileKey) == nil && len(t.punchHole) > 0 {
		hc := renderHolesConcurrently(t.punchHole, ctx)
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
	var wg sync.WaitGroup
	wg.Add(len(holes))

	octx := ctx.New()

	octx.Set(holeTemplateFileKey, true)
	for k, hole := range holes {

		go func(k int, holeCtx hctx.Context, h HoleMarker) {
			defer wg.Done()

			content, err := Render(h.input, holeCtx)
			if err != nil {
				holes[k].err = err
				return
			}
			holes[k].content = content
		}(k, octx.New(), hole)
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

	if filename != "" && cacheEnabled && templateCacheBackend != nil && ctx.Value(holeTemplateFileKey) == nil {
		if filename != "" {
			inCacheTemplate, inCache := templateCacheBackend.Get(filename)
			if inCache &&
				inCacheTemplate != nil &&
				inCacheTemplate.skeleton != "" &&
				len(inCacheTemplate.punchHole) > 0 {
				hc := holesCopy(inCacheTemplate.punchHole)
				hc = renderHolesConcurrently(hc, ctx)
				return fillHoles(inCacheTemplate.skeleton, hc)
			}
		}
	}
	return "", errors.New("no cached template found")
}
