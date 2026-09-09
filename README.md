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
<%= item.Count == 0 %>
<%= totalValue > 0.0 %>
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

When using the compiled VM renderer, static string-key map access is optimized automatically for typed maps such as `map[string]string`, `map[string]uint32`, and nested chains such as `records["primary"].Name`. The VM caches the access plan and typed map key, not the map value itself.

Using maps as options to functions in Plush is useful for passing named options into helpers.

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
<%= partial("row.plush", {name: record.Name, title: "Example"}) %>
```

The keys in the map become local values inside the partial. The values are evaluated from the current render context each time the partial runs.

Parent template:

```erb
<%= partial("row.plush", {name: record.Name, title: record.Meta.Label}) %>
<%= record.Name %>
```

Partial source for `row.plush`:

```erb
<span><%= title %>: <%= name %></span>
```

Go setup:

```go
type Record struct {
  Name string
  Meta Metadata
}

type Metadata struct {
  Label string
}

ctx := plush.NewContextWith(map[string]interface{}{
  "record": Record{
    Name: "Alpha",
    Meta: Metadata{Label: "Example"},
  },
  "partialFeeder": func(name string) (string, error) {
    if name == "row.plush" {
      return `<span><%= title %>: <%= name %></span>`, nil
    }
    return "", fmt.Errorf("unknown partial %s", name)
  },
})

input := `<%= partial("row.plush", {name: record.Name, title: record.Meta.Label}) %>
<%= record.Name %>`

html, err := plush.Render(input, ctx)
```

This renders the partial with `name` and `title` available as local values:

```html
<span>Example: Alpha</span>
Alpha
```

Partial data is scoped to that partial render. Passing `{name: record.Name}` does not replace `record.Name` or `name` in the parent context after the partial finishes.

Nested compiled partials preserve lexical locals and shadowing across each
partial boundary. When a nested partial fails, its template trace retains the
inner source line, including errors raised inside a block helper body, instead
of replacing it with the outer helper-call line.

You can pass literals, variables, struct fields, indexed values, and nested property chains:

```erb
<%= partial("item.plush", {name: item.Name, kind: "example"}) %>
<%= partial("row.plush", {name: records[0].Name, echo: records[0].Name.Echo()}) %>
<%= partial("row.plush", {label: label(record.Name, prefix)}) %>
```

Layouts still use the existing Plush partial behavior:

```erb
<%= partial("card.plush", {name: record.Name, layout: "shell.plush"}) %>
```

When using the compiled VM renderer, simple static-key partial data maps are optimized automatically. The VM reuses the compiled partial bytecode, prepares a small key/value binding plan, and writes the evaluated data values into a scoped partial context. Data-map values may also be helper calls such as `label(record.Name, prefix)`; the VM compiles those calls into the data binding plan and uses direct no-reflect invokers for common helper signatures. It does not cache request data, struct values, helper results, or rendered partial output.

## Performance Recommendations

Plush now has two render engines:

- The classic interpreter, which remains the default.
- The compiled VM renderer, available from `github.com/gobuffalo/plush/v5/vm/plush`.

For one-off template strings, the interpreter is still a good default because the VM has to parse and compile before it can run. For templates rendered repeatedly, use the VM path.

### Best Options

| Use case | Recommended path | Why |
| --- | --- | --- |
| Hot template string reused many times | `vmplush.Compile` once, then `tmpl.Render(ctx)` | Avoids parse and compile on every render |
| File-backed app templates | `plush.SetRenderMode(plush.RenderModeVM)` plus `PlushCacheSetup` | Reuses cached VM bytecode by filename |
| Watched file-backed templates | Trusted partial and top-level bytecode cache APIs | Skips warm source reads and source hashing when the application owns invalidation |
| One-off/dynamic template strings | Default `plush.Render` interpreter | Avoids compile overhead when reuse is unlikely |
| App-specific hot helpers | Optional `vmplush.SetFastHelper` and context-appropriate fast value-helper registration | Skips generic reflection and avoids unnecessary context work on helper hot paths |

### Compiled Template API

Use this when you can hold a compiled template object:

```go
import (
  "log"

  plush "github.com/gobuffalo/plush/v5"
  vmplush "github.com/gobuffalo/plush/v5/vm/plush"
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
  _ "github.com/gobuffalo/plush/v5/vm/plush"
)

