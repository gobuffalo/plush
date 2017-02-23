package plush

// Context holds all of the data for the template that is being rendered.
type Context struct {
	data  map[string]interface{}
	outer *Context
}

// New context containing the current context. Values set on the new context
// will not be set onto the original context, however, the original context's
// values will be available to the new context.
func (c *Context) New() *Context {
	cc := NewContext()
	cc.outer = c
	return cc
}

// Set a value onto the context
func (c *Context) Set(key string, value interface{}) {
	c.data[key] = value
}

// Get a value from the context, or it's parent's context if one exists.
func (c *Context) Get(key string) interface{} {
	if v, ok := c.data[key]; ok {
		return v
	}
	if c.outer != nil {
		return c.outer.Get(key)
	}
	return nil
}

// Has checks the existence of the key in the context.
func (c *Context) Has(key string) bool {
	return c.Get(key) != nil
}

// NewContext returns a fully formed context ready to go
func NewContext() *Context {
	return &Context{
		data:  map[string]interface{}{},
		outer: nil,
	}
}

// NewContextWith returns a fully formed context using the data
// provided.
func NewContextWith(data map[string]interface{}) *Context {
	c := NewContext()
	c.data = data
	return c
}
