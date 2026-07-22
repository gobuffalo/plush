package plush_test

import (
	"strings"
	"testing"
)

type parityLoopMenu struct {
	Items []parityLoopItem
}

type parityLoopItem struct {
	Name  string
	Count int
}

func Test_Parity_Loops_Array_Return(t *testing.T) {
	compareRender(t, `<%= for (i,v) in ["a", "b", "c"] { return v } %>`, emptyContext)
}

func Test_Parity_Loops_Context_Slice(t *testing.T) {
	compareRender(t, `<%= for (i,v) in items { %><%= i %>:<%= v %>;<% } %>`, contextWith(map[string]interface{}{
		"items": []string{"a", "b"},
	}))
}

func Test_Parity_Loops_Key_Only(t *testing.T) {
	compareRender(t, `<%= for (v) in ["a", "b", "c"] {%><%=v%><%} %>`, emptyContext)
}

func Test_Parity_Loops_Key_Value_And_Outer_Same_Identifier(t *testing.T) {
	compareRender(t, `<%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %>`, emptyContext)
	compareRender(t, `<% let i = 10000 %><%= for (i,v) in ["a", "b", "c"] {%><%=i%><%=v%><%} %><%= i %>`, emptyContext)
}

func Test_Parity_Loops_Update_Outer_Binding(t *testing.T) {
	compareRender(t, `<% let varTest = "" %><% for (i,v) in ["a", "b", "c"] {varTest = v} %><%= varTest %>`, emptyContext)
}

func Test_Parity_Loops_Continue(t *testing.T) {
	compareRender(t, `<%= for (i,v) in [1, 2, 3] { if (v == 2) { continue } return v } %>`, emptyContext)
}

func Test_Parity_Loops_Break(t *testing.T) {
	compareRender(t, `<%= for (i,v) in [1, 2, 3] { if (v == 2) { break } return v } %>`, emptyContext)
}

func Test_Parity_Loops_Continue_Output_Accumulation(t *testing.T) {
	compareRender(t, `<%= for (i,v) in [1, 2, 3, 4] {
		%>Start<%
		if (v == 1 || v == 3) {
			%>Odd<%
			continue
		}
		return v
	} %>`, emptyContext)
}

func Test_Parity_Loops_Continue_No_Output(t *testing.T) {
	compareRender(t, `<%= for (i,v) in [1, 2, 3] {
		continue
		return v
	} %>`, emptyContext)
}

func Test_Parity_Loops_Break_Output_Accumulation(t *testing.T) {
	compareRender(t, `<%= for (i,v) in [1, 2, 3, 4] {
		%>Start<%
		if (v == 3) {
			%>Stop<%
			break
		}
		return v
	} %>`, emptyContext)
}

func Test_Parity_Loops_Break_First_Value_With_Output(t *testing.T) {
	compareRender(t, `<%= for (i,v) in [1, 2, 3] {
		if (v == 1) {
			%><%=v%><%
			break
		}
		return v
	} %>`, emptyContext)
}

func Test_Parity_Loops_Single_Entry_Map(t *testing.T) {
	compareRender(t, `<%= for (k,v) in myMap { %><%= k + ":" + v%><% } %>`, contextWith(map[string]interface{}{
		"myMap": map[string]string{
			"a": "A",
		},
	}))
}

func Test_Parity_Loops_Map_Contains_Entries(t *testing.T) {
	input := `<%= for (k,v) in myMap { %><%= k + ":" + v%>;<% } %>`
	factory := contextWith(map[string]interface{}{
		"myMap": map[string]string{
			"a": "A",
			"b": "B",
		},
	})

	interpreterOut, interpreterErr := renderInterpreter(input, factory)
	vmOut, vmErr := renderVM(input, factory)
	if interpreterErr != nil || vmErr != nil {
		t.Fatalf("unexpected errors\ninterpreter: %v\nvm: %v", interpreterErr, vmErr)
	}
	for _, fragment := range []string{"a:A;", "b:B;"} {
		if !strings.Contains(interpreterOut, fragment) {
			t.Fatalf("interpreter output %q missing %q", interpreterOut, fragment)
		}
		if !strings.Contains(vmOut, fragment) {
			t.Fatalf("VM output %q missing %q", vmOut, fragment)
		}
	}
}

func Test_Parity_Loops_Nested_Slices(t *testing.T) {
	compareRender(t, `<%= for (i,row) in rows { %><%= for (j,col) in row { %><%= i %>,<%= j %>:<%= col %>;<% } %><% } %>`, contextWith(map[string]interface{}{
		"rows": [][]string{{"a", "b"}, {"c"}},
	}))
}

func Test_Parity_Loops_Nested_Flash_Style_Condition(t *testing.T) {
	compareRender(t, `<%= for (k, messages) in flash { %><%= for (msg) in messages { %><%= if (len(messages) && messages[0] != "skip") { %><%= k %>:<%= msg %>;<% } %><% } %><% } %>`, contextWith(map[string]interface{}{
		"flash": map[string][]string{"notice": {"Hello", "Bye"}},
	}))
	compareRender(t, `<%= for (k, messages) in flash { %><%= for (msg) in messages { %><%= if (len(messages) && messages[0] != "skip") { %><%= k %>:<%= msg %>;<% } %><% } %><% } %>`, contextWith(map[string]interface{}{
		"flash": map[string][]string{"notice": {"skip", "Bye"}},
	}))
}

func Test_Parity_Loops_Iterator_Helpers(t *testing.T) {
	compareRender(t, `<%= for (v) in range(3,5) { %><%=v%><% } %>|<%= for (v) in between(3,6) { %><%=v%><% } %>|<%= for (v) in until(3) { %><%=v%><% } %>`, emptyContext)
}

func Test_Parity_Loops_Nil_Iterable(t *testing.T) {
	compareRender(t, `<%= for (i,v) in nil { return v } %>`, emptyContext)
	compareBothRenderError(t, `<%= for (i,v) in nilValue { return v } %>`, contextWith(map[string]interface{}{
		"nilValue": nil,
	}))
}

func Test_Parity_Loops_Missing_Map_Key_Iterable(t *testing.T) {
	compareRender(t, `<%= for (k, v) in flash["errors"] { %><%= k %>:<%= v %><% } %>`, contextWith(map[string]interface{}{
		"flash": map[string][]string{},
	}))
}

func Test_Parity_Loops_Prefix_Condition_And_Struct_Field_Concat(t *testing.T) {
	compareRender(t, `<%= if (!userSignedIn) { %>Guest<% } else { %>User<% } %><%= for (item) in menu.Items { %><%= item.Name + " x " + item.Count %>;<% } %>`, contextWith(map[string]interface{}{
		"userSignedIn": false,
		"menu": parityLoopMenu{Items: []parityLoopItem{
			{Name: "One", Count: 2},
			{Name: "Two", Count: 3},
		}},
	}))
}

func Test_Parity_Loops_Iterator_Scope_Does_Not_Leak(t *testing.T) {
	compareBothRenderError(t, `<%= for (i,v) in ["a"] { %><%= i %>:<%= v %><% } %><%= i %>`, emptyContext)
	compareBothRenderError(t, `<%= for (i,v) in ["a"] { %><%= i %>:<%= v %><% } %><%= v %>`, emptyContext)
}
