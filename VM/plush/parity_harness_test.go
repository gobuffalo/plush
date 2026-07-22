package plush_test

import (
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/stretchr/testify/require"
)

type contextFactory func() hctx.Context

func emptyContext() hctx.Context {
	return rootplush.NewContext()
}

func contextWith(data map[string]interface{}) contextFactory {
	return func() hctx.Context {
		return rootplush.NewContextWith(data)
	}
}

func compareRender(t *testing.T, input string, factory contextFactory) {
	t.Helper()

	interpreterOut, interpreterErr := renderInterpreter(input, factory)
	vmOut, vmErr := renderVM(input, factory)

	require.Equalf(t, errorString(interpreterErr), errorString(vmErr), "error mismatch\ninterpreter: %q\nvm:          %q", errorString(interpreterErr), errorString(vmErr))
	require.Equalf(t, interpreterOut, vmOut, "output mismatch\ninterpreter: %q\nvm:          %q", interpreterOut, vmOut)
}

func compareRenderError(t *testing.T, input string, factory contextFactory) {
	t.Helper()

	interpreterOut, interpreterErr := renderInterpreter(input, factory)
	vmOut, vmErr := renderVM(input, factory)

	require.Error(t, interpreterErr, "expected interpreter error, got output %q", interpreterOut)
	require.Error(t, vmErr, "expected VM error, got output %q", vmOut)
	require.Equalf(t, interpreterOut, vmOut, "error output mismatch\ninterpreter: %q\nvm:          %q", interpreterOut, vmOut)
	require.Equalf(t, interpreterErr.Error(), vmErr.Error(), "error mismatch\ninterpreter: %q\nvm:          %q", interpreterErr.Error(), vmErr.Error())
}

func compareBothRenderError(t *testing.T, input string, factory contextFactory) {
	t.Helper()

	interpreterOut, interpreterErr := renderInterpreter(input, factory)
	vmOut, vmErr := renderVM(input, factory)

	require.Error(t, interpreterErr, "expected interpreter error, got output %q", interpreterOut)
	require.Error(t, vmErr, "expected VM error, got output %q", vmOut)
}

func renderInterpreter(input string, factory contextFactory) (string, error) {
	return rootplush.Render(input, factory())
}

func renderVM(input string, factory contextFactory) (string, error) {
	return vmplush.Render(input, factory())
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