plush.SetRenderMode(plush.RenderModeVM)
```

The blank import registers both the normal VM renderer and the trusted
filename-cache renderer. Without it, `RenderModeVM` cannot call the VM path.
Applications normally do not call `RegisterVMRenderer` or
`RegisterTrustedVMCacheRenderer` directly.

### Filename Cache

For file-backed templates, enable a template cache. The interpreter stores parsed ASTs in this cache, and the VM stores compiled bytecode on the same cache entry.

```go
import (
  plush "github.com/gobuffalo/plush/v5"
  _ "github.com/gobuffalo/plush/v5/vm/plush"
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

The filename must be present in the context for filename-backed cache reuse. On the first render, the VM compiles the template. On later renders of the same filename, it can reuse bytecode. The regular path still receives the current source and compares its hash before using the cached entry. It therefore avoids parsing and compilation, but a file-backed application may still read the source before calling Plush.

### Trusted File-Backed Bytecode Cache

Applications that monitor their template files and reliably invalidate cache entries can opt into filename-first bytecode reuse. This lets warm VM renders execute cached bytecode before reading or hashing template source.

The trusted mode uses the existing template cache; it does not create a second bytecode cache and does not cache rendered HTML, request data, helper results, or template source strings.

Configure `PlushCacheSetup`, select `RenderModeVM`, and register the VM renderer as shown in the preceding sections before using this mode.

#### Trusted partials

Enable trusted partial bytecode reuse on each render context:

```go
html, err := plush.BuffaloRendererWithContext(input, data, helpers, func(ctx *plush.Context) {
  plush.EnableTrustedPartialBytecodeCache(ctx)
})
```

The setting is inherited by child contexts. On the first partial call, Plush uses `partialFeeder`, compiles the source, and stores its bytecode. Later calls can find that bytecode by filename before invoking `partialFeeder`. Current context values and the partial data map are still evaluated for every call.

The bytecode cache accepts `.plush`, `.plush.html`, and plain `.html` template
filenames.

Do not enable this option for a dynamic `partialFeeder` that can return different source for the same filename without cache invalidation. Without trusted mode, partials retain source-hash validation.

The complete per-context control surface is:

```go
plush.EnableTrustedPartialBytecodeCache(ctx)
plush.DisableTrustedPartialBytecodeCache(ctx)
plush.SetTrustedPartialBytecodeCache(ctx, enabled)
enabled := plush.TrustedPartialBytecodeCacheEnabled(ctx)
```

#### Trusted top-level templates

Top-level pages and layouts need a physical cache filename that is unique to the source file. A file-backed renderer can try the cache before loading source and fall back to its normal read path on a miss:

```go
import (
  "os"

  plush "github.com/gobuffalo/plush/v5"
  "github.com/gobuffalo/plush/v5/helpers/meta"
  _ "github.com/gobuffalo/plush/v5/vm/plush"
)

func renderTemplate(
  logicalFilename string,
  physicalFilename string,
  data map[string]interface{},
  helpers map[string]interface{},
) (string, error) {
  data[meta.TemplateFileKey] = logicalFilename

  configure := func(ctx *plush.Context) {
    plush.EnableTrustedPartialBytecodeCache(ctx)
  }

  if html, hit, err := plush.BuffaloRendererFromTrustedBytecodeCache(
    physicalFilename,
    data,
    helpers,
    configure,
  ); err != nil || hit {
    return html, err
  }

  source, err := os.ReadFile(physicalFilename)
  if err != nil {
    return "", err
  }

  return plush.BuffaloRendererWithContext(string(source), data, helpers, func(ctx *plush.Context) {
    plush.EnableTrustedPartialBytecodeCache(ctx)
    plush.SetTrustedTopLevelBytecodeCacheFilename(ctx, physicalFilename)
  })
}
```

`BuffaloRendererFromTrustedBytecodeCache` returns `hit == false` when the cache entry is missing, VM mode is disabled, trusted mode was not enabled, or the bytecode requires source-backed interpreter fallback. In those cases, load the source and render normally as shown above.

The physical cache filename is separate from `meta.TemplateFileKey`. Templates and partial path resolution continue to see the logical filename, while the cache can distinguish tenant, template version, and locale files. Include those identities in the physical path:

```text
/templates/client-1/template-a/templates/page.plush.html
/templates/client-2/template-b/templates/page.plush.html
/templates/client-2/template-b/templates/page.plush.fr-ca.html
```

`SetTrustedTopLevelBytecodeCacheFilename` applies to the next top-level VM render only. Plush consumes it before executing child renders so punch holes and nested templates cannot inherit the parent file's cache identity.
`TrustedTopLevelBytecodeCacheFilename` exposes the pending value for custom
file-backed renderers; ordinary callers do not need to read it.

Custom VM integrations can register a compatible `plush.TrustedVMCacheRenderFunc` with `plush.RegisterTrustedVMCacheRenderer`. The standard integration exposes that function directly as `vmplush.RenderTrustedBytecodeCache`.

#### Invalidation requirements

Trusted bytecode is correct only while the application keeps filename cache entries synchronized with the files. When a monitored template changes or is deleted, remove every cache key that may resolve to that file:

```go
cache.Delete(plush.GenerateASTKey(physicalFilename))
```

If an application supports aliases, underscore partial names, locale fallbacks, or logical filename shortcuts, invalidate all corresponding physical and logical keys. File additions and deletions should also clear the application's filename-resolution or alias cache. The next request will miss, read the selected source, compile it, and repopulate the bytecode cache.

Use a full physical path or another globally unique identity when multiple clients share one Plush cache. Two different sources must never use the same trusted cache filename. Interpreter mode does not use the filename-first bytecode path and continues loading top-level and partial source normally.

### VM Context and Partial Compatibility

Recent VM runtime improvements do not require template or helper API changes:

- Frame-local partial contexts keep their first eight local values inline, reuse
  overflow storage for larger scopes, and return pooled contexts after scoped
  loop, partial, and registered fast-helper calls. Custom contexts still use
  their own `New` implementation.
- Fast loop calls reuse argument storage, and fast-helper registry lookup uses a
  typed context lookup when the context supports it. Compatibility contexts
  continue through the existing `Value` lookup.
- Compiled partials containing supported block helpers, including form blocks
  nested inside loops and conditionals, stay on the VM fast path. A block whose
  bytecode cannot use a compatible fast plan still uses the configured
  interpreter fallback.
- Locals and shadowed names remain scoped correctly through nested partial
  chains. Errors raised inside a block helper preserve the innermost template
  line in the partial trace instead of reporting only the outer helper line.

The pooling and inline-storage bounds are performance choices, not template
limits. Larger scopes continue through overflow storage with the same behavior.

### Compiled Render Diagnostics

Plush records lightweight render diagnostics on the render context. Use these when comparing interpreter and VM mode, confirming bytecode-cache hits, or understanding why a template did or did not use the VM fast path.

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
  _ "github.com/gobuffalo/plush/v5/vm/plush"
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
| `VMHotspots` | Optional helper and partial call counts/timings when VM hotspot diagnostics are enabled. Helper diagnostics also separate direct invocations from reflective compatibility calls and retain bounded helper/signature details. |
| `PartialFallbacks` | Optional aggregate and bounded per-partial details for partials that entered the interpreter during a VM render. Enable collection with `EnableRenderPartialFallbackDiagnostics`. |

#### VM Execution Paths

`Mode` and `FastPath` describe different layers of rendering. `Mode` says
which top-level renderer was selected. `FastPath` says which path that renderer
used after parsing or loading cached bytecode.

When `Mode` is `interpreter`, Plush uses the classic AST interpreter and VM
bytecode is disabled for that render.

When `Mode` is `vm`, Plush enters the compiled VM renderer. From there,
`FastPath` can report:

- `static`: the template compiled to static output and no runtime execution was
  needed.
- `fast`: the VM compiled bytecode and used an optimized fast render plan.
- `generic`: the VM compiled bytecode, but the specialized fast planner either
  was not needed or used a generic VM segment for syntax it does not model as a
  custom fast operation yet.
- `interpreter-fallback`: the VM renderer intentionally delegated this render to
  the classic AST interpreter.

The old interpreter remains a compatibility safety net. Plush templates can
exercise many subtle behaviors: helper block scoping, partial context overlays,
form helpers that inject values into a block, assignments, punch holes, budgets,
and older template quirks. The fast render planner is intentionally
conservative; it only handles syntax it can render with the same behavior as the
normal renderer.

If the interpreter fallback is disabled, templates that cannot use specialized
fast operations should run through the `generic` VM bytecode path instead. That
is still VM execution, but it means any remaining VM parity gap becomes visible
as a render error or output difference instead of falling back to the known-good
interpreter behavior. Template-defined functions and function literals use this
generic VM path. Punch holes remain on the punch-hole-specific path so skeleton
caching and hole filling keep their existing behavior. If the classic
interpreter is removed entirely, `RenderModeInterpreter` and
`interpreter-fallback` are no longer available, and applications must rely on VM
compile/runtime support for every template shape they render.

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
  fmt.Printf("direct helper calls: %d in %.3fms\n",
    diagnostics.VMHotspots.HelperDirectCalls,
    diagnostics.VMHelperDirectDurationMilliseconds())
  fmt.Printf("reflection helper calls: %d in %.3fms (%.2f%%)\n",
    diagnostics.VMHotspots.HelperReflectionCalls,
    diagnostics.VMHelperReflectionDurationMilliseconds(),
    diagnostics.VMHelperReflectionPercent())
  fmt.Printf("partial calls: %d\n", diagnostics.VMHotspots.PartialCalls)
  fmt.Printf("partial time: %.3fms\n", diagnostics.VMPartialDurationMilliseconds())
  fmt.Println("helper hotspots:", diagnostics.VMHelperHotspotsHeader())
  fmt.Println("helper call paths:", diagnostics.VMHelperCallPathsHeader())
  fmt.Println("helper call details:", diagnostics.VMHelperCallPathDetailsHeader())
  fmt.Println("partial hotspots:", diagnostics.VMPartialHotspotsHeader())
}
```

`VMHelperHotspotsHeader()` and `VMPartialHotspotsHeader()` return a compact `name:calls:time_ms` list sorted by total time, for example `formatValue:7:34.660;layout.plush:1:26.120`. The list is meant for diagnostics and A/B testing; it is not a stable application data format.

Helper invocation-path diagnostics answer a separate question: whether VM helper calls used ordinary statically typed Go calls or `reflect.Value.Call`:

- `direct` includes generated direct invokers, specialized direct writers, and explicitly registered `FastHelperFunc` or `FastValueHelperFunc` handlers.
- `reflection` is the general compatibility path for signatures that Plush cannot call through one of its compiled invokers. Its duration includes reflective argument preparation and the call.
- `unclassified` covers calls recorded through the older public `AddRenderDiagnosticVMHelperTiming` API. VM call sites use `direct` or `reflection`.

`VMHelperCallPathsHeader()` reports exact aggregate counts, time, and the percentage of all recorded helper calls that used reflection. `VMHelperCallPathDetailsHeader()` reports retained `path`, helper `name`, Go `signature`, call count, and time. For example:

```text
direct-calls=148;reflection-calls=12;unclassified-calls=0;reflection-percent=7.50;direct-time-ms=1.420;reflection-time-ms=3.810;direct-details-dropped=0;reflection-details-dropped=0

