package encoders

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Raw(t *testing.T) {
	r := require.New(t)
	r.Equal(template.HTML("A"), Raw("A"))
}
