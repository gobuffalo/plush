# VM Fast Paths

A map of the fast render paths used by the VM. Each path has one rule: match
normal template behavior exactly, or step aside and let the bytecode VM render.

## Principles

- Fast paths cache plans, bytecode, reflection metadata, and stable context
  binding IDs.
- Fast paths do not cache request values, helper return values, rendered HTML,
  partial output, loop items, or branch decisions.
- Unsupported shapes fall back to the normal VM path instead of changing
  template behavior. Some unsupported AST shapes are wrapped as a generic VM
  segment so VM mode can stay in bytecode execution while the specialized fast
  planner remains conservative.
- Custom fast helpers are optional accelerators. They should return
  `ErrFastUnsupported` when they cannot safely handle the current argument
  values.

## Compiler Plan

The compiler builds a `FastRenderPlan` from the AST when every supported
top-level statement can be represented as a fast segment. If any statement does
not fit the supported shapes, the template still compiles to bytecode and runs
through the normal VM interpreter.

Supported fast render segment kinds include:

- static output
- name output
- property output
- value/path output
- helper calls
- block helper calls
- conditionals
- loops
- partials
- top-level and loop `let` statements
- top-level and loop assignment statements
- top-level and loop index assignments
- top-level `return` statements
- loop `return` statements
- generic VM whole-template segments for bytecode-safe shapes that do not have a
  specialized fast segment yet

The compiler records a reject reason and line number when analysis declines a
fast render plan.

Plush comments (`<%# ... %>`) are treated as no-op statements by the fast
planner, matching normal render behavior.

## Direct Write Opcodes

Direct write opcodes fuse common "load or call, then write" instruction pairs.
They avoid extra stack traffic by writing directly to the current frame output.

Examples include:

- constants and strings
- local, global, and context names
- property reads from locals, globals, and context names
- helper calls and named helper calls

These opcodes still use the same escaping and helper-call rules as regular VM
execution.

## Prepared Render Plans

At runtime, a compiler `FastRenderPlan` is prepared into VM-side plans. Prepared
plans are cached on the compiler plan so repeated renders can reuse the same
analysis.

The runtime tries prepared plans in this order:

1. Static-name plan for templates made of static text and repeated name output.
2. Simple plan for compact mixed output that can be handled with specialized
   operations.
3. Mixed plan for the general supported fast-render shape.

For stable contexts, binding plans cache interned context IDs for the plan's
binding names. The cached binding plan only stores how to find values, not the
values themselves.

## Access Paths

Property and access fast paths specialize common reflection work.

Supported access shapes include:

- exported Go struct fields
- pointer-to-struct field chains
- no-argument method tails
- slice and array indexes
- Go map indexes with static integer or string keys
- dynamic index reads, such as `data[key]`
- string-keyed map property access, when a dotted property can be treated like a
  string map key

Inline caches store reflection lookup metadata for a small number of receiver
types at a call site. They do not store receiver values.

## Helper Calls

The VM has direct helper invokers for common signatures, including zero-arg and
scalar-arg helpers returning strings, trusted HTML, booleans, numbers, objects,
or `(value, error)`.

The reflective helper path handles general Go helpers. It also mirrors
interpreter behavior for supported omitted helper tail arguments:

- `plush.HelperContext` or `hctx.HelperContext`
- `map[string]interface{}` options maps
- a single ordinary trailing parameter before one of those helper arguments,
  filled with that parameter type's Go zero value

Custom fast helpers registered with `SetFastHelper` receive only the arguments
written in the template. They can use `FastWriter.Context()` when they need the
current render context. If a custom fast helper cannot handle the argument
shape, it should return `ErrFastUnsupported` so the normal helper remains the
source of truth.

Fast value helpers have three explicit context-access modes:

- `SetFastNoContextValueHelper` receives only `FastArgs`. The VM does not build
  a scoped helper context and does not synchronize bindings after the call.
- `SetFastReadOnlyValueHelper` receives `FastReadOnlyContext`. It can read root
  bindings and current frame locals, but the interface has no `Set`, `Update`,
  or `New` method. The VM does not synchronize bindings after the call.
- `SetFastValueHelper` is the existing read/write mode. It receives
  `hctx.Context`, and Plush synchronizes binding changes after the call.

Read-only context access is deliberately shallow. A map, slice, pointer
(including a pointer to an array), array containing reference values, or object
returned by `Value` may still refer to mutable application data. A read-only
helper must not mutate such a value. If mutation is necessary, the helper must
either make a defensive copy or use the read/write helper API. The read-only
interface protects context bindings; it is not a deep-freezing or copy-on-read
mechanism.

Registering one value-helper mode for a name replaces the previously registered
mode for that name. All modes preserve `ErrFastUnsupported` fallback to the
normal helper.

Receiver method calls can also be planned when their arguments are representable
as fast values. For example, `builder.RenderControl({name: "Email", type:
inputType})` is planned as a receiver path on `builder`, followed by a method
call step with a planned hash argument.

Method call inline caches store call metadata for the receiver type. Argument
values are evaluated fresh for every render.

Plain helper calls in script tags, such as `<% touch(name) %>`, are planned as
silent calls: the helper still runs and can mutate state, but its return value is
discarded. Dynamic callable values and chained receiver calls can be represented
when their receiver and arguments are fast values, including shapes such as
`helpers[name]("x")` and `makeUser(name).Render("short")`.

## Assignments

Fast plans follow the bytecode distinction between `let` and `=`:

- `let name = value` creates or updates the current local scope.
- `name = value` updates an existing binding; it does not create a new outer
  binding by accident.