path=reflection,name=formatValue,signature=func(example.Value) string,calls=12,time-ms=3.810|path=direct,name=capitalize,signature=func(string) string,calls=148,time-ms=1.420
```

#### Detecting Helper Fast-Path Escapes

A helper fast-path escape occurs when the VM cannot use a typed direct invoker
and uses the reflection compatibility path instead. This does not change helper
behavior or rendered output. It identifies a call that may benefit from a direct
invoker when it is frequent or expensive.

To detect escapes:

1. Enable `EnableRenderVMHotspotDiagnostics` on the render context.
2. Render a representative set of templates and input shapes.
3. Check `HelperReflectionCalls` and `VMHelperReflectionPercent()`.
4. Inspect `HelperCallPaths` or `VMHelperCallPathDetailsHeader()` to find the
   helper name, Go function signature, call count, and elapsed time.
5. Add a generated direct invoker for a generally useful signature, or register
   an application-specific `FastHelperFunc` or `FastValueHelperFunc`.
6. Repeat the same renders and confirm that the helper reports `path=direct`.

```go
diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
if ok && diagnostics.VMHotspots.HelperReflectionCalls > 0 {
  fmt.Printf("reflection calls: %d (%.2f%%)\n",
    diagnostics.VMHotspots.HelperReflectionCalls,
    diagnostics.VMHelperReflectionPercent())

  for _, call := range diagnostics.VMHotspots.HelperCallPaths {
    if call.Path != plush.RenderVMHelperCallReflection {
      continue
    }
    fmt.Printf("name=%s signature=%s calls=%d time=%s\n",
      call.Name, call.Signature, call.Calls, call.Duration)
  }
}
```

The render-level `FastPath` field and helper call paths describe different
layers. A render can report `fast` while one helper inside it reports
`reflection`. Likewise, `generic` still means VM bytecode execution and does not
by itself identify a helper reflection escape.

Prioritize reflected helpers by total time and call count. A nonzero reflection
count is diagnostic information, not a rendering failure. Disable hotspot
diagnostics after the investigation because timing every call adds work.

The aggregate counters remain exact. To bound diagnostic memory, one diagnostics state retains at most eight unique direct and eight unique reflection `name + signature` pairs. Calls for additional pairs still update the totals and increment the matching `DetailsDropped` field. The compact details header returns at most eight retained records and orders reflection entries first, then the most expensive entries. Inspect `diagnostics.VMHotspots.HelperCallPaths` directly when all retained direct and reflection details are needed.

This diagnostic does not record helper arguments, return values, context data, or rendered output. Signatures and helper names can still reveal application structure, so keep these logs and headers internal. Plush caches the calling strategy associated with a function type; it does not cache a request's helper closure or its result.

#### Detecting Partial Interpreter Fallbacks

Partial fallback diagnostics identify the individual partials that entered the
classic interpreter while their surrounding render was using the VM. They are
independent of VM hotspot timing and are disabled by default.

Enable them on the render context, then inspect `PartialFallbacks` after the
render:

```go
ctx := plush.NewContext()
plush.EnableRenderPartialFallbackDiagnostics(ctx)

