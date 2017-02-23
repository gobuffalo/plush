# Plush

Plush is the templating system that [Go](http://golang.org) both needs _and_ deserves. Powerful, flexible, and extendable, Plush is there to make writing your templates that much easier.

## Installation

```text
$ go get -u github.com/gobuffalo/plush
```

## Usage

Plush allows for the ebedding of dynamic code inside of your templates. Take the following example:

```erb
<p><%= "plush is great" %></p>
```

Outputs:

```html
<p>plush is great</p>
```

By using the `<%= %>` tags we tell Plush to dynamically render the inner content, in this case the string `plush is great`, into the template between the `<p></p>` tags.

If we were to change the example to use `<% %>` tags instead the inner content will be evaluated and executed, but not injected into the template:

```erb
<p><% "plush is great" %></p>
```

Outputs:

```html
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







```erb
<html>
<%= if (names && len(names) > 0) { %>
	<ul>
		<%= for (n) in names { %>
			<li><%= capitalize(n) %></li>
		<% } %>
	</ul>
<% } else { %>
	<h1>Sorry, no names. :(</h1>
<% } %>
</html>
```


### Special Thanks
