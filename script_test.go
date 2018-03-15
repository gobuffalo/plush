package plush

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_RunScript(t *testing.T) {
	r := require.New(t)
	bb := &bytes.Buffer{}
	ctx := NewContextWith(map[string]interface{}{
		"out": func(i interface{}) {
			bb.WriteString(fmt.Sprint(i))
		},
	})
	err := RunScript(script, ctx)
	r.NoError(err)
	r.Equal("3hiasdfasdf", bb.String())
}

const script = `let x = "foo"

let a = 1
let b = 2
let c = a + b

out(c)

if (c == 3) {
  out("hi")
}

let x = fn(f) {
  f()
}

x(fn() {
  out("asdfasdf")
})`