_, err := plush.Render(input, ctx)
if err != nil {
  return err
}

diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
if ok {
  fmt.Printf("partial fallback calls: %d\n", diagnostics.PartialFallbacks.Calls)
  fmt.Printf("partial fallback details dropped: %d\n", diagnostics.PartialFallbacks.DetailsDropped)
  for _, detail := range diagnostics.PartialFallbacks.Details {
    fmt.Printf("name=%s reason=%s calls=%d\n", detail.Name, detail.Reason, detail.Calls)
  }
}
```

`RenderDiagnostics.PartialFallbacks` is a
`RenderPartialFallbackDiagnostics` value. Each retained
`RenderPartialFallbackDetail` has a typed `RenderPartialFallbackReason`:

| Reason | Constant | Meaning |
| --- | --- | --- |
| `generic-bytecode` | `RenderPartialFallbackGenericBytecode` | The partial could not use a supported fast plan and VM generic fallback was configured to use the interpreter. |
| `block-call-compatibility` | `RenderPartialFallbackBlockCallCompatibility` | A block helper in the partial does not have a VM-compatible direct path. Supported `form` blocks stay on the VM path. |
| `inherited-interpreter` | `RenderPartialFallbackInheritedInterpreter` | The partial was called from another partial that had already entered interpreter fallback. |

`PartialFallbacks.Calls` is exact for the render. To keep diagnostic memory
bounded when names are dynamic, Plush retains at most 16 unique
`name + reason` details. Each call whose pair is not retained increments
`DetailsDropped`; repeated calls for a retained pair continue updating its count.

Use `SetRenderPartialFallbackDiagnostics` when a boolean setting is more
convenient, or `DisableRenderPartialFallbackDiagnostics` to turn collection off
on a reused context. `RenderPartialFallbackDiagnosticsEnabled` reports the
effective setting. These diagnostics record partial names and reason codes, not
source text, arguments, context values, helper results, or rendered HTML.

Custom VM integrations can record equivalent work with
`AddRenderDiagnosticPartialFallback`. Use
`BeginRenderDiagnosticPartialFallback` around an interpreter-rendered parent so
nested calls are classified as `inherited-interpreter`;
`RenderDiagnosticPartialFallbackActive` reports whether that scope is active.

Supported `form` blocks, including nested loops and the injected form builder,
stay on the compiled path. Unsupported block helpers retain interpreter
compatibility fallback. Errors raised inside nested partials preserve the
innermost helper-block source line while retaining the full parent-to-child
partial filename chain.

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
  if outputSize := diagnostics.OutputSizeHeader(); outputSize != "" {
    w.Header().Set("X-Plush-Output-Size", outputSize)
  }
  if partialOutput := diagnostics.PartialOutputSizeHeader(); partialOutput != "" {
    w.Header().Set("X-Plush-Partial-Output-Size", partialOutput)
  }
  if partialDetails := diagnostics.PartialOutputSizeDetailsHeader(); partialDetails != "" {
    w.Header().Set("X-Plush-Partial-Output-Details", partialDetails)
  }
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
  w.Header().Set("X-Plush-VM-Partial-Fallback-Calls",
    fmt.Sprintf("%d", diagnostics.PartialFallbacks.Calls))
  w.Header().Set("X-Plush-VM-Partial-Fallback-Details-Dropped",
    fmt.Sprintf("%d", diagnostics.PartialFallbacks.DetailsDropped))
  w.Header().Set("X-Plush-VM-Helper-Hotspots",
    diagnostics.VMHelperHotspotsHeader())
  if helperPaths := diagnostics.VMHelperCallPathsHeader(); helperPaths != "" {
    w.Header().Set("X-Plush-VM-Helper-Call-Paths", helperPaths)
  }
  if helperPathDetails := diagnostics.VMHelperCallPathDetailsHeader(); helperPathDetails != "" {
    w.Header().Set("X-Plush-VM-Helper-Call-Details", helperPathDetails)
  }
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

These `X-Plush-*` and `Server-Timing` headers are not emitted by Plush automatically. Plush records the diagnostics; the HTTP application decides which values to expose. In production, put these headers behind a debug or internal-only flag because filenames, helper names, and partial names may reveal application structure.

Common header meanings:

| Header | Meaning |
| --- | --- |
| `X-Plush-Render-Mode` | `interpreter` or `vm`, showing which renderer handled this request. |
| `X-Plush-VM-Bytecode-Cache` | VM cache status. Warm file-backed templates should usually move from `miss-store` on first render to `hit`, `hit-static`, or `hit-source` on later renders. |
| `X-Plush-Template-Filename` | Filename used as the cache key when `meta.TemplateFileKey` was set on the context. |
| `X-Plush-Render-Engine-Time-Ms` | Time spent inside Plush rendering only, in milliseconds. Use this instead of total request time when comparing interpreter vs VM. |
| `X-Plush-Fast-Path` | VM execution path: `static`, `fast`, `generic`, or `interpreter-fallback`. The best steady-state VM path is usually `fast`; `generic` still runs VM bytecode; `interpreter-fallback` means VM mode intentionally delegated unsupported syntax to the classic interpreter. |
| `X-Plush-Punch-Hole-Cache` | Punch-hole cache status for templates that mix static HTML with embedded Plush code. |
| `X-Plush-Output-Size` | Adaptive output-size estimator stats for top-level VM renders. `scope=template` describes a direct file render. For a Buffalo layout, `scope=file` predicts the fully assembled response as the exact current `yield` byte length plus learned layout overhead selected from the root template's bounded `profile` band. `actual` is this render's byte length, `learned` is the prediction before this render, `error` is their percentage difference, and `within-10` is `1` only below 10% error. `min`, `max`, `unstable`, and `limited` describe lifetime variability and whether the allocation policy constrained the hint. `grow-allocated` is capacity added by the explicit speculative grow, while `cap-final` and `unused-cap` report the builder's final capacity and unused portion. |
| `X-Plush-Partial-Output-Size` | Request aggregate for compiled partial calls. `learned` and `actual` are summed across calls, while `absolute-error` prevents over- and under-estimates from cancelling each other. `within-10` counts calls whose learned estimate was below 10% error. `unstable` counts calls whose partial has at least a 4x observed output range; `limited` counts calls where Plush capped the speculative hint; `grow-allocated` reports the capacity actually added by explicit grows. |
| `X-Plush-Partial-Output-Details` | Bounded details for up to eight partial filenames, with per-file call totals, learned/actual error, updated estimate, lifetime range and sample count, instability/limit state, and explicit grow allocation. |
| `X-Plush-Fast-Plan-*` | Static counters captured from the compiled fast plan. These describe template complexity, not elapsed time. |
| `X-Plush-Fast-Plan-Helper-Names` | Helper names the fast plan found while compiling. Useful for spotting expensive helper-heavy templates. |
| `X-Plush-Fast-Plan-Partial-Names` | Partial names the fast plan found while compiling. Useful for spotting partial-heavy templates. |
| `X-Plush-VM-Helper-*` | Optional helper-call count and timing fields. They are zero unless `EnableRenderVMHotspotDiagnostics` was enabled for that context. |
| `X-Plush-VM-Helper-Call-Paths` | Exact direct, reflection, and unclassified helper-call totals; direct/reflection time; reflection percentage; and bounded-detail drop counts. |
| `X-Plush-VM-Helper-Call-Details` | Up to eight retained helper/signature records, with reflection records ordered first. Use this to identify signatures worth adding to the generated direct invokers or registering through `SetFastHelper` or the appropriate fast value-helper mode. |
| `X-Plush-VM-Partial-*` | Optional partial-call count and timing fields. They are zero unless `EnableRenderVMHotspotDiagnostics` was enabled for that context. |
| `X-Plush-VM-Partial-Fallback-*` | Application-mapped fallback totals and bounded-detail drop counts. They are zero unless `EnableRenderPartialFallbackDiagnostics` was enabled for that context. Inspect `diagnostics.PartialFallbacks.Details` for retained partial names, reasons, and counts. |
| `Server-Timing` | Browser/devtools-friendly timing summary. The example maps Plush engine time plus optional VM helper and partial hotspot time into server timing metrics. |

### Adaptive Output-Size Estimator

For a fundamentals-first explanation, implementation details, the formal state
model, and validation evidence, see
[Adaptive Output-Size Estimator](OUTPUT_SIZE_ESTIMATOR.md).

The estimator is implemented across the VM's whole-template, loop, composed-layout, and partial render paths, with bounded state, diagnostics, a runtime off switch, and race coverage.

#### How it works

The estimator is a capacity planner for `strings.Builder`; it is not an output cache. A hint only reserves likely capacity before Plush writes the response. An inaccurate hint can cause an extra allocation or leave unused capacity, but it cannot change the rendered output. The stages below are cumulative layers of the same estimator, not seven rendering passes applied to every request.

**Stage 1: Compile-time metadata**

When Plush compiles a template, it counts bytes that are already known from literal HTML, constant writes, or static fast-plan segments. This becomes `StaticSize`, the safe first-render baseline. The resulting bytecode also owns atomic statistics for the complete template, layout overhead, partial output, and each eligible fast loop. Statistics therefore follow the compiled file instead of a route, request, tenant value, or rendered string.

No sample exists on the first render. Plush starts with the best static or fast-plan hint available, renders normally, and measures the actual bytes afterward. Recompiling a changed source creates new bytecode and fresh statistics, so one template version does not train another version.

**Stage 2: Whole-template learning**

Before a top-level render starts, Plush grows an empty output builder using the larger of the known static size and the learned estimate. After a successful render it records the builder's actual length. Failed renders, nested renders that are not partials, and punch-hole sub-renders do not update these statistics.

The first valid sample becomes the estimate. Later samples use an asymmetric moving update:

- If actual output is larger, the estimate moves halfway toward it so underestimates recover quickly. A single upward sample can contribute at most four times the current estimate.
- If actual output is smaller, the estimate moves one eighth of the way down so one unusually small response does not immediately discard useful capacity.
- Estimate values have a 4 MiB ceiling. This limits speculative growth; it does not limit response size.

The estimate used for the current response is captured before rendering. The newly observed value becomes the estimate for later responses, which is why the first render of newly compiled bytecode learns and the next render benefits.

**Stage 3: Per-loop learning**

Every eligible fast loop owns a separate `bytes per item` statistic, including nested loops. After a successful loop, Plush divides the bytes produced by the number of rendered items and feeds that value through the same fast-up/slow-down update. Silent loops, empty loops, failed loops, and loops whose iterable length is unknown do not produce a growth hint.

On a later render, Plush multiplies learned bytes per item by the current iterable length. This uses today's item count rather than a historical page total, so a list that changes from 10 to 100 items can scale its hint immediately. Explicit loop growth is capped at 256 KiB; normal builder growth handles any remaining output. Request diagnostics aggregate loop predictions and retain bounded details for up to eight iterable-name/source-line pairs.

**Stage 4: Composed layouts and `yield`**

A composed page contains output that Plush already knows exactly: the current rendered `yield`. Learning the whole composed size as one average would over-allocate when small and large pages share a layout. Instead, Plush estimates only layout overhead:

```text
grow hint = exact current yield bytes + learned layout overhead
observed overhead = final layout bytes - current yield bytes
```

The root file's bytecode keeps eight fixed overhead profiles for yield ranges from `0-4k` through `4m+`. Buffalo render-pass identity and template filenames associate one layout render with its root render, preventing unrelated nested work from training that profile. Layout overhead growth is capped at 4 MiB, while the exact current yield is still included in full.

**Stage 5: Partials and variable-output safety**

Each compiled partial owns its own estimate. Plush measures the parent's builder length immediately before and after a successful inline partial, so the sample contains only that partial's bytes. Before the next call, Plush grows the parent only when its spare capacity is insufficient. It avoids explicit partial growth once the parent already has at least 64 KiB of capacity, allowing the parent and loop estimators to remain the primary planners.

For whole templates, layouts, and partials, Plush tracks the lifetime minimum and maximum observed output. When `maximum / minimum` reaches 4, the output is marked unstable. Conservative rules then replace a potentially expensive average:

- Whole-template and layout growth use at most the larger of static bytes and the observed minimum.
- Unstable partial growth is capped at the larger of 64 KiB and its static bytes.
- Loops continue using current item count, so changing cardinality does not require a route-sized historical estimate.

These limits affect only explicit preallocation. Rendering can always grow beyond them as needed.

**Stage 6: Control and diagnostics**

`SetOutputSizeEstimatorEnabled(false)` atomically disables learned template, layout, partial, and loop hints process-wide. Disabled mode records no new samples, but preserves the pre-existing static fast-plan hint. Re-enabling resumes from the statistics already attached to the cached bytecode.

When diagnostics are enabled for a top-level render, they report the estimate used before rendering (`learned`), actual bytes, the updated estimate, sample count, minimum/maximum, instability and limiting decisions, requested hint, real capacity allocated by `Grow`, and final unused capacity. `error` is the absolute difference between the pre-render estimate and actual output as a percentage of actual output; `within-10=1` means that value is strictly below 10%. Loop diagnostics provide request totals plus bounded per-loop bytes-per-item details, and partial diagnostics provide both totals and bounded per-file details.

Diagnostic collection is independent from estimator learning. Use `SetOutputSizeEstimatorDiagnosticsMode` to choose the required observability level:

| Mode | Learning and growth | Root diagnostics | Loop/partial aggregates | Per-loop/partial details |
| --- | --- | --- | --- | --- |
| `OutputSizeEstimatorDiagnosticsOff` | Enabled | No | No | No |
| `OutputSizeEstimatorDiagnosticsSummary` | Enabled | Yes | Yes | No |
| `OutputSizeEstimatorDiagnosticsDetailed` | Enabled | Yes | Yes | Yes |

**Estimator diagnostics and VM hotspot timing are both off by default.** This keeps estimator learning and builder growth enabled without paying the instrumentation cost. Applications must explicitly select summary or detailed estimator diagnostics, and explicitly enable VM hotspot timing, when they need that observability:

```go
previous := plush.SetOutputSizeEstimatorDiagnosticsMode(
    plush.OutputSizeEstimatorDiagnosticsSummary, // opt in temporarily
)
defer plush.SetOutputSizeEstimatorDiagnosticsMode(previous)
```

### Diagnostic Cost

Estimator learning and builder growth remain synchronous so the next render can
use the latest successful observation. Diagnostic collection is separate from
that learning path.

Estimator diagnostics and VM hotspot timing are instrumentation and are not
free. Both default to off. Enable summary mode only when aggregate observability
is required, and enable detailed diagnostics or hotspot timing only during a
bounded investigation. Formatting or exporting detailed records outside the
response path can reduce request latency, but that work still consumes process
CPU, memory, and I/O capacity.

Recommended production configuration:

```text
output-size estimator:             enabled
output-size estimator diagnostics: off (default)
VM hotspot timing:                 disabled
```

Choose summary or detailed diagnostics explicitly when investigating estimator behavior, then return the setting to off. Disabling diagnostics does not disable estimation, learning, or builder growth.

**Stage 7: Verification**

Generic tests exercise first-sample learning, asymmetric updates, cache ownership, changed source, loops, nested loops, layouts, partials, instability limits, disabled mode, and concurrent access. Tests also verify that capacity planning never changes rendered output and that failed renders do not train the estimator.

Each compiled template owns its own statistics. The cache stores estimates beside bytecode, never rendered values or context data. Replacing a cache entry after a source change creates fresh statistics automatically. Layout profiles are fixed at eight entries per root bytecode, so state cannot grow with request paths or input identities.

The estimator is enabled by default. Disable it process-wide at startup or around a controlled test:

```go
previous := plush.SetOutputSizeEstimatorEnabled(false)
defer plush.SetOutputSizeEstimatorEnabled(previous)
```

Disabled mode performs no adaptive learning, layout/partial estimation, or learned loop growth. It preserves the VM's static fast-plan hint, providing a direct pre-estimator baseline without changing rendering semantics.

To measure estimator cost without diagnostic collection, leave the estimator enabled and select `OutputSizeEstimatorDiagnosticsOff`. This differs from `SetOutputSizeEstimatorEnabled(false)`: diagnostics-off mode continues learning and applying capacity hints.

For fair VM measurements, warm file-backed templates until `VMBytecodeCache` reports `hit` and `FastPath` reports `fast`. A first render may report `miss-store` because the VM is parsing, compiling, storing bytecode, and then rendering. Fast-plan counters describe template shape, not elapsed time; use `EngineDurationMilliseconds`, `VMHelperDurationMilliseconds`, and `VMPartialDurationMilliseconds` for timings.

### Automatic VM Fast Paths

Most VM optimizations need no template changes. The VM automatically specializes common Plush shapes:

- static output and mixed static/name templates
- simple mixed templates that combine static text, names, property/access chains, and rendered infix booleans
- repeated name lookups
- safe mixed numeric comparisons such as `uint32 == 0` and `float32 == 3`
- struct fields, nested property chains, indexed property chains, and no-arg method tails
- typed Go map access with static string keys, such as `labels["status"]` and `records["primary"].Name`
- loops over `nil`, strings, slices, structs, and pointers to structs
- simple top-level conditionals whose branches contain static/name/property/access/infix output
- conditionals and infix conditions inside loops
- direct helper calls for common helper signatures
- silent script helper calls for side effects, such as `<% touch(name) %>`
- direct scalar helper calls for common string, int, uint32, bool, and float64 argument shapes
- dynamic callable values and chained receiver calls, such as `helpers[name]("x")` and `makeUser(name).Render("short")`
- top-level `let`, assignment, return, loop return, loop iterator assignment, and index assignment statements
- regex match expressions
- unary negation with `-`
- arrays, hashes, non-string literal hash keys, and dynamic index reads as supported fast values
- partials with no data, including direct linked rendering when the partial body is simple and does not need partial metadata
- partials with simple static-key data maps, such as `partial("row", {name: record.Name})`; the VM prepares the keys and value lookup plan, then reads fresh values each render
- partial data maps with helper-call values, such as `partial("row", {label: label(record.Name, prefix)})`; the VM compiles the call arguments and uses direct value invokers for common helper signatures
- block-aware form helpers inside partials, including nested loops and
  conditionals; unsupported block-helper shapes retain the compatibility path
- lexical locals and shadowed names propagated through nested partial chains
- dynamic partial names and `layout` data values through the regular VM partial helper-call path
- linked partial bodies that contain simple property/access/infix output, such as `<%= record.Name %>` or `<%= labels["status"] %>`
- clean filename cache keys and punch-hole filename checks for file-backed cached renders

The VM caches plans and bytecode, not request values. It does not cache the current record, helper return values, rendered HTML, partial output, or branch decisions.

Helpers whose Go function signature uses an application-defined scalar parameter, such as `func(CustomName, string) string`, still use the safe reflective call path unless an app registers a custom fast helper. Go does not safely convert `func(CustomName, string) string` into `func(string, string) string` even when `CustomName` is backed by `string`.

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

`vmplush.SetFastHelper` optimizes calls whose result is written directly to the render output:

```erb
<%= label(value) %>
```

When the same helper result is needed as a Go value, register one of the fast
value-helper modes too. Value-position calls include assignments, conditions,
arguments to other helpers, loops, and partial data-map values:

```erb
<% let text = label(value) %>
<%= wrap(text) %>
```

The read/write value-helper API uses `hctx.Context` from
`github.com/gobuffalo/plush/v5/helpers/hctx`.

```go
ctx.Set("label", func(value interface{}) string {
  text, ok := value.(string)
  if !ok {
    return ""
  }
  return "[" + text + "]"
})

