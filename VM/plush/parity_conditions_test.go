package plush_test

import "testing"

type parityTruthUser struct {
	Name  string
	Image *string
}

func Test_Parity_Conditions_Unknown_As_Nil(t *testing.T) {
	compareRender(t, `<%= paths == nil %>`, emptyContext)
	compareRender(t, `<%= nil == paths %>`, emptyContext)
	compareRender(t, `<%= !paths %>`, emptyContext)
	compareRender(t, `<%= if (paths) { %>yes<% } else { %>no<% } %>`, emptyContext)
}

func Test_Parity_Conditions_String_Truthiness(t *testing.T) {
	compareRender(t, `<%= if (username && username != "") { return "hi" } else { return "bye" } %>`, contextWith(map[string]interface{}{
		"username": "",
	}))
	compareRender(t, `<%= if (username && username != "") { return "hi" } else { return "bye" } %>`, contextWith(map[string]interface{}{
		"username": "mark",
	}))
}

func Test_Parity_Conditions_Assignment_Inside_If_Scope(t *testing.T) {
	input := `<p><%= if (username && username != "") { username = "hi" } else { username = "bye" } %><%= username %></p>`

	compareRender(t, input, contextWith(map[string]interface{}{
		"username": "",
	}))
	compareRender(t, input, contextWith(map[string]interface{}{
		"username": "foo",
	}))
}

func Test_Parity_Conditions_Unknown_Assignment_Inside_If_Errors(t *testing.T) {
	compareBothRenderError(t, `<p><%= if (username && username != "") { username = "hi" } else { username = "bye" } %><%= username %></p>`, emptyContext)
}

func Test_Parity_Conditions_Block_Scope_Declare_Does_Not_Overwrite_Outer(t *testing.T) {
	compareRender(t, `<p><% let username = "Hello World" %><%= if (username && username != "") {
		let username = "hi"
		username = "hi"
	} else {
		let username = "bye"
		username = "1"
	} %><%= username %></p>`, emptyContext)
}

func Test_Parity_Conditions_Nested_Block_Scope_Overwrite_Outer(t *testing.T) {
	compareRender(t, `<p><% let username = "Hello World" %><%= if (username && username != "") {
		username = "hi"
		if (username == "hi") {
			username = "hi2"
		}
	} else {
		let username = "bye"
		username = "1"
	} %><%= username %></p>`, emptyContext)
}

func Test_Parity_Conditions_Nil_Pointer_Truthiness(t *testing.T) {
	compareRender(t, `<%= user.Name %>:<%= if (user.Image) { return "has image" } else { return "no image" } %>`, contextWith(map[string]interface{}{
		"user": parityTruthUser{Name: "Garn"},
	}))

	image := "bicep.png"
	compareRender(t, `<%= user.Name %>:<%= if (user.Image) { return "has image" } else { return "no image" } %>`, contextWith(map[string]interface{}{
		"user": parityTruthUser{Name: "Scrinch", Image: &image},
	}))
}

func Test_Parity_Conditions_Logical_Unknown_Short_Circuit(t *testing.T) {
	compareRender(t, `<%= if (names && len(names) >= 1) { %>yes<% } else { %>no<% } %>`, emptyContext)
	compareRender(t, `<%= paths || pages %>`, contextWith(map[string]interface{}{
		"paths": "cart",
	}))
	compareRender(t, `<%= pages || paths %>`, contextWith(map[string]interface{}{
		"paths": "cart",
	}))
	compareRender(t, `<%= paths && pages %>`, contextWith(map[string]interface{}{
		"paths": "cart",
	}))
}

func Test_Parity_Conditions_Complex_Or_With_Unknown_Middle(t *testing.T) {
	compareRender(t, `<%= if (paths == "cart" || (page && page.PageTitle != "cafe") || paths == "cart") { %>hi<% } %>`, contextWith(map[string]interface{}{
		"path":  "cart",
		"paths": "cart",
	}))
}

func Test_Parity_Conditions_Direct_Unknown_Still_Errors(t *testing.T) {
	_, interpreterErr := renderInterpreter(`<%= pages %>`, emptyContext)
	_, vmErr := renderVM(`<%= pages %>`, emptyContext)

	if interpreterErr == nil {
		t.Fatal("expected interpreter error")
	}
	if vmErr == nil {
		t.Fatal("expected VM error")
	}
}

func Test_Parity_Conditions_Invalid_Int_Bool_Comparison_Errors(t *testing.T) {
	_, interpreterErr := renderInterpreter(`<% let test = 1 %><%= if (test != true) { return "good"} %>`, emptyContext)
	_, vmErr := renderVM(`<% let test = 1 %><%= if (test != true) { return "good"} %>`, emptyContext)

	if interpreterErr == nil {
		t.Fatal("expected interpreter error")
	}
	if vmErr == nil {
		t.Fatal("expected VM error")
	}
}
