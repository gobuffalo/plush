package plush_test

import (
	"fmt"
	"html/template"
	"testing"

	"golang.org/x/sync/errgroup"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func Test_Context_Set(t *testing.T) {
	r := require.New(t)
	c := plush.NewContext()
	r.Nil(c.Value("foo"))
	c.Set("foo", "bar")
	r.NotNil(c.Value("foo"))
}

func Test_Context_Set_Concurrency(t *testing.T) {
	r := require.New(t)
	c := plush.NewContext()

	wg := errgroup.Group{}
	f := func() error {
		c.Set("a", "b")
		return nil
	}
	wg.Go(f)
	wg.Go(f)
	wg.Go(f)
	err := wg.Wait()
	r.NoError(err)
}

func Test_Context_Get(t *testing.T) {
	r := require.New(t)
	c := plush.NewContext()
	r.Nil(c.Value("foo"))
	c.Set("foo", "bar")
	r.Equal("bar", c.Value("foo"))
}

func Test_New_Sub_Context_Set(t *testing.T) {
	r := require.New(t)

	c := plush.NewContext()
	r.Nil(c.Value("foo"))

	sc := c.New()
	r.Nil(sc.Value("foo"))
	sc.Set("foo", "bar")
	r.Equal("bar", sc.Value("foo"))

	r.Nil(c.Value("foo"))
}

func Test_New_Sub_Context_Get(t *testing.T) {
	r := require.New(t)

	c := plush.NewContext()
	c.Set("foo", "bar")

	sc := c.New()
	r.Equal("bar", sc.Value("foo"))
}

func Test_Context_Override_Helper(t *testing.T) {
	r := require.New(t)
	c := plush.NewContext()
	c.Set("debug", func(i interface{}) template.HTML {
		return template.HTML("DEBUG")
	})
	s := c.Value("debug").(func(interface{}) template.HTML)(nil)
	r.Equal(template.HTML("DEBUG"), s)
}

func Test_New_Context_With_Override_Helper(t *testing.T) {
	r := require.New(t)
	c := plush.NewContextWith(map[string]interface{}{
		"debug": func(i interface{}) template.HTML {
			return template.HTML("DEBUG")
		},
	})

	s := c.Value("debug").(func(interface{}) template.HTML)(nil)
	r.Equal(template.HTML("DEBUG"), s)
}

func Test_New_Context_With_Nil_Helper_Override_Falls_Back_To_Global(t *testing.T) {
	r := require.New(t)
	c := plush.NewContextWith(map[string]interface{}{
		"partial": nil,
	})

	r.NotNil(c.Value("partial"))
}

func Test_New_Context_With_Bulk_Construction_Matches_Set_Behavior(t *testing.T) {
	r := require.New(t)
	c := plush.NewContextWith(map[string]interface{}{
		"name":  "Mido",
		"count": 3,
		"empty": nil,
	})

	r.Equal("Mido", c.Value("name"))
	r.Equal(3, c.Value("count"))
	r.Nil(c.Value("empty"))
	r.True(c.Has("name"))
	r.False(c.Has("empty"))

	id := c.InternID("name")
	value, ok := c.LookupID(id)
	r.True(ok)
	r.Equal("Mido", value)
}

func Test_Context_Intern_I_Ds_Are_Stable_Across_Root_Contexts(t *testing.T) {
	r := require.New(t)

	first := plush.NewContextWith(map[string]interface{}{
		"z":     true,
		"name":  "Mido",
		"title": "Engineer",
	})
	second := plush.NewContextWith(map[string]interface{}{
		"a":     true,
		"title": "Pilot",
		"name":  "Fry",
	})

	nameID := first.InternID("name")
	titleID := first.InternID("title")
	r.Equal(nameID, second.InternID("name"))
	r.Equal(titleID, second.InternID("title"))

	value, ok := first.LookupID(nameID)
	r.True(ok)
	r.Equal("Mido", value)

	value, ok = second.LookupID(nameID)
	r.True(ok)
	r.Equal("Fry", value)

	value, ok = second.LookupID(titleID)
	r.True(ok)
	r.Equal("Pilot", value)
}

func Test_Context_Intern_I_Ds_Are_Safe_Across_Concurrent_Root_Contexts(t *testing.T) {
	r := require.New(t)
	keys := []string{"name", "items", "local", "__plush_internal_render_diagnostics_test__"}

	wg := errgroup.Group{}
	for i := 0; i < 64; i++ {
		i := i
		wg.Go(func() error {
			name := fmt.Sprintf("name-%d", i)
			ctx := plush.NewContextWith(map[string]interface{}{
				"name":  name,
				"items": []string{"a", "b"},
			})
			ids := make([]int, len(keys))

			for j := 0; j < 32; j++ {
				ctx.InternIDs(keys, ids)
				for keyIndex, key := range keys {
					if ids[keyIndex] != ctx.InternID(key) {
						return fmt.Errorf("intern id mismatch for %s", key)
					}
				}
				value, ok := ctx.LookupID(ids[0])
				if !ok || value != name {
					return fmt.Errorf("lookup id mismatch: %v %v", value, ok)
				}
				_ = ctx.Value(keys[3])
			}
			return nil
		})
	}

	r.NoError(wg.Wait())
}

func Test_Context_Shared_Helpers_Do_Not_Leak_Overrides(t *testing.T) {
	r := require.New(t)
	c1 := plush.NewContextWith(map[string]interface{}{
		"debug": func(i interface{}) template.HTML {
			return template.HTML("LOCAL")
		},
	})
	c2 := plush.NewContext()

	local := c1.Value("debug").(func(interface{}) template.HTML)(nil)
	global := c2.Value("debug").(func(interface{}) template.HTML)(nil)
	r.Equal(template.HTML("LOCAL"), local)
	r.NotEqual(template.HTML("LOCAL"), global)
}

func Test_Context_Shared_Helpers_Refreshes_For_New_Helpers(t *testing.T) {
	r := require.New(t)
	name := fmt.Sprintf("test_helper_%p", t)

	before := plush.NewContext()
	r.False(before.Has(name))

	r.NoError(plush.Helpers.Add(name, func() string { return "ok" }))
	after := plush.NewContext()

	r.False(before.Has(name))
	r.True(after.Has(name))
	r.Equal("ok", after.Value(name).(func() string)())
}
