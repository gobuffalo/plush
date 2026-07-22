package vm

import (
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/stretchr/testify/require"
)

func Test_Render_Diagnostics_Fast_Render_Plan_Stats(t *testing.T) {
	plan := &compiler.FastRenderPlan{
		Bindings: []string{"name", "items", "label"},
		Segments: []compiler.FastRenderSegment{
			{Kind: compiler.FastRenderSegmentStatic, Value: "<ul>"},
			{Kind: compiler.FastRenderSegmentName, Value: "name", NameIndex: 0},
			{Kind: compiler.FastRenderSegmentProperty, Value: "user", Property: "Name"},
			{Kind: compiler.FastRenderSegmentCall, Call: &compiler.FastCallPlan{
				Name:      "label",
				NameIndex: 2,
				Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueName, Value: "name", NameIndex: 0}},
			}},
			{Kind: compiler.FastRenderSegmentConditional, Conditional: &compiler.FastConditionalPlan{
				Branches: []compiler.FastConditionalBranch{{
					Condition: compiler.FastValuePlan{
						Kind:     compiler.FastValueInfix,
						Operator: "==",
						Left:     &compiler.FastValuePlan{Kind: compiler.FastValuePath, Value: "product", Path: []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Name"}}},
						Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "Pizza"},
					},
					Segments: []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "pizza"}},
				}},
			}},
			{Kind: compiler.FastRenderSegmentLoop, Loop: &compiler.FastLoopPlan{
				IterableName:      "items",
				IterableNameIndex: 1,
				Parts: []compiler.FastLoopPart{
					{Kind: compiler.FastLoopPartStatic, Value: "<li>"},
					{Kind: compiler.FastLoopPartValueProperty, PropertyCache: compiler.FastRenderSegment{}.PropertyCache},
					{Kind: compiler.FastLoopPartCall, Call: &compiler.FastCallPlan{Name: "label", NameIndex: 2}},
				},
			}},
			{Kind: compiler.FastRenderSegmentPartial, Partial: &compiler.FastPartialPlan{
				Name: "row.plush",
				Data: []compiler.FastPartialDataPair{
					{Key: "title", Value: compiler.FastValuePlan{Kind: compiler.FastValuePath, Value: "product", Path: []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Title"}}}},
				},
			}},
		},
	}

	stats := fastRenderPlanDiagnostics(plan)

	require.Equal(t, 3, stats.Bindings)
	require.Equal(t, len(plan.Segments), stats.Segments)
	require.Equal(t, 3, stats.StaticSegments)
	require.Equal(t, 2, stats.NameSegments)
	require.Equal(t, 4, stats.PropertyReads)
	require.Equal(t, 1, stats.ValueWrites)
	require.Equal(t, 2, stats.HelperCalls)
	require.Equal(t, 1, stats.Conditionals)
	require.Equal(t, 1, stats.Loops)
	require.Equal(t, 3, stats.LoopParts)
	require.Equal(t, 1, stats.Partials)
	require.GreaterOrEqual(t, stats.MaxDepth, 3)
	require.Equal(t, []string{"label"}, stats.HelperNames)
	require.Equal(t, []string{"row.plush"}, stats.PartialNames)
}

func Test_Render_Diagnostics_Context_Expose_Fast_Render_Plan_Stats(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
	})

	tmpl, err := Compile(`<h1><%= name %></h1>`)
	require.NoError(t, err)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "<h1>Mido</h1>", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderModeNameVM, diagnostics.Mode)
	require.Equal(t, plush.VMBytecodeCacheDirect, diagnostics.VMBytecodeCache)
	require.Greater(t, diagnostics.FastPlan.Segments, 0)
	require.Greater(t, diagnostics.FastPlan.NameSegments, 0)
	require.NotZero(t, diagnostics.EngineDuration)
}

func Test_Render_Diagnostics_Caches_Fast_Render_Plan_Stats_On_Bytecode(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
	})

	tmpl, err := Compile(`<h1><%= name %></h1>`)
	require.NoError(t, err)
	require.Nil(t, tmpl.bytecode.FastDiagnostics.Load())

	updateBytecodeDiagnostics(ctx, tmpl.bytecode)
	first := tmpl.bytecode.FastDiagnostics.Load()
	require.NotNil(t, first)

	updateBytecodeDiagnostics(ctx, tmpl.bytecode)
	second := tmpl.bytecode.FastDiagnostics.Load()
	require.Same(t, first, second)

	stats, ok := second.(*plush.RenderFastPlanDiagnostics)
	require.True(t, ok)
	require.Greater(t, stats.Segments, 0)
	require.Greater(t, stats.NameSegments, 0)
}

func Test_Render_Diagnostics_VM_Hotspots_Record_Fast_Helper_And_Partial(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"label": func(value string) string {
			return "slow " + value
		},
		"partialFeeder": func(string) (string, error) {
			return `<%= name %>`, nil
		},
	})
	plush.EnableRenderVMHotspotDiagnostics(ctx)
	SetFastHelper(ctx, "label", func(w FastWriter, args FastArgs) error {
		value, ok := args.String(0)
		require.True(t, ok)
		w.WriteEscapedString("fast " + value)
		return nil
	})

	tmpl, err := Compile(`<%= label(name) %>|<%= partial("row.plush") %>`)
	require.NoError(t, err)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "fast Mido|Mido", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.GreaterOrEqual(t, diagnostics.VMHotspots.HelperCalls, 1)
	require.Greater(t, diagnostics.VMHotspots.HelperDuration, time.Duration(0))
	require.Contains(t, diagnostics.VMHelperHotspotsHeader(), "label:")
	require.Equal(t, 1, diagnostics.VMHotspots.PartialCalls)
	require.Greater(t, diagnostics.VMHotspots.PartialDuration, time.Duration(0))
	require.Contains(t, diagnostics.VMPartialHotspotsHeader(), "row.plush:")
}

func Test_Render_Diagnostics_Nested_Partial_Does_Not_Overwrite_Top_Template(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<%= name %>`, nil
		},
	})
	plush.EnableRenderVMHotspotDiagnostics(ctx)
	ctx.Set(meta.TemplateFileKey, "application.plush")

	out, err := Render(`<%= partial("header.html") %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Mido", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderModeNameVM, diagnostics.Mode)
	require.Equal(t, "application.plush", diagnostics.TemplateFilename)
	require.Equal(t, plush.VMBytecodeCacheMissStore, diagnostics.VMBytecodeCache)
	require.Equal(t, []string{"header.html"}, diagnostics.FastPlan.PartialNames)
	require.Equal(t, 1, diagnostics.VMHotspots.PartialCalls)
	require.Contains(t, diagnostics.VMPartialHotspotsHeader(), "header.html:")
}
