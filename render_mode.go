package plush

import (
	"errors"
	"sync/atomic"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

type RenderMode int32

const (
	RenderModeInterpreter RenderMode = iota
	RenderModeVM
)

var renderMode atomic.Int32
var vmGenericFallback atomic.Bool
var vmRenderer atomic.Value

var ErrVMRendererNotRegistered = errors.New("plush VM renderer is not registered")

type RenderFunc func(string, hctx.Context) (string, error)

func SetRenderMode(mode RenderMode) RenderMode {
	if mode != RenderModeInterpreter && mode != RenderModeVM {
		mode = RenderModeInterpreter
	}
	previous := RenderMode(renderMode.Swap(int32(mode)))
	return previous
}

func GetRenderMode() RenderMode {
	return RenderMode(renderMode.Load())
}

func SetVMGenericFallback(enabled bool) bool {
	return vmGenericFallback.Swap(enabled)
}

func VMGenericFallbackEnabled() bool {
	return vmGenericFallback.Load()
}

func RegisterVMRenderer(renderer RenderFunc) {
	if renderer == nil {
		return
	}
	vmRenderer.Store(renderer)
}

func registeredVMRenderer() (RenderFunc, bool) {
	renderer := vmRenderer.Load()
	if renderer == nil {
		return nil, false
	}
	fn, ok := renderer.(RenderFunc)
	return fn, ok && fn != nil
}
