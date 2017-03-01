# Plush [![Code Climate](https://codeclimate.com/github/gobuffalo/plush/badges/gpa.svg)](https://codeclimate.com/github/gobuffalo/plush) [![Build Status](https://travis-ci.org/gobuffalo/plush.svg?branch=master)](https://travis-ci.org/gobuffalo/plush) [![GoDoc](https://godoc.org/github.com/gobuffalo/plush?status.svg)](https://godoc.org/github.com/gobuffalo/plush)

Plush is the templating system that [Go](http://golang.org) both needs _and_ deserves. Powerful, flexible, and extendable, Plush is there to make writing your templates that much easier.

## Installation

```text
$ go get -u github.com/gobuffalo/plush
```

## Usage

Plush allows for the ebedding of dynamic code inside of your templates. Take the following example:

```erb
<!-- input -->
<p><%= "plush is great" %></p>

<!-- output -->
<p>plush is great</p>
```

By using the `<%= %>` tags we tell Plush to dynamically render the inner content, in this case the string `plush is great`, into the template between the `<p></p>` tags.

If we were to change the example to use `<% %>` tags instead the inner content will be evaluated and executed, but not injected into the template:

```erb
<!-- input -->
<p><% "plush is great" %></p>

<!-- output -->
<p></p>
```

By using the `<% %>` tags we can create variables (and functions!) inside of templates to use later:

```erb
<%
let h = {name: "mark"}
let greet = fn(n) {
  return "hi " + n
}
%>
<h1><%= greet(h["name"]) %></h1>
```

#### Full Example:

```go
html := `<html>
<%= if (names && len(names) > 0) { %>
	<ul>
		<%= for (n) in names { %>
			<li><%= capitalize(n) %></li>
		<% } %>
	</ul>
<% } else { %>
	<h1>Sorry, no names. :(</h1>
<% } %>
</html>`

ctx := NewContext()
ctx.Set("names", []string{"john", "paul", "george", "ringo"})

s, err := Render(html, ctx)
if err != nil {
  log.Fatal(err)
}

fmt.Print(s)
// output: <html>
// <ul>
// 		<li>John</li>
// 		<li>Paul</li>
// 		<li>George</li>
// 		<li>Ringo</li>
// 		</ul>
// </html>
```
## If/Else Statements

The basic syntax of `if/else` statements is as follows:

```erb
<%
if (true) {
  # do something
} else {
  # do something else
}
%>
```

When using `if/else` statements to control output, remember to use the `<%= %>` tag to output the result of the statement:

```erb
<%= if (true) { %>
  <!-- some html here -->
<% } else { %>
  <!-- some other html here -->
<% } %>
```

### Operators

Complex `if` statements can be built in Plush using "common" operators:

* `==` - checks equality of two expressions
* `!=` - checks that the two expressions are not equal
* `<` - checks the left expression is less than the right expression
* `<=` - checks the left expression is less than or equal to the right expression
* `>` - checks the left expression is greater than the right expression
* `>=` - checks the left expression is greater than or equal to the right expression
* `&&` - requires both the left **and** right expression to be true
* `||` - requires either the left **or** right expression to be true

### Grouped Expressions

```erb
<%= if ((1 < 2) && (someFunc() == "hi")) { %>
  <!-- some html here -->
<% } else { %>
  <!-- some other html here -->
<% } %>
```

## For Loops

## Functions

## Custom Functions (Helpers)

## Maps

## Arrays








### Special Thanks

This package absolutely 100% could not have been written without the help of Thorsten Ball's incredible book, [Writing an Interpeter in Go](https://interpreterbook.com).

Not only did the book make understanding the process of writing lexers, parsers, and asts, but it also provided the basis for the syntax of Plush itself.

If you have yet to read Thorsten's book, I can't recommend it enough. Please go and buy it!
