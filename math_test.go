package plush

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

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
		s, err := Render(input, NewContext())
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
		s, err := Render(input, NewContext())
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
		s, err := Render(input, NewContext())
		r.NoError(err)
		r.Equal(tt.res, s)
	}
}

func Test_Render_Operator_UndefinedVar(t *testing.T) {
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
			s, err := Render(input, NewContext())
			if tc.errorExpected {
				r.Error(err, "undefined %s 3 --> '%v'", tc.operator, tc.result)
			} else {
				r.NoError(err, "undefined %s 3 --> '%v'", tc.operator, tc.result)
			}
			r.Equal(tc.result, s, "undefined %s 3", tc.operator)

			input = fmt.Sprintf("<%%= 3 %s unknown %%>", tc.operator)
			s, err = Render(input, NewContext())
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
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render_String_Int_Concat(t *testing.T) {
	r := require.New(t)

	input := `<%= "a"  + 1 %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("a1", s)
}

func Test_Render_Bool_Concat(t *testing.T) {
	r := require.New(t)

	input := `<%= true + 1 %>`
	s, err := Render(input, NewContext())
	r.Equal("true", s)
	r.NoError(err)
}
