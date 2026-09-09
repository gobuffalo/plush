package vm

import (
	"reflect"
	"sync"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/stretchr/testify/require"
)

func Test_VM_Runtime_Plan_Cache_Is_Owned_By_Prepared_Bytecode_Plan(t *testing.T) {
	firstPlan := &compiler.FastRenderPlan{}
	secondPlan := &compiler.FastRenderPlan{}

	first := newFastRenderBindings(firstPlan, rootplush.NewContext()).runtimePlans
	firstAgain := newFastRenderBindings(firstPlan, rootplush.NewContext()).runtimePlans
	second := newFastRenderBindings(secondPlan, rootplush.NewContext()).runtimePlans

	require.NotNil(t, first)
	require.Same(t, first, firstAgain)
	require.NotSame(t, first, second)

	valuePlan := &compiler.FastValuePlan{Kind: compiler.FastValuePath}
	key := fastFieldChainPlanKey{plan: valuePlan, typ: reflect.TypeOf(struct{ Name string }{})}
	stored := &fastFieldChainPlan{}
	require.Same(t, stored, first.storeFieldChainPlan(key, stored))

	got, ok := first.fieldChainPlan(key)
	require.True(t, ok)
	require.Same(t, stored, got)
	_, ok = second.fieldChainPlan(key)
	require.False(t, ok)
}

func Test_VM_Runtime_Plan_Cache_Is_Replaced_With_Recompiled_Bytecode(t *testing.T) {
	type profile struct {
		Name string
	}
	type record struct {
		Profile profile
	}
	const source = `<%= record.Profile.Name %>`

	first, err := Compile(source)
	require.NoError(t, err)
	firstCache := fastRuntimePlanCacheFor(first.bytecode.FastRenderPlan)
	firstOutput, err := first.Render(rootplush.NewContextWith(map[string]interface{}{
		"record": record{Profile: profile{Name: "first"}},
	}))
	require.NoError(t, err)
	require.Equal(t, "first", firstOutput)
	firstCache.mu.RLock()
	firstEntries := len(firstCache.fieldChainPlans) + len(firstCache.accessChainPlans) + len(firstCache.structLoopWriterPlans)
	firstCache.mu.RUnlock()
	require.Positive(t, firstEntries)

	second, err := Compile(source)
	require.NoError(t, err)
	secondCache := fastRuntimePlanCacheFor(second.bytecode.FastRenderPlan)
	secondOutput, err := second.Render(rootplush.NewContextWith(map[string]interface{}{
		"record": record{Profile: profile{Name: "second"}},
	}))
	require.NoError(t, err)
	require.Equal(t, "second", secondOutput)
	secondCache.mu.RLock()
	secondEntries := len(secondCache.fieldChainPlans) + len(secondCache.accessChainPlans) + len(secondCache.structLoopWriterPlans)
	secondCache.mu.RUnlock()
	require.Positive(t, secondEntries)

	require.NotSame(t, first.bytecode, second.bytecode)
	require.NotSame(t, firstCache, secondCache)
}

func Test_VM_Runtime_Plan_Cache_Concurrent_First_Writer_Wins(t *testing.T) {
	cache := &fastRuntimePlanCache{}
	valuePlan := &compiler.FastValuePlan{Kind: compiler.FastValuePath}
	key := fastAccessChainPlanKey{plan: valuePlan, typ: reflect.TypeOf(map[string]string{})}

	const workers = 64
	results := make(chan *fastAccessChainPlan, workers)
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- cache.storeAccessChainPlan(key, &fastAccessChainPlan{})
		}()
	}
	wg.Wait()
	close(results)

	var first *fastAccessChainPlan
	for plan := range results {
		if first == nil {
			first = plan
			continue
		}
		require.Same(t, first, plan)
	}

	cache.mu.RLock()
	require.Len(t, cache.accessChainPlans, 1)
	cache.mu.RUnlock()
}

func Test_VM_Runtime_Plan_Cache_Concurrent_Preparation_Uses_One_Owner(t *testing.T) {
	plan := &compiler.FastRenderPlan{}
	const workers = 64
	results := make(chan *fastRuntimePlanCache, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- fastRuntimePlanCacheFor(plan)
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var owner *fastRuntimePlanCache
	for cache := range results {
		if owner == nil {
			owner = cache
			continue
		}
		require.Same(t, owner, cache)
	}
}

func Test_VM_Runtime_Plan_Cache_Preserves_Negative_Results(t *testing.T) {
	cache := &fastRuntimePlanCache{}
	loop := &compiler.FastLoopPlan{}
	key := fastStructLoopWriterPlanKey{loop: loop, typ: reflect.TypeOf(struct{}{})}

	require.Nil(t, cache.storeStructLoopWriterPlan(key, nil))
	plan, ok := cache.structLoopWriterPlan(key)
	require.True(t, ok)
	require.Nil(t, plan)
}

func Benchmark_VM_Runtime_Plan_Cache_Field_Chain_Hit(b *testing.B) {
	cache := &fastRuntimePlanCache{}
	valuePlan := &compiler.FastValuePlan{Kind: compiler.FastValuePath}
	key := fastFieldChainPlanKey{plan: valuePlan, typ: reflect.TypeOf(struct{ Name string }{})}
	cache.storeFieldChainPlan(key, &fastFieldChainPlan{})
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, _ = cache.fieldChainPlan(key)
	}
}

func Benchmark_VM_Runtime_Plan_Cache_Sync_Map_Hit(b *testing.B) {
	var cache sync.Map
	valuePlan := &compiler.FastValuePlan{Kind: compiler.FastValuePath}
	key := fastFieldChainPlanKey{plan: valuePlan, typ: reflect.TypeOf(struct{ Name string }{})}
	cache.Store(key, &fastFieldChainPlan{})
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, _ = cache.Load(key)
	}
}