vmplush.SetFastHelper(ctx, "label", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
  text, ok := args.String(0)
  if !ok {
    return vmplush.ErrFastUnsupported // fall back to the normal helper
  }

  w.WriteEscapedString("[" + text + "]")
  return nil
})

vmplush.SetFastNoContextValueHelper(ctx, "label", func(args vmplush.FastArgs) (interface{}, error) {
  text, ok := args.String(0)
  if !ok {
    return nil, vmplush.ErrFastUnsupported
  }

  return "[" + text + "]", nil
})
```

#### Choosing a value-helper context mode

Choose the narrowest mode that implements the helper correctly:

| Registration API | Context access | Binding write-back | Typical use |
|---|---|---|---|
| `SetFastNoContextValueHelper` | None; receives `FastArgs` only | No | Pure formatting, parsing, encoding, and other argument-only helpers |
| `SetFastReadOnlyValueHelper` | `Value` and `Has` through `FastReadOnlyContext` | No | Helpers that inspect root bindings or current frame locals without changing them |
| `SetFastValueHelper` | Full `hctx.Context` | Yes | Helpers that set/update bindings or require an API that accepts the full context |

The `label` example above uses no-context mode because its result depends only
on its arguments. This avoids constructing a scoped read/write context and
avoids post-call binding synchronization.

Use read-only mode when the result also depends on a binding:

```go
vmplush.SetFastReadOnlyValueHelper(ctx, "label", func(callCtx vmplush.FastReadOnlyContext, args vmplush.FastArgs) (interface{}, error) {
  text, ok := args.String(0)
  if !ok {
    return nil, vmplush.ErrFastUnsupported
  }

  prefix, ok := callCtx.Value("label_prefix").(string)
  if !ok {
    return nil, vmplush.ErrFastUnsupported
  }
  return prefix + text, nil
})
```

`FastReadOnlyContext` prevents binding mutation through the context interface,
but read-only access is shallow. A map, slice, pointer, array containing
references, or object returned by `Value` can still expose mutable application
data. A read-only helper must treat every reachable value as immutable, make a
defensive copy, or use `SetFastValueHelper` when mutation is intentional.

Registering any value-helper mode for a name replaces its previous fast
value-helper registration. All three modes preserve normal-helper fallback:
keep the normal helper in the context and return `vmplush.ErrFastUnsupported`
when the fast implementation cannot safely handle a call. Matching `Clear...`
functions are available for each registration API.

The template stays normal:

```erb
<%= label(value) %>
```

Fast helpers should only optimize the hot path. They must not cache request values or rendered output. Use `WriteEscapedString` for normal text and `WriteHTML` only for trusted `template.HTML`. Value helpers should return the same Go value as the normal helper. If either fast helper cannot safely handle the current arguments, return `vmplush.ErrFastUnsupported` so the VM can call the regular helper.

`FastArgs` and any context passed to a fast value helper are valid only for the
duration of that helper call because the VM reuses their backing storage. Do
not retain either after the helper returns. A helper that must keep an argument
should retain the value returned by `args.Raw(i)`, not `FastArgs` itself.

#### Runtime source partials

Regular partials remain the preferred option when a template can name source supplied by `partialFeeder`:

```erb
<%= partial("components/banner.plush", {title: title}) %>
```

A custom `FastHelperFunc` can instead use `WriteSourcePartial` when the application already holds the partial source as a string. This renders the source as a named partial through the active VM output without registering it with `partialFeeder`. Eligible compiled partials write directly into the parent builder; unsupported source shapes retain the normal compatibility fallback:

```go
source := `<strong><%= pageTitle %></strong>`

