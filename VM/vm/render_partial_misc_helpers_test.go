package vm

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/stretchr/testify/require"
)

func Test_VM_Setup_Fast_Partial_Nesting_Branches(t *testing.T) {
	setupFastPartialNesting(nil, "row.plush", partialMetaIDs{})

	ctx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(ctx)
	ids := partialMetaIDs{
		alreadyPartialID:   ctx.InternID(vmAlreadyInPartial),
		templateBaseFileID: ctx.InternID(meta.TemplateBaseFileNameKey),
		templateExtID:      ctx.InternID(meta.TemplateExtensionKey),
	}

	setupFastPartialNesting(ctx, "row.plush", ids)
	value, ok := ctx.LookupID(ids.alreadyPartialID)
	require.True(t, ok)
	require.Equal(t, "row.plush", value)

	setupFastPartialNesting(ctx, "nested.html", ids)
	value, ok = ctx.LookupID(ids.templateBaseFileID)
	require.True(t, ok)
	require.Equal(t, "nested", value)
	value, ok = ctx.LookupID(ids.templateExtID)
	require.True(t, ok)
	require.Equal(t, "html", value)

	normal := plush.NewContext()
	setupPartialNesting(normal, "first.plush")
	require.Equal(t, "first.plush", normal.Value(vmAlreadyInPartial))
	setupPartialNesting(normal, "child.html")
	require.Equal(t, "child", normal.Value(meta.TemplateBaseFileNameKey))
	require.Equal(t, "html", normal.Value(meta.TemplateExtensionKey))
}

func Test_VM_Render_Output_Size_Helpers(t *testing.T) {
	now := time.Date(2026, 7, 7, 1, 2, 3, 0, time.UTC)
	ctx := plush.NewContextWith(map[string]interface{}{"TIME_FORMAT": "2006"})
	machine := newRuntimeHelperTestVM(ctx)

	require.Empty(t, machine.stringConstant(-1))
	machine.constants = []object.Object{&object.String{Value: "name"}, &object.Integer{Value: 7}, &object.Native{Value: template.HTML("<b>")}}
	require.Equal(t, "name", machine.stringConstant(0))
	require.Equal(t, "7", machine.stringConstant(1))
	require.Equal(t, "<b>", machine.htmlConstantString(2))
	require.Equal(t, "7", machine.htmlConstantString(1))
	require.Empty(t, machine.htmlConstantString(-1))

	require.Equal(t, 0, machine.estimatedRenderedObjectSize(nil))
	require.Equal(t, 0, machine.estimatedRenderedObjectSize(object.NullObject))
	require.Equal(t, len("ab"), machine.estimatedRenderedObjectSize(&object.String{Value: "ab"}))
	require.Equal(t, 24, machine.estimatedRenderedObjectSize(&object.Float{Value: 1.25}))
	require.Equal(t, 25, machine.estimatedRenderedObjectSize(&object.Array{Elements: []object.Object{&object.Integer{Value: 1}, object.TrueObject}}))
	require.Positive(t, machine.estimatedRenderedObjectSize(&object.Builtin{}))
	require.Equal(t, len(plush.PunchHoleMarkerName(0)), machine.estimatedRenderedGoValueSize(vmHole{}))
	require.Equal(t, len("2026"), machine.estimatedRenderedGoValueSize(now))
	require.Equal(t, len(now.Format(plush.DefaultTimeFormat)), newRuntimeHelperTestVM(plush.NewContext()).estimatedRenderedGoValueSize(now))
	require.Equal(t, 0, machine.estimatedRenderedGoValueSize((*time.Time)(nil)))
	require.Equal(t, len("2026"), machine.estimatedRenderedGoValueSize(&now))
	require.Equal(t, len("<b>"), machine.estimatedRenderedGoValueSize(template.HTML("<b>")))
	require.Equal(t, len("abc"), machine.estimatedRenderedGoValueSize("abc"))
	require.Equal(t, 5, machine.estimatedRenderedGoValueSize(true))
	require.Equal(t, 20, machine.estimatedRenderedGoValueSize(uint32(3)))
	require.Equal(t, 24, machine.estimatedRenderedGoValueSize(float32(1.5)))
	require.Equal(t, 3, machine.estimatedRenderedGoValueSize([]string{"a", "bc"}))
	require.Equal(t, 6, machine.estimatedRenderedGoValueSize([]interface{}{"a", true}))
	require.Equal(t, 4, machine.estimatedRenderedGoValueSize([2]string{"ab", "cd"}))
	require.Zero(t, machine.estimatedRenderedGoValueSize(struct{}{}))

	holeMachine := &VM{ctx: plush.NewContext(), deferHolePositions: true}
	var out strings.Builder
	holeMachine.writeGoValue(&out, vmHole{input: `"hole"`})
	require.Equal(t, plush.PunchHoleMarkerName(0), out.String())
	require.Len(t, *holeMachine.holes, 1)
	require.Equal(t, -1, (*holeMachine.holes)[0].Start())
	require.Equal(t, -1, (*holeMachine.holes)[0].End())
}

func Test_VM_Eval_Fast_Infix_Operator_Branches(t *testing.T) {
	tests := []struct {
		op       string
		left     interface{}
		right    interface{}
		expected bool
	}{
		{"&&", true, "x", true},
		{"||", false, "x", true},
		{"==", uint32(1), 1, true},
		{"!=", uint32(1), 2, true},
		{">", 3, 2, true},
		{">=", 3, 3, true},
		{"<", 2, 3, true},
		{"<=", 3, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got, err := evalFastInfixOperator(tt.op, tt.left, tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}

	_, err := evalFastInfixOperator("??", 1, 2)
	require.ErrorContains(t, err, "unknown fast infix operator")
}
