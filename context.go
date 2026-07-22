package plush

import (
	"context"
	"sync"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

var _ context.Context = &Context{}

// Context holds all of the data for the template that is being rendered.
type Context struct {
	context.Context
	data    *SymbolTable
	helpers *SymbolTable
	outer   *Context
	moot    sync.RWMutex
	budget  *Budget
}

// WithBudget attaches a Budget to this context. Returns self for chaining.
func (c *Context) WithBudget(b *Budget) *Context {
	c.budget = b
	return c
}

// Budget returns the active budget, walking up the outer chain.
// Returns nil if no budget is set (unlimited).
func (c *Context) Budget() *Budget {
	if c.budget != nil {
		return c.budget
	}
	if c.outer != nil {
		return c.outer.Budget()
	}
	return nil
}

// New context containing the current context. Values set on the new context
// will not be set onto the original context, however, the original context's
// values will be available to the new context.
func (c *Context) New() hctx.Context {
	cc := NewContextWithOuter(nil, c)

	return cc
}

// Set a value onto the context
func (c *Context) Set(key string, value interface{}) {
	c.moot.Lock()
	defer c.moot.Unlock()
	c.data.Declare(key, value)
}

func (c *Context) InternID(key string) int {
	c.moot.Lock()
	defer c.moot.Unlock()
	return c.data.localInterner.Intern(key)
}

func (c *Context) InternIDs(keys []string, ids []int) {
	if len(keys) == 0 || len(ids) < len(keys) {
		return
	}
	c.moot.Lock()
	defer c.moot.Unlock()
	c.data.localInterner.mw.Lock()
	defer c.data.localInterner.mw.Unlock()
	for i, key := range keys {
		ids[i] = c.data.localInterner.internUnsafe(key)
	}
}

func (c *Context) SetID(id int, value interface{}) {
	c.moot.Lock()
	defer c.moot.Unlock()
	c.data.DeclareID(id, value)
}

func (c *Context) Update(key string, value interface{}) bool {
	c.moot.Lock()
	defer c.moot.Unlock()
	return c.data.Assign(key, value)
}

func (c *Context) UpdateID(id int, value interface{}) bool {
	c.moot.Lock()
	defer c.moot.Unlock()
	return c.data.AssignID(id, value)
}

// Value from the context, or it's parent's context if one exists.
func (c *Context) Value(key interface{}) interface{} {
	c.moot.RLock()
	defer c.moot.RUnlock()

	if s, ok := key.(string); ok {

		gg, ok := c.data.Resolve(s)
		if ok {
			return gg
		}
		if c.helpers != nil {
			if gg, ok := c.helpers.Resolve(s); ok {
				return gg
			}
		}
	}

	return c.Context.Value(key)
}

func (c *Context) Lookup(key string) (interface{}, bool) {
	c.moot.RLock()
	defer c.moot.RUnlock()
	if value, ok := c.data.Resolve(key); ok {
		return value, true
	}
	if c.helpers != nil {
		return c.helpers.Resolve(key)
	}
	return nil, false
}

func (c *Context) LookupID(id int) (interface{}, bool) {
	c.moot.RLock()
	defer c.moot.RUnlock()
	if value, ok := c.data.ResolveID(id); ok {
		return value, true
	}
	if c.helpers != nil {
		if name, ok := c.data.SymbolName(id); ok {
			return c.helpers.Resolve(name)
		}
	}
	return nil, false
}

// Has checks the existence of the key in the context.
func (c *Context) Has(key string) bool {
	c.moot.RLock()
	defer c.moot.RUnlock()

	if c.data.Has(key) {
		return true
	}
	if c.helpers != nil {
		return c.helpers.Has(key)
	}
	return false
}

// NewContext returns a fully formed context ready to go
func NewContext() *Context {
	return NewContextWith(nil)
}

// NewContextWith returns a fully formed context using the data
// provided.
func NewContextWith(data map[string]interface{}) *Context {
	c := &Context{
		Context: context.Background(),
		data:    newRootScopeFromMap(data),
		helpers: cachedHelperScope(),
		outer:   nil,
	}

	return c
}

// NewContextWith returns a fully formed context using the data
// provided and setting the outer context with the passed
// seccond argument.
func NewContextWithOuter(data map[string]interface{}, out *Context) *Context {
	c := &Context{
		Context: context.Background(),
		data:    newScopeWithCapacity(out.data, len(data)),
		helpers: out.helpers,
		outer:   out,
	}
	for k, v := range data {
		c.data.Declare(k, v)
	}
	return c
}

// NewContextWithContext returns a new plush.Context given another context
func NewContextWithContext(ctx context.Context) *Context {
	c := NewContext()
	c.Context = ctx

	return c
}
