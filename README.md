# Plush [![Code Climate](https://codeclimate.com/github/gobuffalo/plush/badges/gpa.svg)](https://codeclimate.com/github/gobuffalo/plush) [![Build Status](https://travis-ci.org/gobuffalo/plush.svg?branch=master)](https://travis-ci.org/gobuffalo/plush) [![GoDoc](https://godoc.org/github.com/gobuffalo/plush?status.svg)](https://godoc.org/github.com/gobuffalo/plush)

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

## If/Else Statements

## For Loops

## Functions

## Custom Functions (Helpers)

## Maps

## Arrays








### Special Thanks

This package absolutely 100% could not have been written without the help of Thorsten Ball's incredible book, [Writing an Interpeter in Go](https://interpreterbook.com).

Not only did the book make understanding the process of writing lexers, parsers, and asts, but it also provided the basis for the syntax of Plush itself.

If you have yet to read Thorsten's book, I can't recommend it enough. Please go and buy it!
