package plush

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Render_For_Array(t *testing.T) {
	r := require.New(t)
	input := `<% for (i,v) in ["a", "b", "c"] {return v} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_For_Hash(t *testing.T) {
	r := require.New(t)
	input := `<%= for (k,v) in myMap { %><%= k + ":" + v%><% } %>`
	s, err := Render(input, NewContextWith(map[string]interface{}{
		"myMap": map[string]string{
			"a": "A",
			"b": "B",
		},
	}))
	r.NoError(err)
	r.Contains(s, "a:A")
	r.Contains(s, "b:B")
}

func Test_Render_For_Array_Return(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in ["a", "b", "c"] {return v} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render_For_Array_Continue(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		%>Start<%
		if (v == 1 || v ==3 || v == 5 || v == 7 || v == 9) {
			
			
			%>Odd<%
			continue
		}

		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("StartOddStart2StartOddStart4StartOddStart6StartOddStart8StartOddStart10", s)
}

func Test_Render_For_Array_WithNoOutput(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
	
		if (v == 1 || v == 2 || v ==3 || v == 4|| v == 5 || v == 6 || v == 7 || v == 8 || v == 9 || v == 10) {

			continue
		}

		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_For_Array_WithoutContinue(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		if (v == 1 || v ==3 || v == 5 || v == 7 || v == 9) {
		}
		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("12345678910", s)
}

func Test_Render_For_Array_ContinueNoControl(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		continue
		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_For_Array_Break_String(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		%>Start<%
		if (v == 5) {
			
			
			%>Odd<%
			break
		}

		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("Start1Start2Start3Start4StartOdd", s)
}

func Test_Render_For_Array_WithBreakFirstValue(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		if (v == 1 || v ==3 || v == 5 || v == 7 || v == 9) {
			break
		}
		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_For_Array_WithBreakFirstValueWithReturn(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		if (v == 1 || v ==3 || v == 5 || v == 7 || v == 9) {
			%><%=v%><%
			break
		}
		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("1", s)
}
func Test_Render_For_Array_Break(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
		break
		return v   
		} %>`
	s, err := Render(input, NewContext())

	r.NoError(err)
	r.Equal("", s)
}

func Test_Render_For_Array_Key_Only(t *testing.T) {
	r := require.New(t)
	input := `<%= for (v) in ["a", "b", "c"] {%><%=v%><%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("abc", s)
}

func Test_Render_For_Func_Range(t *testing.T) {
	r := require.New(t)
	input := `<%= for (v) in range(3,5) { %><%=v%><% } %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("345", s)
}

func Test_Render_For_Func_Between(t *testing.T) {
	r := require.New(t)
	input := `<%= for (v) in between(3,6) { %><%=v%><% } %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("45", s)
}

func Test_Render_For_Func_Until(t *testing.T) {
	r := require.New(t)
	input := `<%= for (v) in until(3) { %><%=v%><% } %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("012", s)
}

func Test_Render_For_Array_Key_Value(t *testing.T) {
	r := require.New(t)
	input := `<%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %>`
	s, err := Render(input, NewContext())
	r.NoError(err)
	r.Equal("0a1b2c", s)
}

func Test_Render_For_Nil(t *testing.T) {
	r := require.New(t)
	input := `<% for (i,v) in nilValue {return v} %>`
	ctx := NewContext()
	ctx.Set("nilValue", nil)
	s, err := Render(input, ctx)
	r.Error(err)
	r.Equal("", s)
}

func Test_Render_For_Map_Nil_Value(t *testing.T) {
	r := require.New(t)
	input := `
	<%= for (k, v) in flash["errors"] { %>
		Flash:
			<%= k %>:<%= v %>
	<% } %>
`
	ctx := NewContext()
	ctx.Set("flash", map[string][]string{})
	s, err := Render(input, ctx)
	r.NoError(err)
	r.Equal("", strings.TrimSpace(s))
}

type Category struct {
	Products []Product
}
type Product struct {
	Name []string
}

func Test_Render_For_Array_OutofBoundIndex(t *testing.T) {
	r := require.New(t)
	ctx := NewContext()
	product_listing := Category{}
	ctx.Set("product_listing", product_listing)
	input := `<%= for (i, names) in product_listing.Products[0].Name { %>
				<%= splt %>
			<% } %>`
	_, err := Render(input, ctx)
	r.Error(err)
}
