package plush_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

type parityNumericStats struct {
	Count uint64
}

type parityNumericRobot struct {
	Count uint32
	Stats parityNumericStats
}

func (r parityNumericRobot) MethodCount() uint32 {
	return r.Count
}

func Test_Parity_Math_Integer_Operators(t *testing.T) {
	tests := []string{
		`<%= 1 + 3 %>`,
		`<%= 3 - 1 %>`,
		`<%= 10 / 2 %>`,
		`<%= 10 * 2 %>`,
		`<%= 10 > 2 %>`,
		`<%= 10 >= 2 %>`,
		`<%= 10 >= 10 %>`,
		`<%= 2 <= 2 %>`,
		`<%= 10 < 2 %>`,
		`<%= 10 <= 2 %>`,
		`<%= 2 == 2 %>`,
		`<%= 1 != 2 %>`,
	}

	for _, input := range tests {
		compareRender(t, input, emptyContext)
	}
}

func Test_Parity_Math_Float_Operators(t *testing.T) {
	tests := []string{
		`<%= 1.0 + 3.0 %>`,
		`<%= 3.0 - 1.0 %>`,
		`<%= 10.0 / 2.0 %>`,
		`<%= 10.0 * 2.0 %>`,
		`<%= 10.0 > 2.0 %>`,
		`<%= 10.0 >= 2.0 %>`,
		`<%= 10.0 >= 10.0 %>`,
		`<%= 2.0 <= 2.0 %>`,
		`<%= 10.0 < 2.0 %>`,
		`<%= 10.0 <= 2.0 %>`,
		`<%= 2.0 == 2.0 %>`,
		`<%= 1.0 != 2.0 %>`,
	}

	for _, input := range tests {
		compareRender(t, input, emptyContext)
	}
}

func Test_Parity_Math_String_Operators(t *testing.T) {
	tests := []string{
		`<%= "a" + "b" %>`,
		`<%= "a" + "b" + "c" %>`,
		`<%= "a" != "b" %>`,
		`<%= "a" == "a" %>`,
		`<%= "a" == "b" %>`,
		`<%= "a" > "b" %>`,
		`<%= "a" >= "b" %>`,
		`<%= "a" <= "b" %>`,
	}

	for _, input := range tests {
		compareRender(t, input, emptyContext)
	}
}

func Test_Parity_Math_Undefined_Equality_Operators(t *testing.T) {
	compareRender(t, `<%= undefined == 3 %>`, emptyContext)
	compareRender(t, `<%= undefined != 3 %>`, emptyContext)
	compareRender(t, `<%= 3 == unknown %>`, emptyContext)
	compareRender(t, `<%= 3 != unknown %>`, emptyContext)
}

func Test_Parity_Math_Undefined_Arithmetic_Operators_Error(t *testing.T) {
	compareBothRenderError(t, `<%= undefined + 3 %>`, emptyContext)
	compareBothRenderError(t, `<%= 3 + unknown %>`, emptyContext)
	compareBothRenderError(t, `<%= undefined > 3 %>`, emptyContext)
}

func Test_Parity_Math_String_Int_Concat(t *testing.T) {
	compareRender(t, `<%= "a" + 1 %>`, emptyContext)
}

func Test_Parity_Math_Bool_Concat(t *testing.T) {
	compareRender(t, `<%= true + 1 %>`, emptyContext)
}

func Test_Parity_Math_Division_By_Zero_Errors(t *testing.T) {
	compareBothRenderError(t, `<%= 10 / 0 %>`, emptyContext)
	compareBothRenderError(t, `<%= 10.5 / 0.0 %>`, emptyContext)
}

func Test_Parity_Math_Renders_Many_Numeric_Types(t *testing.T) {
	compareRender(t, `<%= i32 %> <%= u32 %> <%= i8 %>`, contextWith(map[string]interface{}{
		"i32": int32(1),
		"u32": uint32(2),
		"i8":  int8(3),
	}))
}

func Test_Parity_Math_Safe_Mixed_Numeric_Comparisons(t *testing.T) {
	ctx := contextWith(map[string]interface{}{
		"i32":     int32(0),
		"i32v":    int32(3),
		"neg":     int32(-1),
		"u32":     uint32(0),
		"u32v":    uint32(3),
		"u64":     uint64(0),
		"u64one":  uint64(1),
		"u64max":  uint64(math.MaxUint64),
		"f32":     float32(3.5),
		"f32i":    float32(3),
		"f64":     float64(3.5),
		"f64i":    float64(3),
		"counts":  map[string]uint32{"active": 0},
		"values":  []uint32{0},
		"robot":   parityNumericRobot{Count: 0, Stats: parityNumericStats{Count: 0}},
		"robots":  []parityNumericRobot{{Count: 1, Stats: parityNumericStats{Count: 0}}},
		"counter": func() uint32 { return 0 },
	})

	tests := []struct {
		input    string
		expected string
	}{
		{`<%= i32 == 0 %>`, "true"},
		{`<%= u32 == 0 %>`, "true"},
		{`<%= u64 == 0 %>`, "true"},
		{`<%= u64one > 0 %>`, "true"},
		{`<%= u64max > 0 %>`, "true"},
		{`<%= neg < 0 %>`, "true"},
		{`<%= neg < u64 %>`, "true"},
		{`<%= neg == u64 %>`, "false"},
		{`<%= u32v == 3 %>`, "true"},
		{`<%= f32 == 3.5 %>`, "true"},
		{`<%= f64 > 3 %>`, "true"},
		{`<%= f32i == 3 %>`, "true"},
		{`<%= f64i == i32v %>`, "true"},
		{`<%= u32v == 3.0 %>`, "true"},
		{`<%= u64one == 1.0 %>`, "true"},
		{`<%= robot.Count == 0 %>`, "true"},
		{`<%= robots[0].Stats.Count == 0 %>`, "true"},
		{`<%= counts["active"] == 0 %>`, "true"},
		{`<%= values[0] == 0 %>`, "true"},
		{`<%= counter() == 0 %>`, "true"},
		{`<%= robot.MethodCount() == 0 %>`, "true"},
		{`<%= robot.Count + 1 %>`, "1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, ctx)
		})
	}
}

func requireBothRender(t *testing.T, input, expected string, factory contextFactory) {
	t.Helper()

	interpreterOut, interpreterErr := renderInterpreter(input, factory)
	require.NoError(t, interpreterErr)
	require.Equal(t, expected, interpreterOut)

	vmOut, vmErr := renderVM(input, factory)
	require.NoError(t, vmErr)
	require.Equal(t, expected, vmOut)
}
