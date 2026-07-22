package plush_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func Test_Render_Int_Math_Division_By_Zero(t *testing.T) {
	r := require.New(t)
	input := `<%= 10 / 0 %>`
	s, err := plush.Render(input, plush.NewContext())
	r.Error(err)
	r.Empty(s)
	r.Contains(err.Error(), "division by zero 10 / 0")
}

func Test_Render_Int_Float_Division_By_Zero(t *testing.T) {
	r := require.New(t)
	input := `<%= 10.5 / 0.0 %>`
	s, err := plush.Render(input, plush.NewContext())
	r.Error(err)
	r.Empty(s)
	r.Contains(err.Error(), "division by zero 10.5 / 0")
}

func Test_Render_Int_Math(t *testing.T) {
	r := require.New(t)

	tests := []struct {
		a   int
		b   int
		op  string
		res string
	}{
		{1, 3, "+", "4"},
		{3, 1, "-", "2"},
		{10, 2, "/", "5"},
		{10, 2, "*", "20"},
		{10, 2, ">", "true"},
		{10, 2, ">=", "true"},
		{10, 10, ">=", "true"},
		{2, 2, "<=", "true"},
		{10, 2, "<", "false"},
		{10, 2, "<=", "false"},
		{2, 2, "==", "true"},
		{1, 2, "!=", "true"},
	}
	for _, tt := range tests {
		input := fmt.Sprintf("<%%= %d %s %d %%>", tt.a, tt.op, tt.b)
		s, err := plush.Render(input, plush.NewContext())
		r.NoError(err)
		r.Equal(tt.res, s)
	}
}

func Test_Render_Float_Math(t *testing.T) {
	r := require.New(t)

	tests := []struct {
		a   float64
		b   float64
		op  string
		res string
	}{
		{1, 3, "+", "4"},
		{3, 1, "-", "2"},
		{10, 2, "/", "5"},
		{10, 2, "*", "20"},
		{10, 2, ">", "true"},
		{10, 2, ">=", "true"},
		{10, 10, ">=", "true"},
		{2, 2, "<=", "true"},
		{10, 2, "<", "false"},
		{10, 2, "<=", "false"},
		{2, 2, "==", "true"},
		{1, 2, "!=", "true"},
	}
	for _, tt := range tests {
		input := fmt.Sprintf("<%%= %f %s %f %%>", tt.a, tt.op, tt.b)
		s, err := plush.Render(input, plush.NewContext())
		r.NoError(err)
		r.Equal(tt.res, s)
	}
}

func Test_Render_String_Math(t *testing.T) {
	r := require.New(t)

	tests := []struct {
		a   string
		b   string
		op  string
		res string
	}{
		{"a", "b", "+", "ab"},
		{"a", "b", "!=", "true"},
		{"a", "a", "==", "true"},
		{"a", "b", "==", "false"},
		{"a", "b", ">", "false"},
		{"a", "b", ">=", "false"},
		{"a", "b", "<=", "true"},
	}

	for _, tt := range tests {
		input := fmt.Sprintf("<%%= %q %s %q %%>", tt.a, tt.op, tt.b)
		s, err := plush.Render(input, plush.NewContext())
		r.NoError(err)
		r.Equal(tt.res, s)
	}
}

func Test_Render_Operator_Undefined_Var(t *testing.T) {
	tests := []struct {
		operator      string
		result        interface{}
		errorExpected bool
	}{
		{"+", "", true},
		{"-", "", true},
		{"/", "", true},
		{"*", "", true},
		{">", "", true},
		{">=", "", true},
		{"<=", "", true},
		{"<", "", true},
		{"==", "false", false},
		{"!=", "true", false},
	}
	for _, tc := range tests {
		t.Run(tc.operator, func(t *testing.T) {
			r := require.New(t)
			input := fmt.Sprintf("<%%= undefined %s 3 %%>", tc.operator)
			s, err := plush.Render(input, plush.NewContext())
			if tc.errorExpected {
				r.Error(err, "undefined %s 3 --> '%v'", tc.operator, tc.result)
			} else {
				r.NoError(err, "undefined %s 3 --> '%v'", tc.operator, tc.result)
			}
			r.Equal(tc.result, s, "undefined %s 3", tc.operator)

			input = fmt.Sprintf("<%%= 3 %s unknown %%>", tc.operator)
			s, err = plush.Render(input, plush.NewContext())
			if tc.errorExpected {
				r.Error(err, "3 %s undefined --> '%v'", tc.operator, tc.result)
			} else {
				r.NoError(err, "3 %s undefined --> '%v'", tc.operator, tc.result)
			}
			r.Equal(tc.result, s, "undefined %s 3", tc.operator)
		})
	}
}

func Test_Render_String_Concat_Multiple(t *testing.T) {
	r := require.New(t)

	input := `<%= "a" + "b" + "c" %>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render_String_Int_Concat(t *testing.T) {
	r := require.New(t)

	input := `<%= "a"  + 1 %>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("a1", s)
}

func Test_Render_Bool_Concat(t *testing.T) {
	r := require.New(t)

	input := `<%= true + 1 %>`
	s, err := plush.Render(input, plush.NewContext())
	r.Equal("true", s)
	r.NoError(err)
}

func Test_Render_Safe_Mixed_Numeric_Comparisons(t *testing.T) {
	type stats struct {
		Count uint64
	}
	type robot struct {
		Count uint32
		Stats stats
	}

	ctx := plush.NewContextWith(map[string]interface{}{
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
		"robot":   robot{Count: 0, Stats: stats{Count: 0}},
		"robots":  []robot{{Count: 1, Stats: stats{Count: 0}}},
		"counter": func() uint32 { return 0 },
	})

	input := `<%= i32 == 0 %>|<%= u32 == 0 %>|<%= u64 == 0 %>|<%= u64one > 0 %>|<%= u64max > 0 %>|<%= neg < 0 %>|<%= neg < u64 %>|<%= neg == u64 %>|<%= u32v == 3 %>|<%= f32 == 3.5 %>|<%= f64 > 3 %>|<%= f32i == 3 %>|<%= f64i == i32v %>|<%= u32v == 3.0 %>|<%= u64one == 1.0 %>|<%= robot.Count == 0 %>|<%= robots[0].Stats.Count == 0 %>|<%= counts["active"] == 0 %>|<%= values[0] == 0 %>|<%= counter() == 0 %>|<%= robot.Count + 1 %>`
	s, err := plush.Render(input, ctx)
	require.NoError(t, err)
	require.Equal(t, "true|true|true|true|true|true|true|false|true|true|true|true|true|true|true|true|true|true|true|true|1", s)
}