- `container[index] = value` mutates the evaluated map, hash, slice, or array
  target.

Assignment values may be literals, names, access paths, helper calls, receiver
method calls, prefix/infix expressions, concatenations, arrays, hashes, and
dynamic index reads.

Index assignments evaluate the container, index, and value at render time. Go
map keys and slice/array indexes use the same conversion and bounds behavior as
the VM helpers.

## Loops

Loop fast paths support iteration over:

- template arrays and hashes
- strings
- Go arrays and slices
- Go maps
- structs and pointers to structs when a struct-loop writer plan can be built

Loop bodies may contain static output, key/value output, property and access
chains, helper calls, nested conditionals, nested loops, partials, `let`,
assignment, index assignment, `return`, `break`, and `continue` when the
compiler can represent those statements in the loop plan.

Output loops (`<%= for ... %>`) write their body output normally. Silent script
loops (`<% for ... %>`) execute their body for side effects but discard any body
output. That covers setup patterns such as:

```plush
<% let collected = [] %>
<% for (_, item) in items { %>
  <% collected = collected + item.Value %>
<% } %>
```

Loop assignments can update outer bindings, but loop-local variables created by
`let` stay scoped to the loop iteration. When a loop has local `let` statements,
the VM renders it with scoped bindings and syncs only non-local assignments back
to the outer binding set.

Loops over `nil` are planned and render as empty loops. Assignments to the
current loop iterator variables are also planned and remain scoped to the
current iteration, so `item = "x"` changes subsequent reads of `item` in that
iteration without leaking to the next iteration or the outer context.

Script `return` inside a loop writes its return value and ends the current
iteration, matching the normal renderer. Output produced earlier in the same
branch is committed when that branch exits through `return`, `break`, or
`continue`; otherwise script conditional body output remains discarded.

Inside an output loop, script conditionals such as `<% if item.Visible { %>`
can commit branch body output when the branch exits through `return`, `break`,
or `continue`. Inside a top-level script loop, the same output is discarded
because the enclosing loop is silent.

Struct-loop writer plans specialize repeated reflection for loop bodies that
render fields, access chains, method calls, and helper calls for each item.

## Block Helpers

Block helper calls are planned when the helper callee and arguments are
representable and the block body can be compiled. The block body is compiled to
bytecode and may itself use a fast render plan.

The helper block receives a scoped context built from the current fast bindings.
For block helpers inside loops, the current loop key and value are added to that
block scope.

Assignments inside a block helper body are allowed when they target names
declared inside that block body, including indexed writes to local maps or
hashes. Assignments to outer bindings are also allowed when the block runs
through the helper block context; the VM syncs the binding changes after the
helper renders the block.

Output block helpers (`<%= helper(...) { %>`) write the helper return value.
Silent block helpers (`<% helper(...) { %>`) execute the helper and render the
block when the helper asks for it, but discard the helper return value. This
matches helper patterns that capture block output for later use:

```plush
<% capture("extraStyle") { %>
  <style>.online { color: green; }</style>
<% } %>
<%= readCapture("extraStyle") %>
```

`form(...) { ... }` is handled through this block-helper path. It is not a
special VM opcode. The VM calls the `form` helper with its planned arguments and
a `plush.HelperContext`; when the helper calls `Block()` or `BlockWith(ctx)`,
the fast block runner renders the compiled block with that helper-provided
context. Bindings created by the helper for the block, such as `f`, remain
visible inside the block, so receiver method calls on helper-provided values can
be planned.

```plush
<%= form({action: submitPath(), method: "POST"}) { %>
  <% let options = {} %>
  <% for (_, option) in availableOptions { %>
    <% options[option.Label] = option.ID %>
  <% } %>
  <%= builder.RenderSelect({name: "Record.OptionID", options: options}) %>
<% } %>
```

## Partials

Partial fast paths optimize common partial calls while keeping partial data
scoped to the partial render.

Supported paths include:

- partials without data
- partials with static-key data maps
- data-map values from literals, names, property/access chains, and helper calls
- linked partial bytecode reuse when the partial body is compatible with inline
  rendering
- dynamic partial names and `layout` data values through the regular VM partial
  helper-call path

Partial data binding plans prepare the keys and value lookup strategy. They read
fresh values for every render.

## Conditions And Operators

Fast value plans support common truthiness, prefix, infix, and concatenation
expressions used by output, conditional, and loop plans.

Supported optimized cases include:

- string concatenation
- unary negation with `-`
- native slice append through `+` in assignment expressions
- logical `&&` and `||`
- numeric equality and ordering across common Go numeric types
- regex match expressions with cached compiled regex patterns
- arrays, hashes, non-string literal hash keys, and dynamic index reads when
  used as supported fast values

When a condition or operand cannot be represented safely, the compiler declines
the fast plan and normal VM bytecode handles the template.

## Generic VM Segments

When the compiler can produce bytecode but the specialized fast planner does not
yet model a template shape, VM mode can keep execution inside the VM by creating
a single generic VM segment. Template-defined functions and function literals
use this path today. Diagnostics report `FastPath` as `generic`, not
`interpreter-fallback`, and `FastReject` stays empty because the VM handled the
template.

Punch holes are intentionally excluded from generic VM segments. They still use
the punch-hole-specific path so skeleton caching and hole filling keep their
existing behavior.

## Diagnostics

Fast render diagnostics record why a fast plan was accepted or rejected and
summarize the shape of accepted plans. Optional hotspot diagnostics can count
and time VM helper and partial calls.

Diagnostics describe the compiled template shape and runtime helper/partial
activity. They do not imply that request values or rendered output were cached.
