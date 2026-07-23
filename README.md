# Plush

[![Standard Test](https://github.com/gobuffalo/plush/actions/workflows/standard-go-test.yml/badge.svg)](https://github.com/gobuffalo/plush/actions/workflows/standard-go-test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gobuffalo/plush/v5.svg)](https://pkg.go.dev/github.com/gobuffalo/plush/v5)

Plush is the templating system that [Go](http://golang.org) both needs _and_ deserves. Powerful, flexible, and extendable, Plush is there to make writing your templates that much easier.

**[Introduction Video](https://blog.gobuffalo.io/introduction-to-plush-82a8a12cf98a#.y9t0g4xq2)**

## Installation

```text
$ go get -u github.com/gobuffalo/plush
```

## Usage

Plush allows for the embedding of dynamic code inside of your templates. Take the following example:

```erb
<!-- input -->
<p><%= "plush is great" %></p>

<!-- output -->
<p>plush is great</p>
```

### Controlling Output

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
<!-- does not print output -->
<%
let h = {name: "mark"}
let greet = fn(n) {
  return "hi " + n
}
%>
<!-- prints output -->
<h1><%= greet(h["name"]) %></h1>
```

#### Recursion

Template functions can call themselves by name. Define the function with `let name = fn(...) { ... }`, then call `name(...)` inside the function body. This works in both the interpreter and the compiled VM renderer.

```erb
<%
let countdown = fn(x) {
  if (x == 0) {
    return "done"
  }
  return countdown(x - 1)
}
%>
<%= countdown(3) %>
```

renders:

```html
done
```

Recursive functions can also close over values from the surrounding template scope:

```erb
<%
let remaining = 3
let tick = fn() {
  if (remaining == 0) {
    return "done"
  } else {
    remaining = remaining - 1
    return tick()
  }
}
%>
<%= tick() %>
```

The important rule is that recursive functions need a stopping condition. Without a base case, the function will keep calling itself until the render fails or a configured render budget stops it. Use `let` and `fn`; Go-style `var a = func() { ... }` is not Plush syntax.

#### Whitespace Trim Output

Use `<%- %>` when you want to render an expression and remove contiguous whitespace immediately around that tag:

```erb
<pre>
<%- "Hello" %>
</pre>
```

renders:

```html
<pre>Hello</pre>
```

Only spaces, tabs, `\r`, and `\n` directly before the opening tag and directly after the closing tag are trimmed. `<%- %>` renders and escapes values the same way as `<%= %>`. Existing `<%= %>` whitespace behavior is unchanged.

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

ctx := plush.NewContext()
ctx.Set("names", []string{"john", "paul", "george", "ringo"})

s, err := plush.Render(html, ctx)
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
## Comments

You can add comments like this:

```erb
<%# This is a comment %>
```

You can also add line comments within a code section

```erb
<%
# this is a comment
not_a_comment()
%>
```

## If/Else Statements

The basic syntax of `if/else if/else` statements is as follows:

```erb
<%
if (true) {
  # do something
} else if (false) {
  # do something
} else {
  # do something else
}
%>
```

Parentheses around `if` and `else if` conditions are optional. The older parenthesized form still works:

```erb
<%= if name == "mark" { %>
  hello mark
<% } else if admin { %>
  hello admin
<% } %>
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
* `~=` - checks a string against a regular expression (`foo ~= "^fo"`)
* `<` - checks the left expression is less than the right expression
* `<=` - checks the left expression is less than or equal to the right expression
* `>` - checks the left expression is greater than the right expression
* `>=` - checks the left expression is greater than or equal to the right expression
* `&&` - requires both the left **and** right expression to be true
* `||` - requires either the left **or** right expression to be true

Numeric equality and ordering are safe across common Go numeric types. This lets backend values such as `int32`, `uint32`, `uint64`, `float32`, and `float64` compare against template integer or float literals without type errors:

```erb
<%= product.Count == 0 %>
<%= order.Total > 0.0 %>
<%= uintValue == 3.0 %>
<%= floatValue == 3 %>
```

The same mixed numeric comparison rules apply to struct fields, map values, indexed values, helper returns, and method returns.

### Grouped Expressions

```erb
<%= if ((1 < 2) && (someFunc() == "hi")) { %>
  <!-- some html here -->
<% } else { %>
  <!-- some other html here -->
<% } %>
```

## Maps

Maps in Plush will get translated to the Go type `map[string]interface{}` when used. Creating, and using maps in Plush is not too different than in JSON:

```erb
<% let h = {key: "value", "a number": 1, bool: true} %>
```

Would become the following in Go:

```go
map[string]interface{}{
  "key": "value",
  "a number": 1,
  "bool": true,
}
```

Accessing maps is just like access a JSON object:

```erb
<%= h["key"] %>
```

Go maps passed through the render context use the same bracket syntax, including typed maps from backend code:

```go
ctx.Set("labels", map[string]string{"status": "ready"})
ctx.Set("counts", map[string]uint32{"active": 7})
```

```erb
<%= labels["status"] %>
<%= counts["active"] %>
```

When using the compiled VM renderer, static string-key map access is optimized automatically for typed maps such as `map[string]string`, `map[string]uint32`, and nested chains such as `robots["bender"].Name`. The VM caches the access plan and typed map key, not the map value itself.

Using maps as options to functions in Plush is incredibly powerful. See the sections on Functions and Helpers to see more examples.

## Arrays

Arrays in Plush will get translated to the Go type `[]interface{}` when used.

```erb
<% let a = [1, 2, "three", "four", h] %>
```

```go
[]interface{}{ 1, 2, "three", "four", h }
```

Arrays in plush can be appended using the following format:

```erb
<% let a = [1, 2, "three", "four", h] %> <% a = a + "hello world"%>
```

If the array passed to plush is not of type `[]interface{}` and an attempt is made to append a value with a data type that does not match the underlying array type, an error will be returned. 

## For Loops

There are three different types that can be looped over: maps, arrays/slices, and iterators. The format for them all looks the same:

```erb
<%= for (key, value) in expression { %>
  <%= key %> <%= value %>
<% } %>
```

You can also  `continue` to the next iteration of the loop:
```erb
for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
  if (i > 0) {
    continue
  }
  return v
}
```

You can terminate the for loop with `break`:
```erb
for (i,v) in [1, 2, 3,4,5,6,7,8,9,10] {
  if (i > 5) {
    break
  }
  return v
}
```

The values inside the `()` part of the statement are the names you wish to give to the key (or index) and the value of the expression. The `expression` can be an array, map, or iterator type.

### Arrays

#### Using Index and Value

```erb
<%= for (i, x) in someArray { %>
  <%= i %> <%= x %>
<% } %>
```

#### Using Just the Value

```erb
<%= for (val) in someArray { %>
  <%= val %>
<% } %>
```

### Maps

#### Using Index and Value

```erb
<%= for (k, v) in someMap { %>
  <%= k %> <%= v %>
<% } %>
```

#### Using Just the Value

```erb
<%= for (v) in someMap { %>
  <%= v %>
<% } %>
```

### Iterators

```go
type ranger struct {
	pos int
	end int
}

func (r *ranger) Next() interface{} {
	if r.pos < r.end {
		r.pos++
		return r.pos
	}
	return nil
}

func betweenHelper(a, b int) Iterator {
	return &ranger{pos: a, end: b - 1}
}
```

```go
html := `<%= for (v) in between(3,6) { return v } %>`

ctx := plush.NewContext()
ctx.Set("between", betweenHelper)

s, err := plush.Render(html, ctx)
if err != nil {
  log.Fatal(err)
}
fmt.Print(s)
// output: 45
```

## Default helpers

Plush ships with a comprehensive list of helpers to make your life easier. For more info check the helpers package.

### Custom Helpers

```go
html := `<p><%= one() %></p>
<p><%= greet("mark")%></p>
<%= can("update") { %>
<p>i can update</p>
<% } %>
<%= can("destroy") { %>
<p>i can destroy</p>
<% } %>
`

ctx := NewContext()

// one() #=> 1
ctx.Set("one", func() int {
  return 1
})

// greet("mark") #=> "Hi mark"
ctx.Set("greet", func(s string) string {
  return fmt.Sprintf("Hi %s", s)
})

// can("update") #=> returns the block associated with it
// can("adsf") #=> ""
ctx.Set("can", func(s string, help HelperContext) (template.HTML, error) {
  if s == "update" {
    h, err := help.Block()
    return template.HTML(h), err
  }
  return "", nil
})

s, err := Render(html, ctx)
if err != nil {
  log.Fatal(err)
}
fmt.Print(s)
// output: <p>1</p>
// <p>Hi mark</p>
// <p>i can update</p>
```

## Partial Rendering With Data Maps

Partials can receive a data map as their second argument:

```erb
<%= partial("row.plush", {name: product.Name, title: "Robot"}) %>
```

The keys in the map become local values inside the partial. The values are evaluated from the current render context each time the partial runs.

Parent template:

```erb
<%= partial("row.plush", {name: product.Name, title: product.Category.Label}) %>
<%= product.Name %>
```

Partial source for `row.plush`:

```erb
<span><%= title %>: <%= name %></span>
```

Go setup:

```go
type Product struct {
  Name     string
  Category Category
}

type Category struct {
  Label string
}

ctx := plush.NewContextWith(map[string]interface{}{
  "product": Product{
    Name:     "Bender",
    Category: Category{Label: "Robot"},
  },
  "partialFeeder": func(name string) (string, error) {
    if name == "row.plush" {
      return `<span><%= title %>: <%= name %></span>`, nil
    }
    return "", fmt.Errorf("unknown partial %s", name)
  },
})

input := `<%= partial("row.plush", {name: product.Name, title: product.Category.Label}) %>
<%= product.Name %>`

html, err := plush.Render(input, ctx)
```

This renders the partial with `name` and `title` available as local values:

```html
<span>Robot: Bender</span>
Bender
```

Partial data is scoped to that partial render. Passing `{name: product.Name}` does not replace `product.Name` or `name` in the parent context after the partial finishes.

You can pass literals, variables, struct fields, indexed values, and nested property chains:

```erb
<%= partial("user.plush", {name: user.Name, role: "admin"}) %>
<%= partial("robot.plush", {name: robots[0].Name, echo: robots[0].Name.Echo()}) %>
<%= partial("row.plush", {label: label(product.Name, prefix)}) %>
```

Layouts still use the existing Plush partial behavior:

```erb
<%= partial("card.plush", {name: product.Name, layout: "shell.plush"}) %>
```

When using the compiled VM renderer, simple static-key partial data maps are optimized automatically. The VM reuses the compiled partial bytecode, prepares a small key/value binding plan, and writes the evaluated data values into a scoped partial context. Data-map values may also be helper calls such as `label(product.Name, prefix)`; the VM compiles those calls into the data binding plan and uses direct no-reflect invokers for common helper signatures. It does not cache request data, struct values, helper results, or rendered partial output.

## Performance Recommendations

Plush now has two render engines:

- The classic interpreter, which remains the default.
- The compiled VM renderer, available from `github.com/gobuffalo/plush/v5/VM/plush`.

For one-off template strings, the interpreter is still a good default because the VM has to parse and compile before it can run. For templates rendered repeatedly, use the VM path.

### Best Options

| Use case | Recommended path | Why |
| --- | --- | --- |
| Hot template string reused many times | `vmplush.Compile` once, then `tmpl.Render(ctx)` | Avoids parse and compile on every render |
| File-backed app templates | `plush.SetRenderMode(plush.RenderModeVM)` plus `PlushCacheSetup` | Reuses cached VM bytecode by filename |
| One-off/dynamic template strings | Default `plush.Render` interpreter | Avoids compile overhead when reuse is unlikely |
| App-specific hot helpers | Optional `vmplush.SetFastHelper` | Skips generic reflection for helper hot paths |

### Compiled Template API

Use this when you can hold a compiled template object:

```go
import (
  "log"

  plush "github.com/gobuffalo/plush/v5"
  vmplush "github.com/gobuffalo/plush/v5/VM/plush"
)

tmpl, err := vmplush.Compile(`<p><%= greet(name) %></p>`)
if err != nil {
  log.Fatal(err)
}

ctx := plush.NewContextWith(map[string]interface{}{
  "name": "mark",
  "greet": func(name string) string {
    return "Hi " + name
  },
})

html, err := tmpl.Render(ctx)
```

This is the fastest path for repeated renders because the template is parsed and compiled only once.

### VM Render Mode

Use this when an application already calls the root `plush.Render` function and you want to switch that render path to the VM. `SetRenderMode` is global, so most applications call it once during startup:

```go
import (
  plush "github.com/gobuffalo/plush/v5"
  _ "github.com/gobuffalo/plush/v5/VM/plush"
)

plush.SetRenderMode(plush.RenderModeVM)
```

The blank import registers the VM renderer. Without it, `RenderModeVM` cannot call the VM path.

### Filename Cache

For file-backed templates, enable a template cache. The interpreter stores parsed ASTs in this cache, and the VM stores compiled bytecode on the same cache entry.

```go
import (
  plush "github.com/gobuffalo/plush/v5"
  _ "github.com/gobuffalo/plush/v5/VM/plush"
  "github.com/gobuffalo/plush/v5/helpers/meta"
  "github.com/gobuffalo/plush/v5/templatecache/inmemory"
)

cache := inmemory.NewMemoryCache()
plush.PlushCacheSetup(cache)
plush.SetRenderMode(plush.RenderModeVM)

ctx := plush.NewContext()
ctx.Set(meta.TemplateFileKey, "templates/items/show.plush.html")

html, err := plush.Render(input, ctx)
```

The filename must be present in the context for filename-backed cache reuse. On the first render, the VM compiles the template. On later renders of the same filename, it can reuse bytecode.

### Compiled Render Diagnostics

Plush records lightweight render diagnostics on the render context. Use these when comparing interpreter and VM mode, confirming bytecode-cache hits, or understanding why a template did or did not use the VM fast path. The diagnostics API lives in [`render_diagnostics.go`](render_diagnostics.go).

Diagnostics are collected automatically during `plush.Render`; no global flag is required for the basic fields.

The normal request flow is:

1. Set `meta.TemplateFileKey` on the Plush context for file-backed templates.
2. Render through `plush.Render`, `BuffaloRendererWithContext`, or a compiled VM template.
3. Read diagnostics after the render with `RenderDiagnosticsFromContext` or `RenderDiagnosticsFromData`.
4. Log the values or expose selected values as internal-only response headers.

```go
import (
  "fmt"

  plush "github.com/gobuffalo/plush/v5"
  _ "github.com/gobuffalo/plush/v5/VM/plush"
  "github.com/gobuffalo/plush/v5/helpers/meta"
)

ctx := plush.NewContext()
ctx.Set(meta.TemplateFileKey, "templates/report/show.plush.html")

plush.SetRenderMode(plush.RenderModeVM)
html, err := plush.Render(input, ctx)
if err != nil {
  return err
}

diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
if ok {
  fmt.Printf(
    "mode=%s cache=%s fast=%s engine=%.3fms\n",
    diagnostics.Mode,
    diagnostics.VMBytecodeCache,
    diagnostics.FastPath,
    diagnostics.EngineDurationMilliseconds(),
  )
}

_ = html
```

Useful fields:

| Field | Meaning |
| --- | --- |
| `Mode` | Render engine used for the call: `interpreter` or `vm`. |
| `TemplateFilename` | Clean filename used for filename-backed cache lookup, when one was present in the context. |
| `VMBytecodeCache` | VM cache state, such as `disabled`, `miss`, `miss-store`, `miss-store-source`, `hit`, `hit-static`, `hit-source`, or `compiled-template`. |
| `FastPath` | VM execution path, such as `static`, `fast`, `generic`, or `interpreter-fallback`. |
| `FastReject` / `FastRejectLine` | Reason and source line when the compiler could not build a fast render plan. |
| `PunchHoleCache` | Punch-hole cache state, such as `disabled`, `hit`, or `miss`. |
| `EngineDuration` | Time spent inside Plush rendering. Use `EngineDurationMilliseconds()` for reporting. |
| `FastPlan` | Static complexity counters for the compiled fast plan: bindings, segments, static segments, name segments, property reads, value writes, helper calls, conditionals, loops, loop parts, partials, max depth, helper names, and partial names. |
| `VMHotspots` | Optional helper and partial call counts/timings when VM hotspot diagnostics are enabled. |

`FastPlan` describes the compiled template shape, not elapsed time. It includes bindings, segments, static segments, name segments, property reads, value writes, helper calls, conditionals, loops, loop parts, partials, max depth, helper names, and partial names.

Hotspot diagnostics are optional. They time VM helper and partial calls, so enable them only when profiling or sampling because they add measurement overhead:

```go
ctx := plush.NewContext()
ctx.Set(meta.TemplateFileKey, "templates/report/show.plush.html")
plush.EnableRenderVMHotspotDiagnostics(ctx)

_, err := plush.Render(input, ctx)
if err != nil {
  return err
}

diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
if ok {
  fmt.Printf("helper calls: %d\n", diagnostics.VMHotspots.HelperCalls)
  fmt.Printf("helper time: %.3fms\n", diagnostics.VMHelperDurationMilliseconds())
  fmt.Printf("partial calls: %d\n", diagnostics.VMHotspots.PartialCalls)
  fmt.Printf("partial time: %.3fms\n", diagnostics.VMPartialDurationMilliseconds())
  fmt.Println("helper hotspots:", diagnostics.VMHelperHotspotsHeader())
  fmt.Println("partial hotspots:", diagnostics.VMPartialHotspotsHeader())
}
```

`VMHelperHotspotsHeader()` and `VMPartialHotspotsHeader()` return a compact `name:calls:time_ms` list sorted by total time, for example `formatValue:7:34.660;layout.plush:1:26.120`. The list is meant for diagnostics and A/B testing; it is not a stable application data format.

If your renderer creates a Plush context internally and then copies local values back into a data map, use `RenderDiagnosticsFromData` after rendering. `BuffaloRendererWithContext` is useful when you also need to set `meta.TemplateFileKey` or enable hotspot diagnostics:

```go
data := map[string]interface{}{
  "title": "Dashboard",
}

html, err := plush.BuffaloRendererWithContext(input, data, helpers, func(ctx *plush.Context) {
  ctx.Set(meta.TemplateFileKey, "templates/report/show.plush.html")
  plush.EnableRenderVMHotspotDiagnostics(ctx)
})
if err != nil {
  return err
}

diagnostics, ok := plush.RenderDiagnosticsFromData(data)
if ok {
  fmt.Printf("mode=%s cache=%s helper_time=%.3fms\n",
    diagnostics.Mode,
    diagnostics.VMBytecodeCache,
    diagnostics.VMHelperDurationMilliseconds(),
  )
}

_ = html
```

If an HTTP integration wants response headers, read the diagnostics after rendering and before writing the HTTP response, then map the fields explicitly:

```go
diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
if ok {
  w.Header().Set("X-Plush-Render-Mode", diagnostics.Mode)
  w.Header().Set("X-Plush-VM-Bytecode-Cache", diagnostics.VMBytecodeCache)
  w.Header().Set("X-Plush-Template-Filename", diagnostics.TemplateFilename)
  w.Header().Set("X-Plush-Fast-Path", diagnostics.FastPath)
  w.Header().Set("X-Plush-Render-Engine-Time-Ms",
    fmt.Sprintf("%.3f", diagnostics.EngineDurationMilliseconds()))
  w.Header().Set("X-Plush-Punch-Hole-Cache", diagnostics.PunchHoleCache)
  w.Header().Set("X-Plush-Fast-Plan-Bindings",
    fmt.Sprintf("%d", diagnostics.FastPlan.Bindings))
  w.Header().Set("X-Plush-Fast-Plan-Segments",
    fmt.Sprintf("%d", diagnostics.FastPlan.Segments))
  w.Header().Set("X-Plush-Fast-Plan-Static-Segments",
    fmt.Sprintf("%d", diagnostics.FastPlan.StaticSegments))
  w.Header().Set("X-Plush-Fast-Plan-Name-Segments",
    fmt.Sprintf("%d", diagnostics.FastPlan.NameSegments))
  w.Header().Set("X-Plush-Fast-Plan-Property-Reads",
    fmt.Sprintf("%d", diagnostics.FastPlan.PropertyReads))
  w.Header().Set("X-Plush-Fast-Plan-Value-Writes",
    fmt.Sprintf("%d", diagnostics.FastPlan.ValueWrites))
  w.Header().Set("X-Plush-Fast-Plan-Helper-Calls",
    fmt.Sprintf("%d", diagnostics.FastPlan.HelperCalls))
  w.Header().Set("X-Plush-Fast-Plan-Conditionals",
    fmt.Sprintf("%d", diagnostics.FastPlan.Conditionals))
  w.Header().Set("X-Plush-Fast-Plan-Loops",
    fmt.Sprintf("%d", diagnostics.FastPlan.Loops))
  w.Header().Set("X-Plush-Fast-Plan-Loop-Parts",
    fmt.Sprintf("%d", diagnostics.FastPlan.LoopParts))
  w.Header().Set("X-Plush-Fast-Plan-Partials",
    fmt.Sprintf("%d", diagnostics.FastPlan.Partials))
  w.Header().Set("X-Plush-Fast-Plan-Max-Depth",
    fmt.Sprintf("%d", diagnostics.FastPlan.MaxDepth))
  w.Header().Set("X-Plush-Fast-Plan-Helper-Names",
    diagnostics.FastPlanHelperNamesHeader())
  w.Header().Set("X-Plush-Fast-Plan-Partial-Names",
    diagnostics.FastPlanPartialNamesHeader())
  w.Header().Set("X-Plush-VM-Helper-Calls",
    fmt.Sprintf("%d", diagnostics.VMHotspots.HelperCalls))
  w.Header().Set("X-Plush-VM-Helper-Time-Ms",
    fmt.Sprintf("%.3f", diagnostics.VMHelperDurationMilliseconds()))
  w.Header().Set("X-Plush-VM-Partial-Calls",
    fmt.Sprintf("%d", diagnostics.VMHotspots.PartialCalls))
  w.Header().Set("X-Plush-VM-Partial-Time-Ms",
    fmt.Sprintf("%.3f", diagnostics.VMPartialDurationMilliseconds()))
  w.Header().Set("X-Plush-VM-Helper-Hotspots",
    diagnostics.VMHelperHotspotsHeader())
  w.Header().Set("X-Plush-VM-Partial-Hotspots",
    diagnostics.VMPartialHotspotsHeader())
  w.Header().Set("Server-Timing",
    fmt.Sprintf(
      "plush;dur=%.3f, plush_helpers;dur=%.3f, plush_partials;dur=%.3f",
      diagnostics.EngineDurationMilliseconds(),
      diagnostics.VMHelperDurationMilliseconds(),
      diagnostics.VMPartialDurationMilliseconds(),
    ))
}
```

These `X-Plush-*` and `Server-Timing` headers are not emitted by Plush automatically. Plush records the diagnostics; the HTTP application decides which values to expose. In production, put these headers behind a debug, benchmark, or internal-only flag because filenames, helper names, and partial names may reveal application structure.

Common header meanings:

| Header | Meaning |
| --- | --- |
| `X-Plush-Render-Mode` | `interpreter` or `vm`, showing which renderer handled this request. |
| `X-Plush-VM-Bytecode-Cache` | VM cache status. Warm file-backed templates should usually move from `miss-store` on first render to `hit`, `hit-static`, or `hit-source` on later renders. |
| `X-Plush-Template-Filename` | Filename used as the cache key when `meta.TemplateFileKey` was set on the context. |
| `X-Plush-Render-Engine-Time-Ms` | Time spent inside Plush rendering only, in milliseconds. Use this instead of total request time when comparing interpreter vs VM. |
| `X-Plush-Fast-Path` | VM execution path: `static`, `fast`, `generic`, or `interpreter-fallback`. The best steady-state VM path is usually `fast`; `interpreter-fallback` means the VM intentionally delegated unsupported syntax to the old interpreter. |
| `X-Plush-Punch-Hole-Cache` | Punch-hole cache status for templates that mix static HTML with embedded Plush code. |
| `X-Plush-Fast-Plan-*` | Static counters captured from the compiled fast plan. These describe template complexity, not elapsed time. |
| `X-Plush-Fast-Plan-Helper-Names` | Helper names the fast plan found while compiling. Useful for spotting expensive helper-heavy templates. |
| `X-Plush-Fast-Plan-Partial-Names` | Partial names the fast plan found while compiling. Useful for spotting partial-heavy templates. |
| `X-Plush-VM-Helper-*` | Optional helper-call count and timing fields. They are zero unless `EnableRenderVMHotspotDiagnostics` was enabled for that context. |
| `X-Plush-VM-Partial-*` | Optional partial-call count and timing fields. They are zero unless `EnableRenderVMHotspotDiagnostics` was enabled for that context. |
| `Server-Timing` | Browser/devtools-friendly timing summary. The example maps Plush engine time plus optional VM helper and partial hotspot time into server timing metrics. |

For fair VM measurements, warm file-backed templates until `VMBytecodeCache` reports `hit` and `FastPath` reports `fast`. A first render may report `miss-store` because the VM is parsing, compiling, storing bytecode, and then rendering. Fast-plan counters describe template shape, not elapsed time; use `EngineDurationMilliseconds`, `VMHelperDurationMilliseconds`, and `VMPartialDurationMilliseconds` for timings.

### Measured Gains

Latest local benchmark checkpoint, VM bytecode cache compared with interpreter AST cache:

| Benchmark set | Time | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Full render-mode matrix, 26 scenarios | 61.2% faster | 59.6% less | 71.8% fewer |
| Realistic mixed template | 59.6% faster | 73.3% less | 81.3% fewer |
| Simple conditional access output | 45.0% faster | 32.8% less | 65.6% fewer |
| Nested access/simple access output | 55.4% faster | 56.7% less | 61.8% fewer |
| Partial simple access bodies | 52.5% faster | 49.6% less | 70.6% fewer |
| Partial rendering with data maps | 53.6% faster | 55.6% less | 68.9% fewer |
| Partial data maps with helper-call values | 63.4% faster | 53.5% less | 68.0% fewer |
| Typed map access | 35.6% faster | 24.8% less | 56.8% fewer |
| Loop helper direct string calls | 69.6% faster | 50.6% less | 74.5% fewer |
| Loop helper direct int calls | 58.9% faster | 51.1% less | 70.3% fewer |
| Mixed numeric operator output | 46.4% faster | 28.3% less | 61.5% fewer |

The full matrix includes static output, variable interpolation, helpers, loops, struct fields, nested access, conditionals, regex, partials, whitespace trim, and mixed numeric comparisons. These numbers come from the latest local post-profile checkpoint using medians from `count=3` / `500ms`. The exact percentage will move with hardware and template shape, but repeated cached VM renders should usually be materially faster than interpreter AST-cache renders.

This checkpoint includes the partial no-overlay fast path, stack-backed partial data locals, helper-call partial data value plans, cleaner cache-key/punch-hole lookup, and lower-allocation request context construction. Some percentages moved because those context changes speed up the interpreter too; the VM partial rows still improved sharply in absolute `ns/op`, `B/op`, and `allocs/op`.

Important caveat: one-shot VM rendering is not the headline number because it includes parse and compile work. Use cached VM render or compiled-template reuse for the real compiled-performance path.

Run the benchmark matrix locally:

```sh
go test ./VM/vm -run '^$' -bench '^BenchmarkRenderModeMatrix/.*/(interpreter_ast_cache|vm_bytecode_cache)$' -benchmem -count=3 -benchtime=500ms
```

### Automatic VM Fast Paths

Most VM optimizations need no template changes. The VM automatically specializes common Plush shapes:

- static output and mixed static/name templates
- simple mixed templates that combine static text, names, property/access chains, and rendered infix booleans
- repeated name lookups
- safe mixed numeric comparisons such as `uint32 == 0` and `float32 == 3`
- struct fields, nested property chains, indexed property chains, and no-arg method tails
- typed Go map access with static string keys, such as `labels["status"]` and `robots["bender"].Name`
- loops over strings, slices, structs, and pointers to structs
- simple top-level conditionals whose branches contain static/name/property/access/infix output
- conditionals and infix conditions inside loops
- direct helper calls for common helper signatures
- direct scalar helper calls for common string, int, uint32, bool, and float64 argument shapes
- regex match expressions
- partials with no data, including direct linked rendering when the partial body is simple and does not need partial metadata
- partials with simple static-key data maps, such as `partial("row", {name: product.Name})`; the VM prepares the keys and value lookup plan, then reads fresh values each render
- partial data maps with helper-call values, such as `partial("row", {label: label(product.Name, prefix)})`; the VM compiles the call arguments and uses direct value invokers for common helper signatures
- linked partial bodies that contain simple property/access/infix output, such as `<%= robot.Name %>` or `<%= labels["status"] %>`
- clean filename cache keys and punch-hole filename checks for file-backed cached renders

The VM caches plans and bytecode, not request values. It does not cache the current product, helper return values, rendered HTML, partial output, or branch decisions.

Helpers whose Go function signature uses an application-defined scalar parameter, such as `func(ProductName, string) string`, still use the safe reflective call path unless an app registers a custom fast helper. Go does not safely convert `func(ProductName, string) string` into `func(string, string) string` even when `ProductName` is backed by `string`.

### Advanced Fast Helpers

The VM already specializes common helper shapes at runtime. For example, this normal helper needs no extra setup:

```go
ctx.Set("greet", func(name string) string {
  return "Hi " + name
})
```

```erb
<%= greet(name) %>
```

For app-specific hot helpers that use broad types like `interface{}` or complex domain values, you can optionally register a custom fast helper. Keep the normal helper in the context for correctness and fallback.

```go
ctx.Set("money", func(value interface{}) string {
  return formatMoney(value)
})

vmplush.SetFastHelper(ctx, "money", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
  amount, ok := args.Float64(0)
  if !ok {
    return vmplush.ErrFastUnsupported // fall back to the normal helper
  }

  w.WriteEscapedString(formatMoneyFloat(amount))
  return nil
})
```

The template stays normal:

```erb
<%= money(amount) %>
```

Fast helpers should only optimize the hot path. They must not cache request values or rendered output. Use `WriteEscapedString` for normal text and `WriteHTML` only for trusted `template.HTML`. If a fast helper cannot safely handle the current arguments, return `vmplush.ErrFastUnsupported` so the VM can call the regular helper.

For generated gRPC/protobuf objects, use `args.Raw(i)` and type assert to the generated message type:

```go
ctx.Set("productName", func(value interface{}) string {
  product, ok := value.(*pb.Product)
  if !ok || product == nil {
    return ""
  }
  return product.GetName()
})

vmplush.SetFastHelper(ctx, "productName", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
  raw, ok := args.Raw(0)
  if !ok {
    return vmplush.ErrFastUnsupported
  }

  product, ok := raw.(*pb.Product)
  if !ok || product == nil {
    return vmplush.ErrFastUnsupported
  }

  w.WriteEscapedString(product.GetName())
  return nil
})
```

The template does not change:

```erb
<%= productName(product) %>
```

This avoids reflection and generic property access for the hot helper body while still keeping the normal helper as a safe fallback.

For nested Go structs, the pattern is the same. Register a normal helper first:

```go
type Product struct {
  Name     string
  Category Category
}

type Category struct {
  Label string
}

ctx.Set("productLabel", func(value interface{}) string {
  product, ok := value.(Product)
  if !ok {
    return ""
  }
  return product.Category.Label + ":" + product.Name
})
```

Then add an optional fast helper for the hot path:

```go
vmplush.SetFastHelper(ctx, "productLabel", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
  raw, ok := args.Raw(0)
  if !ok {
    return vmplush.ErrFastUnsupported
  }

  product, ok := raw.(Product)
  if !ok {
    return vmplush.ErrFastUnsupported
  }

  w.WriteEscapedString(product.Category.Label + ":" + product.Name)
  return nil
})
```

If the context stores pointers, assert the pointer type and nil-check it:

```go
product, ok := raw.(*Product)
if !ok || product == nil {
  return vmplush.ErrFastUnsupported
}

w.WriteEscapedString(product.Category.Label + ":" + product.Name)
```

Normal templates can still use regular Plush access without a fast helper:

```erb
<%= product.Category.Label %>
```

Partial calls with simple data maps are also optimized automatically by the VM. See [Partial Rendering With Data Maps](#partial-rendering-with-data-maps) for syntax and behavior.

## Render Budget

Plush lets you attach a work-unit **budget** to any render to protect against runaway templates — deeply nested loops, recursive partials, or unexpectedly expensive helpers.

A **nil budget = unlimited**, so all existing code is completely unaffected.

### Quick start

```go
b := plush.NewBudget(10_000)
ctx := plush.NewContext()
ctx.Set("products", products)
ctx.WithBudget(b)

html, err := plush.Render(tmpl, ctx)
if errors.Is(err, plush.ErrBudgetExceeded) {
    log.Printf("budget exceeded: used=%d remaining=%d", b.Used(), b.Remaining())
    return errorPage()
}

// One-liner convenience wrapper
html, err = plush.RenderWithBudget(tmpl, 10_000, ctx)
```

### Default operation costs

| Operation | Default cost |
|---|---|
| Loop iteration | 1 |
| Helper / function call | 5 |
| Filter call | 3 |
| Partial / sub-render | 10 |
| Condition check (`if`) | 1 |
| Variable assignment | 0 |
| Object traversal (per segment) | 1 |

### Custom costs

Pass a `BudgetCosts` struct to override any cost:

```go
costs := plush.ZeroCosts()          // start from all-zero
costs.LoopIteration = 1
costs.SubRender     = 25

html, err = plush.RenderWithBudgetConfig(tmpl, 5_000, costs, ctx)
```

### Per-function costs

Override the cost for individual functions registered in the context:

```go
costs := plush.DefaultBudgetCosts()
costs.FunctionCosts = map[string]int64{
    "expensiveQuery": 50, // charged 50 per call instead of the default 5
    "cheapHelper":     1,
}

html, err = plush.RenderWithBudgetConfig(tmpl, 10_000, costs, ctx)
```

Functions not listed in `FunctionCosts` fall back to the `HelperCall` cost.

### Stats report

After rendering, call `b.Stats()` to see exactly where the budget was spent:

```go
b := plush.NewBudget(10_000)
ctx.WithBudget(b)
plush.Render(tmpl, ctx)

s := b.Stats()
fmt.Printf("total=%d  loops=%d  calls=%d  conditions=%d\n",
    s.TotalUsed, s.LoopIterations, s.FunctionCalls, s.ConditionChecks)

for name, units := range s.ByFunction {
    fmt.Printf("  %s: %d units\n", name, units)
}
```

`BudgetStats` fields:

| Field | What it measures |
|---|---|
| `TotalUsed` | Sum of all units spent |
| `LoopIterations` | Units from loop iterations |
| `FunctionCalls` | Units from all function/helper calls |
| `FilterCalls` | Units from filter calls |
| `SubRenders` | Units from partial renders |
| `ConditionChecks` | Units from `if`/`unless` evaluations |
| `Assignments` | Units from variable assignments |
| `ObjectTraversals` | Units from dot-notation traversal |
| `ByFunction` | Per-function breakdown (map of name → units) |



### Special Thanks

This package absolutely, 100%, could not have been written without the help of Thorsten Ball’s incredible books, [Writing an Interpreter in Go](https://interpreterbook.com) and [Writing a Compiler in Go](https://compilerbook.com/).

Not only did the book make understanding the process of writing lexers, parsers, and asts, compilers, vm but it also provided the basis for the syntax of Plush itself.

If you have yet to read Thorsten's book, I can't recommend it enough. Please go and buy it!

---
