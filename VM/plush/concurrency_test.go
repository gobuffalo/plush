package plush_test

import (
	"fmt"
	"sync"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func Test_Phase_15_Concurrent_VM_Renders_With_Separate_Contexts(t *testing.T) {
	input := `<% let local = name %><%= name %>:<%= local %>:<%= for (i, item) in items { %><%= name %>-<%= i %>-<%= item %>;<% } %>`

	runConcurrentChecks(t, 32, func(i int) error {
		name := fmt.Sprintf("n%d", i)
		out, err := vmplush.Render(input, rootplush.NewContextWith(map[string]interface{}{
			"name":  name,
			"items": []string{"a", "b"},
		}))
		if err != nil {
			return err
		}
		expected := fmt.Sprintf("%s:%s:%s-0-a;%s-1-b;", name, name, name, name)
		if out != expected {
			return fmt.Errorf("render %d output mismatch: want %q, got %q", i, expected, out)
		}
		return nil
	})
}

func Test_Phase_15_Concurrent_Compiled_Template_Shared_Bytecode(t *testing.T) {
	tmpl, err := vmplush.Compile(`<% let local = name %><%= local %>|<%= suffix %>|<%= for (i, item) in items { %><%= local %>:<%= item %>;<% } %>`)
	require.NoError(t, err)

	runConcurrentChecks(t, 32, func(i int) error {
		name := fmt.Sprintf("user%d", i)
		suffix := fmt.Sprintf("s%d", i)
		out, err := tmpl.Render(rootplush.NewContextWith(map[string]interface{}{
			"name":   name,
			"suffix": suffix,
			"items":  []string{"x", "y"},
		}))
		if err != nil {
			return err
		}
		expected := fmt.Sprintf("%s|%s|%s:x;%s:y;", name, suffix, name, name)
		if out != expected {
			return fmt.Errorf("compiled render %d output mismatch: want %q, got %q", i, expected, out)
		}
		return nil
	})
}

func Test_Phase_15_Repeated_Compiled_Template_Does_Not_Leak_Globals_Or_Locals(t *testing.T) {
	tmpl, err := vmplush.Compile(`<% let global = name %><% let make = fn(value) { let local = value + suffix; return local } %><%= global %>|<%= make(name) %>`)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("n%d", i)
		suffix := fmt.Sprintf("-s%d", i)
		out, err := tmpl.Render(rootplush.NewContextWith(map[string]interface{}{
			"name":   name,
			"suffix": suffix,
		}))
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%s|%s%s", name, name, suffix), out)
	}
}

func Test_Phase_15_Concurrent_VM_Hole_Rendering(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	input := `<%H value %>|<%= value %>`

	runConcurrentChecks(t, 16, func(i int) error {
		value := fmt.Sprintf("hole%d", i)
		ctx := rootplush.NewContextWith(map[string]interface{}{
			"value": value,
		})
		ctx.Set(meta.TemplateFileKey, fmt.Sprintf("phase15-holes-%d.plush", i))

		out, err := vmplush.Render(input, ctx)
		if err != nil {
			return err
		}
		expected := fmt.Sprintf("%s|%s", value, value)
		if out != expected {
			return fmt.Errorf("hole render %d output mismatch: want %q, got %q", i, expected, out)
		}
		return nil
	})
}

func Test_Phase_15_Concurrent_Root_Render_Mode_VM_Bytecode_Cache_Access(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	input := `<%= name %>|<%= for (i, item) in items { %><%= name %>:<%= item %>;<% } %>`

	runConcurrentChecks(t, 32, func(i int) error {
		name := fmt.Sprintf("cached%d", i)
		ctx := rootplush.NewContextWith(map[string]interface{}{
			"name":  name,
			"items": []string{"a", "b"},
		})
		ctx.Set(meta.TemplateFileKey, "phase15-cache.plush")

		out, err := rootplush.Render(input, ctx)
		if err != nil {
			return err
		}
		expected := fmt.Sprintf("%s|%s:a;%s:b;", name, name, name)
		if out != expected {
			return fmt.Errorf("cached render %d output mismatch: want %q, got %q", i, expected, out)
		}
		return nil
	})

	entry, ok := cache.Get(rootplush.GenerateASTKey("phase15-cache.plush"))
	require.True(t, ok)
	require.NotNil(t, entry.VMBytecode)
}

func runConcurrentChecks(t *testing.T, count int, fn func(int) error) {
	t.Helper()

	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- fn(i)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}
