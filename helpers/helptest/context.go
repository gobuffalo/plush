package helptest

import (
	"context"
	"errors"
	"sync"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

var _ hctx.HelperContext = NewContext()

func NewContext() *HelperContext {
	return &HelperContext{
		Context: context.Background(),
	}
}

type HelperContext struct {
	context.Context
	data           sync.Map
	BlockFn        func() (string, error)
	BlockContextFn func(hctx.Context) (string, error)
	RenderFn       func(string) (string, error)
}

func (f HelperContext) New() hctx.Context {
	fhc := NewContext()
	fhc.Context = f.Context
	f.data.Range(func(k, v interface{}) bool {
		fhc.data.Store(k, v)
		return true
	})
	fhc.BlockFn = f.BlockFn
	fhc.BlockContextFn = f.BlockContextFn
	fhc.RenderFn = f.RenderFn
	return fhc
}

func (f HelperContext) Data() sync.Map {
	var m sync.Map
	f.data.Range(func(k, v interface{}) bool {
		m.Store(k, v)
		return true
	})

	return m
}

func (f HelperContext) Value(key interface{}) interface{} {
	v, ok := f.data.Load(key)
	if ok {
		return v
	}
	return f.Context.Value(key)
}
func (f *HelperContext) Update(key string, value interface{}) (returnData bool) {
	return
}
func (f *HelperContext) Set(key string, value interface{}) {
	f.data.Store(key, value)
}

func (f HelperContext) Block() (string, error) {
	if f.BlockFn == nil {
		return "", errors.New("no block given")
	}
	return f.BlockFn()
}

func (f HelperContext) BlockWith(c hctx.Context) (string, error) {
	if f.BlockContextFn == nil {
		return "", errors.New("no block given")
	}
	return f.BlockContextFn(c)
}

func (f HelperContext) HasBlock() bool {
	return f.BlockFn != nil || f.BlockContextFn != nil
}

func (f HelperContext) Render(s string) (string, error) {
	if f.RenderFn == nil {
		return "", errors.New("render is not available")
	}
	return f.RenderFn(s)
}

// Has checks the existence of the key in the context.
func (c HelperContext) Has(key string) bool {
	return c.Value(key) != nil
}