ctx.Set("runtimeBanner", func(help plush.HelperContext) (template.HTML, error) {
  rendered, err := plush.Render(source, help.Context)
  return template.HTML(rendered), err
})

vmplush.SetFastHelper(ctx, "runtimeBanner", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
  if args.Len() != 0 {
    return vmplush.ErrFastUnsupported
  }

  return w.WriteSourcePartial("runtime/banner.plush", source)
})
```

The two supported forms are:

```go
// Inherit values and helpers from the active render context.
err := w.WriteSourcePartial(name, source)

// Also add values scoped to this source-partial render.
err := w.WriteSourcePartial(name, source, map[string]interface{}{
  "record": record,
  "label":  label,
})
```

When the helper is nested inside another expression, such as
`<%= raw(runtimeBanner()) %>`, the VM must produce its value before the outer
helper runs and therefore cannot provide an active `FastWriter`. Register a
`FastValueHelperFunc` and use `RenderSourcePartial` for that compatibility
case:

```go
vmplush.SetFastValueHelper(ctx, "runtimeBanner", func(ctx hctx.Context, args vmplush.FastArgs) (interface{}, error) {
  if args.Len() != 0 {
    return nil, vmplush.ErrFastUnsupported
  }
  return vmplush.RenderSourcePartial(ctx, "runtime/banner.plush", source)
})
```

`RenderSourcePartial` returns trusted `template.HTML` and intentionally uses a
temporary builder because the caller requested a value. It still uses the same
named bytecode, warm plan, source-hash invalidation, estimator, render budget,
context inheritance, and partial diagnostics as `WriteSourcePartial`. Prefer
the direct `<%= runtimeBanner() %>` form with `WriteSourcePartial` when template
syntax permits it, because that writes into the parent builder without the
temporary value allocation.

The optional data map does not modify same-named values in the parent context. Values held only in VM lexical slots should be read from `FastArgs` and passed in this map; `WriteSourcePartial` automatically inherits values available through the active `hctx.Context`, including helpers, render-budget state, diagnostics, cancellation, and the regular `partialFeeder` used by any nested `partial()` calls.

The name is an identity, not a source lookup. Use a stable name that is unique to the logical template fragment. Plush compares the source hash under that name, so changing the source recompiles the bytecode while repeated unchanged source reuses it. Avoid request identifiers in names because they prevent useful estimator and bytecode reuse. Plush caches bytecode and numeric output-size statistics, never the rendered output or data-map values.

After an eligible source partial renders once, Plush retains a warm plan containing its resolved template identity, source hash, linked bytecode, and binding plan. Later calls with the same parent identity, name, and source hash can apply current context/data values directly to that plan without rebuilding partial metadata or repeating bytecode-cache discovery. Fast loops use the learned partial estimate to size their transactional scratch output before execution. Source that needs special partial metadata, nested partial calls, JavaScript escaping, punch holes, or compatibility fallback continues through the regular partial path.

Warm plans are bounded to 256 distinct runtime-source identities in each active partial-link cache. Changing source under an existing identity replaces that plan rather than consuming another slot. If the cache reaches its boundary, additional identities still render correctly through the normal path; they simply do not receive the warm-plan shortcut. The parent template identity is part of the key, so the same partial name used by separate template roots does not alias within a shared render context.

`WriteSourcePartial` charges the normal partial/sub-render budget and records the call in partial diagnostics. It executes the supplied source as a Plush template, so applications must apply the same trust and validation rules used for any other runtime template source. It is available only while a `FastHelperFunc` owns a valid `FastWriter`; normal template code should continue using `partial()`.

For regular Go structs, use `args.Raw(i)` and type assert to the concrete type. Register the normal helper first:

```go
type Record struct {
  Name string
  Meta Metadata
}

