package plush_test

import "testing"

type parityCategory struct {
	Products []parityProduct
}

type parityProduct struct {
	Name []string
}

func Test_Parity_Variables_Let_And_Assign(t *testing.T) {
	compareRender(t, `<% let message = "hello" %><% message = "bye" %><%= message %>`, emptyContext)
}

func Test_Parity_Variables_Fast_Local_Let_Helper_Call(t *testing.T) {
	compareRender(t, `<% let message = greet(name) %><%= wrap(message) %>|<%= message %>`, contextWith(map[string]interface{}{
		"name":    "Mido",
		"message": "outer",
		"greet": func(name string) string {
			return "hi " + name
		},
		"wrap": func(value string) string {
			return "[" + value + "]"
		},
	}))
}

func Test_Parity_Variables_Unknown_Assignment_Errors(t *testing.T) {
	compareBothRenderError(t, `<% foo = "baz" %>`, emptyContext)
}

func Test_Parity_Variables_Identifiers_With_Digits(t *testing.T) {
	compareRender(t, `<%= my123greet %> <%= name3 %>`, contextWith(map[string]interface{}{
		"my123greet": "hi",
		"name3":      "mark",
	}))
}

func Test_Parity_Variables_Identifier_Ending_In_Number_Loop(t *testing.T) {
	compareRender(t, `<%= for (n) in myvar1 { return n } %>`, contextWith(map[string]interface{}{
		"myvar1": []string{"john", "paul"},
	}))
}

func Test_Parity_Variables_Array_And_Hash_Index(t *testing.T) {
	compareRender(t, `<% let h = {"name": "mark"} %><% let a = ["m", "e"] %><%= h["name"] %><%= a[1] %>`, emptyContext)
}

func Test_Parity_Variables_Hash_Index_Assignment_And_Missing_Key(t *testing.T) {
	compareRender(t, `<p><% let h = {"a": "A"} %><% h["a"] = "C" %><%= h["a"] %>:<%= h["missing"] %></p>`, emptyContext)
}

func Test_Parity_Variables_Array_Index_Assignment(t *testing.T) {
	compareRender(t, `<p><% let a = [1, 2, "three"] %><% a[0] = 3 %><%= a[0] %></p>`, emptyContext)
}

func Test_Parity_Variables_Array_Append(t *testing.T) {
	compareRender(t, `<% let a = [1, "two"] %><% a = a + 3 %><%= a %>`, emptyContext)
}

func Test_Parity_Variables_Native_Slice_Index_Assignment(t *testing.T) {
	type item struct {
		Name string
	}

	compareBothRenderError(t, `<% let a = myArray %><% a[0] = "HELLO WORLD" %>`, contextWith(map[string]interface{}{
		"myArray": []item{{Name: "t"}},
	}))
}

func Test_Parity_Variables_Native_Interface_Slice_Assignment(t *testing.T) {
	type item struct {
		Name string
	}

	compareRender(t, `<% let a = myArray %><% a[0] = "HELLO WORLD" %><%= a %>`, contextWith(map[string]interface{}{
		"myArray": []interface{}{item{Name: "t"}, item{Name: "g"}},
	}))
}

func Test_Parity_Variables_Native_Slice_Append_Success(t *testing.T) {
	compareRender(t, `<% let a = myArray %><% a = a + 1 %><%= a %>`, contextWith(map[string]interface{}{
		"myArray": []interface{}{"a", "b"},
	}))
	compareRender(t, `<% let a = myInts %><% a = a + 3 %><%= a[2] %>`, contextWith(map[string]interface{}{
		"myInts": []int{1, 2},
	}))
	compareRender(t, `<% let a = myStrings %><% a = a + "c" %><%= a[2] %>`, contextWith(map[string]interface{}{
		"myStrings": []string{"a", "b"},
	}))
}

func Test_Parity_Variables_Native_Typed_Slice_Append_Error(t *testing.T) {
	compareBothRenderError(t, `<% let a = myArray %><% a = a + 1 %><%= a %>`, contextWith(map[string]interface{}{
		"myArray": []string{"a", "b"},
	}))
}

func Test_Parity_Variables_Access_Callee_Array(t *testing.T) {
	compareRender(t, `<% let a = product_listing.Products[0].Name[0] %><%= a %>`, contextWith(map[string]interface{}{
		"product_listing": parityCategory{
			Products: []parityProduct{
				{Name: []string{"Buffalo"}},
			},
		},
	}))
}

func Test_Parity_Variables_Access_Callee_Array_Out_Of_Bounds(t *testing.T) {
	compareBothRenderError(t, `<% let a = product_listing.Products[0].Name[0] %><%= a %>`, contextWith(map[string]interface{}{
		"product_listing": parityCategory{},
	}))
}
