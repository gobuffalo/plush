package plush

import (
	"fmt"
	"html/template"

	"github.com/gobuffalo/tags"
	"github.com/gobuffalo/tags/form"
	"github.com/gobuffalo/tags/form/bootstrap"
)

func FormHelper(opts tags.Options, help HelperContext) (template.HTML, error) {
	return Helper(opts, help, func(opts tags.Options) Helperable {
		return form.New(opts)
	})
}

func FormForHelper(model interface{}, opts tags.Options, help HelperContext) (template.HTML, error) {
	return Helper(opts, help, func(opts tags.Options) Helperable {
		return form.NewFormFor(model, opts)
	})
}

func BootstrapFormHelper(opts tags.Options, help HelperContext) (template.HTML, error) {
	return Helper(opts, help, func(opts tags.Options) Helperable {
		return bootstrap.New(opts)
	})
}

func BootstrapFormForHelper(model interface{}, opts tags.Options, help HelperContext) (template.HTML, error) {
	return Helper(opts, help, func(opts tags.Options) Helperable {
		return bootstrap.NewFormFor(model, opts)
	})
}

type Helperable interface {
	SetAuthenticityToken(string)
	Append(...tags.Body)
	HTMLer
}

func Helper(opts tags.Options, help HelperContext, fn func(opts tags.Options) Helperable) (template.HTML, error) {
	form := fn(opts)
	if help.Value("authenticity_token") != nil {
		form.SetAuthenticityToken(fmt.Sprint(help.Value("authenticity_token")))
	}
	ctx := help.Context.New()
	ctx.Set("f", form)
	s, err := help.BlockWith(ctx)
	if err != nil {
		return "", err
	}
	form.Append(s)
	return form.HTML(), nil
}