type Metadata struct {
  Label string
}

ctx.Set("recordLabel", func(value interface{}) string {
  record, ok := value.(Record)
  if !ok {
    return ""
  }
  return record.Meta.Label + ":" + record.Name
})
```

Then add an optional fast helper for the hot path:

```go
vmplush.SetFastHelper(ctx, "recordLabel", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
  raw, ok := args.Raw(0)
  if !ok {
    return vmplush.ErrFastUnsupported
  }

  record, ok := raw.(Record)
  if !ok {
    return vmplush.ErrFastUnsupported
  }

  w.WriteEscapedString(record.Meta.Label + ":" + record.Name)
  return nil
})

vmplush.SetFastValueHelper(ctx, "recordLabel", func(_ hctx.Context, args vmplush.FastArgs) (interface{}, error) {
  raw, ok := args.Raw(0)
  if !ok {
    return nil, vmplush.ErrFastUnsupported
  }

  record, ok := raw.(Record)
  if !ok {
    return nil, vmplush.ErrFastUnsupported
  }

  return record.Meta.Label + ":" + record.Name, nil
})
```

If the context stores pointers, assert the pointer type and nil-check it:

```go
record, ok := raw.(*Record)
if !ok || record == nil {
  return vmplush.ErrFastUnsupported
}

w.WriteEscapedString(record.Meta.Label + ":" + record.Name)
```

This avoids reflection and generic property access for the hot helper body while still keeping the normal helper as a safe fallback.

For helpers that return trusted HTML, use the writer helper to write trusted output and the value helper to return the same `template.HTML` value. A trusted-markup helper can be fast when written directly and when assigned before later use:

```erb
<%= trustedMarkup(value) %>

<% let markup = trustedMarkup(value) %>
<%= markup %>
```

The first call can use `SetFastHelper`; the second also needs the appropriate
fast value-helper mode because Plush must put the helper result on the VM stack
before writing it later.

Normal templates can still use regular Plush access without a fast helper:

```erb
<%= record.Meta.Label %>
```

Partial calls with simple data maps are also optimized automatically by the VM.

## Render Budget

Plush lets you attach a work-unit **budget** to any render to protect against runaway templates — deeply nested loops, recursive partials, or unexpectedly expensive helpers.

A **nil budget = unlimited**, so all existing code is completely unaffected.

### Quick start

```go
b := plush.NewBudget(10_000)
ctx := plush.NewContext()
ctx.Set("records", records)
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
