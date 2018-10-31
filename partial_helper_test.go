package plush

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func Test_PartialHelper_Nil_Context(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{}

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "invalid context")
	r.Equal("", string(html))
}

func Test_PartialHelper_Blank_Data(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Context.data = nil

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "invalid context")
	r.Equal("", string(html))
}

func Test_PartialHelper_Blank_Context(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "could not found")
	r.Equal("", string(html))
}

func Test_PartialHelper_Invalid_Feeder(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", "me-rong")

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "could not found")
	r.Equal("", string(html))
}

func Test_PartialHelper_Invalid_FeederFunction(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) string {
		return "me-rong"
	})

	html, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "could not found")
	r.Equal("", string(html))
}

func Test_PartialHelper_Feeder_Error(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return "", errors.New("me-rong")
	})

	_, err := partialHelper(name, data, help)
	r.Error(err)
	r.Contains(err.Error(), "me-rong")
}

func Test_PartialHelper_Good(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<div class="test">Plush!</div>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<div class="test">Plush!</div>`, string(html))
}

func Test_PartialHelper_With_Data(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{"name": "Yonghwan"}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<div class="test">Hello <%= name %></div>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<div class="test">Hello Yonghwan</div>`, string(html))
}

func Test_PartialHelper_Render_Error(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(string) (string, error) {
		return `<div class="test">Hello <%= name </div>`, nil
	})

	_, err := partialHelper(name, data, help)
	r.Error(err)
}

func Test_PartialHelper_With_Layout(t *testing.T) {
	r := require.New(t)

	name := "index"
	data := map[string]interface{}{
		"name":   "Yonghwan",
		"layout": "container",
	}
	help := HelperContext{Context: NewContext()}
	help.Set("partialFeeder", func(name string) (string, error) {
		if name == "container" {
			return `<html><%= yield %></html>`, nil
		}
		return `<div class="test">Hello <%= name %></div>`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`<html><div class="test">Hello Yonghwan</div></html>`, string(html))
}

func Test_PartialHelper_JavaScript(t *testing.T) {
	r := require.New(t)

	name := "index.js"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("contentType", "application/javascript")
	help.Set("partialFeeder", func(string) (string, error) {
		return `alert('\'Hello\'');`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`alert('\'Hello\'');`, string(html))
}

func Test_PartialHelper_Javascript_With_HTML(t *testing.T) {
	r := require.New(t)

	name := "index.html"
	data := map[string]interface{}{}
	help := HelperContext{Context: NewContext()}
	help.Set("contentType", "application/javascript")
	help.Set("partialFeeder", func(string) (string, error) {
		return `alert('\'Hello\'');`, nil
	})

	html, err := partialHelper(name, data, help)
	r.NoError(err)
	r.Equal(`alert(\'\\\'Hello\\\'\');`, string(html))
}